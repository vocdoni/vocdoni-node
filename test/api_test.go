package test

import (
	"encoding/json"
	"fmt"
	"math/big"
	"testing"
	"time"

	qt "github.com/frankban/quicktest"
	"github.com/google/uuid"
	"go.vocdoni.io/dvote/api"
	"go.vocdoni.io/dvote/crypto/ethereum"
	"go.vocdoni.io/dvote/data/ipfs"
	"go.vocdoni.io/dvote/test/testcommon"
	"go.vocdoni.io/dvote/test/testcommon/testutil"
	"go.vocdoni.io/dvote/types"
	"go.vocdoni.io/dvote/util"
	"go.vocdoni.io/dvote/vochain"
	"go.vocdoni.io/dvote/vochain/state"
	"go.vocdoni.io/dvote/vochain/state/electionprice"
	"go.vocdoni.io/proto/build/go/models"
	"google.golang.org/protobuf/proto"
)

func TestAPIcensusAndVote(t *testing.T) {
	server := testcommon.APIserver{}
	server.Start(t,
		api.ChainHandler,
		api.CensusHandler,
		api.VoteHandler,
		api.AccountHandler,
		api.ElectionHandler,
		api.WalletHandler,
	)
	// Block 1
	server.VochainAPP.AdvanceTestBlock()

	token1 := uuid.New()
	c := testutil.NewTestHTTPclient(t, server.ListenAddr, &token1)

	// create a new census
	resp, code := c.Request("POST", nil, "censuses", "weighted")
	qt.Assert(t, code, qt.Equals, 200)
	censusData := &api.Census{}
	qt.Assert(t, json.Unmarshal(resp, censusData), qt.IsNil)
	id1 := censusData.CensusID.String()

	// add a bunch of keys and values (weights)
	rnd := testutil.NewRandom(1)
	cparts := api.CensusParticipants{}
	for i := 1; i < 10; i++ {
		cparts.Participants = append(cparts.Participants, api.CensusParticipant{
			Key:    rnd.RandomBytes(20),
			Weight: (*types.BigInt)(big.NewInt(int64(1))),
		})
	}
	_, code = c.Request("POST", &cparts, "censuses", id1, "participants")
	qt.Assert(t, code, qt.Equals, 200)

	// add the key we'll use for cast votes
	voterKey := ethereum.SignKeys{}
	qt.Assert(t, voterKey.Generate(), qt.IsNil)

	_, code = c.Request("POST", &api.CensusParticipants{Participants: []api.CensusParticipant{{
		Key:    voterKey.Address().Bytes(),
		Weight: (*types.BigInt)(big.NewInt(1)),
	}}}, "censuses", id1, "participants")
	qt.Assert(t, code, qt.Equals, 200)

	resp, code = c.Request("POST", nil, "censuses", id1, "publish")
	qt.Assert(t, code, qt.Equals, 200)
	qt.Assert(t, json.Unmarshal(resp, censusData), qt.IsNil)
	qt.Assert(t, censusData.CensusID, qt.IsNotNil)
	root := censusData.CensusID

	resp, code = c.Request("GET", nil, "censuses", root.String(), "proof", fmt.Sprintf("%x", voterKey.Address().Bytes()))
	qt.Assert(t, code, qt.Equals, 200)
	qt.Assert(t, json.Unmarshal(resp, censusData), qt.IsNil)
	qt.Assert(t, censusData.Weight.String(), qt.Equals, "1")

	metadataBytes, err := json.Marshal(
		&api.ElectionMetadata{
			Title:       map[string]string{"default": "test election"},
			Description: map[string]string{"default": "test election description"},
			Version:     "1.0",
		})

	qt.Assert(t, err, qt.IsNil)
	metadataURI := ipfs.CalculateCIDv1json(metadataBytes)

	tx := models.Tx{
		Payload: &models.Tx_NewProcess{
			NewProcess: &models.NewProcessTx{
				Txtype: models.TxType_NEW_PROCESS,
				Nonce:  0,
				Process: &models.Process{
					StartBlock:    0,
					BlockCount:    100,
					Status:        models.ProcessStatus_READY,
					CensusRoot:    root,
					CensusOrigin:  models.CensusOrigin_OFF_CHAIN_TREE_WEIGHTED,
					Mode:          &models.ProcessMode{AutoStart: true, Interruptible: true},
					VoteOptions:   &models.ProcessVoteOptions{MaxCount: 1, MaxValue: 1},
					EnvelopeType:  &models.EnvelopeType{},
					Metadata:      &metadataURI,
					MaxCensusSize: 1000,
				},
			},
		},
	}
	txb, err := proto.Marshal(&tx)
	qt.Assert(t, err, qt.IsNil)
	signedTxb, err := server.Account.SignVocdoniTx(txb, server.VochainAPP.ChainID())
	qt.Assert(t, err, qt.IsNil)
	stx := models.SignedTx{Tx: txb, Signature: signedTxb}
	stxb, err := proto.Marshal(&stx)
	qt.Assert(t, err, qt.IsNil)

	election := api.ElectionCreate{
		TxPayload: stxb,
		Metadata:  metadataBytes,
	}
	resp, code = c.Request("POST", election, "elections")
	qt.Assert(t, code, qt.Equals, 200)
	err = json.Unmarshal(resp, &election)
	qt.Assert(t, err, qt.IsNil)

	// Block 2
	server.VochainAPP.AdvanceTestBlock()
	waitUntilHeight(t, c, 2)

	// Send a vote
	votePackage := &state.VotePackage{
		Votes: []int{1},
	}
	votePackageBytes, err := votePackage.Encode()
	qt.Assert(t, err, qt.IsNil)

	vote := &models.VoteEnvelope{
		Nonce:       util.RandomBytes(16),
		ProcessId:   election.ElectionID,
		VotePackage: votePackageBytes,
	}
	vote.Proof = &models.Proof{
		Payload: &models.Proof_Arbo{
			Arbo: &models.ProofArbo{
				Type:            models.ProofArbo_BLAKE2B,
				Siblings:        censusData.CensusProof,
				AvailableWeight: censusData.Value,
				VoteWeight:      censusData.Value,
			},
		},
	}
	stx.Tx, err = proto.Marshal(&models.Tx{Payload: &models.Tx_Vote{Vote: vote}})
	qt.Assert(t, err, qt.IsNil)
	stx.Signature, err = voterKey.SignVocdoniTx(stx.Tx, server.VochainAPP.ChainID())
	qt.Assert(t, err, qt.IsNil)
	stxb, err = proto.Marshal(&stx)
	qt.Assert(t, err, qt.IsNil)

	v := &api.Vote{TxPayload: stxb}
	resp, code = c.Request("POST", v, "votes")
	qt.Assert(t, code, qt.Equals, 200)
	err = json.Unmarshal(resp, &v)
	qt.Assert(t, err, qt.IsNil)

	// Block 3
	server.VochainAPP.AdvanceTestBlock()
	waitUntilHeight(t, c, 3)

	// Verify the vote
	_, code = c.Request("GET", nil, "votes", "verify", election.ElectionID.String(), v.VoteID.String())
	qt.Assert(t, code, qt.Equals, 200)

	// Get the vote and check the data
	resp, code = c.Request("GET", nil, "votes", v.VoteID.String())
	qt.Assert(t, code, qt.Equals, 200)
	v2 := &api.Vote{}
	err = json.Unmarshal(resp, v2)
	qt.Assert(t, err, qt.IsNil)
	qt.Assert(t, v2.VoteID.String(), qt.Equals, v.VoteID.String())
	qt.Assert(t, v2.BlockHeight, qt.Equals, uint32(2))
	qt.Assert(t, *v2.TransactionIndex, qt.Equals, int32(0))

	// TODO (painan): check why the voterID is not present on the reply
	//qt.Assert(t, v2.VoterID.String(), qt.Equals, voterKey.AddressString())
}

func TestAPIaccount(t *testing.T) {
	server := testcommon.APIserver{}
	server.Start(t,
		api.ChainHandler,
		api.CensusHandler,
		api.VoteHandler,
		api.AccountHandler,
		api.ElectionHandler,
		api.WalletHandler,
	)
	token1 := uuid.New()
	c := testutil.NewTestHTTPclient(t, server.ListenAddr, &token1)

	// Block 1
	server.VochainAPP.AdvanceTestBlock()
	waitUntilHeight(t, c, 1)

	// create a new account
	signer := ethereum.SignKeys{}
	qt.Assert(t, signer.Generate(), qt.IsNil)

	// metdata
	meta := &api.AccountMetadata{
		Version: "1.0",
	}
	metaData, err := json.Marshal(meta)
	qt.Assert(t, err, qt.IsNil)

	// transaction
	fp, err := vochain.GenerateFaucetPackage(server.Account, signer.Address(), 50)
	qt.Assert(t, err, qt.IsNil)
	stx := models.SignedTx{}
	infoURI := server.Storage.URIprefix() + ipfs.CalculateCIDv1json(metaData)
	sik, err := signer.AccountSIK(nil)
	qt.Assert(t, err, qt.IsNil)
	stx.Tx, err = proto.Marshal(&models.Tx{Payload: &models.Tx_SetAccount{
		SetAccount: &models.SetAccountTx{
			Txtype:        models.TxType_CREATE_ACCOUNT,
			Nonce:         new(uint32),
			InfoURI:       &infoURI,
			Account:       signer.Address().Bytes(),
			FaucetPackage: fp,
			SIK:           sik,
		},
	}})
	qt.Assert(t, err, qt.IsNil)
	stx.Signature, err = signer.SignVocdoniTx(stx.Tx, server.VochainAPP.ChainID())
	qt.Assert(t, err, qt.IsNil)
	stxb, err := proto.Marshal(&stx)
	qt.Assert(t, err, qt.IsNil)

	// send the transaction and metadata
	accSet := api.AccountSet{
		Metadata:  metaData,
		TxPayload: stxb,
	}
	resp, code := c.Request("POST", &accSet, "accounts")
	qt.Assert(t, code, qt.Equals, 200, qt.Commentf("response: %s", resp))

	// Block 2
	server.VochainAPP.AdvanceTestBlock()
	waitUntilHeight(t, c, 2)

	// check the account exist
	resp, code = c.Request("GET", nil, "accounts", signer.Address().String())
	qt.Assert(t, code, qt.Equals, 200, qt.Commentf("response: %s", resp))
}

func TestAPIElectionCost(t *testing.T) {
	// cheap election
	runAPIElectionCostWithParams(t,
		electionprice.ElectionParameters{
			MaxCensusSize:    100,
			ElectionDuration: 2000,
			EncryptedVotes:   false,
			AnonymousVotes:   false,
			MaxVoteOverwrite: 1,
		},
		10000, 5000,
		5, 1000,
		6)

	// bigger census size, duration, reduced network capacity, etc
	runAPIElectionCostWithParams(t,
		electionprice.ElectionParameters{
			MaxCensusSize:    5000,
			ElectionDuration: 10000,
			EncryptedVotes:   false,
			AnonymousVotes:   false,
			MaxVoteOverwrite: 3,
		},
		200000, 6000,
		10, 100,
		762)

	// very expensive election
	runAPIElectionCostWithParams(t,
		electionprice.ElectionParameters{
			MaxCensusSize:    100000,
			ElectionDuration: 1000000,
			EncryptedVotes:   true,
			AnonymousVotes:   true,
			MaxVoteOverwrite: 10,
		},
		100000, 700000,
		10, 100,
		547026)
}

func runAPIElectionCostWithParams(t *testing.T,
	electionParams electionprice.ElectionParameters,
	startBlock uint32, initialBalance,
	txCostNewProcess, networkCapacity,
	expectedPrice uint64,
) {
	server := testcommon.APIserver{}
	server.Start(t,
		api.ChainHandler,
		api.CensusHandler,
		api.VoteHandler,
		api.AccountHandler,
		api.ElectionHandler,
		api.WalletHandler,
	)

	token1 := uuid.New()
	c := testutil.NewTestHTTPclient(t, server.ListenAddr, &token1)

	err := server.VochainAPP.State.SetTxBaseCost(models.TxType_NEW_PROCESS, txCostNewProcess)
	qt.Assert(t, err, qt.IsNil)
	err = server.VochainAPP.State.SetElectionPriceCalc()
	qt.Assert(t, err, qt.IsNil)
	server.VochainAPP.State.ElectionPriceCalc.SetCapacity(networkCapacity)

	// Block 1
	server.VochainAPP.AdvanceTestBlock()

	signer := createAccount(t, c, server, initialBalance)

	// Block 2
	server.VochainAPP.AdvanceTestBlock()
	waitUntilHeight(t, c, 2)

	censusRoot := createCensus(t, c)

	// first check predictedPrice equals the hardcoded expected

	predictedPrice := predictPriceForElection(t, c, electionParams)
	qt.Assert(t, predictedPrice, qt.Equals, expectedPrice)

	// now check balance before creating election and then after creating,
	// and confirm the balance decreased exactly the expected amount

	qt.Assert(t, requestAccount(t, c, signer.Address().String()).Balance,
		qt.Equals, initialBalance)

	createElection(t, c, signer, electionParams, censusRoot, startBlock, server.VochainAPP.ChainID())

	// Block 3
	server.VochainAPP.AdvanceTestBlock()
	waitUntilHeight(t, c, 3)

	balance := requestAccount(t, c, signer.Address().String()).Balance
	qt.Assert(t, balance,
		qt.Equals, initialBalance-predictedPrice,
		qt.Commentf("endpoint /elections/price predicted cost %d, "+
			"but actual election creation costed %d", predictedPrice, initialBalance-balance))
}

func waitUntilHeight(t testing.TB, c *testutil.TestHTTPclient, h uint32) {
	for {
		resp, code := c.Request("GET", nil, "chain", "info")
		qt.Assert(t, code, qt.Equals, 200)
		chainInfo := api.ChainInfo{}
		err := json.Unmarshal(resp, &chainInfo)
		qt.Assert(t, err, qt.IsNil)
		// check transaction count
		resp, code = c.Request("GET", nil, "chain", "transactions", "count")
		qt.Assert(t, code, qt.Equals, 200)
		txsCount := new(struct {
			Count uint64 `json:"count"`
		})
		err = json.Unmarshal(resp, txsCount)
		qt.Assert(t, err, qt.IsNil)
		qt.Assert(t, txsCount.Count, qt.Equals, chainInfo.TransactionCount)
		if chainInfo.Height >= h {
			break
		}
		time.Sleep(time.Second * 1)
	}
}

func requestAccount(t testing.TB, c *testutil.TestHTTPclient, address string) api.Account {
	resp, code := c.Request("GET", nil, "accounts", address)
	qt.Assert(t, code, qt.Equals, 200, qt.Commentf("response: %s", resp))
	acct := api.Account{}
	err := json.Unmarshal(resp, &acct)
	qt.Assert(t, err, qt.IsNil)
	return acct
}

func createCensus(t testing.TB, c *testutil.TestHTTPclient) (root types.HexBytes) {
	resp, code := c.Request("POST", nil, "censuses", "weighted")
	qt.Assert(t, code, qt.Equals, 200)
	censusData := &api.Census{}
	qt.Assert(t, json.Unmarshal(resp, censusData), qt.IsNil)

	id1 := censusData.CensusID.String()
	resp, code = c.Request("POST", nil, "censuses", id1, "publish")
	qt.Assert(t, code, qt.Equals, 200)
	qt.Assert(t, json.Unmarshal(resp, censusData), qt.IsNil)
	qt.Assert(t, censusData.CensusID, qt.IsNotNil)
	return censusData.CensusID
}

func createElection(t testing.TB, c *testutil.TestHTTPclient,
	signer *ethereum.SignKeys,
	electionParams electionprice.ElectionParameters,
	censusRoot types.HexBytes,
	startBlock uint32,
	chainID string,
) api.ElectionCreate {
	metadataBytes, err := json.Marshal(
		&api.ElectionMetadata{
			Title:       map[string]string{"default": "test election"},
			Description: map[string]string{"default": "test election description"},
			Version:     "1.0",
		})

	qt.Assert(t, err, qt.IsNil)
	metadataURI := ipfs.CalculateCIDv1json(metadataBytes)

	tx := models.Tx_NewProcess{
		NewProcess: &models.NewProcessTx{
			Txtype: models.TxType_NEW_PROCESS,
			Nonce:  0,
			Process: &models.Process{
				StartBlock:   startBlock,
				BlockCount:   electionParams.ElectionDuration,
				Status:       models.ProcessStatus_READY,
				CensusRoot:   censusRoot,
				CensusOrigin: models.CensusOrigin_OFF_CHAIN_TREE_WEIGHTED,
				Mode:         &models.ProcessMode{AutoStart: true, Interruptible: true},
				VoteOptions: &models.ProcessVoteOptions{
					MaxCount:          1,
					MaxValue:          1,
					MaxVoteOverwrites: electionParams.MaxVoteOverwrite,
				},
				EnvelopeType: &models.EnvelopeType{
					EncryptedVotes: electionParams.EncryptedVotes,
					Anonymous:      electionParams.AnonymousVotes,
				},
				Metadata:      &metadataURI,
				MaxCensusSize: electionParams.MaxCensusSize,
			},
		},
	}

	txb, err := proto.Marshal(&models.Tx{Payload: &tx})
	qt.Assert(t, err, qt.IsNil)
	signedTxb, err := signer.SignVocdoniTx(txb, chainID)
	qt.Assert(t, err, qt.IsNil)
	stx := models.SignedTx{Tx: txb, Signature: signedTxb}
	stxb, err := proto.Marshal(&stx)
	qt.Assert(t, err, qt.IsNil)

	election := api.ElectionCreate{
		TxPayload: stxb,
		Metadata:  metadataBytes,
	}
	resp, code := c.Request("POST", election, "elections")
	qt.Assert(t, code, qt.Equals, 200)
	err = json.Unmarshal(resp, &election)
	qt.Assert(t, err, qt.IsNil)

	return election
}

func predictPriceForElection(t testing.TB, c *testutil.TestHTTPclient,
	electionParams electionprice.ElectionParameters) uint64 {
	predicted := struct {
		Price uint64 `json:"price"`
	}{}

	resp, code := c.Request("POST", electionParams, "elections", "price")
	qt.Assert(t, code, qt.Equals, 200)
	err := json.Unmarshal(resp, &predicted)
	qt.Assert(t, err, qt.IsNil)

	return predicted.Price
}

func createAccount(t testing.TB, c *testutil.TestHTTPclient,
	server testcommon.APIserver, initialBalance uint64) *ethereum.SignKeys {
	signer := ethereum.SignKeys{}
	qt.Assert(t, signer.Generate(), qt.IsNil)

	// metadata
	meta := &api.AccountMetadata{
		Version: "1.0",
	}
	metaData, err := json.Marshal(meta)
	qt.Assert(t, err, qt.IsNil)

	fp, err := vochain.GenerateFaucetPackage(server.Account, signer.Address(), initialBalance)
	qt.Assert(t, err, qt.IsNil)

	// transaction
	stx := models.SignedTx{}
	infoURI := "ipfs://" + ipfs.CalculateCIDv1json(metaData)
	sik, err := signer.AccountSIK(nil)
	qt.Assert(t, err, qt.IsNil)
	stx.Tx, err = proto.Marshal(&models.Tx{Payload: &models.Tx_SetAccount{
		SetAccount: &models.SetAccountTx{
			Txtype:        models.TxType_CREATE_ACCOUNT,
			Nonce:         new(uint32),
			InfoURI:       &infoURI,
			Account:       signer.Address().Bytes(),
			FaucetPackage: fp,
			SIK:           sik,
		},
	}})
	qt.Assert(t, err, qt.IsNil)
	stx.Signature, err = signer.SignVocdoniTx(stx.Tx, server.VochainAPP.ChainID())
	qt.Assert(t, err, qt.IsNil)
	stxb, err := proto.Marshal(&stx)
	qt.Assert(t, err, qt.IsNil)

	// send the transaction and metadata
	accSet := api.AccountSet{
		Metadata:  metaData,
		TxPayload: stxb,
	}
	resp, code := c.Request("POST", &accSet, "accounts")
	qt.Assert(t, code, qt.Equals, 200, qt.Commentf("response: %s", resp))

	return &signer
}

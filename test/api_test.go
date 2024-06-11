package test

import (
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"math/big"
	"math/rand"
	"testing"
	"time"

	qt "github.com/frankban/quicktest"
	"github.com/google/uuid"
	"go.vocdoni.io/dvote/api"
	"go.vocdoni.io/dvote/crypto/ethereum"
	"go.vocdoni.io/dvote/crypto/nacl"
	"go.vocdoni.io/dvote/data/ipfs"
	"go.vocdoni.io/dvote/test/testcommon"
	"go.vocdoni.io/dvote/test/testcommon/testutil"
	"go.vocdoni.io/dvote/types"
	"go.vocdoni.io/dvote/util"
	"go.vocdoni.io/dvote/vochain"
	"go.vocdoni.io/dvote/vochain/indexer/indexertypes"
	"go.vocdoni.io/dvote/vochain/processid"
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

	electionParams := electionprice.ElectionParameters{ElectionDuration: 100, MaxCensusSize: 100}
	election := createElection(t, c, server.Account, electionParams, censusData.CensusRoot, 0, server.VochainAPP.ChainID(), false)

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

	stx := models.SignedTx{}
	stx.Tx, err = proto.Marshal(&models.Tx{Payload: &models.Tx_Vote{Vote: vote}})
	qt.Assert(t, err, qt.IsNil)
	stx.Signature, err = voterKey.SignVocdoniTx(stx.Tx, server.VochainAPP.ChainID())
	qt.Assert(t, err, qt.IsNil)
	stxb, err := proto.Marshal(&stx)
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
	qt.Assert(t, v2.VoterID.String(), qt.Equals, hex.EncodeToString(voterKey.Address().Bytes()))
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
	initBalance := uint64(80)
	signer := createAccount(t, c, server, initBalance)

	// Block 2
	server.VochainAPP.AdvanceTestBlock()
	waitUntilHeight(t, c, 2)

	// check the account exist
	resp, code := c.Request("GET", nil, "accounts", signer.Address().String())
	qt.Assert(t, code, qt.Equals, 200, qt.Commentf("response: %s", resp))

	// get accounts count
	resp, code = c.Request("GET", nil, "accounts", "count")
	qt.Assert(t, code, qt.Equals, 200, qt.Commentf("response: %s", resp))

	countAccts := struct {
		Count uint64 `json:"count"`
	}{}

	err := json.Unmarshal(resp, &countAccts)
	qt.Assert(t, err, qt.IsNil)

	// 2 accounts must exist: the previously new created account and the auxiliary
	// account used to transfer to the new account
	qt.Assert(t, countAccts.Count, qt.Equals, uint64(2))

	// get the accounts info
	resp, code = c.Request("GET", nil, "accounts", "page", "0")
	qt.Assert(t, code, qt.Equals, 200, qt.Commentf("response: %s", resp))

	accts := struct {
		Accounts []indexertypes.Account `json:"accounts"`
	}{}

	err = json.Unmarshal(resp, &accts)
	qt.Assert(t, err, qt.IsNil)

	// the second account must be the account created in the test
	// due 'ORDER BY balance DESC' in the sql query
	gotAcct := accts.Accounts[1]

	// compare the address in list of accounts with the account address previously created
	qt.Assert(t, gotAcct.Address.String(), qt.Equals, hex.EncodeToString(signer.Address().Bytes()))

	// compare the balance expected for the new account in the account list
	qt.Assert(t, gotAcct.Balance, qt.Equals, initBalance)

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

func TestAPIAccountTokentxs(t *testing.T) {
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
	initBalance := uint64(80)
	signer := createAccount(t, c, server, initBalance)

	// Block 2
	server.VochainAPP.AdvanceTestBlock()
	waitUntilHeight(t, c, 2)

	// create another new account
	signer2 := createAccount(t, c, server, initBalance)

	// Block 3
	server.VochainAPP.AdvanceTestBlock()
	waitUntilHeight(t, c, 3)

	// check the account1 exists
	resp, code := c.Request("GET", nil, "accounts", signer.Address().String())
	qt.Assert(t, code, qt.Equals, 200, qt.Commentf("response: %s", resp))

	// check the account2 exists
	resp, code = c.Request("GET", nil, "accounts", signer2.Address().String())
	qt.Assert(t, code, qt.Equals, 200, qt.Commentf("response: %s", resp))

	// transaction send token from account 1 to account 2
	amountAcc1toAcct2 := uint64(25)
	sendTokensTx(t, c, signer, signer2, server.VochainAPP.ChainID(), 0, amountAcc1toAcct2)

	// Block 4
	server.VochainAPP.AdvanceTestBlock()
	waitUntilHeight(t, c, 4)

	// transaction send token from account 2 to account 1
	amountAcc2toAcct1 := uint64(10)
	sendTokensTx(t, c, signer2, signer, server.VochainAPP.ChainID(), 0, amountAcc2toAcct1)

	// Block 5
	server.VochainAPP.AdvanceTestBlock()
	waitUntilHeight(t, c, 5)

	// get the token transfers received and sent for account 1
	resp, code = c.Request("GET", nil, "accounts", signer.Address().Hex(), "transfers", "page", "0")
	qt.Assert(t, code, qt.Equals, 200, qt.Commentf("response: %s", resp))

	tokenTxs := new(struct {
		Transfers indexertypes.TokenTransfersAccount `json:"transfers"`
	})
	err := json.Unmarshal(resp, tokenTxs)
	qt.Assert(t, err, qt.IsNil)

	// get the total token transfers count for account 1
	resp, code = c.Request("GET", nil, "accounts", signer.Address().Hex(), "transfers", "count")
	qt.Assert(t, code, qt.Equals, 200, qt.Commentf("response: %s", resp))

	countTnsAcc := struct {
		Count uint64 `json:"count"`
	}{}
	err = json.Unmarshal(resp, &countTnsAcc)
	qt.Assert(t, err, qt.IsNil)

	totalTokenTxs := uint64(len(tokenTxs.Transfers.Received) + len(tokenTxs.Transfers.Sent))

	// compare count of total token transfers for the account 1 using the two response
	qt.Assert(t, totalTokenTxs, qt.Equals, countTnsAcc.Count)

	// get the token transfers received and sent for account 2
	resp, code = c.Request("GET", nil, "accounts", signer2.Address().Hex(), "transfers", "page", "0")
	qt.Assert(t, code, qt.Equals, 200, qt.Commentf("response: %s", resp))

	tokenTxs2 := new(struct {
		Transfers indexertypes.TokenTransfersAccount `json:"transfers"`
	})
	err = json.Unmarshal(resp, tokenTxs2)
	qt.Assert(t, err, qt.IsNil)

	// get the total token transfers count for account 2
	resp, code = c.Request("GET", nil, "accounts", signer2.Address().Hex(), "transfers", "count")
	qt.Assert(t, code, qt.Equals, 200, qt.Commentf("response: %s", resp))

	countTnsAcc2 := struct {
		Count uint64 `json:"count"`
	}{}
	err = json.Unmarshal(resp, &countTnsAcc2)
	qt.Assert(t, err, qt.IsNil)

	totalTokenTxs2 := uint64(len(tokenTxs2.Transfers.Received) + len(tokenTxs2.Transfers.Sent))
	// compare count of total token transfers for the account 2 using the two response
	qt.Assert(t, totalTokenTxs2, qt.Equals, countTnsAcc2.Count)

	resp, code = c.Request("GET", nil, "accounts", "page", "0")
	qt.Assert(t, code, qt.Equals, 200, qt.Commentf("response: %s", resp))

	accts := struct {
		Accounts []indexertypes.Account `json:"accounts"`
	}{}

	err = json.Unmarshal(resp, &accts)
	qt.Assert(t, err, qt.IsNil)

	// the second account must be the account 2 created in the test
	// due 'ORDER BY balance DESC' in the sql query. For this account
	// was transfer an initial balance: 80, received from the account: 25 and
	// sent to the account 1: 10 tokens
	gotAcct2 := accts.Accounts[1]

	// compare the address in list of accounts with the account2 address previously created
	qt.Assert(t, gotAcct2.Address.String(), qt.Equals, hex.EncodeToString(signer2.Address().Bytes()))

	// compare the balance expected for the account 2 in the account list
	qt.Assert(t, gotAcct2.Balance, qt.Equals, initBalance+amountAcc1toAcct2-amountAcc2toAcct1)

	gotAcct1 := accts.Accounts[2]

	// compare the address in list of accounts with the account1 address previously created
	qt.Assert(t, gotAcct1.Address.String(), qt.Equals, hex.EncodeToString(signer.Address().Bytes()))

	// compare the balance expected for the account 1 in the account list
	qt.Assert(t, gotAcct1.Balance, qt.Equals, initBalance+amountAcc2toAcct1-amountAcc1toAcct2)

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

	createElection(t, c, signer, electionParams, censusRoot, startBlock, server.VochainAPP.ChainID(), false)

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
	encryptedMetadata bool,
) api.ElectionCreate {
	metadataBytes, err := json.Marshal(
		&api.ElectionMetadata{
			Title:       map[string]string{"default": "test election"},
			Description: map[string]string{"default": "test election description"},
			Version:     "1.0",
		})
	var metadataEncryptionKey []byte
	if encryptedMetadata {
		sk, err := nacl.Generate(rand.New(rand.NewSource(1)))
		qt.Assert(t, err, qt.IsNil)
		metadataBytes, err = nacl.Anonymous.Encrypt(metadataBytes, sk.Public())
		qt.Assert(t, err, qt.IsNil)
		metadataEncryptionKey = sk.Bytes()
	}

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
				Mode:         &models.ProcessMode{AutoStart: true, Interruptible: true, EncryptedMetaData: encryptedMetadata},
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
	election.MetadataEncryptionPrivKey = metadataEncryptionKey
	return election
}

func predictPriceForElection(t testing.TB, c *testutil.TestHTTPclient, electionParams electionprice.ElectionParameters) uint64 {
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

func sendTokensTx(t testing.TB, c *testutil.TestHTTPclient,
	signerFrom, signerTo *ethereum.SignKeys, chainID string, nonce uint32, amount uint64) {
	var err error
	stx := models.SignedTx{}
	stx.Tx, err = proto.Marshal(&models.Tx{
		Payload: &models.Tx_SendTokens{
			SendTokens: &models.SendTokensTx{
				Txtype: models.TxType_SET_ACCOUNT_INFO_URI,
				Nonce:  nonce,
				From:   signerFrom.Address().Bytes(),
				To:     signerTo.Address().Bytes(),
				Value:  amount,
			},
		}})
	qt.Assert(t, err, qt.IsNil)

	stx.Signature, err = signerFrom.SignVocdoniTx(stx.Tx, chainID)
	qt.Assert(t, err, qt.IsNil)

	txData, err := proto.Marshal(&stx)
	qt.Assert(t, err, qt.IsNil)

	tx := &api.Transaction{Payload: txData}

	resp, code := c.Request("POST", tx, "chain", "transactions")
	qt.Assert(t, code, qt.Equals, 200, qt.Commentf("response: %s", resp))
}

func TestAPIBuildElectionID(t *testing.T) {
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
	signer := createAccount(t, c, server, 80)

	// Block 2
	server.VochainAPP.AdvanceTestBlock()
	waitUntilHeight(t, c, 2)

	// create a new process
	censusRoot := createCensus(t, c)

	// Block 3
	server.VochainAPP.AdvanceTestBlock()
	waitUntilHeight(t, c, 3)

	// create request for calling api elections/id
	body := api.BuildElectionID{
		Delta:          processid.BuildNextProcessID,
		OrganizationID: signer.Address().Bytes(),
		CensusOrigin:   int32(models.CensusOrigin_OFF_CHAIN_TREE_WEIGHTED.Number()),
		EnvelopeType: struct {
			Serial         bool `json:"serial"`
			Anonymous      bool `json:"anonymous"`
			EncryptedVotes bool `json:"encryptedVotes"`
			UniqueValues   bool `json:"uniqueValues"`
			CostFromWeight bool `json:"costFromWeight"`
		}{
			Serial:         false,
			Anonymous:      false,
			EncryptedVotes: false,
			UniqueValues:   false,
			CostFromWeight: false,
		},
	}
	resp, code := c.Request("POST", body, "elections", "id")
	qt.Assert(t, code, qt.Equals, 200, qt.Commentf("response: %s", resp))

	nextElectionID := struct {
		ElectionID string `json:"electionID"`
	}{}
	err := json.Unmarshal(resp, &nextElectionID)
	qt.Assert(t, err, qt.IsNil)

	// test building n+1 election ID
	body.Delta = processid.BuildNextProcessID + 1
	resp, code = c.Request("POST", body, "elections", "id")
	qt.Assert(t, code, qt.Equals, 200, qt.Commentf("response: %s", resp))

	futureElectionID := struct {
		ElectionID string `json:"electionID"`
	}{}
	err = json.Unmarshal(resp, &futureElectionID)
	qt.Assert(t, err, qt.IsNil)

	// create a new election
	electionParams := electionprice.ElectionParameters{ElectionDuration: 100, MaxCensusSize: 100}
	response := createElection(t, c, signer, electionParams, censusRoot, 0, server.VochainAPP.ChainID(), false)

	// Block 4
	server.VochainAPP.AdvanceTestBlock()
	waitUntilHeight(t, c, 4)

	// check next election id is the same as the election id created
	qt.Assert(t, nextElectionID.ElectionID, qt.Equals, response.ElectionID.String())

	// now build last processID, after election was created.
	body.Delta = processid.BuildLastProcessID
	resp, code = c.Request("POST", body, "elections", "id")
	qt.Assert(t, code, qt.Equals, 200, qt.Commentf("response: %s", resp))

	lastElectionID := struct {
		ElectionID string `json:"electionID"`
	}{}
	err = json.Unmarshal(resp, &lastElectionID)
	qt.Assert(t, err, qt.IsNil)

	qt.Assert(t, lastElectionID, qt.Equals, nextElectionID)

	// and finally query again next processID, should match futureProcessID
	body.Delta = processid.BuildNextProcessID
	resp, code = c.Request("POST", body, "elections", "id")
	qt.Assert(t, code, qt.Equals, 200, qt.Commentf("response: %s", resp))

	newNextElectionID := struct {
		ElectionID string `json:"electionID"`
	}{}
	err = json.Unmarshal(resp, &newNextElectionID)
	qt.Assert(t, err, qt.IsNil)

	qt.Assert(t, newNextElectionID, qt.Equals, futureElectionID)

}

func TestAPIEncryptedMetadata(t *testing.T) {
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
	signer := createAccount(t, c, server, 80)

	// Block 2
	server.VochainAPP.AdvanceTestBlock()
	waitUntilHeight(t, c, 2)

	// create a new process
	censusRoot := createCensus(t, c)

	// Block 3
	server.VochainAPP.AdvanceTestBlock()
	waitUntilHeight(t, c, 3)

	// create a new election
	electionParams := electionprice.ElectionParameters{ElectionDuration: 100, MaxCensusSize: 100}
	electionResponse := createElection(t, c, signer, electionParams, censusRoot, 0, server.VochainAPP.ChainID(), true)

	// Block 4
	server.VochainAPP.AdvanceTestBlock()
	waitUntilHeight(t, c, 4)
	resp, code := c.Request("GET", nil, "elections", electionResponse.ElectionID.String())
	qt.Assert(t, code, qt.Equals, 200)
	var election api.Election
	err := json.Unmarshal(resp, &election)
	qt.Assert(t, err, qt.IsNil)
	qt.Assert(t, election.Metadata, qt.Not(qt.IsNil))

	// try to decrypt the metadata
	sk, err := nacl.DecodePrivate(electionResponse.MetadataEncryptionPrivKey.String())
	qt.Assert(t, err, qt.IsNil)
	metadataBytes, err := base64.StdEncoding.DecodeString(election.Metadata.(string))
	qt.Assert(t, err, qt.IsNil)
	decryptedMetadata, err := sk.Decrypt(metadataBytes)
	qt.Assert(t, err, qt.IsNil)

	// check the metadata decrypted is the same as the metadata sent
	var metadata api.ElectionMetadata
	err = json.Unmarshal(decryptedMetadata, &metadata)
	qt.Assert(t, err, qt.IsNil)
	qt.Assert(t, metadata.Title["default"], qt.Equals, "test election")
}

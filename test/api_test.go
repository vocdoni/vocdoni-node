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
	"go.vocdoni.io/dvote/apiclient"
	"go.vocdoni.io/dvote/crypto/ethereum"
	"go.vocdoni.io/dvote/data/ipfs"
	"go.vocdoni.io/dvote/test/testcommon"
	"go.vocdoni.io/dvote/test/testcommon/testutil"
	"go.vocdoni.io/dvote/types"
	"go.vocdoni.io/dvote/util"
	"go.vocdoni.io/dvote/vochain"
	"go.vocdoni.io/dvote/vochain/state"
	"go.vocdoni.io/proto/build/go/models"
	"google.golang.org/protobuf/proto"
)

type testElection struct {
	server     testcommon.APIserver
	c          *testutil.TestHTTPclient
	election   api.ElectionCreate
	censusData *api.Census

	voters []voter
}

type voter struct {
	key   *ethereum.SignKeys
	proof *apiclient.CensusProof
	vote  *api.Vote
}

func TestAPIcensusAndVote(t *testing.T) {
	te := NewTestElection(t, 10)
	te.CreateCensusAndElection(t)
	// Block 2
	te.server.VochainAPP.AdvanceTestBlock()

	te.VoteAll(t)

	// Block 3
	te.server.VochainAPP.AdvanceTestBlock()
	//waitUntilHeight(t, te.c, 3)

	te.VerifyVotes(t)
}

func NewTestElection(t testing.TB, nvotes int) *testElection {
	te := &testElection{}

	// Server
	te.server = testcommon.APIserver{}
	te.server.Start(t,
		api.ChainHandler,
		api.CensusHandler,
		api.VoteHandler,
		api.AccountHandler,
		api.ElectionHandler,
		api.WalletHandler,
	)
	// Block 1
	te.server.VochainAPP.AdvanceTestBlock()

	// Client
	token1 := uuid.New()
	te.c = testutil.NewTestHTTPclient(t, te.server.ListenAddr, &token1)

	// Voters
	for i := 0; i < nvotes; i++ {
		k := ethereum.NewSignKeys()
		qt.Assert(t, k.Generate(), qt.IsNil)
		te.voters = append(te.voters, voter{key: k})
	}

	return te
}

func (te *testElection) CreateCensusAndElection(t testing.TB) {
	// create a new census
	resp, code := te.c.Request("POST", nil, "censuses", "weighted")
	qt.Assert(t, code, qt.Equals, 200)
	te.censusData = &api.Census{}
	qt.Assert(t, json.Unmarshal(resp, te.censusData), qt.IsNil)
	id1 := te.censusData.CensusID.String()

	// add a bunch of keys and values (weights)
	cparts := api.CensusParticipants{}
	for _, voter := range te.voters {
		cparts.Participants = append(cparts.Participants, api.CensusParticipant{
			Key:    voter.key.Address().Bytes(),
			Weight: (*types.BigInt)(big.NewInt(1)),
		})
	}
	_, code = te.c.Request("POST", &cparts, "censuses", id1, "participants")
	qt.Assert(t, code, qt.Equals, 200)

	resp, code = te.c.Request("POST", nil, "censuses", id1, "publish")
	qt.Assert(t, code, qt.Equals, 200)
	qt.Assert(t, json.Unmarshal(resp, te.censusData), qt.IsNil)
	qt.Assert(t, te.censusData.CensusID, qt.IsNotNil)
	root := te.censusData.CensusID

	for i, voter := range te.voters {
		censusData := &api.Census{}
		resp, code = te.c.Request("GET", nil, "censuses", root.String(),
			"proof", fmt.Sprintf("%x", voter.key.Address().Bytes()))
		qt.Assert(t, code, qt.Equals, 200)
		qt.Assert(t, json.Unmarshal(resp, censusData), qt.IsNil)
		qt.Assert(t, censusData.Weight.String(), qt.Equals, "1")
		te.voters[i].proof = &apiclient.CensusProof{
			Proof:     censusData.Proof,
			LeafValue: censusData.Value,
		}
	}

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
					MaxCensusSize: uint64(len(te.voters)),
				},
			},
		},
	}
	txb, err := proto.Marshal(&tx)
	qt.Assert(t, err, qt.IsNil)
	signedTxb, err := te.server.Account.SignVocdoniTx(txb, te.server.VochainAPP.ChainID())
	qt.Assert(t, err, qt.IsNil)
	stx := models.SignedTx{Tx: txb, Signature: signedTxb}
	stxb, err := proto.Marshal(&stx)
	qt.Assert(t, err, qt.IsNil)

	te.election = api.ElectionCreate{
		TxPayload: stxb,
		Metadata:  metadataBytes,
	}
	resp, code = te.c.Request("POST", te.election, "elections")
	qt.Assert(t, code, qt.Equals, 200)
	err = json.Unmarshal(resp, &te.election)
	qt.Assert(t, err, qt.IsNil)
}

func (te *testElection) VoteAll(t testing.TB) {
	for i, voter := range te.voters {
		te.voters[i].vote = te.Vote(t, voter)
	}
}

// Vote sends a vote
func (te *testElection) Vote(t testing.TB, voter voter) *api.Vote {
	// Send a vote
	votePackage := &state.VotePackage{
		Votes: []int{1},
	}
	votePackageBytes, err := votePackage.Encode()
	qt.Assert(t, err, qt.IsNil)

	vote := &models.VoteEnvelope{
		Nonce:       util.RandomBytes(16),
		ProcessId:   te.election.ElectionID,
		VotePackage: votePackageBytes,
	}
	vote.Proof = &models.Proof{
		Payload: &models.Proof_Arbo{
			Arbo: &models.ProofArbo{
				Type:       models.ProofArbo_BLAKE2B,
				Siblings:   voter.proof.Proof,
				LeafWeight: voter.proof.LeafValue,
			},
		},
	}
	stx := models.SignedTx{}
	stx.Tx, err = proto.Marshal(&models.Tx{Payload: &models.Tx_Vote{Vote: vote}})
	qt.Assert(t, err, qt.IsNil)
	stx.Signature, err = voter.key.SignVocdoniTx(stx.Tx, te.server.VochainAPP.ChainID())
	qt.Assert(t, err, qt.IsNil)
	stxb, err := proto.Marshal(&stx)
	qt.Assert(t, err, qt.IsNil)

	v := &api.Vote{TxPayload: stxb}
	resp, code := te.c.Request("POST", v, "votes")
	if code == 500 {
		t.Logf("%s", resp)
	}
	qt.Assert(t, code, qt.Equals, 200)
	err = json.Unmarshal(resp, &v)
	qt.Assert(t, err, qt.IsNil)
	return v
}

func (te *testElection) VerifyVotes(t testing.TB) {
	for _, voter := range te.voters {
		// Verify the vote
		_, code := te.c.Request("GET", nil, "votes", "verify",
			te.election.ElectionID.String(), voter.vote.VoteID.String())
		qt.Assert(t, code, qt.Equals, 200)

		// Get the vote and check the data
		resp, code := te.c.Request("GET", nil, "votes", voter.vote.VoteID.String())
		qt.Assert(t, code, qt.Equals, 200)
		v2 := &api.Vote{}
		err := json.Unmarshal(resp, v2)
		qt.Assert(t, err, qt.IsNil)
		qt.Assert(t, v2.VoteID.String(), qt.Equals, voter.vote.VoteID.String())
		qt.Assert(t, v2.BlockHeight, qt.Equals, uint32(2))
		qt.Assert(t, v2.VoterID.String(), qt.Equals, fmt.Sprintf("%x", voter.key.Address().Bytes()))
	}
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
	stx.Tx, err = proto.Marshal(&models.Tx{Payload: &models.Tx_SetAccount{
		SetAccount: &models.SetAccountTx{
			Txtype:        models.TxType_CREATE_ACCOUNT,
			Nonce:         new(uint32),
			InfoURI:       &infoURI,
			Account:       signer.Address().Bytes(),
			FaucetPackage: fp,
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

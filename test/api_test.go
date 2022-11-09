package test

import (
	"encoding/json"
	"fmt"
	"testing"
	"time"

	qt "github.com/frankban/quicktest"
	"github.com/google/uuid"
	"go.vocdoni.io/dvote/api"
	"go.vocdoni.io/dvote/crypto/ethereum"
	"go.vocdoni.io/dvote/data"
	"go.vocdoni.io/dvote/test/testcommon"
	"go.vocdoni.io/dvote/test/testcommon/testutil"
	"go.vocdoni.io/dvote/util"
	"go.vocdoni.io/dvote/vochain"
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
	resp, code := c.Request("GET", nil, "census", "create", "weighted")
	qt.Assert(t, code, qt.Equals, 200)
	censusData := &api.Census{}
	qt.Assert(t, json.Unmarshal(resp, censusData), qt.IsNil)
	id1 := censusData.CensusID.String()

	// add a bunch of keys and values (weights)
	rnd := testutil.NewRandom(1)
	for i := 0; i < 10; i++ {
		key := fmt.Sprintf("%x", rnd.RandomBytes(32))
		t.Log(key)
		_, code = c.Request("GET", nil, "census", id1, "add", key, "1")
		qt.Assert(t, code, qt.Equals, 200)
	}

	// add the key we'll use for cast votes
	voterKey := ethereum.SignKeys{}
	qt.Assert(t, voterKey.Generate(), qt.IsNil)

	_, code = c.Request("GET", nil, "census", id1, "add", fmt.Sprintf("%x", voterKey.PublicKey()), "1")
	qt.Assert(t, code, qt.Equals, 200)

	resp, code = c.Request("GET", nil, "census", id1, "publish")
	qt.Assert(t, code, qt.Equals, 200)
	qt.Assert(t, json.Unmarshal(resp, censusData), qt.IsNil)
	qt.Assert(t, censusData.CensusID, qt.IsNotNil)
	root := censusData.CensusID

	resp, code = c.Request("GET", nil, "census", root.String(), "proof", fmt.Sprintf("%x", voterKey.PublicKey()))
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
	metadataURI := data.IPFScontentIdentifier(metadataBytes)

	tx := models.Tx{
		Payload: &models.Tx_NewProcess{
			NewProcess: &models.NewProcessTx{
				Txtype: models.TxType_NEW_PROCESS,
				Nonce:  0,
				Process: &models.Process{
					StartBlock:   0,
					BlockCount:   100,
					Status:       models.ProcessStatus_READY,
					CensusRoot:   root,
					CensusOrigin: models.CensusOrigin_OFF_CHAIN_TREE_WEIGHTED,
					Mode:         &models.ProcessMode{AutoStart: true, Interruptible: true},
					VoteOptions:  &models.ProcessVoteOptions{MaxCount: 1, MaxValue: 1},
					EnvelopeType: &models.EnvelopeType{},
					Metadata:     &metadataURI,
				},
			},
		},
	}
	txb, err := proto.Marshal(&tx)
	qt.Assert(t, err, qt.IsNil)
	signedTxb, err := server.Signer.SignVocdoniTx(txb, server.VochainAPP.ChainID())
	qt.Assert(t, err, qt.IsNil)
	stx := models.SignedTx{Tx: txb, Signature: signedTxb}
	stxb, err := proto.Marshal(&stx)
	qt.Assert(t, err, qt.IsNil)

	election := api.ElectionCreate{
		TxPayload: stxb,
		Metadata:  metadataBytes,
	}
	resp, code = c.Request("POST", election, "election", "create")
	qt.Assert(t, code, qt.Equals, 200)
	err = json.Unmarshal(resp, &election)
	qt.Assert(t, err, qt.IsNil)

	// Block 2
	server.VochainAPP.AdvanceTestBlock()
	waitUntilHeight(t, c, 2)

	// Send a vote
	votePackage := &vochain.VotePackage{
		Votes: []int{1},
	}
	votePackageBytes, err := json.Marshal(votePackage)
	qt.Assert(t, err, qt.IsNil)

	vote := &models.VoteEnvelope{
		Nonce:       util.RandomBytes(16),
		ProcessId:   election.ElectionID,
		VotePackage: votePackageBytes,
	}
	vote.Proof = &models.Proof{
		Payload: &models.Proof_Arbo{
			Arbo: &models.ProofArbo{
				Type:     models.ProofArbo_BLAKE2B,
				Siblings: censusData.Proof,
				Value:    censusData.Value,
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
	resp, code = c.Request("POST", v, "vote", "submit")
	qt.Assert(t, code, qt.Equals, 200)
	err = json.Unmarshal(resp, &v)
	qt.Assert(t, err, qt.IsNil)

	// Block 3
	server.VochainAPP.AdvanceTestBlock()
	waitUntilHeight(t, c, 3)

	_, code = c.Request("GET", nil, "vote", v.VoteID.String(), election.ElectionID.String(), "verify")
	qt.Assert(t, code, qt.Equals, 200)
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
	meta := &api.OrganizationMetadata{
		Version: "1.0",
	}
	metaData, err := json.Marshal(meta)
	qt.Assert(t, err, qt.IsNil)

	// transaction
	stx := models.SignedTx{}
	stx.Tx, err = proto.Marshal(&models.Tx{Payload: &models.Tx_SetAccountInfo{
		SetAccountInfo: &models.SetAccountInfoTx{
			Txtype:  models.TxType_SET_ACCOUNT_INFO,
			Nonce:   0,
			InfoURI: "ipfs://1234",
			Account: signer.Address().Bytes(),
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
	resp, code := c.Request("POST", &accSet, "account")
	qt.Assert(t, code, qt.Equals, 200, qt.Commentf("response: %s", resp))

	// Block 2
	server.VochainAPP.AdvanceTestBlock()
	waitUntilHeight(t, c, 2)

	// TODO: This is not working, should be checked!
	// check the account exist
	//resp, code = c.Request("GET", nil, "account", signer.Address().String())
	//qt.Assert(t, code, qt.Equals, 200, qt.Commentf("response: %s", resp))
}

func waitUntilHeight(t testing.TB, c *testutil.TestHTTPclient, h uint32) {
	for {
		resp, code := c.Request("GET", nil, "chain", "info")
		qt.Assert(t, code, qt.Equals, 200)
		chainInfo := api.ChainInfo{}
		err := json.Unmarshal(resp, &chainInfo)
		qt.Assert(t, err, qt.IsNil)
		if *chainInfo.Height >= h {
			break
		}
		time.Sleep(time.Second * 1)
	}
}

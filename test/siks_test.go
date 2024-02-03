package test

import (
	"encoding/json"
	"math/big"
	"testing"

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
	"go.vocdoni.io/dvote/vochain/ist"
	"go.vocdoni.io/proto/build/go/models"
	"google.golang.org/protobuf/proto"
)

func TestTempSIKs(t *testing.T) {
	c := qt.New(t)

	server := testcommon.APIserver{}
	server.Start(t,
		api.ChainHandler,
		api.CensusHandler,
		api.AccountHandler,
		api.ElectionHandler,
		api.WalletHandler,
		api.SIKHandler,
	)
	token1 := uuid.New()
	client := testutil.NewTestHTTPclient(t, server.ListenAddr, &token1)

	// Block 1
	server.VochainAPP.AdvanceTestBlock()
	waitUntilHeight(t, client, 1)

	// create a new account
	admin := ethereum.SignKeys{}
	c.Assert(admin.Generate(), qt.IsNil)

	// metdata
	meta := &api.AccountMetadata{
		Version: "1.0",
	}
	metaData, err := json.Marshal(meta)
	c.Assert(err, qt.IsNil)

	// transaction
	faucetPkg, err := vochain.GenerateFaucetPackage(server.Account, admin.Address(), 10000)
	c.Assert(err, qt.IsNil)
	stx := models.SignedTx{}
	infoURI := server.Storage.URIprefix() + ipfs.CalculateCIDv1json(metaData)
	sik, err := admin.AccountSIK(nil)
	c.Assert(err, qt.IsNil)
	stx.Tx, err = proto.Marshal(&models.Tx{Payload: &models.Tx_SetAccount{
		SetAccount: &models.SetAccountTx{
			Txtype:        models.TxType_CREATE_ACCOUNT,
			Nonce:         new(uint32),
			InfoURI:       &infoURI,
			Account:       admin.Address().Bytes(),
			FaucetPackage: faucetPkg,
			SIK:           sik,
		},
	}})
	c.Assert(err, qt.IsNil)
	stx.Signature, err = admin.SignVocdoniTx(stx.Tx, server.VochainAPP.ChainID())
	c.Assert(err, qt.IsNil)
	stxb, err := proto.Marshal(&stx)
	c.Assert(err, qt.IsNil)
	// send the transaction and metadata
	accSet := api.AccountSet{
		Metadata:  metaData,
		TxPayload: stxb,
	}
	resp, code := client.Request("POST", &accSet, "accounts")
	c.Assert(code, qt.Equals, 200, qt.Commentf("response: %s", resp))

	// Block 2
	server.VochainAPP.AdvanceTestBlock()
	waitUntilHeight(t, client, 2)

	// create a new census
	resp, code = client.Request("POST", nil, "censuses", api.CensusTypeZKWeighted)
	c.Assert(code, qt.Equals, 200)
	censusData := &api.Census{}
	c.Assert(json.Unmarshal(resp, censusData), qt.IsNil)
	censusId := censusData.CensusID.String()
	// add a voter key
	voter := ethereum.NewSignKeys()
	c.Assert(voter.Generate(), qt.IsNil)
	cparts := api.CensusParticipants{Participants: []api.CensusParticipant{
		{Key: voter.Address().Bytes(), Weight: (*types.BigInt)(big.NewInt(int64(1)))},
	}}
	_, code = client.Request("POST", &cparts, "censuses", censusId, "participants")
	c.Assert(code, qt.Equals, 200)
	// publish and get the root
	resp, code = client.Request("POST", nil, "censuses", censusId, "publish")
	c.Assert(code, qt.Equals, 200)
	c.Assert(json.Unmarshal(resp, censusData), qt.IsNil)
	c.Assert(censusData.CensusID, qt.IsNotNil)
	censusRoot := censusData.CensusID
	// create an anonymous election
	metadataBytes, err := json.Marshal(
		&api.ElectionMetadata{
			Title:       map[string]string{"default": "test election"},
			Description: map[string]string{"default": "test election description"},
			Version:     "1.0",
		})
	c.Assert(err, qt.IsNil)
	metadataURI := ipfs.CalculateCIDv1json(metadataBytes)
	var electionId types.HexBytes = util.RandomBytes(types.ProcessIDsize)
	tempSIKs := true
	process := &models.Process{
		EntityId:      admin.Address().Bytes(),
		ProcessId:     electionId,
		StartBlock:    0,
		BlockCount:    100,
		Status:        models.ProcessStatus_READY,
		CensusRoot:    censusRoot,
		CensusOrigin:  models.CensusOrigin_OFF_CHAIN_TREE_WEIGHTED,
		CensusURI:     &censusData.URI,
		Mode:          &models.ProcessMode{AutoStart: true, Interruptible: true},
		VoteOptions:   &models.ProcessVoteOptions{MaxCount: 1, MaxValue: 1},
		EnvelopeType:  &models.EnvelopeType{Anonymous: true},
		Metadata:      &metadataURI,
		MaxCensusSize: 1000,
		TempSIKs:      &tempSIKs,
	}
	c.Assert(server.VochainAPP.State.AddProcess(process), qt.IsNil)

	// Block 3
	server.VochainAPP.AdvanceTestBlock()
	waitUntilHeight(t, client, 3)
	// register voter temporal sik
	voterSIK, err := voter.AccountSIK(nil)
	c.Assert(err, qt.IsNil)
	c.Assert(server.VochainAPP.State.SetAddressSIK(voter.Address(), voterSIK), qt.IsNil)
	c.Assert(server.VochainAPP.State.IncreaseRegisterSIKCounter(electionId), qt.IsNil)
	c.Assert(server.VochainAPP.State.AssignSIKToElection(electionId, voter.Address()), qt.IsNil)

	// Block 4
	server.VochainAPP.AdvanceTestBlock()
	waitUntilHeight(t, client, 4)
	// schedule election results and finishing election
	c.Assert(server.VochainAPP.State.SetProcessStatus(electionId, models.ProcessStatus_ENDED, true), qt.IsNil)
	c.Assert(server.VochainAPP.Istc.Schedule(ist.Action{
		TypeID:     ist.ActionCommitResults,
		ElectionID: electionId,
		Height:     5,
		ID:         electionId,
	}), qt.IsNil)

	// Block 6
	server.VochainAPP.AdvanceTestBlock()
	server.VochainAPP.AdvanceTestBlock()
	waitUntilHeight(t, client, 6)
	// check that the election has ended
	electionData := &api.Election{}
	resp, code = client.Request("GET", nil, "elections", electionId.String())
	c.Assert(code, qt.Equals, 200)
	c.Assert(json.Unmarshal(resp, electionData), qt.IsNil)
	c.Assert(electionData.Status, qt.Equals, "RESULTS")
	// check that the voter sik has been deleted
	_, err = server.VochainAPP.State.SIKFromAddress(voter.Address())
	c.Assert(err, qt.IsNotNil, qt.Commentf("voter sik should have been deleted: %x", voter.Address()))
}

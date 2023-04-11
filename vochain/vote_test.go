package vochain

import (
	"encoding/json"
	"testing"

	qt "github.com/frankban/quicktest"
	abcitypes "github.com/tendermint/tendermint/abci/types"
	"go.vocdoni.io/dvote/censustree"
	"go.vocdoni.io/dvote/crypto/ethereum"
	"go.vocdoni.io/dvote/db/metadb"
	"go.vocdoni.io/dvote/types"
	"go.vocdoni.io/dvote/util"
	"go.vocdoni.io/proto/build/go/models"
	"google.golang.org/protobuf/proto"
)

// testCreateKeysAndBuildCensus creates a bunch of random keys and a new census tree.
// It returns the keys, the census root and the proofs for each key.
func testCreateKeysAndBuildCensus(t *testing.T, size int) ([]*ethereum.SignKeys, []byte, [][]byte) {
	db := metadb.NewTest(t)
	tr, err := censustree.New(censustree.Options{Name: "testcensus", ParentDB: db,
		MaxLevels: censustree.DefaultMaxLevels, CensusType: models.Census_ARBO_BLAKE2B})
	if err != nil {
		t.Fatal(err)
	}

	keys := ethereum.NewSignKeysBatch(size)
	hashedKeys := [][]byte{}
	for _, k := range keys {
		c, err := tr.Hash(k.Address().Bytes())
		qt.Check(t, err, qt.IsNil)
		c = c[:censustree.DefaultMaxKeyLen]
		err = tr.Add(c, nil)
		qt.Check(t, err, qt.IsNil)
		hashedKeys = append(hashedKeys, c)
	}

	tr.Publish()
	var proofs [][]byte
	for i := range keys {
		_, proof, err := tr.GenProof(hashedKeys[i])
		qt.Check(t, err, qt.IsNil)
		proofs = append(proofs, proof)
	}
	root, err := tr.Root()
	qt.Check(t, err, qt.IsNil)
	return keys, root, proofs
}

func testBuildSignedVote(t *testing.T, electionID []byte, key *ethereum.SignKeys,
	proof []byte, votePackage []int, chainID string) *models.SignedTx {
	var stx models.SignedTx
	var err error
	vp, err := json.Marshal(votePackage)
	qt.Check(t, err, qt.IsNil)
	vote := &models.VoteEnvelope{
		Nonce:     util.RandomBytes(32),
		ProcessId: electionID,
		Proof: &models.Proof{
			Payload: &models.Proof_Arbo{
				Arbo: &models.ProofArbo{
					Type:     models.ProofArbo_BLAKE2B,
					Siblings: proof,
					KeyType:  models.ProofArbo_ADDRESS,
				},
			},
		},
		VotePackage: vp,
	}

	stx.Tx, err = proto.Marshal(&models.Tx{
		Payload: &models.Tx_Vote{Vote: vote},
	})
	qt.Check(t, err, qt.IsNil)
	stx.Signature, err = key.SignVocdoniTx(stx.Tx, chainID)
	qt.Check(t, err, qt.IsNil)
	return &stx
}

func TestVoteOverwrite(t *testing.T) {
	app := TestBaseApplication(t)
	keys, root, proofs := testCreateKeysAndBuildCensus(t, 10)
	censusURI := ipfsUrl
	pid := util.RandomBytes(types.ProcessIDsize)
	process := &models.Process{
		ProcessId:    pid,
		StartBlock:   0,
		EnvelopeType: &models.EnvelopeType{EncryptedVotes: false},
		Mode: &models.ProcessMode{
			AutoStart: true,
		},
		VoteOptions: &models.ProcessVoteOptions{
			MaxCount:          3,
			MaxValue:          3,
			MaxVoteOverwrites: 2,
		},
		Status:        models.ProcessStatus_READY,
		EntityId:      util.RandomBytes(types.EthereumAddressSize),
		CensusRoot:    root,
		CensusURI:     &censusURI,
		CensusOrigin:  models.CensusOrigin_OFF_CHAIN_TREE,
		BlockCount:    1024,
		MaxCensusSize: 10,
	}
	err := app.State.AddProcess(process)
	qt.Check(t, err, qt.IsNil)
	app.AdvanceTestBlock()

	var cktx abcitypes.RequestCheckTx
	var detx abcitypes.RequestDeliverTx
	var cktxresp abcitypes.ResponseCheckTx
	var detxresp abcitypes.ResponseDeliverTx

	// send 9 votes, should be fine
	for i := 1; i < 10; i++ {
		stx := testBuildSignedVote(t, pid, keys[i], proofs[i], []int{1, 0, 1}, app.ChainID())
		cktx.Tx, err = proto.Marshal(stx)
		qt.Check(t, err, qt.IsNil)
		cktxresp = app.CheckTx(cktx)
		qt.Check(t, cktxresp.Code, qt.Equals, uint32(0))

		detx.Tx, err = proto.Marshal(stx)
		qt.Check(t, err, qt.IsNil)
		detxresp = app.DeliverTx(detx)
		qt.Check(t, detxresp.Code, qt.Equals, uint32(0))

		app.AdvanceTestBlock()
	}

	// Send the only missing vote, should be fine
	stx := testBuildSignedVote(t, pid, keys[0], proofs[0], []int{1, 2, 3}, app.ChainID())

	cktx.Tx, err = proto.Marshal(stx)
	qt.Check(t, err, qt.IsNil)
	cktxresp = app.CheckTx(cktx)
	qt.Check(t, cktxresp.Code, qt.Equals, uint32(0))

	detx.Tx, err = proto.Marshal(stx)
	qt.Check(t, err, qt.IsNil)
	detxresp = app.DeliverTx(detx)
	qt.Check(t, detxresp.Code, qt.Equals, uint32(0))

	app.AdvanceTestBlock()

	// Second vote (overwrite)
	stx = testBuildSignedVote(t, pid, keys[0], proofs[0], []int{1, 2, 1}, app.ChainID())

	cktx.Tx, err = proto.Marshal(stx)
	qt.Check(t, err, qt.IsNil)
	cktxresp = app.CheckTx(cktx)
	qt.Check(t, cktxresp.Code, qt.Equals, uint32(0))

	detx.Tx, err = proto.Marshal(stx)
	qt.Check(t, err, qt.IsNil)
	detxresp = app.DeliverTx(detx)
	qt.Check(t, detxresp.Code, qt.Equals, uint32(0))

	app.AdvanceTestBlock()

	// Third vote (overwrite)
	stx = testBuildSignedVote(t, pid, keys[0], proofs[0], []int{1, 1, 1}, app.ChainID())

	cktx.Tx, err = proto.Marshal(stx)
	qt.Check(t, err, qt.IsNil)
	cktxresp = app.CheckTx(cktx)
	qt.Check(t, cktxresp.Code, qt.Equals, uint32(0))

	detx.Tx, err = proto.Marshal(stx)
	qt.Check(t, err, qt.IsNil)
	detxresp = app.DeliverTx(detx)
	qt.Check(t, detxresp.Code, qt.Equals, uint32(0))

	app.AdvanceTestBlock()

	// Fourth vote (should fail since we have already voted 1 time + 2 overwrites)
	stx = testBuildSignedVote(t, pid, keys[0], proofs[0], []int{3, 1, 1}, app.ChainID())
	cktx.Tx, err = proto.Marshal(stx)
	qt.Check(t, err, qt.IsNil)
	cktxresp = app.CheckTx(cktx)
	qt.Check(t, cktxresp.Code, qt.Equals, uint32(1))

	vote, err := app.State.Vote(pid, detxresp.Data, false)
	qt.Check(t, err, qt.IsNil)
	qt.Check(t, vote.GetOverwriteCount(), qt.Equals, uint32(2))
}

func TestMaxCensusSize(t *testing.T) {
	app := TestBaseApplication(t)

	// set the global max census size to 20
	err := app.State.SetMaxProcessSize(20)
	qt.Check(t, err, qt.IsNil)

	// create a census with 10 keys
	keys, root, proofs := testCreateKeysAndBuildCensus(t, 11)
	censusURI := ipfsUrl

	// create a process with max census size 10
	pid := util.RandomBytes(types.ProcessIDsize)
	process := &models.Process{
		ProcessId:    pid,
		StartBlock:   0,
		EnvelopeType: &models.EnvelopeType{EncryptedVotes: false},
		Mode: &models.ProcessMode{
			AutoStart: true,
		},
		VoteOptions: &models.ProcessVoteOptions{
			MaxCount:          3,
			MaxValue:          3,
			MaxVoteOverwrites: 2,
		},
		Status:        models.ProcessStatus_READY,
		EntityId:      util.RandomBytes(types.EthereumAddressSize),
		CensusRoot:    root,
		CensusURI:     &censusURI,
		CensusOrigin:  models.CensusOrigin_OFF_CHAIN_TREE,
		BlockCount:    1024,
		MaxCensusSize: 10,
	}
	err = app.State.AddProcess(process)
	qt.Check(t, err, qt.IsNil)
	app.AdvanceTestBlock()

	// define a function to send a vote
	vote := func(i int) uint32 {
		var cktx abcitypes.RequestCheckTx
		var detx abcitypes.RequestDeliverTx
		stx := testBuildSignedVote(t, pid, keys[i], proofs[i], []int{1, 2, 3}, app.ChainID())
		cktx.Tx, err = proto.Marshal(stx)
		qt.Check(t, err, qt.IsNil)
		cktxresp := app.CheckTx(cktx)
		if cktxresp.Code != 0 {
			return cktxresp.Code
		}
		detx.Tx, err = proto.Marshal(stx)
		qt.Check(t, err, qt.IsNil)
		detxresp := app.DeliverTx(detx)
		return detxresp.Code
	}

	// send 10 votes, should be fine
	for i := 0; i < 10; i++ {
		qt.Check(t, vote(i), qt.Equals, uint32(0))
		app.AdvanceTestBlock()
	}

	// the 11th vote should fail
	qt.Check(t, vote(10), qt.Equals, uint32(1))
}

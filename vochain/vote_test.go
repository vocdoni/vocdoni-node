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
		MaxLevels: 256, CensusType: models.Census_ARBO_BLAKE2B})
	if err != nil {
		t.Fatal(err)
	}

	keys := util.CreateEthRandomKeysBatch(size)
	hashedKeys := [][]byte{}
	for _, k := range keys {
		c, err := tr.Hash(k.PublicKey())
		qt.Check(t, err, qt.IsNil)
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
					KeyType:  models.ProofArbo_PUBKEY,
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
		Status:       models.ProcessStatus_READY,
		EntityId:     util.RandomBytes(types.EthereumAddressSize),
		CensusRoot:   root,
		CensusURI:    &censusURI,
		CensusOrigin: models.CensusOrigin_OFF_CHAIN_TREE,
		BlockCount:   1024,
	}
	err := app.State.AddProcess(process)
	qt.Check(t, err, qt.IsNil)
	app.Commit()
	app.AdvanceTestBlock()

	var cktx abcitypes.RequestCheckTx
	var detx abcitypes.RequestDeliverTx
	var cktxresp abcitypes.ResponseCheckTx
	var detxresp abcitypes.ResponseDeliverTx

	// First vote
	stx := testBuildSignedVote(t, pid, keys[0], proofs[0], []int{1, 2, 3}, app.ChainID())

	cktx.Tx, err = proto.Marshal(stx)
	qt.Check(t, err, qt.IsNil)
	cktxresp = app.CheckTx(cktx)
	qt.Check(t, cktxresp.Code, qt.Equals, uint32(0))

	detx.Tx, err = proto.Marshal(stx)
	qt.Check(t, err, qt.IsNil)
	detxresp = app.DeliverTx(detx)
	qt.Check(t, detxresp.Code, qt.Equals, uint32(0))

	app.Commit()
	app.AdvanceTestBlock()

	// Second vote
	stx = testBuildSignedVote(t, pid, keys[0], proofs[0], []int{1, 2, 1}, app.ChainID())

	cktx.Tx, err = proto.Marshal(stx)
	qt.Check(t, err, qt.IsNil)
	cktxresp = app.CheckTx(cktx)
	qt.Check(t, cktxresp.Code, qt.Equals, uint32(0))

	detx.Tx, err = proto.Marshal(stx)
	qt.Check(t, err, qt.IsNil)
	detxresp = app.DeliverTx(detx)
	qt.Check(t, detxresp.Code, qt.Equals, uint32(0))

	app.Commit()
	app.AdvanceTestBlock()

	// Third vote
	stx = testBuildSignedVote(t, pid, keys[0], proofs[0], []int{1, 1, 1}, app.ChainID())

	cktx.Tx, err = proto.Marshal(stx)
	qt.Check(t, err, qt.IsNil)
	cktxresp = app.CheckTx(cktx)
	qt.Check(t, cktxresp.Code, qt.Equals, uint32(0))

	detx.Tx, err = proto.Marshal(stx)
	qt.Check(t, err, qt.IsNil)
	detxresp = app.DeliverTx(detx)
	qt.Check(t, detxresp.Code, qt.Equals, uint32(0))

	app.Commit()
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

	// TODO: add an indexer and check that the vote conent has actually been overwritten
}

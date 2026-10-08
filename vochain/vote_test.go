package vochain

import (
	"context"
	"encoding/json"
	"math/big"
	"testing"

	cometabcitypes "github.com/cometbft/cometbft/abci/types"
	qt "github.com/frankban/quicktest"
	"go.vocdoni.io/dvote/censustree"
	"go.vocdoni.io/dvote/config"
	"go.vocdoni.io/dvote/crypto/ethereum"
	"go.vocdoni.io/dvote/db/metadb"
	"go.vocdoni.io/dvote/tree/arbo"
	"go.vocdoni.io/dvote/types"
	"go.vocdoni.io/dvote/util"
	"go.vocdoni.io/proto/build/go/models"
	"google.golang.org/protobuf/proto"
)

// testCreateKeysAndBuildCensus creates a bunch of random keys and a new zk
// friendly census tree (using Poseidon as tree hash and the weight provided as
// leaf value). It returns the keys, the census root and the proofs for each key.
func testCreateKeysAndBuildWeightedZkCensus(t *testing.T, size int, weight *big.Int) ([]*ethereum.SignKeys, []byte, [][]byte) {
	db := metadb.NewTest(t)
	tr, err := censustree.New(censustree.Options{
		Name: "testcensus", ParentDB: db,
		MaxLevels: censustree.DefaultMaxLevels, CensusType: models.Census_ARBO_POSEIDON,
	})
	if err != nil {
		t.Fatal(err)
	}

	encWeight := arbo.BigIntToBytesLE(arbo.HashFunctionPoseidon.Len(), weight)
	keys := ethereum.NewSignKeysBatch(size)
	for _, k := range keys {
		qt.Check(t, err, qt.IsNil)
		err = tr.Add(k.Address().Bytes(), encWeight)
		qt.Check(t, err, qt.IsNil)
	}

	root, err := tr.Root()
	qt.Check(t, err, qt.IsNil)
	var proofs [][]byte
	for _, k := range keys {
		_, proof, err := tr.GenProof(k.Address().Bytes())
		qt.Check(t, err, qt.IsNil)
		proofs = append(proofs, proof)
		valid, err := tr.VerifyProof(k.Address().Bytes(), encWeight, proof, root)
		qt.Check(t, err, qt.IsNil)
		qt.Check(t, valid, qt.IsTrue)
	}
	return keys, root, proofs
}

// testCreateKeysAndBuildCensus creates a bunch of random keys and a new census tree.
// It returns the keys, the census root and the proofs for each key.
func testCreateKeysAndBuildCensus(t *testing.T, size int) ([]*ethereum.SignKeys, []byte, [][]byte) {
	db := metadb.NewTest(t)
	tr, err := censustree.New(censustree.Options{
		Name: "testcensus", ParentDB: db,
		MaxLevels: censustree.DefaultMaxLevels, CensusType: models.Census_ARBO_BLAKE2B,
	})
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
	proof []byte, votePackage []int, chainID string,
) *models.SignedTx {
	return testBuildSignedVoteWithMetadataHash(t, electionID, key, proof, votePackage, chainID, nil)
}

// testBuildSignedVoteWithMetadataHash builds a signed vote which attests the
// given election metadata hash.
func testBuildSignedVoteWithMetadataHash(t *testing.T, electionID []byte, key *ethereum.SignKeys,
	proof []byte, votePackage []int, chainID string, metadataHash []byte,
) *models.SignedTx {
	return testBuildSignedVoteWithHashes(t, electionID, key, proof, votePackage, chainID, metadataHash, nil)
}

// testBuildSignedVoteWithHashes builds a signed vote which attests the given
// election and parent election metadata hashes.
func testBuildSignedVoteWithHashes(t *testing.T, electionID []byte, key *ethereum.SignKeys,
	proof []byte, votePackage []int, chainID string, metadataHash, parentMetadataHash []byte,
) *models.SignedTx {
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
		VotePackage:        vp,
		MetadataHash:       metadataHash,
		ParentMetadataHash: parentMetadataHash,
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
	censusURI := ipfsUrlTest
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

	cktx := new(cometabcitypes.CheckTxRequest)
	var cktxresp *cometabcitypes.CheckTxResponse

	// send 9 votes, should be fine
	for i := 1; i < 10; i++ {
		stx := testBuildSignedVote(t, pid, keys[i], proofs[i], []int{1, 0, 1}, app.ChainID())
		cktx.Tx, err = proto.Marshal(stx)
		qt.Check(t, err, qt.IsNil)
		cktxresp, _ = app.CheckTx(context.TODO(), cktx)
		qt.Check(t, cktxresp.Code, qt.Equals, uint32(0))

		txb, err := proto.Marshal(stx)
		qt.Check(t, err, qt.IsNil)
		detxresp := app.deliverTx(txb)
		qt.Check(t, detxresp.Code, qt.Equals, uint32(0))

		app.AdvanceTestBlock()
	}

	// Send the only missing vote, should be fine
	stx := testBuildSignedVote(t, pid, keys[0], proofs[0], []int{1, 2, 3}, app.ChainID())

	cktx.Tx, err = proto.Marshal(stx)
	qt.Check(t, err, qt.IsNil)
	cktxresp, _ = app.CheckTx(context.TODO(), cktx)
	qt.Check(t, cktxresp.Code, qt.Equals, uint32(0))

	txb, err := proto.Marshal(stx)
	qt.Check(t, err, qt.IsNil)
	detxresp := app.deliverTx(txb)
	qt.Check(t, detxresp.Code, qt.Equals, uint32(0))

	app.AdvanceTestBlock()

	// Second vote (overwrite)
	stx = testBuildSignedVote(t, pid, keys[0], proofs[0], []int{1, 2, 1}, app.ChainID())

	cktx.Tx, err = proto.Marshal(stx)
	qt.Check(t, err, qt.IsNil)
	cktxresp, _ = app.CheckTx(context.TODO(), cktx)
	qt.Check(t, cktxresp.Code, qt.Equals, uint32(0))

	txb, err = proto.Marshal(stx)
	qt.Check(t, err, qt.IsNil)
	detxresp = app.deliverTx(txb)
	qt.Check(t, detxresp.Code, qt.Equals, uint32(0))

	app.AdvanceTestBlock()

	// Third vote (overwrite)
	stx = testBuildSignedVote(t, pid, keys[0], proofs[0], []int{1, 1, 1}, app.ChainID())

	cktx.Tx, err = proto.Marshal(stx)
	qt.Check(t, err, qt.IsNil)
	cktxresp, _ = app.CheckTx(context.TODO(), cktx)
	qt.Check(t, cktxresp.Code, qt.Equals, uint32(0))

	txb, err = proto.Marshal(stx)
	qt.Check(t, err, qt.IsNil)
	detxresp = app.deliverTx(txb)
	qt.Check(t, detxresp.Code, qt.Equals, uint32(0))

	app.AdvanceTestBlock()

	// Fourth vote (should fail since we have already voted 1 time + 2 overwrites)
	stx = testBuildSignedVote(t, pid, keys[0], proofs[0], []int{3, 1, 1}, app.ChainID())
	cktx.Tx, err = proto.Marshal(stx)
	qt.Check(t, err, qt.IsNil)
	cktxresp, _ = app.CheckTx(context.TODO(), cktx)
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
	censusURI := ipfsUrlTest

	// create a process with max census size 10
	pid := util.RandomBytes(types.ProcessIDsize)
	process := &models.Process{
		ProcessId:    pid,
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
		StartTime:     0,
		Duration:      100,
		MaxCensusSize: 10,
	}
	err = app.State.AddProcess(process)
	qt.Check(t, err, qt.IsNil)
	app.AdvanceTestBlock()

	// define a function to send a vote
	vote := func(i int) uint32 {
		cktx := new(cometabcitypes.CheckTxRequest)
		stx := testBuildSignedVote(t, pid, keys[i], proofs[i], []int{1, 2, 3}, app.ChainID())
		cktx.Tx, err = proto.Marshal(stx)
		qt.Check(t, err, qt.IsNil)
		cktxresp, _ := app.CheckTx(context.TODO(), cktx)
		if cktxresp.Code != 0 {
			return cktxresp.Code
		}
		txb, err := proto.Marshal(stx)
		qt.Check(t, err, qt.IsNil)
		detxresp := app.deliverTx(txb)
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

// testMetadataVoteSetup creates an election with a committed metadata hash,
// owned by an account able to update it, and a census of voters.
func testMetadataVoteSetup(t *testing.T) (*BaseApplication, *ethereum.SignKeys, *models.Process,
	[]*ethereum.SignKeys, [][]byte,
) {
	app, accounts := createTestBaseApplicationAndAccounts(t, 10)
	voters, root, proofs := testCreateKeysAndBuildCensus(t, 4)
	process := testMetadataProcess(accounts[0].Address().Bytes())
	process.ProcessId = util.RandomBytes(types.ProcessIDsize)
	process.Mode = &models.ProcessMode{AutoStart: true, Interruptible: true}
	process.VoteOptions = &models.ProcessVoteOptions{MaxCount: 3, MaxValue: 3}
	process.CensusRoot = root
	process.BlockCount = 1024
	qt.Assert(t, app.State.AddProcess(process), qt.IsNil)
	app.AdvanceTestBlock()
	return app, accounts[0], process, voters, proofs
}

func testSendVote(app *BaseApplication, stx *models.SignedTx, forCommit bool) uint32 {
	txb, err := proto.Marshal(stx)
	if err != nil {
		return 1
	}
	if forCommit {
		return app.deliverTx(txb).Code
	}
	resp, _ := app.CheckTx(context.TODO(), &cometabcitypes.CheckTxRequest{Tx: txb})
	return resp.Code
}

func TestVoteMetadataHash(t *testing.T) {
	app, _, process, voters, proofs := testMetadataVoteSetup(t)
	pid := process.ProcessId

	// a vote attesting the committed metadata is accepted
	stx := testBuildSignedVoteWithMetadataHash(t, pid, voters[0], proofs[0], []int{1, 0, 1},
		app.ChainID(), process.MetadataHash)
	qt.Assert(t, testSendVote(app, stx, false), qt.Equals, uint32(0))
	qt.Assert(t, testSendVote(app, stx, true), qt.Equals, uint32(0))
	app.AdvanceTestBlock()

	// a vote without the metadata hash is rejected
	stx = testBuildSignedVote(t, pid, voters[1], proofs[1], []int{1, 0, 1}, app.ChainID())
	qt.Assert(t, testSendVote(app, stx, false), qt.Not(qt.Equals), uint32(0))

	// a vote attesting another metadata is rejected
	stx = testBuildSignedVoteWithMetadataHash(t, pid, voters[1], proofs[1], []int{1, 0, 1},
		app.ChainID(), util.RandomBytes(32))
	qt.Assert(t, testSendVote(app, stx, false), qt.Not(qt.Equals), uint32(0))
}

// TestVoteMetadataHashChangedInFlight checks that a vote accepted into the
// mempool is rejected when the metadata is updated before it is included.
func TestVoteMetadataHashChangedInFlight(t *testing.T) {
	app, owner, process, voters, proofs := testMetadataVoteSetup(t)
	pid := process.ProcessId

	stx := testBuildSignedVoteWithMetadataHash(t, pid, voters[0], proofs[0], []int{1, 0, 1},
		app.ChainID(), process.MetadataHash)
	qt.Assert(t, testSendVote(app, stx, false), qt.Equals, uint32(0))

	uri := "https://example.com/metadata/2.json"
	newHash := util.RandomBytes(32)
	qt.Assert(t, testSetProcessMetadata(t, pid, owner, app, &uri, newHash), qt.IsNil)

	qt.Assert(t, testSendVote(app, stx, true), qt.Not(qt.Equals), uint32(0))

	// voting again attesting the new metadata is accepted
	stx = testBuildSignedVoteWithMetadataHash(t, pid, voters[0], proofs[0], []int{1, 0, 1},
		app.ChainID(), newHash)
	qt.Assert(t, testSendVote(app, stx, false), qt.Equals, uint32(0))
	qt.Assert(t, testSendVote(app, stx, true), qt.Equals, uint32(0))
}

func TestVoteMetadataHashWithoutCommittedHash(t *testing.T) {
	app := TestBaseApplication(t)
	voters, root, proofs := testCreateKeysAndBuildCensus(t, 2)
	process := testMetadataProcess(util.RandomBytes(types.EthereumAddressSize))
	process.ProcessId = util.RandomBytes(types.ProcessIDsize)
	process.Mode = &models.ProcessMode{AutoStart: true}
	process.VoteOptions = &models.ProcessVoteOptions{MaxCount: 3, MaxValue: 3}
	process.CensusRoot = root
	process.BlockCount = 1024
	process.MetadataHash = nil
	qt.Assert(t, app.State.AddProcess(process), qt.IsNil)
	app.AdvanceTestBlock()

	// without a committed hash, votes must not attest one
	stx := testBuildSignedVoteWithMetadataHash(t, process.ProcessId, voters[0], proofs[0], []int{1, 0, 1},
		app.ChainID(), util.RandomBytes(32))
	qt.Assert(t, testSendVote(app, stx, false), qt.Not(qt.Equals), uint32(0))

	stx = testBuildSignedVote(t, process.ProcessId, voters[0], proofs[0], []int{1, 0, 1}, app.ChainID())
	qt.Assert(t, testSendVote(app, stx, false), qt.Equals, uint32(0))
	qt.Assert(t, testSendVote(app, stx, true), qt.Equals, uint32(0))
}

// TestVoteMetadataHashBeforeForkLTS13 checks that on vocdoni/LTS/1.3 votes are
// not checked against the metadata hash before its config.Forks MetadataFork height.
func TestVoteMetadataHashBeforeForkLTS13(t *testing.T) {
	app, _, process, voters, proofs := testMetadataVoteSetup(t)
	app.SetChainID("vocdoni/LTS/1.3")
	app.State.SetHeight(config.ForksForChainID("vocdoni/LTS/1.3").MetadataFork - 1)

	stx := testBuildSignedVote(t, process.ProcessId, voters[0], proofs[0], []int{1, 0, 1}, app.ChainID())
	qt.Assert(t, testSendVote(app, stx, false), qt.Equals, uint32(0))
	qt.Assert(t, testSendVote(app, stx, true), qt.Equals, uint32(0))
}

// testParentVoteSetup creates a metadata-only parent election and an election
// linked to it, both with a committed metadata hash and owned by an account able
// to update them, and a census of voters.
func testParentVoteSetup(t *testing.T) (*BaseApplication, *ethereum.SignKeys, *models.Process, *models.Process,
	[]*ethereum.SignKeys, [][]byte,
) {
	app, owner, process, voters, proofs := testMetadataVoteSetup(t)
	parent := testMetadataOnlyProcess(owner.Address().Bytes())
	parent.ProcessId = util.RandomBytes(types.ProcessIDsize)
	parent.BlockCount = 1024
	qt.Assert(t, app.State.AddProcess(parent), qt.IsNil)
	process.ParentProcessId = parent.ProcessId
	qt.Assert(t, app.State.UpdateProcess(process, process.ProcessId), qt.IsNil)
	app.AdvanceTestBlock()
	return app, owner, parent, process, voters, proofs
}

func TestVoteMetadataOnlyProcess(t *testing.T) {
	app, _, parent, _, voters, proofs := testParentVoteSetup(t)

	// a metadata-only process takes no votes
	stx := testBuildSignedVoteWithMetadataHash(t, parent.ProcessId, voters[0], proofs[0], []int{1, 0, 1},
		app.ChainID(), parent.MetadataHash)
	qt.Assert(t, testSendVote(app, stx, false), qt.Not(qt.Equals), uint32(0))
	qt.Assert(t, testSendVote(app, stx, true), qt.Not(qt.Equals), uint32(0))
}

func TestVoteParentMetadataHash(t *testing.T) {
	app, _, parent, process, voters, proofs := testParentVoteSetup(t)
	pid := process.ProcessId

	// a vote attesting both the election and the parent metadata is accepted
	stx := testBuildSignedVoteWithHashes(t, pid, voters[0], proofs[0], []int{1, 0, 1},
		app.ChainID(), process.MetadataHash, parent.MetadataHash)
	qt.Assert(t, testSendVote(app, stx, false), qt.Equals, uint32(0))
	qt.Assert(t, testSendVote(app, stx, true), qt.Equals, uint32(0))
	app.AdvanceTestBlock()

	// a vote without the parent metadata hash is rejected
	stx = testBuildSignedVoteWithMetadataHash(t, pid, voters[1], proofs[1], []int{1, 0, 1},
		app.ChainID(), process.MetadataHash)
	qt.Assert(t, testSendVote(app, stx, false), qt.Not(qt.Equals), uint32(0))

	// a vote attesting another parent metadata is rejected
	stx = testBuildSignedVoteWithHashes(t, pid, voters[1], proofs[1], []int{1, 0, 1},
		app.ChainID(), process.MetadataHash, util.RandomBytes(32))
	qt.Assert(t, testSendVote(app, stx, false), qt.Not(qt.Equals), uint32(0))

	// a vote attesting the parent metadata as the election one is rejected
	stx = testBuildSignedVoteWithHashes(t, pid, voters[1], proofs[1], []int{1, 0, 1},
		app.ChainID(), parent.MetadataHash, parent.MetadataHash)
	qt.Assert(t, testSendVote(app, stx, false), qt.Not(qt.Equals), uint32(0))
}

// TestVoteParentMetadataHashChangedInFlight checks that a vote accepted into the
// mempool is rejected when the parent metadata is updated before it is included.
func TestVoteParentMetadataHashChangedInFlight(t *testing.T) {
	app, owner, parent, process, voters, proofs := testParentVoteSetup(t)
	pid := process.ProcessId

	stx := testBuildSignedVoteWithHashes(t, pid, voters[0], proofs[0], []int{1, 0, 1},
		app.ChainID(), process.MetadataHash, parent.MetadataHash)
	qt.Assert(t, testSendVote(app, stx, false), qt.Equals, uint32(0))

	uri := "https://example.com/metadata/parent-2.json"
	newHash := util.RandomBytes(32)
	qt.Assert(t, testSetProcessMetadata(t, parent.ProcessId, owner, app, &uri, newHash), qt.IsNil)

	qt.Assert(t, testSendVote(app, stx, true), qt.Not(qt.Equals), uint32(0))

	// voting again attesting the new parent metadata is accepted
	stx = testBuildSignedVoteWithHashes(t, pid, voters[0], proofs[0], []int{1, 0, 1},
		app.ChainID(), process.MetadataHash, newHash)
	qt.Assert(t, testSendVote(app, stx, false), qt.Equals, uint32(0))
	qt.Assert(t, testSendVote(app, stx, true), qt.Equals, uint32(0))
}

func TestVoteParentMetadataHashWithoutParent(t *testing.T) {
	app, _, process, voters, proofs := testMetadataVoteSetup(t)

	// without a parent, votes must not attest a parent metadata hash
	stx := testBuildSignedVoteWithHashes(t, process.ProcessId, voters[0], proofs[0], []int{1, 0, 1},
		app.ChainID(), process.MetadataHash, util.RandomBytes(32))
	qt.Assert(t, testSendVote(app, stx, false), qt.Not(qt.Equals), uint32(0))

	stx = testBuildSignedVoteWithHashes(t, process.ProcessId, voters[0], proofs[0], []int{1, 0, 1},
		app.ChainID(), process.MetadataHash, nil)
	qt.Assert(t, testSendVote(app, stx, false), qt.Equals, uint32(0))
	qt.Assert(t, testSendVote(app, stx, true), qt.Equals, uint32(0))
}

// TestVoteParentMetadataHashBeforeForkLTS13 checks that on vocdoni/LTS/1.3, where
// the parent fork is not scheduled, votes are not checked against the parent
// metadata hash.
func TestVoteParentMetadataHashBeforeForkLTS13(t *testing.T) {
	app, _, _, process, voters, proofs := testParentVoteSetup(t)
	app.SetChainID("vocdoni/LTS/1.3")
	app.State.SetHeight(config.ForksForChainID("vocdoni/LTS/1.3").MetadataFork)

	stx := testBuildSignedVoteWithMetadataHash(t, process.ProcessId, voters[0], proofs[0], []int{1, 0, 1},
		app.ChainID(), process.MetadataHash)
	qt.Assert(t, testSendVote(app, stx, false), qt.Equals, uint32(0))
	qt.Assert(t, testSendVote(app, stx, true), qt.Equals, uint32(0))
}

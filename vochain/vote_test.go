package vochain

import (
	"context"
	"encoding/json"
	"testing"

	qt "github.com/frankban/quicktest"
	"go.vocdoni.io/dvote/config"
	"go.vocdoni.io/dvote/crypto/ethereum"
	"go.vocdoni.io/dvote/test/testcommon/testvoteproof"
	"go.vocdoni.io/dvote/types"
	"go.vocdoni.io/dvote/util"
	"go.vocdoni.io/proto/build/go/models"
	"google.golang.org/protobuf/proto"
)

func testBuildSignedVote(t *testing.T, electionID []byte, key *ethereum.SignKeys,
	proof *models.ProofCA, votePackage []int, chainID string,
) *models.SignedTx {
	return testBuildSignedVoteWithMetadataHash(t, electionID, key, proof, votePackage, chainID, nil)
}

// testBuildSignedVoteWithMetadataHash builds a signed vote which attests the
// given election metadata hash.
func testBuildSignedVoteWithMetadataHash(t *testing.T, electionID []byte, key *ethereum.SignKeys,
	proof *models.ProofCA, votePackage []int, chainID string, metadataHash []byte,
) *models.SignedTx {
	var stx models.SignedTx
	var err error
	vp, err := json.Marshal(votePackage)
	qt.Check(t, err, qt.IsNil)
	vote := &models.VoteEnvelope{
		Nonce:        util.RandomBytes(32),
		ProcessId:    electionID,
		Proof:        &models.Proof{Payload: &models.Proof_Ca{Ca: proof}},
		VotePackage:  vp,
		MetadataHash: metadataHash,
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
	pid := util.RandomBytes(types.ProcessIDsize)
	keys, root, proofs := testvoteproof.CreateKeysAndBuildCensus(t, 10, pid)
	censusURI := ipfsUrlTest
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
		CensusOrigin:  models.CensusOrigin_OFF_CHAIN_CA_V2,
		BlockCount:    1024,
		MaxCensusSize: 10,
	}
	err := app.State.AddProcess(process)
	qt.Check(t, err, qt.IsNil)
	app.AdvanceTestBlock()

	cktx := new(CheckTxRequest)
	var cktxresp *CheckTxResponse

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

	// create a census with 11 keys
	pid := util.RandomBytes(types.ProcessIDsize)
	keys, root, proofs := testvoteproof.CreateKeysAndBuildCensus(t, 11, pid)
	censusURI := ipfsUrlTest

	// create a process with max census size 10
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
		CensusOrigin:  models.CensusOrigin_OFF_CHAIN_CA_V2,
		StartTime:     0,
		Duration:      100,
		MaxCensusSize: 10,
	}
	err = app.State.AddProcess(process)
	qt.Check(t, err, qt.IsNil)
	app.AdvanceTestBlock()

	// define a function to send a vote
	vote := func(i int) uint32 {
		cktx := new(CheckTxRequest)
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
	[]*ethereum.SignKeys, []*models.ProofCA,
) {
	app, accounts := createTestBaseApplicationAndAccounts(t, 10)
	process := testMetadataProcess(accounts[0].Address().Bytes())
	process.ProcessId = util.RandomBytes(types.ProcessIDsize)
	voters, root, proofs := testvoteproof.CreateKeysAndBuildCensus(t, 4, process.ProcessId)
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
	resp, _ := app.CheckTx(context.TODO(), &CheckTxRequest{Tx: txb})
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
	process := testMetadataProcess(util.RandomBytes(types.EthereumAddressSize))
	process.ProcessId = util.RandomBytes(types.ProcessIDsize)
	voters, root, proofs := testvoteproof.CreateKeysAndBuildCensus(t, 2, process.ProcessId)
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

package vochain

import (
	"context"
	"testing"

	cometabcitypes "github.com/cometbft/cometbft/abci/types"
	qt "github.com/frankban/quicktest"
	"go.vocdoni.io/dvote/crypto/ethereum"
	"go.vocdoni.io/dvote/types"
	"go.vocdoni.io/dvote/util"
	"go.vocdoni.io/dvote/vochain/state"
	"go.vocdoni.io/proto/build/go/models"
	"google.golang.org/protobuf/proto"
)

func TestMerkleTreeProof(t *testing.T) {
	app := TestBaseApplication(t)
	keys, root, proofs := testCreateKeysAndBuildCensus(t, 101)

	// we save the last key for the next test and remove it from the lists
	lastKey := keys[len(keys)-1]
	keys = keys[:len(keys)-2]
	lastProof := proofs[len(proofs)-1]
	proofs = proofs[:len(proofs)-2]

	censusURI := ipfsUrlTest
	pid := util.RandomBytes(types.ProcessIDsize)

	process := &models.Process{
		ProcessId:     pid,
		StartBlock:    0,
		EnvelopeType:  &models.EnvelopeType{EncryptedVotes: false},
		Mode:          &models.ProcessMode{},
		VoteOptions:   &models.ProcessVoteOptions{MaxCount: 1, MaxValue: 1},
		Status:        models.ProcessStatus_READY,
		EntityId:      util.RandomBytes(types.EthereumAddressSize),
		CensusRoot:    root,
		CensusURI:     &censusURI,
		CensusOrigin:  models.CensusOrigin_OFF_CHAIN_TREE,
		BlockCount:    1024,
		MaxCensusSize: 200,
	}
	err := app.State.AddProcess(process)
	qt.Assert(t, err, qt.IsNil)

	app.AdvanceTestBlock()

	cktx := new(cometabcitypes.CheckTxRequest)

	// send the votes (but not the last one), should be ok
	for i, s := range keys {
		stx := testBuildSignedVote(t, pid, s, proofs[i], []int{1, 2, 3, 4}, app.ChainID())

		cktx.Tx, err = proto.Marshal(stx)
		qt.Assert(t, err, qt.IsNil)
		cktxresp, _ := app.CheckTx(context.Background(), cktx)
		qt.Assert(t, cktxresp.Code, qt.Equals, uint32(0))

		txb, err := proto.Marshal(stx)
		qt.Assert(t, err, qt.IsNil)
		detxresp := app.deliverTx(txb)
		qt.Assert(t, detxresp.Code, qt.Equals, uint32(0))

		if i%5 == 0 {
			_, err = app.CommitState()
			qt.Assert(t, err, qt.IsNil)
			app.AdvanceTestBlock()
		}
	}
	app.AdvanceTestBlock()
	vp, err := state.NewVotePackage([]int{1, 2, 3, 4}).Encode()
	qt.Assert(t, err, qt.IsNil)

	// send the las vote multiple time, first attempt should be ok. The rest should fail.
	// we modify the nonce on each attempt to avoid the cache
	vote := &models.VoteEnvelope{
		ProcessId: pid,
		Proof: &models.Proof{
			Payload: &models.Proof_Arbo{
				Arbo: &models.ProofArbo{
					Type:     models.ProofArbo_BLAKE2B,
					Siblings: lastProof,
				},
			},
		},
		VotePackage: vp,
	}

	for i := 0; i < 10; i++ {
		stx := &models.SignedTx{}
		vote.Nonce = util.RandomBytes(32)
		stx.Tx, err = proto.Marshal(&models.Tx{
			Payload: &models.Tx_Vote{Vote: vote},
		})
		qt.Assert(t, err, qt.IsNil)
		stx.Signature, err = lastKey.SignVocdoniTx(stx.Tx, app.chainID)
		qt.Assert(t, err, qt.IsNil)

		cktx.Tx, err = proto.Marshal(stx)
		qt.Assert(t, err, qt.IsNil)

		cktxresp, _ := app.CheckTx(context.Background(), cktx)
		if i == 0 && cktxresp.Code != 0 {
			t.Fatalf("checkTx returned err on first valid vote: %s", cktxresp.Data)
		}
		if i > 0 && cktxresp.Code == 0 {
			t.Fatalf("checkTx returned 0 for vote %d, an error was expected", i)
		}

		txb, err := proto.Marshal(stx)
		qt.Assert(t, err, qt.IsNil)

		detxresp := app.deliverTx(txb)
		if i == 0 && detxresp.Code != 0 {
			t.Fatalf("devlierTx returned err on first valid vote: %s", detxresp.Data)
		}
		if i > 0 && detxresp.Code == 0 {
			t.Fatalf("deliverTx returned 0, an error was expected")
		}
		if i%2 == 0 {
			app.AdvanceTestBlock()
		}
	}
}

func TestCSPproof(t *testing.T) {
	app := TestBaseApplication(t)
	csp := ethereum.SignKeys{}
	err := csp.Generate()
	qt.Assert(t, err, qt.IsNil)

	pid := util.RandomBytes(types.ProcessIDsize)
	process := &models.Process{
		ProcessId:     pid,
		StartBlock:    0,
		EnvelopeType:  &models.EnvelopeType{EncryptedVotes: false},
		Mode:          new(models.ProcessMode),
		Status:        models.ProcessStatus_READY,
		EntityId:      util.RandomBytes(types.EthereumAddressSize),
		CensusRoot:    csp.PublicKey(),
		CensusOrigin:  models.CensusOrigin_OFF_CHAIN_CA,
		BlockCount:    1024,
		MaxCensusSize: 1000,
	}
	t.Logf("adding process %x", process.ProcessId)
	qt.Assert(t, app.State.AddProcess(process), qt.IsNil)

	// Test 20 valid votes
	vp, err := state.NewVotePackage([]int{1, 2, 3, 4}).Encode()
	qt.Assert(t, err, qt.IsNil)

	keys := ethereum.NewSignKeysBatch(20)
	for _, k := range keys {
		bundle := &models.CAbundle{
			ProcessId: pid,
			Address:   k.Address().Bytes(),
		}
		bundleBytes, err := proto.Marshal(bundle)
		qt.Assert(t, err, qt.IsNil)
		signature, err := csp.SignEthereum(bundleBytes)
		qt.Assert(t, err, qt.IsNil)

		proof := &models.ProofCA{
			Bundle:    bundle,
			Type:      models.ProofCA_ECDSA,
			Signature: signature,
		}
		testCSPsendVotes(t, pid, vp, k, proof, app, true)
	}

	// Test invalid vote
	k := ethereum.SignKeys{}
	qt.Assert(t, k.Generate(), qt.IsNil)
	bundle := &models.CAbundle{
		ProcessId: pid,
		Address:   k.Address().Bytes(),
	}
	bundleBytes, err := proto.Marshal(bundle)
	qt.Assert(t, err, qt.IsNil)

	csp2 := ethereum.SignKeys{}
	qt.Assert(t, csp2.Generate(), qt.IsNil)

	signature, err := csp2.SignEthereum(bundleBytes)
	qt.Assert(t, err, qt.IsNil)

	proof := &models.ProofCA{
		Bundle:    bundle,
		Type:      models.ProofCA_ECDSA,
		Signature: signature,
	}
	testCSPsendVotes(t, pid, vp, &k, proof, app, false)
}

func testCSPsendVotes(t *testing.T, pid []byte, vp []byte, signer *ethereum.SignKeys,
	proof *models.ProofCA, app *BaseApplication, expectedResult bool,
) {
	cktx := new(cometabcitypes.CheckTxRequest)
	var cktxresp *cometabcitypes.CheckTxResponse
	var stx models.SignedTx
	var err error

	tx := &models.VoteEnvelope{
		Nonce:       util.RandomBytes(32),
		ProcessId:   pid,
		Proof:       &models.Proof{Payload: &models.Proof_Ca{Ca: proof}},
		VotePackage: vp,
	}

	stx.Tx, err = proto.Marshal(&models.Tx{
		Payload: &models.Tx_Vote{
			Vote: tx,
		},
	})
	qt.Assert(t, err, qt.IsNil)

	stx.Signature, err = signer.SignVocdoniTx(stx.Tx, app.chainID)
	qt.Assert(t, err, qt.IsNil)

	cktx.Tx, err = proto.Marshal(&stx)
	qt.Assert(t, err, qt.IsNil)

	cktxresp, _ = app.CheckTx(context.Background(), cktx)
	if cktxresp.Code != 0 {
		if expectedResult {
			t.Fatalf("checkTx failed: %s", cktxresp.Data)
		}
	} else {
		if !expectedResult {
			t.Fatalf("checkTx success, but expected result is fail")
		}
	}
	txb, err := proto.Marshal(&stx)
	qt.Assert(t, err, qt.IsNil)
	detxresp := app.deliverTx(txb)
	if detxresp.Code != 0 {
		if expectedResult {
			t.Fatalf("deliverTx failed: %s", detxresp.Data)
		}
	} else {
		if !expectedResult {
			t.Fatalf("deliverTx success, but expected result is fail")
		}
	}
	_, err = app.CommitState()
	qt.Assert(t, err, qt.IsNil)
}

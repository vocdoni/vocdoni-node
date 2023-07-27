package vochain

import (
	"encoding/json"
	"math/big"
	"testing"

	qt "github.com/frankban/quicktest"
	"go.vocdoni.io/dvote/crypto/zk"
	"go.vocdoni.io/dvote/crypto/zk/circuit"
	"go.vocdoni.io/dvote/crypto/zk/prover"
	"go.vocdoni.io/dvote/types"
	"go.vocdoni.io/dvote/util"
	"go.vocdoni.io/dvote/vochain/state"
	"go.vocdoni.io/dvote/vochain/transaction/vochaintx"
	"go.vocdoni.io/proto/build/go/models"
)

func TestVoteCheckZkSNARK(t *testing.T) {
	c := qt.New(t)
	// create test app and load zk circuit
	app := TestBaseApplication(t)
	devCircuit, err := circuit.LoadZkCircuitByTag(circuit.DefaultCircuitConfigurationTag)
	c.Assert(err, qt.IsNil)
	app.TransactionHandler.ZkCircuit = devCircuit
	// set initial inputs
	testWeight := big.NewInt(10)
	accounts, censusRoot, proofs := testCreateKeysAndBuildWeightedZkCensus(t, 10, testWeight)
	testAccount := accounts[0]
	testProof := proofs[0]
	testSiblings, err := zk.ProofToCircomSiblings(testProof)
	c.Assert(err, qt.IsNil)
	// add the test account sik to the test app
	testSIK, err := testAccount.AccountSIK(nil)
	c.Assert(err, qt.IsNil)
	c.Assert(app.State.SetAddressSIK(testAccount.Address(), testSIK), qt.IsNil)
	// get siktree root and proof
	app.State.Tx.Lock()
	sikTree, err := app.State.Tx.DeepSubTree(state.StateTreeCfg(state.TreeSIK))
	c.Assert(err, qt.IsNil)
	sikRoot, err := sikTree.Root()
	c.Assert(err, qt.IsNil)
	_, sikProof, err := sikTree.GenProof(testAccount.Address().Bytes())
	c.Assert(err, qt.IsNil)
	app.State.Tx.Unlock()
	sikSiblings, err := zk.ProofToCircomSiblings(sikProof)
	c.Assert(err, qt.IsNil)
	// mock an election ID and add it to the state process block registry with
	// its startBlock
	electionId := util.RandomBytes(32)
	entityID := util.RandomBytes(types.EntityIDsize)
	c.Assert(app.State.ProcessBlockRegistry.SetStartBlock(electionId, 100), qt.IsNil)
	// create and add the process to the state with the census created
	process := &models.Process{
		ProcessId: electionId,
		EntityId:  entityID,
		EnvelopeType: &models.EnvelopeType{
			Anonymous: true,
		},
		Mode:          &models.ProcessMode{},
		VoteOptions:   &models.ProcessVoteOptions{MaxCount: 1},
		Status:        models.ProcessStatus_READY,
		CensusRoot:    censusRoot,
		StartBlock:    0,
		BlockCount:    3,
		MaxCensusSize: 100,
	}
	err = app.State.AddProcess(process)
	c.Assert(err, qt.IsNil)
	_, err = app.State.Process(electionId, false)
	c.Assert(err, qt.IsNil)
	// generate circuit inputs and the zk proof
	inputs, err := circuit.GenerateCircuitInput(circuit.CircuitInputsParameters{
		Account:         testAccount,
		ElectionId:      electionId,
		CensusRoot:      process.CensusRoot,
		SIKRoot:         sikRoot,
		CensusSiblings:  testSiblings,
		SIKSiblings:     sikSiblings,
		AvailableWeight: testWeight,
	})
	c.Assert(err, qt.IsNil)
	encInputs, err := json.Marshal(inputs)
	c.Assert(err, qt.IsNil)
	proof, err := prover.Prove(devCircuit.ProvingKey, devCircuit.Wasm, encInputs)
	c.Assert(err, qt.IsNil)
	// generate nullifier
	nullifier, err := testAccount.AccountSIKnullifier(electionId, nil)
	c.Assert(err, qt.IsNil)
	// encode the zk proof and create the vote tx
	protoProof, err := zk.ProverProofToProtobufZKProof(proof, nil, nil, nil, nil, nil)
	c.Assert(err, qt.IsNil)
	vtx := &models.VoteEnvelope{
		ProcessId:   electionId,
		VotePackage: testWeight.Bytes(),
		Nullifier:   nullifier,
		Proof: &models.Proof{
			Payload: &models.Proof_ZkSnark{
				ZkSnark: protoProof,
			},
		},
	}
	signature := []byte{}
	txBytes := []byte{}
	txID := [32]byte{}
	commit := false
	// check the vote tx
	_, err = app.TransactionHandler.VoteTxCheck(&vochaintx.Tx{
		Tx:         &models.Tx{Payload: &models.Tx_Vote{Vote: vtx}},
		Signature:  signature,
		SignedBody: txBytes,
		TxID:       txID,
	}, commit)
	c.Assert(err, qt.IsNil)
}

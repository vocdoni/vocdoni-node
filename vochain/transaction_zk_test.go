package vochain

import (
	"context"
	"math/big"
	"testing"

	qt "github.com/frankban/quicktest"
	"go.vocdoni.io/dvote/crypto/zk"
	"go.vocdoni.io/dvote/crypto/zk/circuit"
	"go.vocdoni.io/dvote/crypto/zk/prover"
	"go.vocdoni.io/dvote/tree/arbo"
	"go.vocdoni.io/dvote/types"
	"go.vocdoni.io/dvote/util"
	"go.vocdoni.io/dvote/vochain/transaction/vochaintx"
	models "go.vocdoni.io/proto/build/go/models"
)

func TestVoteCheckZkSNARK(t *testing.T) {
	app := TestBaseApplication(t)

	devConfig := circuit.CircuitsConfigurations[circuit.DefaultCircuitConfigurationTag]
	devCircuit, err := circuit.LoadZkCircuit(context.Background(), devConfig)
	qt.Assert(t, err, qt.IsNil)
	app.TransactionHandler.ZkCircuit = devCircuit

	processID := util.RandomBytes(types.ProcessIDsize)
	entityID := util.RandomBytes(types.EntityIDsize)
	censusRoot, ok := new(big.Int).SetString("10496064962946632764987634464751340919908597350627468905102211271224127873035", 10)
	qt.Assert(t, ok, qt.IsTrue)

	process := &models.Process{
		ProcessId: processID,
		EntityId:  entityID,
		EnvelopeType: &models.EnvelopeType{
			Anonymous: true,
		},
		Mode:          &models.ProcessMode{},
		VoteOptions:   &models.ProcessVoteOptions{MaxCount: 1},
		Status:        models.ProcessStatus_READY,
		CensusRoot:    make([]byte, 32), // emtpy hash
		StartBlock:    0,
		BlockCount:    3,
		MaxCensusSize: 100,
	}
	err = app.State.AddProcess(process)
	qt.Assert(t, err, qt.IsNil)
	_, err = app.State.Process(processID, false)
	qt.Assert(t, err, qt.IsNil)

	process, err = app.State.Process(processID, false)
	qt.Assert(t, err, qt.IsNil)
	process.CensusRoot = arbo.BigIntToBytes(32, censusRoot)
	err = app.State.UpdateProcess(process, processID)
	qt.Assert(t, err, qt.IsNil)

	proof := []byte(`{"pi_a":["21158713212294548026677000563764167209272759671976866712664167798559051202646","6034092600241427382393284530371885277965501508874433381064419215945014132128","1"],"pi_b":[["3266693092133849765080082495146214118776772951994743649670105567788500990913","11438329347684113431829025805514334037171052060709551105891698682784042838602"],["15407735792470062236368054442309427192794290489751614407182885978595493069014","19275403188498582245192654060074828094221466750742514931847112746396601405846"],["1","0"]],"pi_c":["2509932769569282676285537767124587450934534958560941562700329085891096418979","12539123792181279555744538589401927950609420421690526254651926376926051596539","1"]}`)
	pubSignals := []byte(`["18517551409637235305922365793037451371","135271561984151624501280044000043030166","10496064962946632764987634464751340919908597350627468905102211271224127873035","13830839320176376721270728875863016529251254252806875185281289627544884475042","10","242167611561429044353397595226328893827","161959801082294318672475063060030470563"]`)

	parsedProof, err := prover.ParseProof(proof, pubSignals)
	qt.Assert(t, err, qt.IsNil)

	nullifierBI, ok := new(big.Int).SetString("13830839320176376721270728875863016529251254252806875185281289627544884475042", 10)
	qt.Assert(t, ok, qt.IsTrue)
	nullifier := arbo.BigIntToBytes(32, nullifierBI)

	weight := new(big.Int).SetInt64(1)
	testVote := &models.VoteEnvelope{
		ProcessId:   processID,
		VotePackage: weight.Bytes(),
		Nullifier:   nullifier,
	}
	protoProof, err := zk.ProverProofToProtobufZKProof(parsedProof,
		testVote.ProcessId, process.CensusRoot, testVote.Nullifier, weight)
	qt.Assert(t, err, qt.IsNil)

	voteValue := big.NewInt(1).Bytes()
	vtx := &models.VoteEnvelope{
		ProcessId:   processID,
		VotePackage: voteValue,
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

	_, err = app.TransactionHandler.VoteTxCheck(&vochaintx.VochainTx{
		Tx:         &models.Tx{Payload: &models.Tx_Vote{Vote: vtx}},
		Signature:  signature,
		SignedBody: txBytes,
		TxID:       txID,
	}, commit)
	qt.Assert(t, err, qt.IsNil)
}

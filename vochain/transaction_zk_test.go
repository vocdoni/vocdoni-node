package vochain

import (
	"context"
	"crypto/sha256"
	"math/big"
	"testing"

	qt "github.com/frankban/quicktest"
	"go.vocdoni.io/dvote/crypto/zk"
	"go.vocdoni.io/dvote/crypto/zk/circuit"
	"go.vocdoni.io/dvote/crypto/zk/prover"
	"go.vocdoni.io/dvote/tree/arbo"
	"go.vocdoni.io/dvote/vochain/transaction/vochaintx"
	models "go.vocdoni.io/proto/build/go/models"
)

func TestVoteCheckZkSNARK(t *testing.T) {
	app := TestBaseApplication(t)

	devConfig := circuit.CircuitsConfigurations[circuit.DefaultCircuitConfigurationTag]
	devCircuit, err := circuit.LoadZkCircuit(context.Background(), devConfig)
	qt.Assert(t, err, qt.IsNil)
	app.TransactionHandler.ZkCircuit = devCircuit

	processId := sha256.Sum256(big.NewInt(10).Bytes())
	entityId := []byte("entityid-test")
	censusRoot, ok := new(big.Int).SetString("17197968086525850061240358121255330450603444008188511982219335207976133189652", 10)
	qt.Assert(t, ok, qt.IsTrue)

	process := &models.Process{
		ProcessId: processId[:],
		EntityId:  entityId,
		EnvelopeType: &models.EnvelopeType{
			Anonymous: true,
		},
		Mode:        &models.ProcessMode{},
		VoteOptions: &models.ProcessVoteOptions{MaxCount: 1},
		Status:      models.ProcessStatus_READY,
		CensusRoot:  make([]byte, 32), // emtpy hash
		StartBlock:  0,
		BlockCount:  3,
	}
	err = app.State.AddProcess(process)
	qt.Assert(t, err, qt.IsNil)
	_, err = app.State.Process(processId[:], false)
	qt.Assert(t, err, qt.IsNil)

	process, err = app.State.Process(processId[:], false)
	qt.Assert(t, err, qt.IsNil)
	process.CensusRoot = arbo.BigIntToBytes(32, censusRoot)
	err = app.State.UpdateProcess(process, processId[:])
	qt.Assert(t, err, qt.IsNil)

	proof := []byte(`{"pi_a":["774443786201396917676737444097389889590511023711300768252119115374128608173","16884500337762645607599312676528674391820483747223221119160761516733887918983","1"],"pi_b":[["6242482011661867277508207465279986486533560990664961220081784589705918818963","5591840421274989243111594800972512504610121656938058840872359604491816401529"],["15709419991794213763837907769024697741843216860852603003921566978888577578409","13508126497661229516962244202725789199142520969832106511823459513481822836233"],["1","0"]],"pi_c":["14595032166132371153246939769050851036569240268758849108947189870489925995447","3119575948683279753908374163253340452776599075135944655828304070339636194137","1"]}`)
	pubSignals := []byte(`["242108076058607163538102198631955675649","142667662805314151155817304537028292174","17197968086525850061240358121255330450603444008188511982219335207976133189652","4295509861249984880361571032347194270863089509149412623993065795982837479793","1","302689215824177652345211539748426020171","205062086841587857568430695525160476881"]`)

	parsedProof, err := prover.ParseProof(proof, pubSignals)
	qt.Assert(t, err, qt.IsNil)

	nullifierBI, ok := new(big.Int).SetString("4295509861249984880361571032347194270863089509149412623993065795982837479793", 10)
	qt.Assert(t, ok, qt.IsTrue)
	nullifier := arbo.BigIntToBytes(32, nullifierBI)

	weight := new(big.Int).SetInt64(1)
	testVote := &models.VoteEnvelope{
		ProcessId:   processId[:],
		VotePackage: weight.Bytes(),
		Nullifier:   nullifier,
	}
	protoProof, err := zk.ProverProofToProtobufZKProof(parsedProof,
		testVote.ProcessId, process.CensusRoot, testVote.Nullifier, weight)
	qt.Assert(t, err, qt.IsNil)

	voteValue := big.NewInt(1).Bytes()
	vtx := &models.VoteEnvelope{
		ProcessId:   processId[:],
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

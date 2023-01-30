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
	censusRoot, ok := new(big.Int).SetString("321719647862408781603871395864861393328840037430074042287229013237429257409", 10)
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

	// TODO: Are the following lines still required? If they're not, remove the
	// commented block.

	process, err = app.State.Process(processId[:], false)
	qt.Assert(t, err, qt.IsNil)
	process.CensusRoot = arbo.BigIntToBytes(32, censusRoot)
	err = app.State.UpdateProcess(process, processId[:])
	qt.Assert(t, err, qt.IsNil)

	proof := []byte(`{"pi_a":["3247888763029956753408141331484295716458651298769989171039186835284426193008","7134791323816506699544808397190836318717377727159157640966551877634320816664","1"],"pi_b":[["9394175208780986720100214593804580678786516383129548841097042424166175540452","17400372645051269557372739006794625206691862080338810842478782903811740354050"],["19401950441832289555286730802841123823293955194780448036326055475695126454089","3153584049183736205554410425971434362982898457236627918383924774864546765678"],["1","0"]],"pi_c":["11872494957803601998632082970823017389457750741724449048108681825655443871695","10799515217692663983208262019823580922195527099593295188710579916768622599794","1"]}`)
	pubSignals := []byte(`["242108076058607163538102198631955675649","142667662805314151155817304537028292174","321719647862408781603871395864861393328840037430074042287229013237429257409","4295509861249984880361571032347194270863089509149412623993065795982837479793","1","302689215824177652345211539748426020171","205062086841587857568430695525160476881"]`)

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

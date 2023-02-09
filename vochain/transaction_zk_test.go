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
	censusRoot, ok := new(big.Int).SetString("10834813840936125543440448909275549716704951620777238419802271487067882674959", 10)
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

	proof := []byte(`{"pi_a":["14791871643283486440377447815555767131057731183987782198658604782313093653011","12844621807393834231681376983771489950762585041054882864725171407412856782028","1"],"pi_b":[["12433231251593101940238668379952274322791006498748085481580628380849405363593","11548936652526706538409104096374532525655590906015435878015308716537028011631"],["6755251268726059455802689994436625939251794835945859580236341833003971121636","19536667068290149294292058701929683533831089434610529080094737473231595535697"],["1","0"]],"pi_c":["19226530815931438997395650829411738085200314639671771077952565395475165218413","11719209956821525621612289051039267855140722150160992341393205997946738622305","1"]}`)
	pubSignals := []byte(`["131872440970052243731409192994346921254","319566938749843934830526552542497782783","10834813840936125543440448909275549716704951620777238419802271487067882674959","12175702186054423772707071181312602181618718988777502075780940568723235915174","1","302689215824177652345211539748426020171","205062086841587857568430695525160476881"]`)

	parsedProof, err := prover.ParseProof(proof, pubSignals)
	qt.Assert(t, err, qt.IsNil)

	nullifierBI, ok := new(big.Int).SetString("12175702186054423772707071181312602181618718988777502075780940568723235915174", 10)
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

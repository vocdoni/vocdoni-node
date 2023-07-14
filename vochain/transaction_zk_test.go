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
	"go.vocdoni.io/proto/build/go/models"
)

func TestVoteCheckZkSNARK(t *testing.T) {
	app := TestBaseApplication(t)

	devConfig := circuit.CircuitsConfigurations[circuit.DefaultCircuitConfigurationTag]
	devCircuit, err := circuit.LoadZkCircuit(context.Background(), devConfig)
	qt.Assert(t, err, qt.IsNil)
	app.TransactionHandler.ZkCircuit = devCircuit

	processID := util.RandomBytes(types.ProcessIDsize)
	entityID := util.RandomBytes(types.EntityIDsize)
	censusRoot, ok := new(big.Int).SetString("9648097671895227980635547440138252408620932735432414005198889501421724580801", 10)
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
		CensusRoot:    make([]byte, 32), // empty hash
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

	proof := []byte(`{"pi_a":["19764894289759221387811379468772349267536030470546887274683548738940701730439","5792513619603071842043716023794313624978194310832451867630749326549635605387","1"],"pi_b":[["10683786454734433386142083724805646334979263158571029745229692341537825162103","12665844383580779207513795436185896170026211022851772333469566436498972927674"],["19067483500901411335434353413436798323129358673897323962868496141244822063056","7247388818765782029207077162875530889035852785205181235750626226047860944918"],["1","0"]],"pi_c":["3504509374402914604927510590914922258919507403260244487834027259506460364200","13697748365818674173853217442999405535746859977867736356001389760112197129205","1"]}`)
	pubSignals := []byte(`["102349190794087733531672488128345440122","159684336652054988991215779568000532806","7140136014127625663910579628905928757615653089657689013988444419691340878163","10","242108076058607163538102198631955675649","142667662805314151155817304537028292174","6301539214985517835540203464643022046871736911692210761263269199935609817666","9648097671895227980635547440138252408620932735432414005198889501421724580801"]`)

	parsedProof, err := prover.ParseProof(proof, pubSignals)
	qt.Assert(t, err, qt.IsNil)

	nullifierBI, ok := new(big.Int).SetString("7140136014127625663910579628905928757615653089657689013988444419691340878163", 10)
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

	_, err = app.TransactionHandler.VoteTxCheck(&vochaintx.Tx{
		Tx:         &models.Tx{Payload: &models.Tx_Vote{Vote: vtx}},
		Signature:  signature,
		SignedBody: txBytes,
		TxID:       txID,
	}, commit)
	qt.Assert(t, err, qt.IsNil)
}

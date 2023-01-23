package vochain

import (
	"context"
	"crypto/sha256"
	"math/big"
	"testing"

	qt "github.com/frankban/quicktest"
	"github.com/vocdoni/arbo"
	"go.vocdoni.io/dvote/crypto/zk"
	"go.vocdoni.io/dvote/crypto/zk/circuit"
	"go.vocdoni.io/dvote/crypto/zk/prover"
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

	proof := []byte(`{"pi_a":["2957546178009760850355659133375708838241747312069267323388700501932756388801","12625382869576748444607468191317832575538074530559965599524653975624207047366","1"],"pi_b":[["13773538541838073368502890527643985842772674306822459019965410608828594337298","11899229132740276662537864617463896098667033797486170150360573444081302507492"],["20727379697327122382792515736453901157256422564411007216318251099364487650832","11377671289575129003619036211390137347608265777315381485897664480644768018215"],["1","0"]],"pi_c":["6893085441277923177083572011027014550725412601104441949319558996891945033160","12694921975636511647104559140449350724579343649607032729614636681955579457745","1"]}`)
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

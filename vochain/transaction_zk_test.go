package vochain

import (
	"context"
	"encoding/binary"
	"encoding/hex"
	"math/big"
	"testing"

	qt "github.com/frankban/quicktest"
	"go.vocdoni.io/dvote/crypto/ethereum"
	"go.vocdoni.io/dvote/crypto/zk"
	"go.vocdoni.io/dvote/crypto/zk/circuit"
	"go.vocdoni.io/dvote/crypto/zk/prover"
	"go.vocdoni.io/dvote/tree/arbo"
	"go.vocdoni.io/dvote/types"
	"go.vocdoni.io/dvote/util"
	"go.vocdoni.io/dvote/vochain/state"
	"go.vocdoni.io/dvote/vochain/transaction/vochaintx"
	"go.vocdoni.io/proto/build/go/models"
)

func TestVoteCheckZkSNARK(t *testing.T) {
	c := qt.New(t)

	app := TestBaseApplication(t)

	devConfig := circuit.CircuitsConfigurations[circuit.DefaultCircuitConfigurationTag]
	devCircuit, err := circuit.LoadZkCircuit(context.Background(), devConfig)
	c.Assert(err, qt.IsNil)
	app.TransactionHandler.ZkCircuit = devCircuit

	testAccount := ethereum.NewSignKeys()
	c.Assert(testAccount.AddHexKey("f8170f3fed314494451b095f659ee47aac441375da49c61ec93f0b9e164ca696"), qt.IsNil)
	testSik, err := testAccount.Sik()
	c.Assert(err, qt.IsNil)
	c.Assert(app.State.SetAddressSIK(testAccount.Address(), testSik), qt.IsNil)

	electionId, _ := hex.DecodeString("7faeab7a7d250527d614e952ae8e446825bd1124c6def410844c7c383d1519a6")
	entityID := util.RandomBytes(types.EntityIDsize)
	sikRootBI, ok := new(big.Int).SetString("16627475007515029427962795751015560755634579788233571578304075542815574030920", 10)
	c.Assert(ok, qt.IsTrue)
	sikRoot := arbo.BigIntToBytes(32, sikRootBI)

	app.State.Tx.Lock()

	siksTree, err := app.State.Tx.DeepSubTree(state.StateTreeCfg(state.TreeSIK))
	c.Assert(err, qt.IsNil)
	key := make([]byte, 32)
	binary.LittleEndian.PutUint32(key, 100)
	siksTree.NoState().Set(key, sikRoot)
	app.State.Tx.Unlock()

	censusRoot, ok := new(big.Int).SetString("9596485198706849316703682514364659712964781362184000813587723079930027161527", 10)
	c.Assert(ok, qt.IsTrue)

	process := &models.Process{
		ProcessId: electionId,
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
	c.Assert(err, qt.IsNil)
	_, err = app.State.Process(electionId, false)
	c.Assert(err, qt.IsNil)

	process, err = app.State.Process(electionId, false)
	c.Assert(err, qt.IsNil)
	process.CensusRoot = arbo.BigIntToBytes(32, censusRoot)
	err = app.State.UpdateProcess(process, electionId)
	c.Assert(err, qt.IsNil)

	proof := []byte(`{"pi_a":["19181097946057346448645741273660182862186988979497299681742769924511239280354","12186678693115250470087763294006640132371203711130638961677665240765030237532","1"],"pi_b":[["8629478498612844512061794564914282679844272835063429792249607134504040565783","7268873945098951811736400542565283762818397275322004229743820906389373843966"],["1871417068893063084239950378680194074729328747480784375291372494988176563578","6339264415603411385790861052399685549842280320827123289701487975028429081463"],["1","0"]],"pi_c":["15881307625402166268112455343456768931162226780226649957401511451733261275521","12457622423730451773571910009687349725485804373050232457200820540993177041061","1"]}`)
	pubSignals := []byte(`["102349190794087733531672488128345440122","159684336652054988991215779568000532806","1286691942454454834079320777458400195645787350656514561000638703823007350879","10","242108076058607163538102198631955675649","142667662805314151155817304537028292174","16627475007515029427962795751015560755634579788233571578304075542815574030920","9596485198706849316703682514364659712964781362184000813587723079930027161527"]`)

	parsedProof, err := prover.ParseProof(proof, pubSignals)
	c.Assert(err, qt.IsNil)

	nullifierBI, ok := new(big.Int).SetString("1286691942454454834079320777458400195645787350656514561000638703823007350879", 10)
	c.Assert(ok, qt.IsTrue)
	nullifier := arbo.BigIntToBytes(32, nullifierBI)

	weight := new(big.Int).SetInt64(1)
	testVote := &models.VoteEnvelope{
		ProcessId:   electionId,
		VotePackage: weight.Bytes(),
		Nullifier:   nullifier,
	}
	protoProof, err := zk.ProverProofToProtobufZKProof(parsedProof,
		testVote.ProcessId, sikRoot, process.CensusRoot, testVote.Nullifier, weight)
	c.Assert(err, qt.IsNil)

	voteValue := big.NewInt(1).Bytes()
	vtx := &models.VoteEnvelope{
		ProcessId:   electionId,
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
	c.Assert(err, qt.IsNil)
}

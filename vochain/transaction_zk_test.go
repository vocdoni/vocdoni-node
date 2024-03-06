package vochain

import (
	"encoding/json"
	"math/big"
	"testing"
	"time"

	qt "github.com/frankban/quicktest"
	"go.vocdoni.io/dvote/crypto/ethereum"
	"go.vocdoni.io/dvote/crypto/zk"
	"go.vocdoni.io/dvote/crypto/zk/circuit"
	"go.vocdoni.io/dvote/crypto/zk/prover"
	"go.vocdoni.io/dvote/tree/arbo"
	"go.vocdoni.io/dvote/types"
	"go.vocdoni.io/dvote/util"
	"go.vocdoni.io/dvote/vochain/transaction/vochaintx"
	"go.vocdoni.io/proto/build/go/models"
	"google.golang.org/protobuf/proto"
)

func TestVoteCheckZkSNARK(t *testing.T) {
	c := qt.New(t)
	// create test app and load zk circuit
	app := TestBaseApplication(t)
	err := circuit.Init()
	c.Assert(err, qt.IsNil)
	// set initial inputs
	testWeight := big.NewInt(10)
	testVotePackage := util.RandomBytes(16)
	accounts, censusRoot, proofs := testCreateKeysAndBuildWeightedZkCensus(t, 10, testWeight)
	testAccount := accounts[0]
	testProof := proofs[0]
	testSiblings, err := zk.ProofToCircomSiblings(testProof)
	c.Assert(err, qt.IsNil)
	// add the test account sik to the test app
	testSIK, err := testAccount.AccountSIK(nil)
	c.Assert(err, qt.IsNil)
	c.Assert(app.State.SetAddressSIK(testAccount.Address(), testSIK), qt.IsNil)
	c.Assert(app.State.FetchValidSIKRoots(), qt.IsNil)
	// get siktree root and proof
	_, sikProof, err := app.State.SIKGenProof(testAccount.Address())
	c.Assert(err, qt.IsNil)
	sikSiblings, err := zk.ProofToCircomSiblings(sikProof)
	c.Assert(err, qt.IsNil)
	// mock an election ID and add it to the state process block registry with
	// its startBlock
	electionId := util.RandomBytes(types.ProcessIDsize)
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
		CensusOrigin:  models.CensusOrigin_OFF_CHAIN_TREE_WEIGHTED,
		Duration:      uint32((time.Second * 60).Seconds()),
		MaxCensusSize: 100,
	}
	err = app.State.AddProcess(process)
	c.Assert(err, qt.IsNil)
	_, err = app.State.Process(electionId, false)
	c.Assert(err, qt.IsNil)
	// advance the app block so the SIK tree is updated
	app.AdvanceTestBlock()

	// generate circuit inputs and the zk proof
	sikRoot, err := app.State.SIKRoot()
	c.Assert(err, qt.IsNil)
	inputs, err := circuit.GenerateCircuitInput(circuit.CircuitInputsParameters{
		Account:         testAccount,
		ElectionId:      electionId,
		CensusRoot:      process.CensusRoot,
		SIKRoot:         sikRoot,
		VotePackage:     testVotePackage,
		CensusSiblings:  testSiblings,
		SIKSiblings:     sikSiblings,
		AvailableWeight: testWeight,
	})
	c.Assert(err, qt.IsNil)
	encInputs, err := json.Marshal(inputs)
	c.Assert(err, qt.IsNil)
	proof, err := prover.Prove(circuit.Global().ProvingKey, circuit.Global().Wasm, encInputs)
	c.Assert(err, qt.IsNil)
	// generate nullifier
	nullifier, err := testAccount.AccountSIKnullifier(electionId, nil)
	c.Assert(err, qt.IsNil)
	// encode the zk proof and create the vote tx
	protoProof, err := zk.ProverProofToProtobufZKProof(proof, nil, nil, nil, nil, nil)
	c.Assert(err, qt.IsNil)
	vtx := &models.VoteEnvelope{
		ProcessId:   electionId,
		VotePackage: testVotePackage,
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

func testBuildSignedRegisterSIKTx(t *testing.T, account *ethereum.SignKeys, pid,
	proof, availableWeight []byte, chainID string,
) *vochaintx.Tx {
	c := qt.New(t)

	sik, err := account.AccountSIK(nil)
	c.Assert(err, qt.IsNil)

	tx := &models.Tx{
		Payload: &models.Tx_RegisterSIK{
			RegisterSIK: &models.RegisterSIKTx{
				SIK:        sik,
				ElectionId: pid,
				CensusProof: &models.Proof{
					Payload: &models.Proof_Arbo{
						Arbo: &models.ProofArbo{
							Type:            models.ProofArbo_POSEIDON,
							Siblings:        proof,
							AvailableWeight: availableWeight,
							KeyType:         models.ProofArbo_ADDRESS,
						},
					},
				},
			},
		},
	}

	txPayload, err := proto.Marshal(tx)
	c.Assert(err, qt.IsNil)
	stx := &models.SignedTx{Tx: txPayload}
	stx.Signature, err = account.SignVocdoniTx(txPayload, chainID)
	c.Assert(err, qt.IsNil)
	bstx, err := proto.Marshal(stx)
	c.Assert(err, qt.IsNil)

	resTx := new(vochaintx.Tx)
	c.Assert(resTx.Unmarshal(bstx, chainID), qt.IsNil)
	return resTx
}

func TestMaxCensusSizeRegisterSIKTx(t *testing.T) {
	c := qt.New(t)

	// create test app and load zk circuit
	app := TestBaseApplication(t)
	// initial accounts
	testWeight := big.NewInt(10)
	accounts, censusRoot, proofs := testCreateKeysAndBuildWeightedZkCensus(t, 10, testWeight)

	// create a process with max census size 10
	pid := util.RandomBytes(types.ProcessIDsize)
	process := &models.Process{
		ProcessId: pid,
		EntityId:  util.RandomBytes(types.EntityIDsize),
		EnvelopeType: &models.EnvelopeType{
			Anonymous: true,
		},
		Mode:          &models.ProcessMode{},
		VoteOptions:   &models.ProcessVoteOptions{MaxCount: 1},
		Status:        models.ProcessStatus_READY,
		CensusRoot:    censusRoot,
		StartBlock:    0,
		BlockCount:    3,
		MaxCensusSize: 10,
		CensusOrigin:  models.CensusOrigin_OFF_CHAIN_TREE_WEIGHTED,
	}
	c.Assert(app.State.AddProcess(process), qt.IsNil)
	app.AdvanceTestBlock()

	encWeight := arbo.BigIntToBytes(arbo.HashFunctionPoseidon.Len(), testWeight)
	tx := testBuildSignedRegisterSIKTx(t, accounts[0], pid, proofs[0], encWeight, app.chainID)
	_, _, _, _, err := app.TransactionHandler.RegisterSIKTxCheck(tx)
	c.Assert(err, qt.IsNil)

	for i := 0; i < 10; i++ {
		c.Assert(app.State.IncreaseRegisterSIKCounter(pid), qt.IsNil)
	}
	_, _, _, _, err = app.TransactionHandler.RegisterSIKTxCheck(tx)
	c.Assert(err, qt.IsNotNil)
	c.Assert(err, qt.ErrorMatches, "process MaxCensusSize reached")
}

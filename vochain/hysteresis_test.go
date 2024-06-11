package vochain

import (
	"encoding/json"
	"math/big"
	"sync"
	"testing"

	qt "github.com/frankban/quicktest"
	"go.vocdoni.io/dvote/crypto/ethereum"
	"go.vocdoni.io/dvote/crypto/zk"
	"go.vocdoni.io/dvote/crypto/zk/circuit"
	"go.vocdoni.io/dvote/crypto/zk/prover"
	"go.vocdoni.io/dvote/util"
	"go.vocdoni.io/dvote/vochain/processid"
	"go.vocdoni.io/dvote/vochain/state"
	"go.vocdoni.io/dvote/vochain/transaction/vochaintx"
	"go.vocdoni.io/proto/build/go/models"
)

func TestHysteresis(t *testing.T) {
	c := qt.New(t)

	// create test app and load zk circuit
	app := TestBaseApplication(t)
	err := circuit.Init()
	c.Assert(err, qt.IsNil)

	// initial accounts
	testWeight := big.NewInt(10)
	testVotePackage := util.RandomBytes(16)
	accounts, censusRoot, proofs := testCreateKeysAndBuildWeightedZkCensus(t, 3, testWeight)

	// add the test accounts siks to the test app
	accountOrganizer := ethereum.NewSignKeys()
	c.Assert(accountOrganizer.Generate(), qt.IsNil)
	c.Assert(app.State.CreateAccount(accountOrganizer.Address(), "", nil, 1000), qt.IsNil)
	for _, account := range accounts {
		testSIK, err := account.AccountSIK(nil)
		c.Assert(err, qt.IsNil)
		c.Assert(app.State.SetAddressSIK(account.Address(), testSIK), qt.IsNil)
		c.Assert(app.State.FetchValidSIKRoots(), qt.IsNil)
	}

	// create a process with max census size 10
	process := &models.Process{
		EntityId: accountOrganizer.Address().Bytes(),
		EnvelopeType: &models.EnvelopeType{
			Anonymous: true,
		},
		Mode:          &models.ProcessMode{},
		VoteOptions:   &models.ProcessVoteOptions{MaxCount: 1},
		Status:        models.ProcessStatus_READY,
		CensusRoot:    censusRoot,
		StartBlock:    1,
		BlockCount:    1000,
		MaxCensusSize: 10,
		CensusOrigin:  models.CensusOrigin_OFF_CHAIN_TREE_WEIGHTED,
	}
	procID, err := processid.BuildProcessID(process, app.State, processid.BuildNextProcessID)
	c.Assert(err, qt.IsNil)
	pid := procID.Marshal()
	process.ProcessId = pid
	c.Check(app.State.AddProcess(process), qt.IsNil)

	// check the process is added
	app.AdvanceTestBlock()
	_, err = app.State.Process(pid, true)
	c.Assert(err, qt.IsNil)

	// get siktree root, sik proofs and zkproof
	var zkProofs []*models.ProofZkSNARK
	sikTree, err := app.State.MainTreeView().DeepSubTree(state.StateTreeCfg(state.TreeSIK))
	c.Assert(err, qt.IsNil)
	sikRoot, err := sikTree.Root()
	c.Assert(err, qt.IsNil)
	wg := sync.WaitGroup{}
	mtx := sync.Mutex{}
	for i := range accounts {
		wg.Add(1)
		go func() {
			_, sikProof, err := sikTree.GenProof(accounts[i].Address().Bytes())
			c.Assert(err, qt.IsNil)

			sikSiblings, err := zk.ProofToCircomSiblings(sikProof)
			c.Assert(err, qt.IsNil)

			censusSiblings, err := zk.ProofToCircomSiblings(proofs[i])
			c.Assert(err, qt.IsNil)

			// get zkproof
			inputs, err := circuit.GenerateCircuitInput(circuit.CircuitInputsParameters{
				Account:         accounts[i],
				ElectionId:      pid,
				CensusRoot:      censusRoot,
				SIKRoot:         sikRoot,
				VotePackage:     testVotePackage,
				CensusSiblings:  censusSiblings,
				SIKSiblings:     sikSiblings,
				AvailableWeight: testWeight,
			})
			c.Assert(err, qt.IsNil)
			encInputs, err := json.Marshal(inputs)
			c.Assert(err, qt.IsNil)

			zkProof, err := prover.Prove(circuit.Global().ProvingKey, circuit.Global().Wasm, encInputs)
			c.Assert(err, qt.IsNil)

			protoZkProof, err := zk.ProverProofToProtobufZKProof(zkProof, nil, nil, nil, nil, nil)
			c.Assert(err, qt.IsNil)
			mtx.Lock()
			zkProofs = append(zkProofs, protoZkProof)
			mtx.Unlock()
			wg.Done()
		}()
	}
	wg.Wait()

	validVotes := len(accounts) / 2
	for i, account := range accounts[:validVotes] {
		nullifier, err := account.AccountSIKnullifier(pid, nil)
		c.Assert(err, qt.IsNil)

		vtx := &models.VoteEnvelope{
			ProcessId:   pid,
			VotePackage: testVotePackage,
			Nullifier:   nullifier,
			Proof: &models.Proof{
				Payload: &models.Proof_ZkSnark{
					ZkSnark: zkProofs[i],
				},
			},
		}
		_, err = app.TransactionHandler.VoteTxCheck(&vochaintx.Tx{
			Tx:         &models.Tx{Payload: &models.Tx_Vote{Vote: vtx}},
			Signature:  []byte{},
			SignedBody: []byte{},
			TxID:       [32]byte{},
		}, true)
		c.Assert(err, qt.IsNil)
	}

	mockNewSIK := func() {
		_account := ethereum.NewSignKeys()
		c.Assert(_account.Generate(), qt.IsNil)
		_sik, err := _account.AccountSIK(nil)
		c.Assert(err, qt.IsNil)
		c.Assert(app.State.SetAddressSIK(_account.Address(), _sik), qt.IsNil)
		c.Assert(app.State.FetchValidSIKRoots(), qt.IsNil)
	}

	for i := 0; i < state.SIKROOT_HYSTERESIS_BLOCKS; i++ {
		mockNewSIK()
		app.AdvanceTestBlock()
	}

	for i := 0; i < state.SIKROOT_HYSTERESIS_BLOCKS; i++ {
		mockNewSIK()
		app.AdvanceTestBlock()
	}

	for i, account := range accounts[validVotes:] {
		nullifier, err := account.AccountSIKnullifier(pid, nil)
		c.Assert(err, qt.IsNil)

		vtx := &models.VoteEnvelope{
			ProcessId:   pid,
			VotePackage: testVotePackage,
			Nullifier:   nullifier,
			Proof: &models.Proof{
				Payload: &models.Proof_ZkSnark{
					ZkSnark: zkProofs[i],
				},
			},
		}
		_, err = app.TransactionHandler.VoteTxCheck(&vochaintx.Tx{
			Tx:         &models.Tx{Payload: &models.Tx_Vote{Vote: vtx}},
			Signature:  []byte{},
			SignedBody: []byte{},
			TxID:       [32]byte{},
		}, true)
		c.Assert(err, qt.IsNotNil)
		c.Assert(err.Error(), qt.Contains, "expired sik")
	}
}

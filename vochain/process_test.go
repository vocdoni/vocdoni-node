package vochain

import (
	"context"
	"fmt"
	"strings"
	"testing"

	cometabcitypes "github.com/cometbft/cometbft/abci/types"
	qt "github.com/frankban/quicktest"
	"go.vocdoni.io/dvote/config"
	"go.vocdoni.io/dvote/crypto/ethereum"
	"go.vocdoni.io/dvote/types"
	"go.vocdoni.io/dvote/util"
	"go.vocdoni.io/dvote/vochain/genesis"
	vstate "go.vocdoni.io/dvote/vochain/state"
	"go.vocdoni.io/dvote/vochain/state/electionprice"
	"go.vocdoni.io/proto/build/go/models"
	"google.golang.org/protobuf/proto"
)

const ipfsUrlTest = "ipfs://123456789"

func TestNewProcessCheckTxDeliverTxCommitTransitions(t *testing.T) {
	app, accounts := createTestBaseApplicationAndAccounts(t, 10)

	// define process
	censusURI := ipfsUrlTest
	pid := util.RandomBytes(types.ProcessIDsize)
	process := &models.Process{
		ProcessId:     pid,
		StartBlock:    0,
		EnvelopeType:  &models.EnvelopeType{EncryptedVotes: false},
		Mode:          &models.ProcessMode{Interruptible: true},
		VoteOptions:   &models.ProcessVoteOptions{MaxCount: 16, MaxValue: 16},
		Status:        models.ProcessStatus_READY,
		EntityId:      accounts[0].Address().Bytes(),
		CensusRoot:    util.RandomBytes(32),
		CensusURI:     &censusURI,
		CensusOrigin:  models.CensusOrigin_OFF_CHAIN_TREE,
		BlockCount:    1024,
		MaxCensusSize: 100,
	}

	// create process with entityID (should work)
	qt.Assert(t, testCreateProcess(t, accounts[0], app, process), qt.IsNotNil)
	// all get accounts assume account is not nil
	entityAcc, err := app.State.GetAccount(accounts[0].Address(), false)
	qt.Assert(t, err, qt.IsNil)
	qt.Assert(t, entityAcc.Balance, qt.Equals, uint64(9990))
	qt.Assert(t, entityAcc.Nonce, qt.Equals, uint32(1))
	qt.Assert(t, entityAcc.ProcessIndex, qt.Equals, uint32(1))

	// create process with delegate (should work)
	qt.Assert(t, testCreateProcess(t, accounts[1], app, process), qt.IsNotNil)
	entityAcc, err = app.State.GetAccount(accounts[0].Address(), false)
	qt.Assert(t, err, qt.IsNil)
	qt.Assert(t, entityAcc.Balance, qt.Equals, uint64(9990))
	qt.Assert(t, entityAcc.Nonce, qt.Equals, uint32(1))
	qt.Assert(t, entityAcc.ProcessIndex, qt.Equals, uint32(2))
	delegateAcc, err := app.State.GetAccount(accounts[1].Address(), false)
	qt.Assert(t, err, qt.IsNil)
	qt.Assert(t, delegateAcc.Balance, qt.Equals, uint64(9990))
	qt.Assert(t, delegateAcc.Nonce, qt.Equals, uint32(1))
	qt.Assert(t, delegateAcc.ProcessIndex, qt.Equals, uint32(0))

	// create process with a non delegate to another entityID (should not work)
	qt.Assert(t, testCreateProcess(t, accounts[2], app, process), qt.IsNil)
	entityAcc, err = app.State.GetAccount(accounts[0].Address(), false)
	qt.Assert(t, err, qt.IsNil)
	qt.Assert(t, entityAcc.Balance, qt.Equals, uint64(9990))
	qt.Assert(t, entityAcc.Nonce, qt.Equals, uint32(1))
	qt.Assert(t, entityAcc.ProcessIndex, qt.Equals, uint32(2))
	randomAcc, err := app.State.GetAccount(accounts[2].Address(), false)
	qt.Assert(t, err, qt.IsNil)
	qt.Assert(t, randomAcc.Balance, qt.Equals, uint64(10000))
	qt.Assert(t, randomAcc.Nonce, qt.Equals, uint32(0))
	qt.Assert(t, randomAcc.ProcessIndex, qt.Equals, uint32(0))

	// create process with status PAUSED (should work)
	process.Status = models.ProcessStatus_PAUSED
	qt.Assert(t, testCreateProcess(t, accounts[1], app, process), qt.IsNotNil)
	// create process with status different than READY or PAUSED (should not work)
	process.Status = models.ProcessStatus_CANCELED
	qt.Assert(t, testCreateProcessWithErr(t, accounts[1], app, process),
		qt.ErrorMatches, ".*status must be READY or PAUSED.*")
	process.Status = models.ProcessStatus_PROCESS_UNKNOWN
	qt.Assert(t, testCreateProcessWithErr(t, accounts[1], app, process),
		qt.ErrorMatches, ".*status must be READY or PAUSED.*")
	process.Status = models.ProcessStatus_ENDED
	qt.Assert(t, testCreateProcessWithErr(t, accounts[1], app, process),
		qt.ErrorMatches, ".*status must be READY or PAUSED.*")
	process.Status = models.ProcessStatus_RESULTS
	qt.Assert(t, testCreateProcessWithErr(t, accounts[1], app, process),
		qt.ErrorMatches, ".*status must be READY or PAUSED.*")
}

func TestProcessSetStatusCheckTxDeliverTxCommitTransitions(t *testing.T) {
	app, keys := createTestBaseApplicationAndAccounts(t, 10)

	// add a process with status=READY and interruptible=true
	censusURI := ipfsUrlTest

	process := &models.Process{
		StartTime:     0,
		EnvelopeType:  &models.EnvelopeType{EncryptedVotes: false},
		Mode:          &models.ProcessMode{Interruptible: true},
		VoteOptions:   &models.ProcessVoteOptions{MaxCount: 16, MaxValue: 16},
		Status:        models.ProcessStatus_READY,
		EntityId:      keys[0].Address().Bytes(),
		CensusRoot:    util.RandomBytes(32),
		CensusURI:     &censusURI,
		CensusOrigin:  models.CensusOrigin_OFF_CHAIN_TREE,
		Duration:      1024,
		MaxCensusSize: 100,
	}
	pid := testCreateProcess(t, keys[0], app, process)
	app.AdvanceTestBlock()

	// Set it to PAUSE (should work)
	status := models.ProcessStatus_PAUSED
	qt.Assert(t, testSetProcessStatus(t, pid, keys[0], app, &status), qt.IsNil)
	app.AdvanceTestBlock()

	// Set it to READY (should work)
	status = models.ProcessStatus_READY
	qt.Assert(t, testSetProcessStatus(t, pid, keys[0], app, &status), qt.IsNil)
	app.AdvanceTestBlock()

	// Set it to PAUSED by delegate (should work)
	status = models.ProcessStatus_PAUSED
	qt.Assert(t, testSetProcessStatus(t, pid, keys[1], app, &status), qt.IsNil)
	app.AdvanceTestBlock()

	// Set it to READY by delegate (should work)
	status = models.ProcessStatus_READY
	qt.Assert(t, testSetProcessStatus(t, pid, keys[1], app, &status), qt.IsNil)
	app.AdvanceTestBlock()

	// Set it to ENDED (should work)
	status = models.ProcessStatus_ENDED
	qt.Assert(t, testSetProcessStatus(t, pid, keys[0], app, &status), qt.IsNil)
	app.AdvanceTestBlock()

	// Set it to RESULTS (should not work)
	status = models.ProcessStatus_RESULTS
	qt.Assert(t, testSetProcessStatus(t, pid, keys[0], app, &status), qt.IsNotNil)
	app.AdvanceTestBlock()

	// Set it to READY (should fail)
	status = models.ProcessStatus_READY
	qt.Assert(t, testSetProcessStatus(t, pid, keys[0], app, &status), qt.IsNotNil)
	app.AdvanceTestBlock()

	// Add a process with status=PAUSED and interruptible=true
	censusURI = ipfsUrlTest
	process = &models.Process{
		StartTime:     0,
		EnvelopeType:  &models.EnvelopeType{EncryptedVotes: false},
		Mode:          &models.ProcessMode{Interruptible: true},
		VoteOptions:   &models.ProcessVoteOptions{MaxCount: 16, MaxValue: 16},
		Status:        models.ProcessStatus_PAUSED,
		EntityId:      keys[0].Address().Bytes(),
		CensusRoot:    util.RandomBytes(32),
		CensusURI:     &censusURI,
		CensusOrigin:  models.CensusOrigin_OFF_CHAIN_TREE,
		Duration:      1024,
		MaxCensusSize: 100,
	}
	t.Logf("adding PAUSED process %x", process.ProcessId)
	pid = testCreateProcess(t, keys[0], app, process)
	app.AdvanceTestBlock()

	// Set it to READY (should work)
	status = models.ProcessStatus_READY
	qt.Assert(t, testSetProcessStatus(t, pid, keys[0], app, &status), qt.IsNil)
	app.AdvanceTestBlock()

	// Set it to PAUSED (should work)
	status = models.ProcessStatus_PAUSED
	qt.Assert(t, testSetProcessStatus(t, pid, keys[0], app, &status), qt.IsNil)
	app.AdvanceTestBlock()

	// Set it to CANCELED (should work)
	status = models.ProcessStatus_CANCELED
	qt.Assert(t, testSetProcessStatus(t, pid, keys[0], app, &status), qt.IsNil)
	app.AdvanceTestBlock()

	// Set it to READY (should fail)
	status = models.ProcessStatus_READY
	qt.Assert(t, testSetProcessStatus(t, pid, keys[0], app, &status), qt.IsNotNil)
	app.AdvanceTestBlock()

	// Add a process with status=PAUSE and interruptible=false
	censusURI = ipfsUrlTest
	process = &models.Process{
		StartTime:     0,
		EnvelopeType:  &models.EnvelopeType{EncryptedVotes: false},
		Mode:          &models.ProcessMode{Interruptible: false, AutoStart: false},
		VoteOptions:   &models.ProcessVoteOptions{MaxCount: 16, MaxValue: 16},
		Status:        models.ProcessStatus_PAUSED,
		EntityId:      keys[0].Address().Bytes(),
		CensusRoot:    util.RandomBytes(32),
		CensusURI:     &censusURI,
		CensusOrigin:  models.CensusOrigin_OFF_CHAIN_TREE,
		Duration:      1024,
		MaxCensusSize: 100,
	}
	t.Logf("adding PAUSED process %x", process.ProcessId)
	pid = testCreateProcess(t, keys[0], app, process)
	app.AdvanceTestBlock()

	// Set it to READY (should work)
	status = models.ProcessStatus_READY
	qt.Assert(t, testSetProcessStatus(t, pid, keys[0], app, &status), qt.IsNil)
	app.AdvanceTestBlock()

	// Set it to PAUSE (should fail)
	status = models.ProcessStatus_PAUSED
	qt.Assert(t, testSetProcessStatus(t, pid, keys[0], app, &status), qt.IsNotNil)
	app.AdvanceTestBlock()

	// Set it to ENDED (should fail)
	status = models.ProcessStatus_ENDED
	qt.Assert(t, testSetProcessStatus(t, pid, keys[0], app, &status), qt.IsNotNil)
}

func testSetProcessStatus(t *testing.T, pid []byte, txSender *ethereum.SignKeys,
	app *BaseApplication, status *models.ProcessStatus,
) error {
	return testSendSetProcessTx(t, app, txSender, &models.SetProcessTx{
		Txtype:    models.TxType_SET_PROCESS_STATUS,
		ProcessId: pid,
		Status:    status,
	})
}

// testSendSetProcessTx signs a SetProcessTx with the sender current nonce and
// runs it through CheckTx, DeliverTx and Commit.
func testSendSetProcessTx(t *testing.T, app *BaseApplication, txSender *ethereum.SignKeys,
	tx *models.SetProcessTx,
) error {
	var stx models.SignedTx
	var err error

	// assume account is not nil
	txSenderAcc, err := app.State.GetAccount(txSender.Address(), false)
	if err != nil {
		return fmt.Errorf("cannot get tx sender account %s with error %w", txSender.Address(), err)
	}
	tx.Nonce = txSenderAcc.Nonce
	stx.Tx, err = proto.Marshal(&models.Tx{Payload: &models.Tx_SetProcess{SetProcess: tx}})
	if err != nil {
		return fmt.Errorf("cannot mashal tx %w", err)
	}
	if stx.Signature, err = txSender.SignVocdoniTx(stx.Tx, app.chainID); err != nil {
		return fmt.Errorf("cannot sign tx %+v with error %w", tx, err)
	}

	_, err = testCheckTxDeliverTxCommit(t, app, &stx)
	return err
}

func TestProcessSetCensusCheckTxDeliverTxCommitTransitions(t *testing.T) {
	app, keys := createTestBaseApplicationAndAccounts(t, 10)

	// Add a process with status=READY and interruptible=true
	censusURI := ipfsUrlTest
	censusURI2 := "ipfs://987654321"
	pid := util.RandomBytes(types.ProcessIDsize)
	pid2 := util.RandomBytes(types.ProcessIDsize)
	pid3 := util.RandomBytes(types.ProcessIDsize)
	process := &models.Process{
		ProcessId:     pid,
		StartBlock:    0,
		EnvelopeType:  &models.EnvelopeType{EncryptedVotes: false},
		Mode:          &models.ProcessMode{Interruptible: true, DynamicCensus: true},
		Status:        models.ProcessStatus_READY,
		EntityId:      keys[0].Address().Bytes(),
		CensusRoot:    util.RandomBytes(32),
		CensusURI:     &censusURI,
		CensusOrigin:  models.CensusOrigin_OFF_CHAIN_TREE,
		BlockCount:    1024,
		MaxCensusSize: 100,
	}

	process2 := &models.Process{
		ProcessId:     pid2,
		StartBlock:    0,
		EnvelopeType:  &models.EnvelopeType{EncryptedVotes: false},
		Mode:          &models.ProcessMode{Interruptible: true},
		Status:        models.ProcessStatus_READY,
		EntityId:      keys[0].Address().Bytes(),
		CensusRoot:    util.RandomBytes(32),
		CensusURI:     &censusURI2,
		CensusOrigin:  models.CensusOrigin_OFF_CHAIN_TREE,
		BlockCount:    1024,
		MaxCensusSize: 100,
	}

	process3 := &models.Process{
		ProcessId:     pid3,
		StartBlock:    0,
		EnvelopeType:  &models.EnvelopeType{EncryptedVotes: false},
		Mode:          &models.ProcessMode{Interruptible: true, DynamicCensus: true},
		Status:        models.ProcessStatus_READY,
		EntityId:      keys[0].Address().Bytes(),
		CensusRoot:    util.RandomBytes(32),
		CensusURI:     &censusURI2,
		CensusOrigin:  models.CensusOrigin_OFF_CHAIN_TREE_WEIGHTED,
		BlockCount:    1024,
		MaxCensusSize: 100,
	}
	t.Logf("adding READY process %x", process.ProcessId)
	qt.Assert(t, app.State.AddProcess(process), qt.IsNil)
	t.Logf("adding READY process %x", process2.ProcessId)
	qt.Assert(t, app.State.AddProcess(process2), qt.IsNil)
	t.Logf("adding READY process %x", process3.ProcessId)
	qt.Assert(t, app.State.AddProcess(process3), qt.IsNil)

	// Set census  (should work)
	qt.Assert(t, testSetProcessCensus(t, pid, keys[0], app, []byte{1, 2, 3}, &censusURI2, 0), qt.IsNil)

	// Set census by delegate (should work)
	qt.Assert(t, testSetProcessCensus(t, pid, keys[1], app, []byte{3, 2, 1}, &censusURI2, 0), qt.IsNil)

	// Set census  (should not work)
	qt.Assert(t, testSetProcessCensus(t, pid2, keys[0], app, []byte{1, 2, 3}, &censusURI2, 0), qt.IsNotNil)

	// Set census  (should not work)
	qt.Assert(t, testSetProcessCensus(t, pid3, keys[2], app, []byte{1, 2, 3}, &censusURI2, 0), qt.IsNotNil)
}

func testSetProcessCensus(t *testing.T, pid []byte, txSender *ethereum.SignKeys,
	app *BaseApplication, censusRoot []byte, censusURI *string, censusSize uint64,
) error {
	var stx models.SignedTx
	var err error

	txSenderAcc, err := app.State.GetAccount(txSender.Address(), false)
	if err != nil {
		return fmt.Errorf("cannot get tx sender account %s with error %w", txSender.Address(), err)
	}

	tx := &models.SetProcessTx{
		Txtype:     models.TxType_SET_PROCESS_CENSUS,
		Nonce:      txSenderAcc.Nonce,
		ProcessId:  pid,
		CensusRoot: censusRoot,
		CensusURI:  censusURI,
		CensusSize: &censusSize,
	}
	if stx.Tx, err = proto.Marshal(&models.Tx{Payload: &models.Tx_SetProcess{SetProcess: tx}}); err != nil {
		return fmt.Errorf("cannot mashal tx %w", err)
	}
	if stx.Signature, err = txSender.SignVocdoniTx(stx.Tx, app.chainID); err != nil {
		return fmt.Errorf("cannot sign tx %+v with error %w", tx, err)
	}

	_, err = testCheckTxDeliverTxCommit(t, app, &stx)
	return err
}

func testSetProcessDuration(t *testing.T, pid []byte, txSender *ethereum.SignKeys,
	app *BaseApplication, duration uint32,
) error {
	return testSendSetProcessTx(t, app, txSender, &models.SetProcessTx{
		Txtype:    models.TxType_SET_PROCESS_DURATION,
		ProcessId: pid,
		Duration:  &duration,
	})
}

func TestCount(t *testing.T) {
	app := TestBaseApplication(t)
	count, err := app.State.CountProcesses(false)
	qt.Assert(t, err, qt.IsNil)
	qt.Assert(t, count, qt.Equals, uint64(0))

	count, err = app.State.CountProcesses(true)
	qt.Assert(t, err, qt.IsNil)
	qt.Assert(t, count, qt.Equals, uint64(0))
}

// creates a test vochain application and returns the following keys:
// [entity, delegate, random]
// the application will have the accounts of the keys already initialized, as well as
// the burn account and all tx costs set to txCostNumber
func createTestBaseApplicationAndAccounts(t *testing.T,
	txCostNumber uint64,
) (*BaseApplication, []*ethereum.SignKeys) {
	app := TestBaseApplication(t)
	keys := make([]*ethereum.SignKeys, 0)
	for i := 0; i < 4; i++ {
		key := &ethereum.SignKeys{}
		qt.Assert(t, key.Generate(), qt.IsNil)
		keys = append(keys, key)
	}
	// create burn account
	qt.Assert(t, app.State.SetAccount(vstate.BurnAddress, &vstate.Account{}), qt.IsNil)

	// create delegate
	qt.Assert(t, app.State.SetAccount(keys[1].Address(),
		&vstate.Account{Account: models.Account{Balance: 10000}},
	), qt.IsNil)

	// create entity account and add delegate
	delegates := make([][]byte, 1)
	delegates[0] = keys[1].Address().Bytes()
	qt.Assert(t, app.State.SetAccount(keys[0].Address(),
		&vstate.Account{Account: models.Account{
			Balance:       10000,
			DelegateAddrs: delegates,
		}},
	), qt.IsNil)

	// create random account
	qt.Assert(t, app.State.SetAccount(keys[2].Address(),
		&vstate.Account{Account: models.Account{Balance: 10000}},
	), qt.IsNil)

	// set tx costs
	for _, cost := range genesis.TxCostNameToTxTypeMap {
		qt.Assert(t, app.State.SetTxBaseCost(cost, txCostNumber), qt.IsNil)
	}
	app.State.ElectionPriceCalc.SetBasePrice(10)
	app.State.ElectionPriceCalc.SetCapacity(2000)
	testCommitState(t, app)

	return app, keys
}

// testCreateProcess creates a process with the given parameters via transaction.
// It returns the process ID if the transaction was successful, or nil otherwise.
func testCreateProcess(t *testing.T, txSender *ethereum.SignKeys, app *BaseApplication, process *models.Process) []byte {
	pid, err := testCreateProcessWithErrAndData(t, txSender, app, process)
	if err != nil {
		return nil
	}
	return pid
}

func testCreateProcessWithErr(t *testing.T, txSender *ethereum.SignKeys, app *BaseApplication, process *models.Process) error {
	_, err := testCreateProcessWithErrAndData(t, txSender, app, process)
	return err
}

func testCreateProcessWithErrAndData(t *testing.T, txSender *ethereum.SignKeys, app *BaseApplication, process *models.Process) ([]byte, error) {
	var stx models.SignedTx

	// assume account is not nil
	txSenderAcc, err := app.State.GetAccount(txSender.Address(), false)
	qt.Assert(t, err, qt.IsNil, qt.Commentf("cannot get tx sender account %s with error %w", txSender.Address(), err))

	// create tx
	tx := &models.NewProcessTx{
		Txtype:  models.TxType_NEW_PROCESS,
		Nonce:   txSenderAcc.Nonce,
		Process: process,
	}
	stx.Tx, err = proto.Marshal(&models.Tx{Payload: &models.Tx_NewProcess{NewProcess: tx}})
	qt.Assert(t, err, qt.IsNil, qt.Commentf("cannot mashal tx %w", err))

	stx.Signature, err = txSender.SignVocdoniTx(stx.Tx, app.chainID)
	qt.Assert(t, err, qt.IsNil, qt.Commentf("cannot sign tx %+v with error %w", tx, err))

	return testCheckTxDeliverTxCommit(t, app, &stx)
}

func testCheckTxDeliverTxCommit(t *testing.T, app *BaseApplication, stx *models.SignedTx) ([]byte, error) {
	cktx := new(cometabcitypes.CheckTxRequest)
	var err error
	// checkTx()
	cktx.Tx, err = proto.Marshal(stx)
	if err != nil {
		return nil, fmt.Errorf("mashaling failed: %w", err)
	}
	cktxresp, _ := app.CheckTx(context.Background(), cktx)
	if cktxresp.Code != 0 {
		return cktxresp.Data, fmt.Errorf("checkTx failed: %s", cktxresp.Data)
	}
	// deliverTx()
	tx, err := proto.Marshal(stx)
	if err != nil {
		return nil, fmt.Errorf("mashaling failed: %w", err)
	}
	detxresp := app.deliverTx(tx)
	if detxresp.Code != 0 {
		return detxresp.Data, fmt.Errorf("deliverTx failed: %s", detxresp.Data)
	}
	// commit()
	testCommitState(t, app)
	return detxresp.Data, nil
}

func TestGlobalMaxProcessSize(t *testing.T) {
	app, accounts := createTestBaseApplicationAndAccounts(t, 10)
	qt.Assert(t, app.State.SetMaxProcessSize(10), qt.IsNil)
	app.AdvanceTestBlock()

	// define process
	censusURI := ipfsUrlTest
	process := &models.Process{
		StartBlock:    1,
		EnvelopeType:  &models.EnvelopeType{EncryptedVotes: false},
		Mode:          &models.ProcessMode{Interruptible: true},
		VoteOptions:   &models.ProcessVoteOptions{MaxCount: 16, MaxValue: 16},
		Status:        models.ProcessStatus_READY,
		EntityId:      accounts[0].Address().Bytes(),
		CensusRoot:    util.RandomBytes(32),
		CensusURI:     &censusURI,
		CensusOrigin:  models.CensusOrigin_OFF_CHAIN_TREE,
		Duration:      60,
		MaxCensusSize: 5,
	}

	// create process with maxcensussize < 10 (should work)
	qt.Assert(t, testCreateProcessWithErr(t, accounts[0], app, process), qt.IsNil)

	// create process with maxcensussize > 10 (should fail)
	process.MaxCensusSize = 20
	qt.Assert(t, testCreateProcessWithErr(t, accounts[0], app, process), qt.IsNotNil)
}

func TestSetProcessCensusSize(t *testing.T) {
	app, accounts := createTestBaseApplicationAndAccounts(t, 10)

	// define process
	censusURI := ipfsUrlTest
	process := &models.Process{
		StartBlock:    1,
		EnvelopeType:  &models.EnvelopeType{EncryptedVotes: false},
		Mode:          &models.ProcessMode{Interruptible: true, DynamicCensus: false},
		VoteOptions:   &models.ProcessVoteOptions{MaxCount: 16, MaxValue: 16},
		Status:        models.ProcessStatus_READY,
		EntityId:      accounts[0].Address().Bytes(),
		CensusRoot:    util.RandomBytes(32),
		CensusURI:     &censusURI,
		CensusOrigin:  models.CensusOrigin_OFF_CHAIN_TREE,
		Duration:      60 * 60,
		MaxCensusSize: 2,
	}

	// create the process
	pid := testCreateProcess(t, accounts[0], app, process)
	app.AdvanceTestBlock()

	proc, err := app.State.Process(pid, true)
	qt.Assert(t, err, qt.IsNil)
	qt.Assert(t, proc.MaxCensusSize, qt.Equals, uint64(2))

	// Set census size with root (should failg since dynamicCensus=false)
	qt.Assert(t, testSetProcessCensus(t, pid, accounts[0], app, util.RandomBytes(32), nil, 5), qt.IsNotNil)
	app.AdvanceTestBlock()

	// Set census size (should work)
	qt.Assert(t, testSetProcessCensus(t, pid, accounts[0], app, nil, nil, 5), qt.IsNil)
	app.AdvanceTestBlock()

	proc, err = app.State.Process(pid, true)
	qt.Assert(t, err, qt.IsNil)
	qt.Assert(t, proc.MaxCensusSize, qt.Equals, uint64(5))

	// Set census size (without new root) (should work)
	qt.Assert(t, testSetProcessCensus(t, pid, accounts[0], app, nil, nil, 10), qt.IsNil)
	app.AdvanceTestBlock()

	proc, err = app.State.Process(pid, true)
	qt.Assert(t, err, qt.IsNil)
	qt.Assert(t, proc.MaxCensusSize, qt.Equals, uint64(10))
	qt.Assert(t, proc.CensusRoot, qt.IsNotNil)

	// Set census size (with same root and no URI) (should work)
	qt.Assert(t, testSetProcessCensus(t, pid, accounts[0], app, proc.CensusRoot, nil, 12), qt.IsNil)
	app.AdvanceTestBlock()

	proc, err = app.State.Process(pid, true)
	qt.Assert(t, err, qt.IsNil)
	qt.Assert(t, proc.MaxCensusSize, qt.Equals, uint64(12))
	qt.Assert(t, proc.CensusRoot, qt.IsNotNil)

	// Set census size (with same root and different URI) (should fail)
	uri := "ipfs://987654321"
	qt.Assert(t, testSetProcessCensus(t, pid, accounts[0], app, proc.CensusRoot, &uri, 13), qt.IsNotNil)
	app.AdvanceTestBlock()

	proc, err = app.State.Process(pid, true)
	qt.Assert(t, err, qt.IsNil)
	qt.Assert(t, proc.MaxCensusSize, qt.Equals, uint64(12))
	qt.Assert(t, proc.CensusRoot, qt.IsNotNil)

	// Set smaller census size (should fail)
	qt.Assert(t, testSetProcessCensus(t, pid, accounts[0], app, nil, nil, 5), qt.IsNotNil)
	app.AdvanceTestBlock()

	proc, err = app.State.Process(pid, true)
	qt.Assert(t, err, qt.IsNil)
	qt.Assert(t, proc.MaxCensusSize, qt.Equals, uint64(12))

	// Check cost is increased with larger census size (should work)
	account, err := app.State.GetAccount(accounts[0].Address(), true)
	qt.Assert(t, err, qt.IsNil)
	oldBalance := account.Balance

	qt.Assert(t, testSetProcessCensus(t, pid, accounts[0], app, nil, nil, 20000), qt.IsNil)

	account, err = app.State.GetAccount(accounts[0].Address(), true)
	qt.Assert(t, err, qt.IsNil)
	newBalance := account.Balance

	// check that newBalance is at least 100 tokens less than oldBalance
	qt.Assert(t, oldBalance-newBalance >= 100, qt.IsTrue)

	// define a new process, this time with dynamicCensus=true
	process = &models.Process{
		StartBlock:    0,
		EnvelopeType:  &models.EnvelopeType{EncryptedVotes: false},
		Mode:          &models.ProcessMode{Interruptible: true, DynamicCensus: true},
		VoteOptions:   &models.ProcessVoteOptions{MaxCount: 16, MaxValue: 16},
		Status:        models.ProcessStatus_READY,
		EntityId:      accounts[0].Address().Bytes(),
		CensusRoot:    util.RandomBytes(32),
		CensusURI:     &censusURI,
		CensusOrigin:  models.CensusOrigin_OFF_CHAIN_TREE,
		Duration:      60 * 60,
		MaxCensusSize: 2,
	}

	// create the process
	pid = testCreateProcess(t, accounts[0], app, process)
	app.AdvanceTestBlock()

	proc, err = app.State.Process(pid, true)
	qt.Assert(t, err, qt.IsNil)
	qt.Assert(t, proc.MaxCensusSize, qt.Equals, uint64(2))

	// Set census size with root (should work since dynamicCensus=true)
	qt.Assert(t, testSetProcessCensus(t, pid, accounts[0], app, util.RandomBytes(32), nil, 5), qt.IsNil)
	app.AdvanceTestBlock()

	// Set census size with root (should work since dynamicCensus=true)
	qt.Assert(t, testSetProcessCensus(t, pid, accounts[0], app, util.RandomBytes(32), &uri, 5), qt.IsNil)
	app.AdvanceTestBlock()
}

func TestSetProcessDuration(t *testing.T) {
	app, accounts := createTestBaseApplicationAndAccounts(t, 10)

	// define process
	censusURI := ipfsUrlTest
	process := &models.Process{
		StartBlock:    1,
		EnvelopeType:  &models.EnvelopeType{EncryptedVotes: false},
		Mode:          &models.ProcessMode{Interruptible: true, DynamicCensus: false},
		VoteOptions:   &models.ProcessVoteOptions{MaxCount: 16, MaxValue: 16},
		Status:        models.ProcessStatus_READY,
		EntityId:      accounts[0].Address().Bytes(),
		CensusRoot:    util.RandomBytes(32),
		CensusURI:     &censusURI,
		CensusOrigin:  models.CensusOrigin_OFF_CHAIN_TREE,
		Duration:      60,
		MaxCensusSize: 2,
	}

	// create the process
	pid := testCreateProcess(t, accounts[0], app, process)
	app.AdvanceTestBlock()

	proc, err := app.State.Process(pid, true)
	qt.Assert(t, err, qt.IsNil)
	qt.Assert(t, proc.Duration, qt.Equals, uint32(60))

	// Set lower duration (should workd)
	qt.Assert(t, testSetProcessDuration(t, pid, accounts[0], app, 50), qt.IsNil)
	app.AdvanceTestBlock()

	proc, err = app.State.Process(pid, true)
	qt.Assert(t, err, qt.IsNil)
	qt.Assert(t, proc.Duration, qt.Equals, uint32(50))

	// Set higher duration (should work)
	qt.Assert(t, testSetProcessDuration(t, pid, accounts[0], app, 80), qt.IsNil)
	app.AdvanceTestBlock()

	proc, err = app.State.Process(pid, true)
	qt.Assert(t, err, qt.IsNil)
	qt.Assert(t, proc.Duration, qt.Equals, uint32(80))

	// Check cost is increased with larger duration (should work)
	account, err := app.State.GetAccount(accounts[0].Address(), true)
	qt.Assert(t, err, qt.IsNil)
	oldBalance := account.Balance

	qt.Assert(t, testSetProcessDuration(t, pid, accounts[0], app, 2000000), qt.IsNil)

	account, err = app.State.GetAccount(accounts[0].Address(), true)
	qt.Assert(t, err, qt.IsNil)
	newBalance := account.Balance

	// check that newBalance is at least 30 tokens less than oldBalance
	qt.Assert(t, oldBalance-newBalance >= 30, qt.IsTrue)
}

// TestNewProcessSoftDeprecatesLegacyCSPOrigin covers the LTS/1.3 soft deprecation
// of CensusOrigin_OFF_CHAIN_CA: process creation still succeeds so integrators
// currently on the SDK's legacy origin keep working, but the CheckTx response
// carries a warning in its Log field that surfaces to API clients (and via
// log.Warnw to validator operators). The follow-up LTS/1.4 change will flip
// this to a hard rejection.
func TestNewProcessSoftDeprecatesLegacyCSPOrigin(t *testing.T) {
	app, accounts := createTestBaseApplicationAndAccounts(t, 2)

	buildProcess := func(origin models.CensusOrigin) *models.Process {
		return &models.Process{
			EnvelopeType:  &models.EnvelopeType{EncryptedVotes: false},
			Mode:          &models.ProcessMode{Interruptible: true},
			VoteOptions:   &models.ProcessVoteOptions{MaxCount: 1, MaxValue: 1},
			Status:        models.ProcessStatus_READY,
			EntityId:      accounts[0].Address().Bytes(),
			CensusRoot:    util.RandomBytes(33),
			CensusOrigin:  origin,
			Duration:      10240,
			MaxCensusSize: 10,
		}
	}

	// legacy OFF_CHAIN_CA: accepted, warning surfaced in CheckTx Log
	log := checkTxLog(t, app, accounts[0], buildProcess(models.CensusOrigin_OFF_CHAIN_CA))
	qt.Assert(t, log, qt.Contains, "OFF_CHAIN_CA is deprecated")

	// OFF_CHAIN_CA_V2: accepted, no warning
	log = checkTxLog(t, app, accounts[1], buildProcess(models.CensusOrigin_OFF_CHAIN_CA_V2))
	qt.Assert(t, log, qt.Equals, "")
}

// checkTxLog signs and runs CheckTx for a NewProcess and returns the response
// Log field (the API surfaces it as the ElectionCreate.Warning field). The test
// asserts a Code=0 acceptance; use it only where the transaction is expected to
// pass its checks.
func checkTxLog(t *testing.T, app *BaseApplication, txSender *ethereum.SignKeys, process *models.Process) string {
	t.Helper()
	txSenderAcc, err := app.State.GetAccount(txSender.Address(), false)
	qt.Assert(t, err, qt.IsNil)
	tx := &models.NewProcessTx{
		Txtype:  models.TxType_NEW_PROCESS,
		Nonce:   txSenderAcc.Nonce,
		Process: process,
	}
	var stx models.SignedTx
	stx.Tx, err = proto.Marshal(&models.Tx{Payload: &models.Tx_NewProcess{NewProcess: tx}})
	qt.Assert(t, err, qt.IsNil)
	stx.Signature, err = txSender.SignVocdoniTx(stx.Tx, app.chainID)
	qt.Assert(t, err, qt.IsNil)
	rawTx, err := proto.Marshal(&stx)
	qt.Assert(t, err, qt.IsNil)
	resp, err := app.CheckTx(context.Background(), &cometabcitypes.CheckTxRequest{Tx: rawTx})
	qt.Assert(t, err, qt.IsNil)
	qt.Assert(t, resp.Code, qt.Equals, uint32(0), qt.Commentf("CheckTx failed: %s", resp.Data))
	return resp.Log
}

func testSetProcessMetadata(t *testing.T, pid []byte, txSender *ethereum.SignKeys,
	app *BaseApplication, uri *string, hash []byte,
) error {
	return testSendSetProcessTx(t, app, txSender, &models.SetProcessTx{
		Txtype:       models.TxType_SET_PROCESS_METADATA,
		ProcessId:    pid,
		Metadata:     uri,
		MetadataHash: hash,
	})
}

func testMetadataProcess(entityID []byte) *models.Process {
	censusURI := ipfsUrlTest
	metadataURI := "https://example.com/metadata/1.json"
	return &models.Process{
		EnvelopeType:  &models.EnvelopeType{EncryptedVotes: false},
		Mode:          &models.ProcessMode{Interruptible: true},
		VoteOptions:   &models.ProcessVoteOptions{MaxCount: 16, MaxValue: 16},
		Status:        models.ProcessStatus_READY,
		EntityId:      entityID,
		CensusRoot:    util.RandomBytes(32),
		CensusURI:     &censusURI,
		CensusOrigin:  models.CensusOrigin_OFF_CHAIN_TREE,
		Duration:      1024,
		MaxCensusSize: 100,
		Metadata:      &metadataURI,
		MetadataHash:  util.RandomBytes(32),
	}
}

func TestProcessSetMetadata(t *testing.T) {
	app, keys := createTestBaseApplicationAndAccounts(t, 10)

	process := testMetadataProcess(keys[0].Address().Bytes())
	initialHash := process.MetadataHash
	pid := testCreateProcess(t, keys[0], app, process)
	qt.Assert(t, pid, qt.IsNotNil)
	app.AdvanceTestBlock()

	proc, err := app.State.Process(pid, true)
	qt.Assert(t, err, qt.IsNil)
	qt.Assert(t, proc.MetadataHash, qt.DeepEquals, initialHash)

	// owner updates the content behind the same URI (should work)
	uri := process.GetMetadata()
	hash := util.RandomBytes(32)
	qt.Assert(t, testSetProcessMetadata(t, pid, keys[0], app, &uri, hash), qt.IsNil)
	app.AdvanceTestBlock()
	proc, err = app.State.Process(pid, true)
	qt.Assert(t, err, qt.IsNil)
	qt.Assert(t, proc.GetMetadata(), qt.Equals, uri)
	qt.Assert(t, proc.MetadataHash, qt.DeepEquals, hash)

	// same URI and hash (should fail)
	qt.Assert(t, testSetProcessMetadata(t, pid, keys[0], app, &uri, hash),
		qt.ErrorMatches, ".*same URI and hash.*")

	// delegate sets a new URI (should work)
	uri2 := "ipfs://bafkreigh2akiscaildcqabsyg3dfr6chu3fgpregiymsck7e7aqa4s52zy"
	hash2 := util.RandomBytes(32)
	qt.Assert(t, testSetProcessMetadata(t, pid, keys[1], app, &uri2, hash2), qt.IsNil)
	app.AdvanceTestBlock()
	proc, err = app.State.Process(pid, true)
	qt.Assert(t, err, qt.IsNil)
	qt.Assert(t, proc.GetMetadata(), qt.Equals, uri2)
	qt.Assert(t, proc.MetadataHash, qt.DeepEquals, hash2)

	// non delegate (should fail)
	qt.Assert(t, testSetProcessMetadata(t, pid, keys[2], app, &uri, util.RandomBytes(32)),
		qt.ErrorMatches, ".*unauthorized.*")

	// invalid URI or hash (should fail)
	empty := ""
	longURI := "https://example.com/" + strings.Repeat("a", types.MaxURLLength)
	for _, tc := range []struct {
		name    string
		uri     *string
		hash    []byte
		wantErr string
	}{
		{"missing URI", nil, util.RandomBytes(32), ".*metadata URI must be.*"},
		{"empty URI", &empty, util.RandomBytes(32), ".*metadata URI must be.*"},
		{"too long URI", &longURI, util.RandomBytes(32), ".*metadata URI must be.*"},
		{"missing hash", &uri, nil, ".*metadata hash must be.*"},
		{"too long hash", &uri, util.RandomBytes(types.MaxMetadataHashSize + 1), ".*metadata hash must be.*"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			qt.Assert(t, testSetProcessMetadata(t, pid, keys[0], app, tc.uri, tc.hash), qt.ErrorMatches, tc.wantErr)
		})
	}

	// PAUSED (should work)
	status := models.ProcessStatus_PAUSED
	qt.Assert(t, testSetProcessStatus(t, pid, keys[0], app, &status), qt.IsNil)
	app.AdvanceTestBlock()
	qt.Assert(t, testSetProcessMetadata(t, pid, keys[0], app, &uri, util.RandomBytes(32)), qt.IsNil)
	app.AdvanceTestBlock()

	// ENDED (should fail)
	status = models.ProcessStatus_ENDED
	qt.Assert(t, testSetProcessStatus(t, pid, keys[0], app, &status), qt.IsNil)
	app.AdvanceTestBlock()
	qt.Assert(t, testSetProcessMetadata(t, pid, keys[0], app, &uri, util.RandomBytes(32)),
		qt.ErrorMatches, ".*invalid status.*")

	// CANCELED (should fail)
	pid2 := testCreateProcess(t, keys[0], app, testMetadataProcess(keys[0].Address().Bytes()))
	qt.Assert(t, pid2, qt.IsNotNil)
	app.AdvanceTestBlock()
	status = models.ProcessStatus_CANCELED
	qt.Assert(t, testSetProcessStatus(t, pid2, keys[0], app, &status), qt.IsNil)
	app.AdvanceTestBlock()
	qt.Assert(t, testSetProcessMetadata(t, pid2, keys[0], app, &uri, util.RandomBytes(32)),
		qt.ErrorMatches, ".*invalid status.*")
}

func TestNewProcessMetadataLimits(t *testing.T) {
	app, keys := createTestBaseApplicationAndAccounts(t, 10)

	// too long hash (should fail)
	process := testMetadataProcess(keys[0].Address().Bytes())
	process.MetadataHash = util.RandomBytes(types.MaxMetadataHashSize + 1)
	qt.Assert(t, testCreateProcessWithErr(t, keys[0], app, process),
		qt.ErrorMatches, ".*metadata hash too long.*")

	// any hash length up to the limit is accepted, the chain does not interpret it
	process = testMetadataProcess(keys[0].Address().Bytes())
	process.MetadataHash = util.RandomBytes(5)
	qt.Assert(t, testCreateProcess(t, keys[0], app, process), qt.IsNotNil)

	// too long URI (should fail)
	process = testMetadataProcess(keys[0].Address().Bytes())
	longURI := "https://example.com/" + strings.Repeat("a", types.MaxURLLength)
	process.Metadata = &longURI
	qt.Assert(t, testCreateProcessWithErr(t, keys[0], app, process),
		qt.ErrorMatches, ".*metadata URI too long.*")

	// no hash (should work, the hash is optional on creation)
	process = testMetadataProcess(keys[0].Address().Bytes())
	process.MetadataHash = nil
	qt.Assert(t, testCreateProcess(t, keys[0], app, process), qt.IsNotNil)

	// explicitly empty hash (should work, same as no hash)
	process = testMetadataProcess(keys[0].Address().Bytes())
	process.MetadataHash = []byte{}
	qt.Assert(t, testCreateProcess(t, keys[0], app, process), qt.IsNotNil)
}

// TestProcessMetadataForkLTS13 checks that on vocdoni/LTS/1.3 the metadata
// fork stays inactive before its config.Forks MetadataFork height, behaving as
// binaries without them, and activate at that height.
func TestProcessMetadataForkLTS13(t *testing.T) {
	app, keys := createTestBaseApplicationAndAccounts(t, 10)
	app.SetChainID("vocdoni/LTS/1.3")
	forkHeight := config.ForksForChainID("vocdoni/LTS/1.3").MetadataFork
	app.State.SetHeight(forkHeight - 1)

	// before activation: NewProcessTx does not check the metadata URI nor hash
	longURI := "https://example.com/" + strings.Repeat("a", types.MaxURLLength)
	process := testMetadataProcess(keys[0].Address().Bytes())
	process.Metadata = &longURI
	process.MetadataHash = util.RandomBytes(types.MaxMetadataHashSize + 1)
	pid := testCreateProcess(t, keys[0], app, process)
	qt.Assert(t, pid, qt.IsNotNil)

	// before activation: SET_PROCESS_METADATA is an unknown tx type
	uri := "https://example.com/metadata/2.json"
	qt.Assert(t, testSetProcessMetadata(t, pid, keys[0], app, &uri, util.RandomBytes(32)),
		qt.ErrorMatches, ".*unknown setProcess tx type.*")

	// at activation: the fork rules apply
	app.State.SetHeight(forkHeight)
	qt.Assert(t, testCreateProcessWithErr(t, keys[0], app, process),
		qt.ErrorMatches, ".*metadata URI too long.*")
	qt.Assert(t, testSetProcessMetadata(t, pid, keys[0], app, &uri, util.RandomBytes(32)), qt.IsNil)
}

// testMetadataOnlyProcess returns a metadata-only process: no voteOptions,
// envelope type nor census, just the metadata shared by its children.
func testMetadataOnlyProcess(entityID []byte) *models.Process {
	metadataURI := "https://example.com/metadata/parent.json"
	return &models.Process{
		Mode:         &models.ProcessMode{Interruptible: true},
		Status:       models.ProcessStatus_READY,
		EntityId:     entityID,
		Duration:     1024,
		Metadata:     &metadataURI,
		MetadataHash: util.RandomBytes(32),
	}
}

func TestNewMetadataOnlyProcess(t *testing.T) {
	app, keys := createTestBaseApplicationAndAccounts(t, 10)

	// a metadata-only process is accepted and costs as an election without census
	account, err := app.State.GetAccount(keys[0].Address(), false)
	qt.Assert(t, err, qt.IsNil)
	oldBalance := account.Balance
	process := testMetadataOnlyProcess(keys[0].Address().Bytes())
	pid := testCreateProcess(t, keys[0], app, process)
	qt.Assert(t, pid, qt.IsNotNil)
	account, err = app.State.GetAccount(keys[0].Address(), false)
	qt.Assert(t, err, qt.IsNil)
	wantCost := app.State.ElectionPriceCalc.Price(&electionprice.ElectionParameters{ElectionDurationSeconds: 1024})
	qt.Assert(t, oldBalance-account.Balance, qt.Equals, wantCost)

	proc, err := app.State.Process(pid, false)
	qt.Assert(t, err, qt.IsNil)
	qt.Assert(t, vstate.IsMetadataOnlyProcess(proc), qt.IsTrue)
	qt.Assert(t, proc.EnvelopeType, qt.IsNil)
	qt.Assert(t, proc.MetadataHash, qt.DeepEquals, process.MetadataHash)
	// neither a census origin nor an envelope type are encoded in its id
	qt.Assert(t, pid[26:28], qt.DeepEquals, []byte{0, 0})

	// partial combinations are rejected
	censusURI := ipfsUrlTest
	empty := ""
	for _, tc := range []struct {
		name    string
		modify  func(p *models.Process)
		wantErr string
	}{
		{"envelope type", func(p *models.Process) { p.EnvelopeType = &models.EnvelopeType{} }, ".*cannot have an envelopeType.*"},
		{"census origin", func(p *models.Process) { p.CensusOrigin = models.CensusOrigin_OFF_CHAIN_TREE }, ".*cannot have a census.*"},
		{"census root", func(p *models.Process) { p.CensusRoot = util.RandomBytes(32) }, ".*cannot have a census.*"},
		{"census URI", func(p *models.Process) { p.CensusURI = &censusURI }, ".*cannot have a census.*"},
		{"max census size", func(p *models.Process) { p.MaxCensusSize = 10 }, ".*cannot have a census.*"},
		{"no process mode", func(p *models.Process) { p.Mode = nil }, ".*missing required fields.*"},
		{"no metadata URI", func(p *models.Process) { p.Metadata = nil }, ".*requires a metadata URI.*"},
		{"empty metadata URI", func(p *models.Process) { p.Metadata = &empty }, ".*requires a metadata URI.*"},
		{"no metadata hash", func(p *models.Process) { p.MetadataHash = nil }, ".*requires a metadata hash.*"},
		{"ended status", func(p *models.Process) { p.Status = models.ProcessStatus_ENDED }, ".*status must be READY or PAUSED.*"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			p := testMetadataOnlyProcess(keys[0].Address().Bytes())
			tc.modify(p)
			qt.Assert(t, testCreateProcessWithErr(t, keys[0], app, p), qt.ErrorMatches, tc.wantErr)
		})
	}

	// a PAUSED metadata-only process is accepted
	process = testMetadataOnlyProcess(keys[0].Address().Bytes())
	process.Status = models.ProcessStatus_PAUSED
	qt.Assert(t, testCreateProcess(t, keys[0], app, process), qt.IsNotNil)
}

func TestMetadataOnlyProcessSetProcess(t *testing.T) {
	app, keys := createTestBaseApplicationAndAccounts(t, 10)
	pid := testCreateProcess(t, keys[0], app, testMetadataOnlyProcess(keys[0].Address().Bytes()))
	qt.Assert(t, pid, qt.IsNotNil)
	app.AdvanceTestBlock()

	// metadata and duration can be updated
	uri := "https://example.com/metadata/parent-2.json"
	hash := util.RandomBytes(32)
	qt.Assert(t, testSetProcessMetadata(t, pid, keys[0], app, &uri, hash), qt.IsNil)
	app.AdvanceTestBlock()
	proc, err := app.State.Process(pid, true)
	qt.Assert(t, err, qt.IsNil)
	qt.Assert(t, proc.GetMetadata(), qt.Equals, uri)
	qt.Assert(t, proc.MetadataHash, qt.DeepEquals, hash)
	qt.Assert(t, testSetProcessDuration(t, pid, keys[0], app, 2048), qt.IsNil)
	app.AdvanceTestBlock()

	// it has no census to update
	censusURI := ipfsUrlTest
	qt.Assert(t, testSetProcessCensus(t, pid, keys[0], app, util.RandomBytes(32), &censusURI, 0),
		qt.ErrorMatches, ".*census of a metadata-only process.*")

	// it can be paused, resumed and ended, but never reaches RESULTS
	for _, status := range []models.ProcessStatus{models.ProcessStatus_PAUSED, models.ProcessStatus_READY} {
		qt.Assert(t, testSetProcessStatus(t, pid, keys[0], app, &status), qt.IsNil)
		app.AdvanceTestBlock()
	}
	status := models.ProcessStatus_RESULTS
	qt.Assert(t, testSetProcessStatus(t, pid, keys[0], app, &status), qt.ErrorMatches, ".*RESULTS.*")
	status = models.ProcessStatus_ENDED
	qt.Assert(t, testSetProcessStatus(t, pid, keys[0], app, &status), qt.IsNil)
	for range 4 {
		app.AdvanceTestBlock()
	}
	proc, err = app.State.Process(pid, true)
	qt.Assert(t, err, qt.IsNil)
	qt.Assert(t, proc.Status, qt.Equals, models.ProcessStatus_ENDED)
	qt.Assert(t, proc.Results, qt.IsNil)

	// once ended, its metadata cannot change anymore
	qt.Assert(t, testSetProcessMetadata(t, pid, keys[0], app, &uri, util.RandomBytes(32)),
		qt.ErrorMatches, ".*invalid status.*")

	// a canceled metadata-only process
	pid = testCreateProcess(t, keys[0], app, testMetadataOnlyProcess(keys[0].Address().Bytes()))
	qt.Assert(t, pid, qt.IsNotNil)
	app.AdvanceTestBlock()
	status = models.ProcessStatus_CANCELED
	qt.Assert(t, testSetProcessStatus(t, pid, keys[0], app, &status), qt.IsNil)
	app.AdvanceTestBlock()
}

// TestMetadataOnlyProcessEndsWithoutResults checks that a metadata-only process
// reaching its end time is ENDED by the IST, which computes no results for it.
func TestMetadataOnlyProcessEndsWithoutResults(t *testing.T) {
	app, keys := createTestBaseApplicationAndAccounts(t, 10)
	process := testMetadataOnlyProcess(keys[0].Address().Bytes())
	process.Duration = 3
	pid := testCreateProcess(t, keys[0], app, process)
	qt.Assert(t, pid, qt.IsNotNil)
	for range 8 {
		app.AdvanceTestBlock()
	}
	proc, err := app.State.Process(pid, true)
	qt.Assert(t, err, qt.IsNil)
	qt.Assert(t, proc.Status, qt.Equals, models.ProcessStatus_ENDED)
	qt.Assert(t, proc.Results, qt.IsNil)
}

func TestNewProcessParentLink(t *testing.T) {
	app, keys := createTestBaseApplicationAndAccounts(t, 10)
	parentID := testCreateProcess(t, keys[0], app, testMetadataOnlyProcess(keys[0].Address().Bytes()))
	qt.Assert(t, parentID, qt.IsNotNil)
	app.AdvanceTestBlock()

	// an election linked to a metadata-only parent of the same organization is accepted
	child := testMetadataProcess(keys[0].Address().Bytes())
	child.ParentProcessId = parentID
	childID := testCreateProcess(t, keys[0], app, child)
	qt.Assert(t, childID, qt.IsNotNil)
	proc, err := app.State.Process(childID, false)
	qt.Assert(t, err, qt.IsNil)
	qt.Assert(t, proc.ParentProcessId, qt.DeepEquals, parentID)

	// so is one created by a delegate of the organization
	child = testMetadataProcess(keys[0].Address().Bytes())
	child.ParentProcessId = parentID
	qt.Assert(t, testCreateProcess(t, keys[1], app, child), qt.IsNotNil)

	// and a metadata-only one, which can then not be a parent itself
	metadataOnlyChild := testMetadataOnlyProcess(keys[0].Address().Bytes())
	metadataOnlyChild.ParentProcessId = parentID
	metadataOnlyChildID := testCreateProcess(t, keys[0], app, metadataOnlyChild)
	qt.Assert(t, metadataOnlyChildID, qt.IsNotNil)
	app.AdvanceTestBlock()

	// a parent of another organization
	otherParentID := testCreateProcess(t, keys[2], app, testMetadataOnlyProcess(keys[2].Address().Bytes()))
	qt.Assert(t, otherParentID, qt.IsNotNil)
	app.AdvanceTestBlock()

	for _, tc := range []struct {
		name     string
		parentID []byte
		wantErr  string
	}{
		{"child of a child", metadataOnlyChildID, ".*has a parent itself.*"},
		{"parent with votes", childID, ".*is not metadata-only.*"},
		{"other organization", otherParentID, ".*belongs to another organization.*"},
		{"not found", util.RandomBytes(types.ProcessIDsize), ".*cannot get parent process.*"},
		{"short id", parentID[:20], ".*invalid parentProcessId size.*"},
		{"long id", append(append([]byte{}, parentID...), 0x01), ".*invalid parentProcessId size.*"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			p := testMetadataProcess(keys[0].Address().Bytes())
			p.ParentProcessId = tc.parentID
			qt.Assert(t, testCreateProcessWithErr(t, keys[0], app, p), qt.ErrorMatches, tc.wantErr)
		})
	}
}

// TestProcessParentForkLTS13 checks that on vocdoni/LTS/1.3, where the parent
// fork is not scheduled, NewProcessTx behaves as binaries without it.
func TestProcessParentForkLTS13(t *testing.T) {
	qt.Assert(t, config.ForksForChainID("vocdoni/LTS/1.3").ParentFork, qt.Equals, uint32(config.NotScheduled))
	qt.Assert(t, config.ForksForChainID("vocdoni/DEV/36").ParentFork, qt.Equals, uint32(config.NotScheduled))

	app, keys := createTestBaseApplicationAndAccounts(t, 10)
	app.SetChainID("vocdoni/LTS/1.3")
	app.State.SetHeight(config.ForksForChainID("vocdoni/LTS/1.3").MetadataFork)

	// a process without voteOptions is rejected as before
	qt.Assert(t, testCreateProcessWithErr(t, keys[0], app, testMetadataOnlyProcess(keys[0].Address().Bytes())),
		qt.ErrorMatches, ".*missing required fields \\(voteOptions, envelopeType or processMode\\).*")

	// parentProcessId is not checked
	process := testMetadataProcess(keys[0].Address().Bytes())
	process.ParentProcessId = util.RandomBytes(5)
	qt.Assert(t, testCreateProcess(t, keys[0], app, process), qt.IsNotNil)
}

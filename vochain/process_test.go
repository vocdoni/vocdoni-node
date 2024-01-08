package vochain

import (
	"context"
	"fmt"
	"testing"

	cometabcitypes "github.com/cometbft/cometbft/abci/types"
	qt "github.com/frankban/quicktest"
	"go.vocdoni.io/dvote/crypto/ethereum"
	"go.vocdoni.io/dvote/types"
	"go.vocdoni.io/dvote/util"
	"go.vocdoni.io/dvote/vochain/genesis"
	vstate "go.vocdoni.io/dvote/vochain/state"
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
	qt.Assert(t, testNewProcess(t, process.ProcessId, accounts[0], app, process), qt.IsNil)
	// all get accounts assume account is not nil
	entityAcc, err := app.State.GetAccount(accounts[0].Address(), false)
	qt.Assert(t, err, qt.IsNil)
	qt.Assert(t, entityAcc.Balance, qt.Equals, uint64(9990))
	qt.Assert(t, entityAcc.Nonce, qt.Equals, uint32(1))
	qt.Assert(t, entityAcc.ProcessIndex, qt.Equals, uint32(1))

	// create process with delegate (should work)
	qt.Assert(t, testNewProcess(t, process.ProcessId, accounts[1], app, process), qt.IsNil)
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
	qt.Assert(t, testNewProcess(t, process.ProcessId, accounts[2], app, process), qt.IsNotNil)
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
	qt.Assert(t, testNewProcess(t, process.ProcessId, accounts[1], app, process), qt.IsNil)
	// create process with status different than READY or PAUSED (should not work)
	process.Status = models.ProcessStatus_CANCELED
	qt.Assert(t, testNewProcess(t, process.ProcessId, accounts[1], app, process),
		qt.ErrorMatches, ".*status must be READY or PAUSED.*")
	process.Status = models.ProcessStatus_PROCESS_UNKNOWN
	qt.Assert(t, testNewProcess(t, process.ProcessId, accounts[1], app, process),
		qt.ErrorMatches, ".*status must be READY or PAUSED.*")
	process.Status = models.ProcessStatus_ENDED
	qt.Assert(t, testNewProcess(t, process.ProcessId, accounts[1], app, process),
		qt.ErrorMatches, ".*status must be READY or PAUSED.*")
	process.Status = models.ProcessStatus_RESULTS
	qt.Assert(t, testNewProcess(t, process.ProcessId, accounts[1], app, process),
		qt.ErrorMatches, ".*status must be READY or PAUSED.*")
}

func testNewProcess(t *testing.T, _ []byte, txSender *ethereum.SignKeys,
	app *BaseApplication, process *models.Process) error {
	var stx models.SignedTx
	var err error

	// assumes account is not nil
	txSenderAcc, err := app.State.GetAccount(txSender.Address(), false)
	if err != nil {
		return fmt.Errorf("cannot get tx sender account %s with error %w", txSender.Address(), err)
	}
	// create tx
	tx := &models.NewProcessTx{
		Txtype:  models.TxType_NEW_PROCESS,
		Nonce:   txSenderAcc.Nonce,
		Process: process,
	}
	stx.Tx, err = proto.Marshal(&models.Tx{Payload: &models.Tx_NewProcess{NewProcess: tx}})
	if err != nil {
		return fmt.Errorf("cannot mashal tx %w", err)
	}
	if stx.Signature, err = txSender.SignVocdoniTx(stx.Tx, app.chainID); err != nil {
		return fmt.Errorf("cannot sign tx %+v with error %w", tx, err)
	}

	return testCheckTxDeliverTxCommit(t, app, &stx)
}

func TestProcessSetStatusCheckTxDeliverTxCommitTransitions(t *testing.T) {
	app, keys := createTestBaseApplicationAndAccounts(t, 10)
	// add a process with status=READY and interruptible=true
	censusURI := ipfsUrlTest
	pid := util.RandomBytes(types.ProcessIDsize)
	process := &models.Process{
		ProcessId:     pid,
		StartBlock:    0,
		EnvelopeType:  &models.EnvelopeType{EncryptedVotes: false},
		Mode:          &models.ProcessMode{Interruptible: true},
		VoteOptions:   &models.ProcessVoteOptions{MaxCount: 16, MaxValue: 16},
		Status:        models.ProcessStatus_READY,
		EntityId:      keys[0].Address().Bytes(),
		CensusRoot:    util.RandomBytes(32),
		CensusURI:     &censusURI,
		CensusOrigin:  models.CensusOrigin_OFF_CHAIN_TREE,
		BlockCount:    1024,
		MaxCensusSize: 100,
	}
	qt.Assert(t, app.State.AddProcess(process), qt.IsNil)

	// Set it to PAUSE (should work)
	status := models.ProcessStatus_PAUSED
	qt.Assert(t, testSetProcessStatus(t, pid, keys[0], app, &status), qt.IsNil)

	// Set it to READY (should work)
	status = models.ProcessStatus_READY
	qt.Assert(t, testSetProcessStatus(t, pid, keys[0], app, &status), qt.IsNil)

	// Set it to PAUSED by delegate (should work)
	status = models.ProcessStatus_PAUSED
	qt.Assert(t, testSetProcessStatus(t, pid, keys[1], app, &status), qt.IsNil)
	// Set it to READY by delegate (should work)
	status = models.ProcessStatus_READY
	qt.Assert(t, testSetProcessStatus(t, pid, keys[1], app, &status), qt.IsNil)

	// Set it to ENDED (should work)
	status = models.ProcessStatus_ENDED
	qt.Assert(t, testSetProcessStatus(t, pid, keys[0], app, &status), qt.IsNil)

	// Set it to RESULTS (should not work)
	status = models.ProcessStatus_RESULTS
	qt.Assert(t, testSetProcessStatus(t, pid, keys[0], app, &status), qt.IsNotNil)

	// Set it to READY (should fail)
	status = models.ProcessStatus_READY
	qt.Assert(t, testSetProcessStatus(t, pid, keys[0], app, &status), qt.IsNotNil)

	// Add a process with status=PAUSED and interruptible=true
	censusURI = ipfsUrlTest
	pid = util.RandomBytes(types.ProcessIDsize)
	process = &models.Process{
		ProcessId:     pid,
		StartBlock:    0,
		EnvelopeType:  &models.EnvelopeType{EncryptedVotes: false},
		Mode:          &models.ProcessMode{Interruptible: true},
		Status:        models.ProcessStatus_PAUSED,
		EntityId:      keys[0].Address().Bytes(),
		CensusRoot:    util.RandomBytes(32),
		CensusURI:     &censusURI,
		CensusOrigin:  models.CensusOrigin_OFF_CHAIN_TREE,
		BlockCount:    1024,
		MaxCensusSize: 100,
	}
	t.Logf("adding PAUSED process %x", process.ProcessId)
	qt.Assert(t, app.State.AddProcess(process), qt.IsNil)

	// Set it to READY (should work)
	status = models.ProcessStatus_READY
	qt.Assert(t, testSetProcessStatus(t, pid, keys[0], app, &status), qt.IsNil)

	// Set it to PAUSED (should work)
	status = models.ProcessStatus_PAUSED
	qt.Assert(t, testSetProcessStatus(t, pid, keys[0], app, &status), qt.IsNil)

	// Set it to CANCELED (should work)
	status = models.ProcessStatus_CANCELED
	qt.Assert(t, testSetProcessStatus(t, pid, keys[0], app, &status), qt.IsNil)

	// Set it to READY (should fail)
	status = models.ProcessStatus_READY
	qt.Assert(t, testSetProcessStatus(t, pid, keys[0], app, &status), qt.IsNotNil)

	// Add a process with status=PAUSE and interruptible=false
	censusURI = ipfsUrlTest
	pid = util.RandomBytes(types.ProcessIDsize)
	process = &models.Process{
		ProcessId:     pid,
		StartBlock:    10,
		EnvelopeType:  &models.EnvelopeType{EncryptedVotes: false},
		Mode:          &models.ProcessMode{Interruptible: false, AutoStart: false},
		Status:        models.ProcessStatus_PAUSED,
		EntityId:      keys[0].Address().Bytes(),
		CensusRoot:    util.RandomBytes(32),
		CensusURI:     &censusURI,
		CensusOrigin:  models.CensusOrigin_OFF_CHAIN_TREE,
		BlockCount:    1024,
		MaxCensusSize: 100,
	}
	t.Logf("adding PAUSED process %x", process.ProcessId)
	qt.Assert(t, app.State.AddProcess(process), qt.IsNil)

	// Set it to READY (should work)
	status = models.ProcessStatus_READY
	qt.Assert(t, testSetProcessStatus(t, pid, keys[0], app, &status), qt.IsNil)

	// Set it to PAUSE (should fail)
	status = models.ProcessStatus_PAUSED
	qt.Assert(t, testSetProcessStatus(t, pid, keys[0], app, &status), qt.IsNotNil)

	// Set it to ENDED (should fail)
	status = models.ProcessStatus_ENDED
	t.Logf("height: %d", app.State.CurrentHeight())
	qt.Assert(t, testSetProcessStatus(t, pid, keys[0], app, &status), qt.IsNotNil)
}

func testSetProcessStatus(t *testing.T, pid []byte, txSender *ethereum.SignKeys,
	app *BaseApplication, status *models.ProcessStatus) error {
	var stx models.SignedTx
	var err error

	// assume account is not nil
	txSenderAcc, err := app.State.GetAccount(txSender.Address(), false)
	if err != nil {
		return fmt.Errorf("cannot get tx sender account %s with error %w", txSender.Address(), err)
	}
	// create tx
	tx := &models.SetProcessTx{
		Txtype:    models.TxType_SET_PROCESS_STATUS,
		Nonce:     txSenderAcc.Nonce,
		ProcessId: pid,
		Status:    status,
	}
	stx.Tx, err = proto.Marshal(&models.Tx{Payload: &models.Tx_SetProcess{SetProcess: tx}})
	if err != nil {
		return fmt.Errorf("cannot mashal tx %w", err)
	}
	if stx.Signature, err = txSender.SignVocdoniTx(stx.Tx, app.chainID); err != nil {
		return fmt.Errorf("cannot sign tx %+v with error %w", tx, err)
	}

	return testCheckTxDeliverTxCommit(t, app, &stx)
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
		CensusOrigin:  models.CensusOrigin_ERC20,
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
	qt.Assert(t, testSetProcessCensus(t, pid, keys[0], app, []byte{1, 2, 3}, &censusURI2), qt.IsNil)

	// Set census by delegate (should work)
	qt.Assert(t, testSetProcessCensus(t, pid, keys[1], app, []byte{3, 2, 1}, &censusURI2), qt.IsNil)

	// Set census  (should not work)
	qt.Assert(t, testSetProcessCensus(t, pid2, keys[0], app, []byte{1, 2, 3}, &censusURI2), qt.IsNotNil)

	// Set census  (should not work)
	qt.Assert(t, testSetProcessCensus(t, pid3, keys[2], app, []byte{1, 2, 3}, &censusURI2), qt.IsNotNil)
}

func testSetProcessCensus(t *testing.T, pid []byte, txSender *ethereum.SignKeys,
	app *BaseApplication, censusRoot []byte, censusURI *string) error {
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
	}
	if stx.Tx, err = proto.Marshal(&models.Tx{
		Payload: &models.Tx_SetProcess{SetProcess: tx}},
	); err != nil {
		return fmt.Errorf("cannot mashal tx %w", err)
	}
	if stx.Signature, err = txSender.SignVocdoniTx(stx.Tx, app.chainID); err != nil {
		return fmt.Errorf("cannot sign tx %+v with error %w", tx, err)
	}

	return testCheckTxDeliverTxCommit(t, app, &stx)
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
	txCostNumber uint64) (*BaseApplication, []*ethereum.SignKeys) {
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
	testCommitState(t, app)
	return app, keys
}

func testCheckTxDeliverTxCommit(t *testing.T, app *BaseApplication, stx *models.SignedTx) error {
	cktx := new(cometabcitypes.CheckTxRequest)
	var err error
	// checkTx()
	cktx.Tx, err = proto.Marshal(stx)
	if err != nil {
		return fmt.Errorf("mashaling failed: %w", err)
	}
	cktxresp, _ := app.CheckTx(context.Background(), cktx)
	if cktxresp.Code != 0 {
		return fmt.Errorf("checkTx failed: %s", cktxresp.Data)
	}
	// deliverTx()
	tx, err := proto.Marshal(stx)
	if err != nil {
		return fmt.Errorf("mashaling failed: %w", err)
	}
	detxresp := app.deliverTx(tx)
	if detxresp.Code != 0 {
		return fmt.Errorf("deliverTx failed: %s", detxresp.Data)
	}
	// commit()
	testCommitState(t, app)
	return nil
}

func TestGlobalMaxProcessSize(t *testing.T) {
	app, accounts := createTestBaseApplicationAndAccounts(t, 10)
	app.State.SetMaxProcessSize(10)
	app.AdvanceTestBlock()

	// define process
	censusURI := ipfsUrlTest
	pid := util.RandomBytes(types.ProcessIDsize)
	process := &models.Process{
		ProcessId:     pid,
		StartBlock:    1,
		EnvelopeType:  &models.EnvelopeType{EncryptedVotes: false},
		Mode:          &models.ProcessMode{Interruptible: true},
		VoteOptions:   &models.ProcessVoteOptions{MaxCount: 16, MaxValue: 16},
		Status:        models.ProcessStatus_READY,
		EntityId:      accounts[0].Address().Bytes(),
		CensusRoot:    util.RandomBytes(32),
		CensusURI:     &censusURI,
		CensusOrigin:  models.CensusOrigin_OFF_CHAIN_TREE,
		BlockCount:    1024,
		MaxCensusSize: 20,
	}

	// create process with entityID (should fail)
	qt.Assert(t, testNewProcess(t, process.ProcessId, accounts[0], app, process), qt.IsNotNil)

	// create process with entityID (should work)
	process.MaxCensusSize = 5
	qt.Assert(t, testNewProcess(t, process.ProcessId, accounts[0], app, process), qt.IsNil)
}

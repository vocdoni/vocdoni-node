package vochain

import (
	"fmt"
	"testing"

	"github.com/ethereum/go-ethereum/common"
	abcitypes "github.com/tendermint/tendermint/abci/types"
	"go.vocdoni.io/dvote/crypto/ethereum"
	"go.vocdoni.io/dvote/types"
	"go.vocdoni.io/dvote/util"
	models "go.vocdoni.io/proto/build/go/models"
	"google.golang.org/protobuf/proto"
)

func TestProcessSetStatusTransition(t *testing.T) {
	app, err := NewBaseApplication(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	oracle := ethereum.SignKeys{}
	if err := oracle.Generate(); err != nil {
		t.Fatal(err)
	}
	if err := app.State.AddOracle(common.HexToAddress(oracle.AddressString())); err != nil {
		t.Fatal(err)
	}

	// Add a process with status=READY and interruptible=true
	censusURI := "ipfs://123456789"
	pid := util.RandomBytes(types.ProcessIDsize)
	process := &models.Process{
		ProcessId:    pid,
		StartBlock:   0,
		EnvelopeType: &models.EnvelopeType{EncryptedVotes: false},
		Mode:         &models.ProcessMode{Interruptible: true},
		VoteOptions:  &models.ProcessVoteOptions{MaxCount: 16, MaxValue: 16},
		Status:       models.ProcessStatus_READY,
		EntityId:     util.RandomBytes(types.EthereumAddressSize),
		CensusRoot:   util.RandomBytes(32),
		CensusURI:    &censusURI,
		CensusOrigin: models.CensusOrigin_OFF_CHAIN_TREE,
		BlockCount:   1024,
	}
	t.Logf("adding READY process %x", process.ProcessId)
	if err := app.State.AddProcess(process); err != nil {
		t.Fatal(err)
	}

	// Set it to PAUSE (should work)
	status := models.ProcessStatus_PAUSED
	if err := testSetProcessStatus(t, pid, &oracle, app, &status); err != nil {
		t.Fatal(err)
	}

	// Set it to READY (should work)
	status = models.ProcessStatus_READY
	if err := testSetProcessStatus(t, pid, &oracle, app, &status); err != nil {
		t.Fatal(err)
	}

	// Set it to ENDED (should work)
	status = models.ProcessStatus_ENDED
	if err := testSetProcessStatus(t, pid, &oracle, app, &status); err != nil {
		t.Fatal(err)
	}

	// Set it to RESULTS (should work)
	status = models.ProcessStatus_RESULTS
	if err := testSetProcessStatus(t, pid, &oracle, app, &status); err != nil {
		t.Fatal(err)
	}

	// Set it to READY (should fail)
	status = models.ProcessStatus_READY
	if err := testSetProcessStatus(t, pid, &oracle, app, &status); err == nil {
		t.Fatal("results to ready should not be valid")
	}

	// Add a process with status=PAUSED and interruptible=true
	censusURI = "ipfs://123456789"
	pid = util.RandomBytes(types.ProcessIDsize)
	process = &models.Process{
		ProcessId:    pid,
		StartBlock:   0,
		EnvelopeType: &models.EnvelopeType{EncryptedVotes: false},
		Mode:         &models.ProcessMode{Interruptible: true},
		Status:       models.ProcessStatus_PAUSED,
		EntityId:     util.RandomBytes(types.EthereumAddressSize),
		CensusRoot:   util.RandomBytes(32),
		CensusURI:    &censusURI,
		CensusOrigin: models.CensusOrigin_OFF_CHAIN_TREE,
		BlockCount:   1024,
	}
	t.Logf("adding PAUSED process %x", process.ProcessId)
	app.State.AddProcess(process)

	// Set it to READY (should work)
	status = models.ProcessStatus_READY
	if err := testSetProcessStatus(t, pid, &oracle, app, &status); err != nil {
		t.Fatal(err)
	}

	// Set it to PAUSED (should work)
	status = models.ProcessStatus_PAUSED
	if err := testSetProcessStatus(t, pid, &oracle, app, &status); err != nil {
		t.Fatal(err)
	}

	// Set it to ENDED (should fail)
	status = models.ProcessStatus_ENDED
	if err := testSetProcessStatus(t, pid, &oracle, app, &status); err == nil {
		t.Fatal("paused to ended should not be valid")
	}

	// Set it to CANCELED (should work)
	status = models.ProcessStatus_CANCELED
	if err := testSetProcessStatus(t, pid, &oracle, app, &status); err != nil {
		t.Fatal(err)
	}

	// Set it to READY (should fail)
	status = models.ProcessStatus_READY
	if err := testSetProcessStatus(t, pid, &oracle, app, &status); err == nil {
		t.Fatal("cancel to ready should not be valid")
	}

	// Add a process with status=PAUSE and interruptible=false
	censusURI = "ipfs://123456789"
	pid = util.RandomBytes(types.ProcessIDsize)
	process = &models.Process{
		ProcessId:    pid,
		StartBlock:   10,
		EnvelopeType: &models.EnvelopeType{EncryptedVotes: false},
		Mode:         &models.ProcessMode{Interruptible: false, AutoStart: false},
		Status:       models.ProcessStatus_PAUSED,
		EntityId:     util.RandomBytes(types.EthereumAddressSize),
		CensusRoot:   util.RandomBytes(32),
		CensusURI:    &censusURI,
		CensusOrigin: models.CensusOrigin_OFF_CHAIN_TREE,
		BlockCount:   1024,
	}
	t.Logf("adding PAUSED process %x", process.ProcessId)
	app.State.AddProcess(process)

	// Set it to READY (should work)
	status = models.ProcessStatus_READY
	if err := testSetProcessStatus(t, pid, &oracle, app, &status); err != nil {
		t.Fatal(err)
	}

	// Set it to PAUSE (should fail)
	status = models.ProcessStatus_PAUSED
	if err := testSetProcessStatus(t, pid, &oracle, app, &status); err == nil {
		t.Fatal("ready to paused should not be possible if interruptible=false")
	}

	// Set it to ENDED (should fail)
	status = models.ProcessStatus_ENDED
	t.Logf("height: %d", app.State.Header(false).Height)
	if err := testSetProcessStatus(t, pid, &oracle, app, &status); err == nil {
		t.Fatal("ready to ended should not be valid if interruptible=false")
	}
}

func testSetProcessStatus(t *testing.T, pid []byte, oracle *ethereum.SignKeys, app *BaseApplication, status *models.ProcessStatus) error {
	var cktx abcitypes.RequestCheckTx
	var detx abcitypes.RequestDeliverTx
	var cktxresp abcitypes.ResponseCheckTx
	var detxresp abcitypes.ResponseDeliverTx
	var vtx models.Tx

	tx := &models.SetProcessTx{
		Txtype:    models.TxType_SET_PROCESS_STATUS,
		Nonce:     util.RandomBytes(32),
		ProcessId: pid,
		Status:    status,
	}
	txBytes, err := proto.Marshal(tx)
	if err != nil {
		t.Fatal(err)
	}

	if vtx.Signature, err = oracle.Sign(txBytes); err != nil {
		t.Fatal(err)
	}
	vtx.Payload = &models.Tx_SetProcess{SetProcess: tx}

	if cktx.Tx, err = proto.Marshal(&vtx); err != nil {
		t.Fatal(err)
	}
	cktxresp = app.CheckTx(cktx)
	if cktxresp.Code != 0 {
		return fmt.Errorf("checkTx failed: %s", cktxresp.Data)
	}
	if detx.Tx, err = proto.Marshal(&vtx); err != nil {
		t.Fatal(err)
	}
	detxresp = app.DeliverTx(detx)
	if detxresp.Code != 0 {
		return fmt.Errorf("deliverTx failed: %s", detxresp.Data)
	}
	app.Commit()
	return nil
}

func TestProcessSetResultsTransition(t *testing.T) {
	app, err := NewBaseApplication(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	oracle := ethereum.SignKeys{}
	if err := oracle.Generate(); err != nil {
		t.Fatal(err)
	}
	if err := app.State.AddOracle(common.HexToAddress(oracle.AddressString())); err != nil {
		t.Fatal(err)
	}

	// Add a process with status=READY and interruptible=true
	censusURI := "ipfs://123456789"
	pid := util.RandomBytes(types.ProcessIDsize)
	process := &models.Process{
		ProcessId:    pid,
		StartBlock:   0,
		EnvelopeType: &models.EnvelopeType{EncryptedVotes: false},
		Mode:         &models.ProcessMode{Interruptible: true},
		Status:       models.ProcessStatus_READY,
		EntityId:     util.RandomBytes(types.EthereumAddressSize),
		CensusRoot:   util.RandomBytes(32),
		CensusURI:    &censusURI,
		CensusOrigin: models.CensusOrigin_OFF_CHAIN_TREE,
		BlockCount:   1024,
	}
	t.Logf("adding READY process %x", process.ProcessId)
	app.State.AddProcess(process)

	// Set results  (should not work)
	votes := make([]*models.QuestionResult, 1)
	votes[0] = &models.QuestionResult{
		Question: [][]byte{{1}},
	}
	results := &models.ProcessResult{
		ProcessId: process.ProcessId,
		EntityId:  process.EntityId,
		Votes:     votes,
	}
	if err := testSetProcessResults(t, pid, &oracle, app, results); err != nil {
		t.Logf("adding results while process ready should not work")
	}

	// Set it to PAUSE
	status := models.ProcessStatus_PAUSED
	if err := testSetProcessStatus(t, pid, &oracle, app, &status); err != nil {
		t.Fatal(err)
	}
	// Set results  (should not work)
	if err := testSetProcessResults(t, pid, &oracle, app, results); err != nil {
		t.Logf("adding results while process paused should not work")
	}

	// Set it to READY
	status = models.ProcessStatus_READY
	if err := testSetProcessStatus(t, pid, &oracle, app, &status); err != nil {
		t.Fatal(err)
	}

	// Set it to ENDED (should work)
	status = models.ProcessStatus_ENDED
	if err := testSetProcessStatus(t, pid, &oracle, app, &status); err != nil {
		t.Fatal(err)
	}
	if err := testSetProcessResults(t, pid, &oracle, app, results); err != nil {
		t.Fatal("adding results while process ended should work")
	}

	// status results already added by the previous tx

	// Set results  (should not work)
	if err := testSetProcessResults(t, pid, &oracle, app, results); err != nil {
		t.Logf("adding results cannot be added twice")
	}
}

func testSetProcessResults(t *testing.T, pid []byte, oracle *ethereum.SignKeys, app *BaseApplication, results *models.ProcessResult) error {
	var cktx abcitypes.RequestCheckTx
	var detx abcitypes.RequestDeliverTx
	var cktxresp abcitypes.ResponseCheckTx
	var detxresp abcitypes.ResponseDeliverTx
	var vtx models.Tx

	tx := &models.SetProcessTx{
		Txtype:    models.TxType_SET_PROCESS_RESULTS,
		Nonce:     util.RandomBytes(32),
		ProcessId: pid,
		Results:   results,
	}
	txBytes, err := proto.Marshal(tx)
	if err != nil {
		t.Fatal(err)
	}

	if vtx.Signature, err = oracle.Sign(txBytes); err != nil {
		t.Fatal(err)
	}
	vtx.Payload = &models.Tx_SetProcess{SetProcess: tx}

	if cktx.Tx, err = proto.Marshal(&vtx); err != nil {
		t.Fatal(err)
	}
	cktxresp = app.CheckTx(cktx)
	if cktxresp.Code != 0 {
		return fmt.Errorf("checkTx failed: %s", cktxresp.Data)
	}
	if detx.Tx, err = proto.Marshal(&vtx); err != nil {
		t.Fatal(err)
	}
	detxresp = app.DeliverTx(detx)
	if detxresp.Code != 0 {
		return fmt.Errorf("deliverTx failed: %s", detxresp.Data)
	}
	app.Commit()
	return nil
}

func TestProcessSetCensusTransition(t *testing.T) {
	app, err := NewBaseApplication(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	oracle := ethereum.SignKeys{}
	if err := oracle.Generate(); err != nil {
		t.Fatal(err)
	}
	if err := app.State.AddOracle(common.HexToAddress(oracle.AddressString())); err != nil {
		t.Fatal(err)
	}

	// Add a process with status=READY and interruptible=true
	censusURI := "ipfs://123456789"
	censusURI2 := "ipfs://987654321"
	pid := util.RandomBytes(types.ProcessIDsize)
	pid2 := util.RandomBytes(types.ProcessIDsize)
	pid3 := util.RandomBytes(types.ProcessIDsize)
	process := &models.Process{
		ProcessId:    pid,
		StartBlock:   0,
		EnvelopeType: &models.EnvelopeType{EncryptedVotes: false},
		Mode:         &models.ProcessMode{Interruptible: true, DynamicCensus: true},
		Status:       models.ProcessStatus_READY,
		EntityId:     util.RandomBytes(types.EthereumAddressSize),
		CensusRoot:   util.RandomBytes(32),
		CensusURI:    &censusURI,
		CensusOrigin: models.CensusOrigin_OFF_CHAIN_TREE,
		BlockCount:   1024,
	}

	process2 := &models.Process{
		ProcessId:    pid2,
		StartBlock:   0,
		EnvelopeType: &models.EnvelopeType{EncryptedVotes: false},
		Mode:         &models.ProcessMode{Interruptible: true},
		Status:       models.ProcessStatus_READY,
		EntityId:     util.RandomBytes(types.EthereumAddressSize),
		CensusRoot:   util.RandomBytes(32),
		CensusURI:    &censusURI2,
		CensusOrigin: models.CensusOrigin_OFF_CHAIN_TREE,
		BlockCount:   1024,
	}

	process3 := &models.Process{
		ProcessId:    pid3,
		StartBlock:   0,
		EnvelopeType: &models.EnvelopeType{EncryptedVotes: false},
		Mode:         &models.ProcessMode{Interruptible: true, DynamicCensus: true},
		Status:       models.ProcessStatus_READY,
		EntityId:     util.RandomBytes(types.EthereumAddressSize),
		CensusRoot:   util.RandomBytes(32),
		CensusURI:    &censusURI2,
		CensusOrigin: models.CensusOrigin_ERC20,
		BlockCount:   1024,
	}
	t.Logf("adding READY process %x", process.ProcessId)
	if err := app.State.AddProcess(process); err != nil {
		t.Fatal(err)
	}
	t.Logf("adding READY process %x", process2.ProcessId)
	if err := app.State.AddProcess(process2); err != nil {
		t.Fatal(err)
	}
	t.Logf("adding READY process %x", process3.ProcessId)
	if err := app.State.AddProcess(process3); err != nil {
		t.Fatal(err)
	}

	// Set census  (should work)
	if err := testSetProcessCensus(t, pid, &oracle, app, []byte{1, 2, 3}, &censusURI2); err != nil {
		t.Fatalf("update census should work if dynamic and off chain census: %s", err)
	}

	// Set census  (should not work)
	if err := testSetProcessCensus(t, pid2, &oracle, app, []byte{1, 2, 3}, &censusURI2); err != nil {
		t.Logf("update census should not work if dynamic census is set to false: %s", err)
	} else {
		t.Fatal("update census should not work if dynamic census is set to false")
	}

	// Set census  (should not work)
	if err := testSetProcessCensus(t, pid3, &oracle, app, []byte{1, 2, 3}, &censusURI2); err != nil {
		t.Logf("update census should not work if on chain census: %s", err)
	} else {
		t.Fatal("update census should not work if on chain census")
	}
}

func testSetProcessCensus(t *testing.T, pid []byte, oracle *ethereum.SignKeys, app *BaseApplication, censusRoot []byte, censusURI *string) error {
	var cktx abcitypes.RequestCheckTx
	var detx abcitypes.RequestDeliverTx
	var cktxresp abcitypes.ResponseCheckTx
	var detxresp abcitypes.ResponseDeliverTx
	var vtx models.Tx

	tx := &models.SetProcessTx{
		Txtype:     models.TxType_SET_PROCESS_CENSUS,
		Nonce:      util.RandomBytes(32),
		ProcessId:  pid,
		CensusRoot: censusRoot,
		CensusURI:  censusURI,
	}
	txBytes, err := proto.Marshal(tx)
	if err != nil {
		t.Fatal(err)
	}

	if vtx.Signature, err = oracle.Sign(txBytes); err != nil {
		t.Fatal(err)
	}
	vtx.Payload = &models.Tx_SetProcess{SetProcess: tx}

	if cktx.Tx, err = proto.Marshal(&vtx); err != nil {
		t.Fatal(err)
	}
	cktxresp = app.CheckTx(cktx)
	if cktxresp.Code != 0 {
		return fmt.Errorf("checkTx failed: %s", cktxresp.Data)
	}
	if detx.Tx, err = proto.Marshal(&vtx); err != nil {
		t.Fatal(err)
	}
	detxresp = app.DeliverTx(detx)
	if detxresp.Code != 0 {
		return fmt.Errorf("deliverTx failed: %s", detxresp.Data)
	}
	app.Commit()
	return nil
}

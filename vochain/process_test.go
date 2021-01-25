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

func TestProcessTransition(t *testing.T) {
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
	mkuri := "ipfs://123456789"
	pid := util.RandomBytes(types.ProcessIDsize)
	process := &models.Process{
		ProcessId:    pid,
		StartBlock:   0,
		EnvelopeType: &models.EnvelopeType{EncryptedVotes: false},
		Mode:         &models.ProcessMode{Interruptible: true},
		Status:       models.ProcessStatus_READY,
		EntityId:     util.RandomBytes(types.EntityIDsize),
		CensusRoot:   util.RandomBytes(32),
		CensusURI:    &mkuri,
		CensusOrigin: models.CensusOrigin_OFF_CHAIN_TREE,
		BlockCount:   1024,
	}
	t.Logf("adding READY process %x", process.ProcessId)
	app.State.AddProcess(process)

	// Set it to PAUSE (should work)
	status := models.ProcessStatus_PAUSED
	if err := testSetProcess(t, pid, &oracle, app, &status); err != nil {
		t.Fatal(err)
	}

	// Set it to READY (should work)
	status = models.ProcessStatus_READY
	if err := testSetProcess(t, pid, &oracle, app, &status); err != nil {
		t.Fatal(err)
	}

	// Set it to ENDED (should work)
	status = models.ProcessStatus_ENDED
	if err := testSetProcess(t, pid, &oracle, app, &status); err != nil {
		t.Fatal(err)
	}

	// Set it to RESULTS (should work)
	status = models.ProcessStatus_RESULTS
	if err := testSetProcess(t, pid, &oracle, app, &status); err != nil {
		t.Fatal(err)
	}

	// Set it to READY (should fail)
	status = models.ProcessStatus_READY
	if err := testSetProcess(t, pid, &oracle, app, &status); err == nil {
		t.Fatal("results to ready should not be valid")
	}

	// Add a process with status=PAUSED and interruptible=true
	mkuri = "ipfs://123456789"
	pid = util.RandomBytes(types.ProcessIDsize)
	process = &models.Process{
		ProcessId:    pid,
		StartBlock:   0,
		EnvelopeType: &models.EnvelopeType{EncryptedVotes: false},
		Mode:         &models.ProcessMode{Interruptible: true},
		Status:       models.ProcessStatus_PAUSED,
		EntityId:     util.RandomBytes(types.EntityIDsize),
		CensusRoot:   util.RandomBytes(32),
		CensusURI:    &mkuri,
		CensusOrigin: models.CensusOrigin_OFF_CHAIN_TREE,
		BlockCount:   1024,
	}
	t.Logf("adding PAUSED process %x", process.ProcessId)
	app.State.AddProcess(process)

	// Set it to READY (should work)
	status = models.ProcessStatus_READY
	if err := testSetProcess(t, pid, &oracle, app, &status); err != nil {
		t.Fatal(err)
	}

	// Set it to PAUSED (should work)
	status = models.ProcessStatus_PAUSED
	if err := testSetProcess(t, pid, &oracle, app, &status); err != nil {
		t.Fatal(err)
	}

	// Set it to ENDED (should fail)
	status = models.ProcessStatus_ENDED
	if err := testSetProcess(t, pid, &oracle, app, &status); err == nil {
		t.Fatal("paused to ended should not be valid")
	}

	// Set it to CANCELED (should work)
	status = models.ProcessStatus_CANCELED
	if err := testSetProcess(t, pid, &oracle, app, &status); err != nil {
		t.Fatal(err)
	}

	// Set it to READY (should fail)
	status = models.ProcessStatus_READY
	if err := testSetProcess(t, pid, &oracle, app, &status); err == nil {
		t.Fatal("cancel to ready should not be valid")
	}

	// Add a process with status=PAUSE and interruptible=false
	mkuri = "ipfs://123456789"
	pid = util.RandomBytes(types.ProcessIDsize)
	process = &models.Process{
		ProcessId:    pid,
		StartBlock:   10,
		EnvelopeType: &models.EnvelopeType{EncryptedVotes: false},
		Mode:         &models.ProcessMode{Interruptible: false, AutoStart: false},
		Status:       models.ProcessStatus_PAUSED,
		EntityId:     util.RandomBytes(types.EntityIDsize),
		CensusRoot:   util.RandomBytes(32),
		CensusURI:    &mkuri,
		CensusOrigin: models.CensusOrigin_OFF_CHAIN_TREE,
		BlockCount:   1024,
	}
	t.Logf("adding PAUSED process %x", process.ProcessId)
	app.State.AddProcess(process)

	// Set it to READY (should work)
	status = models.ProcessStatus_READY
	if err := testSetProcess(t, pid, &oracle, app, &status); err != nil {
		t.Fatal(err)
	}

	// Set it to PAUSE (should fail)
	status = models.ProcessStatus_PAUSED
	if err := testSetProcess(t, pid, &oracle, app, &status); err == nil {
		t.Fatal("ready to paused should not be possible if interruptible=false")
	}

	// Set it to ENDED (should fail)
	status = models.ProcessStatus_ENDED
	t.Logf("height: %d", app.State.Header(false).Height)
	if err := testSetProcess(t, pid, &oracle, app, &status); err == nil {
		t.Fatal("ready to ended should not be valid if interruptible=false")
	}
}

func testSetProcess(t *testing.T, pid []byte, oracle *ethereum.SignKeys, app *BaseApplication, status *models.ProcessStatus) error {
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

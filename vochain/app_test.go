package vochain

import (
	"encoding/hex"
	"fmt"
	"testing"

	"github.com/ethereum/go-ethereum/common"
	abcitypes "github.com/tendermint/tendermint/abci/types"
	models "github.com/vocdoni/dvote-protobuf/build/go/models"
	tree "gitlab.com/vocdoni/go-dvote/censustree/gravitontree"
	"gitlab.com/vocdoni/go-dvote/crypto/ethereum"
	"gitlab.com/vocdoni/go-dvote/crypto/snarks"
	"gitlab.com/vocdoni/go-dvote/test/testcommon/testutil"
	"gitlab.com/vocdoni/go-dvote/types"
	"gitlab.com/vocdoni/go-dvote/util"
	"google.golang.org/protobuf/proto"
)

func TestCheckTX(t *testing.T) {
	app, err := NewBaseApplication(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}

	tr, err := tree.NewTree("testchecktx", t.TempDir())
	if err != nil {
		t.Fatal(err)
	}

	keys := createEthRandomKeysBatch(1000)
	claims := []string{}
	for _, k := range keys {
		pub, _ := k.HexString()
		pub, err = ethereum.DecompressPubKey(pub)
		if err != nil {
			t.Fatal(err)
		}
		pubb, err := hex.DecodeString(pub)
		if err != nil {
			t.Fatal(err)
		}
		c := snarks.Poseidon.Hash(pubb)
		tr.AddClaim(c, nil)
		claims = append(claims, string(c))
	}
	mkuri := "ipfs://123456789"
	pid := util.RandomBytes(types.ProcessIDsize)
	process := &models.Process{
		ProcessId:    pid,
		StartBlock:   0,
		EnvelopeType: &models.EnvelopeType{EncryptedVotes: false},
		Mode:         &models.ProcessMode{},
		Status:       models.ProcessStatus_READY,
		EntityId:     util.RandomBytes(types.EntityIDsize),
		CensusMkRoot: tr.Root(),
		CensusMkURI:  &mkuri,
		CensusOrigin: models.CensusOrigin_OFF_CHAIN,
		BlockCount:   1024,
	}
	t.Logf("adding process %s", process.String())
	app.State.AddProcess(process)

	var cktx abcitypes.RequestCheckTx
	var detx abcitypes.RequestDeliverTx

	var cktxresp abcitypes.ResponseCheckTx
	var detxresp abcitypes.ResponseDeliverTx

	var vtx models.Tx
	var proof string
	var hexsignature string
	vp := []byte("[1,2,3,4]")
	for i, s := range keys {
		proof, err = tr.GenProof([]byte(claims[i]), nil)
		if err != nil {
			t.Fatal(err)
		}
		tx := &models.VoteEnvelope{
			Nonce:       util.RandomBytes(32),
			ProcessId:   pid,
			Proof:       &models.Proof{Payload: &models.Proof_Graviton{Graviton: &models.ProofGraviton{Siblings: testutil.Hex2byte(t, proof)}}},
			VotePackage: vp,
		}
		txBytes, err := proto.Marshal(tx)
		if err != nil {
			t.Fatal(err)
		}
		if hexsignature, err = s.Sign(txBytes); err != nil {
			t.Fatal(err)
		}
		vtx.Payload = &models.Tx_Vote{Vote: tx}
		vtx.Signature = testutil.Hex2byte(t, hexsignature)

		if cktx.Tx, err = proto.Marshal(&vtx); err != nil {
			t.Fatal(err)
		}
		cktxresp = app.CheckTx(cktx)
		if cktxresp.Code != 0 {
			t.Fatalf(fmt.Sprintf("checkTX failed: %s", cktxresp.Data))
		}
		if detx.Tx, err = proto.Marshal(&vtx); err != nil {
			t.Fatal(err)
		}
		detxresp = app.DeliverTx(detx)
		if detxresp.Code != 0 {
			t.Fatalf(fmt.Sprintf("deliverTX failed: %s", detxresp.Data))
		}
		app.Commit()
	}
}

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
		CensusMkRoot: util.RandomBytes(32),
		CensusMkURI:  &mkuri,
		CensusOrigin: models.CensusOrigin_OFF_CHAIN,
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
		CensusMkRoot: util.RandomBytes(32),
		CensusMkURI:  &mkuri,
		CensusOrigin: models.CensusOrigin_OFF_CHAIN,
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
		CensusMkRoot: util.RandomBytes(32),
		CensusMkURI:  &mkuri,
		CensusOrigin: models.CensusOrigin_OFF_CHAIN,
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
	var hexsignature string

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

	if hexsignature, err = oracle.Sign(txBytes); err != nil {
		t.Fatal(err)
	}
	vtx.Payload = &models.Tx_SetProcess{SetProcess: tx}
	vtx.Signature = testutil.Hex2byte(t, hexsignature)

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

// CreateEthRandomKeysBatch creates a set of eth random signing keys
func createEthRandomKeysBatch(n int) []*ethereum.SignKeys {
	s := make([]*ethereum.SignKeys, n)
	for i := 0; i < n; i++ {
		s[i] = ethereum.NewSignKeys()
		if err := s[i].Generate(); err != nil {
			panic(err)
		}
	}
	return s
}

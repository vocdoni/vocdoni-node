package test

import (
	"testing"

	"go.vocdoni.io/dvote/test/testcommon"
	"go.vocdoni.io/dvote/util"
	"go.vocdoni.io/dvote/vochain"
	models "go.vocdoni.io/proto/build/go/models"
)

func TestCreateProcess(t *testing.T) {
	// TODO(mvdan): re-enable once tests are hermetic
	// t.Parallel()

	s := testcommon.NewVochainStateWithOracles(t)
	vtx := models.Tx{}
	vtx.Payload = &models.Tx_NewProcess{NewProcess: testcommon.HardcodedNewProcessTx}
	err := testcommon.SignAndPrepareTx(&vtx)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := vochain.NewProcessTxCheck(&vtx, s); err != nil {
		t.Errorf("cannot validate new process tx: %s", err)
	}

	// add process
	_, err = vochain.AddTx(&vtx, s, util.Random32(), true)
	if err != nil {
		t.Errorf("cannot create process: %s", err)
	}

	// cannot add same process
	if _, err = vochain.AddTx(&vtx, s, util.Random32(), true); err == nil {
		t.Errorf("same process added: %s", err)
	}

	// bad oracle signature
	vtx.Signature[12] = byte(0xFF)
	vtx.Signature[14] = byte(0xFF)
	vtx.Signature[16] = byte(0xFF)
	if _, err = vochain.AddTx(&vtx, s, util.Random32(), true); err == nil {
		t.Errorf("process added by non oracle: %s", err)
	}
}

/*
func TestSubmitEnvelope(t *testing.T) {
	t.Parallel()

	s := testcommon.NewVochainStateWithProcess(t)
	bytes, err := json.Marshal(*testcommon.HardcodedNewVoteTx)
	if err != nil {
		t.Errorf("cannot mashal process: %+v", *testcommon.HardcodedNewVoteTx)
	}
	_, err = vochain.ValidateAndDeliverTx(bytes, s)
	if err != nil {
		t.Errorf("cannot submit envelope: %s", err)
	}
	// cannot add same envelope
	_, err = vochain.ValidateAndDeliverTx(bytes, s)
	if err == nil {
		t.Errorf("cannot submit envelope twice: %s", err)
	}
	// cannot add to non existent process
	badpid := testcommon.HardcodedNewVoteTx
	badpid.ProcessID = "0x2"
	bytes, err = json.Marshal(badpid)
	if err != nil {
		t.Errorf("cannot mashal process: %+v", badpid)
	}
	_, err = vochain.ValidateAndDeliverTx(bytes, s)
	if err == nil {
		t.Errorf("cannot submit envelope twice: %s", err)
	}
}
*/

package test

import (
	"testing"

	models "github.com/vocdoni/dvote-protobuf/build/go/models"
	"gitlab.com/vocdoni/go-dvote/test/testcommon"
	"gitlab.com/vocdoni/go-dvote/vochain"
)

/*
func TestVoteTxCheck(t *testing.T) {
	var err error
	t.Parallel()

	s := testcommon.NewVochainStateWithProcess(t)
	tx := testcommon.HardcodedNewVoteTx
	tx.Signature, err = testcommon.SignTx(tx)
	if err != nil {
		t.Error(err)
	}
	if _, err := vochain.VoteTxCheck(tx, s, false); err != nil {
		t.Errorf("cannot validate vote: %s", err)
	}
}

func TestAdminTxCheckAddOracle(t *testing.T) {
	t.Parallel()

	s := testcommon.NewVochainStateWithOracles(t)
	if err := vochain.AdminTxCheck(*testcommon.HardcodedAdminTxAddOracle, s); err != nil {
		t.Errorf("cannot add oracle: %s", err)
	}
}

func TestAdminTxCheckRemoveOracle(t *testing.T) {
	t.Parallel()

	s := testcommon.NewVochainStateWithOracles(t)
	if err := vochain.AdminTxCheck(*testcommon.HardcodedAdminTxRemoveOracle, s); err != nil {
		t.Errorf("cannot remove oracle: %s", err)
	}
}

func TestAdminTxCheckAddValidator(t *testing.T) {
	t.Parallel()

	s := testcommon.NewVochainStateWithValidators(t)
	if err := vochain.AdminTxCheck(*testcommon.HardcodedAdminTxAddValidator, s); err != nil {
		t.Errorf("cannot add validator: %s", err)
	}
}

func TestAdminTxCheckRemoveValidator(t *testing.T) {
	t.Parallel()

	s := testcommon.NewVochainStateWithValidators(t)
	if err := vochain.AdminTxCheck(*testcommon.HardcodedAdminTxRemoveValidator, s); err != nil {
		t.Errorf("cannot remove validator: %s", err)
	}
}
*/

func TestCreateProcess(t *testing.T) {
	// TODO(mvdan): re-enable once
	// https://gitlab.com/vocdoni/go-dvote/-/issues/172 is fixed.
	// t.Parallel()

	s := testcommon.NewVochainStateWithOracles(t)
	vtx := models.Tx{}
	vtx.Tx = &models.Tx_NewProcess{NewProcess: testcommon.HardcodedNewProcessTx}
	err := testcommon.SignAndPrepareTx(&vtx)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := vochain.NewProcessTxCheck(&vtx, s); err != nil {
		t.Errorf("cannot validate new process tx: %s", err)
	}

	// add process
	_, err = vochain.AddTx(&vtx, s, true)
	if err != nil {
		t.Errorf("cannot create process: %s", err)
	}

	// cannot add same process
	if _, err = vochain.AddTx(&vtx, s, true); err == nil {
		t.Errorf("same process added: %s", err)
	}

	// bad oracle signature
	vtx.Signature[12] = byte(0xFF)
	vtx.Signature[14] = byte(0xFF)
	vtx.Signature[16] = byte(0xFF)
	if _, err = vochain.AddTx(&vtx, s, true); err == nil {
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

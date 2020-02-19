package test

import (
	"encoding/json"
	"os"
	"testing"

	testcommon "gitlab.com/vocdoni/go-dvote/test/test_common"
	"gitlab.com/vocdoni/go-dvote/vochain"
)

func TestNewProcessTxCheck(t *testing.T) {
	os.RemoveAll("/tmp/db")
	s := testcommon.NewVochainStateWithOracles() // vochain.NewVochainState("/tmp/db")
	if s == nil {
		t.Error("cannot create state")
	}
	if err := vochain.NewProcessTxCheck(*testcommon.HardcodedNewProcessTx, s); err != nil {
		t.Errorf("cannot validate new process tx: %s", err)
	}
}

/*
func TestVoteTxCheck(t *testing.T) {
	os.RemoveAll("/tmp/db")
	s := testcommon.NewVochainStateWithProcess()
	if s == nil {
		t.Error("cannot create state")
	}
	if err := vochain.VoteTxCheck(*testcommon.HardcodedNewVoteTx, s); err != nil {
		t.Errorf("cannot validate vote: %s", err)
	}
}
*/
func TestAdminTxCheckAddOracle(t *testing.T) {
	os.RemoveAll("/tmp/db")
	s := testcommon.NewVochainStateWithOracles()
	if s == nil {
		t.Error("cannot create state")
	}
	if err := vochain.AdminTxCheck(*testcommon.HardcodedAdminTxAddOracle, s); err != nil {
		t.Errorf("cannot add oracle: %s", err)
	}
}

func TestAdminTxCheckRemoveOracle(t *testing.T) {
	os.RemoveAll("/tmp/db")
	s := testcommon.NewVochainStateWithOracles()
	if s == nil {
		t.Error("cannot create state")
	}
	if err := vochain.AdminTxCheck(*testcommon.HardcodedAdminTxRemoveOracle, s); err != nil {
		t.Errorf("cannot remove oracle: %s", err)
	}
}

func TestAdminTxCheckAddValidator(t *testing.T) {
	os.RemoveAll("/tmp/db")
	s := testcommon.NewVochainStateWithValidators()
	if s == nil {
		t.Error("cannot create state")
	}
	if err := vochain.AdminTxCheck(*testcommon.HardcodedAdminTxAddValidator, s); err != nil {
		t.Errorf("cannot add validator: %s", err)
	}
}

func TestAdminTxCheckRemoveValidator(t *testing.T) {
	os.RemoveAll("/tmp/db")
	s := testcommon.NewVochainStateWithValidators()
	if s == nil {
		t.Error("cannot create state")
	}
	if err := vochain.AdminTxCheck(*testcommon.HardcodedAdminTxRemoveValidator, s); err != nil {
		t.Errorf("cannot remove validator: %s", err)
	}
}

func TestCreateProcess(t *testing.T) {
	os.RemoveAll("/tmp/db")
	s := testcommon.NewVochainStateWithOracles() // vochain.NewVochainState("/tmp/db")
	if s == nil {
		t.Error("cannot create state")
	}
	bytes, err := json.Marshal(*testcommon.HardcodedNewProcessTx)
	if err != nil {
		t.Errorf("cannot mashal process: %+v", *testcommon.HardcodedNewProcessTx)
	}
	_, err = vochain.ValidateAndDeliverTx(bytes, s)
	if err != nil {
		t.Errorf("cannot create process: %s", err)
	}
	// cannot add same process
	_, err = vochain.ValidateAndDeliverTx(bytes, s)
	if err == nil {
		t.Errorf("same process added: %s", err)
	}
	// cannot add process if not oracle
	badoracle := testcommon.HardcodedNewProcessTx
	badoracle.Signature = "a25259cff9ce3a709e517c6a01e445f216212f58f553fa26d25566b7c731339242ef9a0df0235b53a819a64ebf2c3394fb6b56138c5113cc1905c68ffcebb1971c"
	bytes, err = json.Marshal(badoracle)
	if err != nil {
		t.Errorf("cannot mashal process: %+v", badoracle)
	}
	_, err = vochain.ValidateAndDeliverTx(bytes, s)
	if err == nil {
		t.Errorf("process added by non oracle: %s", err)
	}
}

/*
func TestSubmitEnvelope(t *testing.T) {
	os.RemoveAll("/tmp/db")
	s := testcommon.NewVochainStateWithProcess() // vochain.NewVochainState("/tmp/db")
	if s == nil {
		t.Error("cannot create state")
	}
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

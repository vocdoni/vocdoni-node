package test

import (
	"os"
	"testing"

	testcommon "gitlab.com/vocdoni/go-dvote/test/test_common"
	vochain "gitlab.com/vocdoni/go-dvote/vochain"
)

func TestNewProcessTxCheck(t *testing.T) {
	os.RemoveAll("/tmp/db")
	s := testcommon.NewVochainStateWithOracles() //vochain.NewVochainState("/tmp/db")
	if s == nil {
		t.Error("cannot create state")
	}
	if err := vochain.NewProcessTxCheck(testcommon.HardcodedNewProcessTx, s); err != nil {
		t.Errorf("cannot validate new process tx: %s", err.Error())
	}
}

func TestVoteTxCheck(t *testing.T) {
	os.RemoveAll("/tmp/db")
	s := testcommon.NewVochainStateWithProcess()
	if s == nil {
		t.Error("cannot create state")
	}
	if err := vochain.VoteTxCheck(testcommon.HardcodedNewVoteTx, s); err != nil {
		t.Errorf("cannot validate vote: %s", err.Error())
	}
}

func TestAdminTxCheckAddOracle(t *testing.T) {
	os.RemoveAll("/tmp/db")
	s := testcommon.NewVochainStateWithOracles()
	if s == nil {
		t.Error("cannot create state")
	}
	if err := vochain.AdminTxCheck(testcommon.HardcodedAdminTxAddOracle, s); err != nil {
		t.Errorf("cannot add oracle: %s", err.Error())
	}
}

func TestAdminTxCheckRemoveOracle(t *testing.T) {
	os.RemoveAll("/tmp/db")
	s := testcommon.NewVochainStateWithOracles()
	if s == nil {
		t.Error("cannot create state")
	}
	if err := vochain.AdminTxCheck(testcommon.HardcodedAdminTxRemoveOracle, s); err != nil {
		t.Errorf("cannot remove oracle: %s", err.Error())
	}
}

func TestAdminTxCheckAddValidator(t *testing.T) {
	os.RemoveAll("/tmp/db")
	s := testcommon.NewVochainStateWithValidators()
	if s == nil {
		t.Error("cannot create state")
	}
	if err := vochain.AdminTxCheck(testcommon.HardcodedAdminTxAddValidator, s); err != nil {
		t.Errorf("cannot add validator: %s", err.Error())
	}
}

func TestAdminTxCheckRemoveValidator(t *testing.T) {
	os.RemoveAll("/tmp/db")
	s := testcommon.NewVochainStateWithValidators()
	if s == nil {
		t.Error("cannot create state")
	}
	if err := vochain.AdminTxCheck(testcommon.HardcodedAdminTxRemoveValidator, s); err != nil {
		t.Errorf("cannot remove validator: %s", err.Error())
	}
}

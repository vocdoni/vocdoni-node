package test

import (
	"fmt"
	"os"
	"testing"

	"github.com/tendermint/go-amino"

	"gitlab.com/vocdoni/go-dvote/log"
	testcommon "gitlab.com/vocdoni/go-dvote/test/test_common"
	iavl "gitlab.com/vocdoni/go-dvote/vochain"
)

func TestVochainState(t *testing.T) {
	log.InitLogger("error", "stdout")
	os.RemoveAll("/tmp/db")
	c := amino.NewCodec()
	s, err := iavl.NewVochainState("/tmp/db", c)
	if err != nil {
		t.Errorf("cannot create vochain state (%s)", err)
	}
	for i := 0; i < 10; i++ {
		s.AppTree.Set([]byte(string(i)), []byte(fmt.Sprintf("number %d", i)))
		s.ProcessTree.Set([]byte(string(i+1)), []byte(fmt.Sprintf("number %d", i+1)))
		s.VoteTree.Set([]byte(string(i+2)), []byte(fmt.Sprintf("number %d", i+2)))
	}
	s.AppTree.SaveVersion()
	s.ProcessTree.SaveVersion()
	s.VoteTree.SaveVersion()

	appHash := fmt.Sprintf("%x", s.AppTree.Hash())
	processHash := fmt.Sprintf("%x", s.ProcessTree.Hash())
	voteHash := fmt.Sprintf("%x", s.VoteTree.Hash())

	if appHash != "7b72f3fec170cfbc2f537f0d65cf8921b9d8203bce599b810ae30451d89bf9ad" {
		t.Errorf("app hash is not correct %s", appHash)
	}

	if processHash != "01faa0aa2aa87033832276fd7132c564b2a68e75b70635c1e33354e53d24d12c" {
		t.Errorf("app hash is not correct %s", appHash)
	}

	if voteHash != "8faf5a59b9443523f10cedfbd3b91582e97ae0171b01d717a9c38dd6b88b1e4b" {
		t.Errorf("app hash is not correct %s", appHash)
	}
}

func TestAddOracle(t *testing.T) {
	s := testcommon.NewVochainStateWithOracles()
	if s != nil {
		if err := s.AddOracle("0x414896B0BC763b8762456DB00F9c76EBd49979C4"); err != nil {
			t.Error(err)
		}
	}
}

func TestRemoveOracle(t *testing.T) {
	s := testcommon.NewVochainStateWithOracles()
	if s != nil {
		if err := s.RemoveOracle(testcommon.OracleListHardcoded[0]); err != nil {
			t.Error(err)
		}
	}
}

func TestGetOracles(t *testing.T) {
	s := testcommon.NewVochainStateWithOracles()
	if s != nil {
		oracles, err := s.Oracles()
		if err != nil {
			t.Error(err)
		}
		for i, v := range testcommon.OracleListHardcoded {
			if oracles[i] != v {
				t.Error("oracle address does not match")
			}
		}
	}
}

func TestAddValidator(t *testing.T) {
	s := testcommon.NewVochainStateWithValidators()
	if s != nil {
		if err := s.AddValidator(testcommon.HardcodedValidator.PubKey.Value, testcommon.HardcodedValidator.Power); err != nil {
			t.Error(err)
		}
	}
}

func TestRemoveValidator(t *testing.T) {
	s := testcommon.NewVochainStateWithValidators()
	if s != nil {
		if err := s.RemoveValidator(testcommon.ValidatorListHardcoded[0].Address); err != nil {
			t.Error(err)
		}
	}
}

func TestGetValidators(t *testing.T) {
	s := testcommon.NewVochainStateWithValidators()
	if s != nil {
		validators, err := s.Validators()
		if err != nil {
			t.Error(err)
		}
		for i, v := range testcommon.ValidatorListHardcoded {
			if validators[i].PubKey.Value != v.PubKey.Value {
				t.Error("validator pubkey not match")
			}
		}
	}
}

func TestAddProcess(t *testing.T) {
	s := testcommon.NewVochainStateWithProcess()
	if s == nil {
		t.Error("cannot create state")
	}
	if err := s.AddProcess(testcommon.ProcessHardcoded, "0xe9d5e8d791f51179e218c606f83f5967ab272292a6dbda887853d81f7a1d5105"); err != nil {
		t.Error(err)
	}
}

func TestGetProcess(t *testing.T) {
	s := testcommon.NewVochainStateWithProcess()
	if s == nil {
		t.Error("cannot create state")
	}
	if _, err := s.Process("0xe9d5e8d791f51179e218c606f83f5967ab272292a6dbda887853d81f7a1d5105"); err != nil {
		t.Error(err)
	}
}

func TestAddVote(t *testing.T) {
	s := testcommon.NewVochainStateWithProcess()
	if s != nil {
		if err := s.AddVote(testcommon.VoteHardcoded); err != nil {
			t.Error(err)
		}
	}
}

func TestGetEnvelope(t *testing.T) {
	s := testcommon.NewVochainStateWithProcess()
	if s != nil {
		if err := s.AddVote(testcommon.VoteHardcoded); err != nil {
			t.Error(err)
		}
		if _, err := s.Envelope("0xe9d5e8d791f51179e218c606f83f5967ab272292a6dbda887853d81f7a1d5105_5592f1c18e2a15953f355c34b247d751da307338c994000b9a65db1dc14cc6c0"); err != nil {
			t.Error(err)
		}
	}
}

func TestCountVotes(t *testing.T) {
	s := testcommon.NewVochainStateWithProcess()
	if s != nil {
		if err := s.AddVote(testcommon.VoteHardcoded); err != nil {
			t.Error(err)
		}
		if _, err := s.Envelope("0xe9d5e8d791f51179e218c606f83f5967ab272292a6dbda887853d81f7a1d5105_5592f1c18e2a15953f355c34b247d751da307338c994000b9a65db1dc14cc6c0"); err != nil {
			t.Error(err)
		}
	}
	c := s.CountVotes("0xe9d5e8d791f51179e218c606f83f5967ab272292a6dbda887853d81f7a1d5105")
	if c != 1 {
		t.Errorf("number of votes should be 1, received %d", c)
	}
}

func TestGetEnvelopeList(t *testing.T) {
	s := testcommon.NewVochainStateWithProcess()
	if s != nil {
		if err := s.AddVote(testcommon.VoteHardcoded); err != nil {
			t.Error(err)
		}
		if _, err := s.Envelope("0xe9d5e8d791f51179e218c606f83f5967ab272292a6dbda887853d81f7a1d5105_5592f1c18e2a15953f355c34b247d751da307338c994000b9a65db1dc14cc6c0"); err != nil {
			t.Error(err)
		}
	}
	nullifiers := s.EnvelopeList("0xe9d5e8d791f51179e218c606f83f5967ab272292a6dbda887853d81f7a1d5105", 0, 1)
	if nullifiers[0] != "5592f1c18e2a15953f355c34b247d751da307338c994000b9a65db1dc14cc6c0" {
		t.Errorf("bad nullifier recovered, expected: 5592f1c18e2a15953f355c34b247d751da307338c994000b9a65db1dc14cc6c0, got: %s", nullifiers[0])
	}
}

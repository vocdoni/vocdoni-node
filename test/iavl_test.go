package test

import (
	"fmt"
	"math/rand"
	"os"
	"testing"

	"github.com/tendermint/go-amino"
	"github.com/tendermint/tendermint/privval"

	testcommon "gitlab.com/vocdoni/go-dvote/test/testcommon"
	"gitlab.com/vocdoni/go-dvote/vochain"
)

func TestVochainState(t *testing.T) {
	os.RemoveAll("/tmp/db")
	c := amino.NewCodec()
	s, err := vochain.NewState("/tmp/db", c)
	if err != nil {
		t.Errorf("cannot create vochain state (%s)", err)
	}
	for i := 0; i < 10; i++ {
		s.AppTree.Set([]byte(fmt.Sprintf("%d", i)), []byte(fmt.Sprintf("number %d", i)))
		s.ProcessTree.Set([]byte(fmt.Sprintf("%d", i+1)), []byte(fmt.Sprintf("number %d", i+1)))
		s.VoteTree.Set([]byte(fmt.Sprintf("%d", i+2)), []byte(fmt.Sprintf("number %d", i+2)))
	}
	s.AppTree.SaveVersion()
	s.ProcessTree.SaveVersion()
	s.VoteTree.SaveVersion()

	appHash := fmt.Sprintf("%x", s.AppTree.Hash())
	processHash := fmt.Sprintf("%x", s.ProcessTree.Hash())
	voteHash := fmt.Sprintf("%x", s.VoteTree.Hash())

	if appHash != "0e7629c22261bde17ddd23970280c3a7eac63777aea5be57e65ae66f65047d37" {
		t.Errorf("app hash is not correct: %s", appHash)
	}

	if processHash != "b97b7bdf7c92b1c48077e347bcf492beee46873966460115f8eaa2131cc601eb" {
		t.Errorf("process hash is not correct: %s", processHash)
	}

	if voteHash != "33e0fe2e01bc3ad286c2539ce3782497398d9960ee4b6714f46cfe6f54640ea5" {
		t.Errorf("vote hash is not correct: %s", voteHash)
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
	rint := rand.Int()
	val := privval.GenFilePV(fmt.Sprintf("/tmp/vochainBenchmark%d", rint), fmt.Sprintf("/tmp/vochainBenchmark%d", rint))
	if s != nil {
		if err := s.AddValidator(val.GetPubKey(), 10); err != nil {
			t.Error(err)
		}
	}
}

/*
func TestRemoveValidator(t *testing.T) {
	s := testcommon.NewVochainStateWithValidators()
	if s != nil {
		if err := s.RemoveValidator(testcommon.ValidatorListHardcoded[1].GetAddress().String()); err != nil {
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
			if validators[i].PubKey.Equals(v.GetPubKey()) {
				t.Error("validator pubkey not match")
			}
		}
	}
}
*/

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

func TestCancelProcess(t *testing.T) {
	s := testcommon.NewVochainStateWithProcess()
	if s == nil {
		t.Error("cannot create state")
	}
	if err := s.CancelProcess("0xe9d5e8d791f51179e218c606f83f5967ab272292a6dbda887853d81f7a1d5105"); err != nil {
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

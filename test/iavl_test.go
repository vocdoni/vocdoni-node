package test

import (
	"fmt"
	"math/rand"
	"testing"

	"github.com/ethereum/go-ethereum/common"
	"github.com/tendermint/tendermint/privval"
	models "go.vocdoni.io/proto/build/go/models"

	"go.vocdoni.io/dvote/test/testcommon"
	"go.vocdoni.io/dvote/test/testcommon/testutil"
	"go.vocdoni.io/dvote/vochain"
)

func TestVochainState(t *testing.T) {
	t.Parallel()

	s, err := vochain.NewState(t.TempDir())
	if err != nil {
		t.Fatalf("cannot create vochain state (%s)", err)
	}

	// This used to panic due to nil *ImmutableTree fields.
	exists := s.EnvelopeExists([]byte("foo"), []byte("bar"), false)
	if exists {
		t.Errorf("expected EnvelopeExists to return false")
	}

	for i := 0; i < 10; i++ {
		s.Store.Tree(vochain.AppTree).Add([]byte(fmt.Sprintf("%d", i)), []byte(fmt.Sprintf("number %d", i)))
		s.Store.Tree(vochain.ProcessTree).Add([]byte(fmt.Sprintf("%d", i+1)), []byte(fmt.Sprintf("number %d", i+1)))
		s.Store.Tree(vochain.VoteTree).Add([]byte(fmt.Sprintf("%d", i+2)), []byte(fmt.Sprintf("number %d", i+2)))
	}
	s.Save()

	ah := s.Store.Hash()
	if ah == nil {
		t.Error(ah)
	}
}

func TestAddOracle(t *testing.T) {
	t.Parallel()
	s := testcommon.NewVochainStateWithOracles(t)
	if err := s.AddOracle(common.HexToAddress("414896B0BC763b8762456DB00F9c76EBd49979C4")); err != nil {
		t.Error(err)
	}
}

func TestRemoveOracle(t *testing.T) {
	t.Parallel()
	s := testcommon.NewVochainStateWithOracles(t)
	if err := s.RemoveOracle(testcommon.OracleListHardcoded[0]); err != nil {
		t.Error(err)
	}
}

func TestGetOracles(t *testing.T) {
	t.Parallel()

	s := testcommon.NewVochainStateWithOracles(t)
	oracles, err := s.Oracles(false)
	if err != nil {
		t.Error(err)
	}
	for i, v := range testcommon.OracleListHardcoded {
		if oracles[i] != v {
			t.Error("oracle address does not match")
		}
	}
}

func TestAddValidator(t *testing.T) {
	t.Parallel()

	s := testcommon.NewVochainStateWithValidators(t)
	rint := rand.Int()
	val := privval.GenFilePV(fmt.Sprintf("/tmp/vochainBenchmark%d", rint), fmt.Sprintf("/tmp/vochainBenchmark%d", rint))
	pubk, err := val.GetPubKey()
	if err != nil {
		t.Error(err)
	}
	validator := &models.Validator{
		Address: pubk.Address(),
		PubKey:  pubk.Bytes(),
		Power:   10,
	}
	if err := s.AddValidator(validator); err != nil {
		t.Error(err)
	}
}

/*
func TestRemoveValidator(t *testing.T) {
	t.Parallel()

	s := testcommon.NewVochainStateWithValidators(t)
	if err := s.RemoveValidator(testcommon.ValidatorListHardcoded[1].GetAddress().String()); err != nil {
		t.Error(err)
	}
}

func TestGetValidators(t *testing.T) {
	t.Parallel()

	s := testcommon.NewVochainStateWithValidators(t)
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
*/

func TestAddProcess(t *testing.T) {
	t.Parallel()

	s := testcommon.NewVochainStateWithProcess(t)
	if err := s.AddProcess(testcommon.ProcessHardcoded); err != nil {
		t.Error(err)
	}
}

func TestGetProcess(t *testing.T) {
	t.Parallel()

	s := testcommon.NewVochainStateWithProcess(t)
	if _, err := s.Process(testutil.Hex2byte(t, "e9d5e8d791f51179e218c606f83f5967ab272292a6dbda887853d81f7a1d5105"), false); err != nil {
		t.Error(err)
	}
}

func TestCancelProcess(t *testing.T) {
	t.Parallel()

	s := testcommon.NewVochainStateWithProcess(t)
	if err := s.CancelProcess(testutil.Hex2byte(t, "e9d5e8d791f51179e218c606f83f5967ab272292a6dbda887853d81f7a1d5105")); err != nil {
		t.Error(err)
	}
}

func TestAddVote(t *testing.T) {
	t.Parallel()

	s := testcommon.NewVochainStateWithProcess(t)
	if err := s.AddVote(testcommon.VoteHardcoded()); err != nil {
		t.Error(err)
	}
}

func TestGetEnvelope(t *testing.T) {
	t.Parallel()

	s := testcommon.NewVochainStateWithProcess(t)
	if err := s.AddVote(testcommon.VoteHardcoded()); err != nil {
		t.Error(err)
	}
	if _, err := s.Envelope(testutil.Hex2byte(t, "e9d5e8d791f51179e218c606f83f5967ab272292a6dbda887853d81f7a1d5105"),
		testutil.Hex2byte(t, "5592f1c18e2a15953f355c34b247d751da307338c994000b9a65db1dc14cc6c0"), false); err != nil {
		t.Error(err)
	}
}

func TestCountVotes(t *testing.T) {
	t.Parallel()

	s := testcommon.NewVochainStateWithProcess(t)
	if err := s.AddVote(testcommon.VoteHardcoded()); err != nil {
		t.Error(err)
	}
	if _, err := s.Envelope(testutil.Hex2byte(t, "e9d5e8d791f51179e218c606f83f5967ab272292a6dbda887853d81f7a1d5105"),
		testutil.Hex2byte(t, "5592f1c18e2a15953f355c34b247d751da307338c994000b9a65db1dc14cc6c0"), false); err != nil {
		t.Error(err)
	}
	c := s.CountVotes(testutil.Hex2byte(t, "e9d5e8d791f51179e218c606f83f5967ab272292a6dbda887853d81f7a1d5105"), false)
	if c != 1 {
		t.Errorf("number of votes should be 1, received %d", c)
	}
}

func TestGetEnvelopeList(t *testing.T) {
	t.Parallel()

	s := testcommon.NewVochainStateWithProcess(t)
	if err := s.AddVote(testcommon.VoteHardcoded()); err != nil {
		t.Error(err)
	}
	if _, err := s.Envelope(testutil.Hex2byte(t, "e9d5e8d791f51179e218c606f83f5967ab272292a6dbda887853d81f7a1d5105"),
		testutil.Hex2byte(t, "5592f1c18e2a15953f355c34b247d751da307338c994000b9a65db1dc14cc6c0"), false); err != nil {
		t.Error(err)
	}
	nullifiers := s.EnvelopeList(testutil.Hex2byte(t, "e9d5e8d791f51179e218c606f83f5967ab272292a6dbda887853d81f7a1d5105"), 0, 1, false)
	if string(nullifiers[0]) != string(testutil.Hex2byte(t, "5592f1c18e2a15953f355c34b247d751da307338c994000b9a65db1dc14cc6c0")) {
		t.Errorf("bad nullifier recovered, expected: 5592f1c18e2a15953f355c34b247d751da307338c994000b9a65db1dc14cc6c0, got: %s", nullifiers[0])
	}
}

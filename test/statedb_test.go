package test

import (
	"fmt"
	"math/rand"
	"testing"

	cometprivval "github.com/cometbft/cometbft/privval"
	qt "github.com/frankban/quicktest"
	"go.vocdoni.io/proto/build/go/models"

	"go.vocdoni.io/dvote/test/testcommon"
	"go.vocdoni.io/dvote/test/testcommon/testutil"
)

func TestAddValidator(t *testing.T) {
	t.Parallel()

	s := testcommon.NewVochainStateWithValidators(t)
	rint := rand.Int()
	tmp := t.TempDir()
	val, err := cometprivval.GenFilePV(
		fmt.Sprintf("%s/vochainBenchmark_keyfile%d", tmp, rint),
		fmt.Sprintf("%s/vochainBenchmark_statefile%d", tmp, rint),
		nil,
	)
	qt.Assert(t, err, qt.IsNil)

	pubk, err := val.GetPubKey()
	qt.Assert(t, err, qt.IsNil)
	validator := &models.Validator{
		Address: pubk.Address(),
		PubKey:  pubk.Bytes(),
		Power:   10,
	}
	err = s.AddValidator(validator)
	qt.Assert(t, err, qt.IsNil)
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

	s := testcommon.NewVochainState(t)
	err := s.AddProcess(testcommon.ProcessHardcoded)
	qt.Assert(t, err, qt.IsNil)
}

func TestGetProcess(t *testing.T) {
	t.Parallel()

	s := testcommon.NewVochainStateWithProcess(t)
	_, err := s.Process(testutil.Hex2byte(t, "e9d5e8d791f51179e218c606f83f5967ab272292a6dbda887853d81f7a1d5105"), false)
	qt.Assert(t, err, qt.IsNil)
}

func TestCancelProcess(t *testing.T) {
	t.Parallel()

	s := testcommon.NewVochainStateWithProcess(t)
	err := s.CancelProcess(testutil.Hex2byte(t, "e9d5e8d791f51179e218c606f83f5967ab272292a6dbda887853d81f7a1d5105"))
	qt.Assert(t, err, qt.IsNil)
}

func TestAddVote(t *testing.T) {
	t.Parallel()

	s := testcommon.NewVochainStateWithProcess(t)
	err := s.AddVote(testcommon.NewVoteHardcoded())
	qt.Assert(t, err, qt.IsNil)
}

func TestGetEnvelope(t *testing.T) {
	t.Parallel()

	s := testcommon.NewVochainStateWithProcess(t)
	err := s.AddVote(testcommon.NewVoteHardcoded())
	qt.Assert(t, err, qt.IsNil)
	_, err = s.Vote(testutil.Hex2byte(t, "e9d5e8d791f51179e218c606f83f5967ab272292a6dbda887853d81f7a1d5105"),
		testutil.Hex2byte(t, "5592f1c18e2a15953f355c34b247d751da307338c994000b9a65db1dc14cc6c0"), false)
	qt.Assert(t, err, qt.IsNil)
}

func TestCountVotes(t *testing.T) {
	t.Parallel()

	s := testcommon.NewVochainStateWithProcess(t)
	err := s.AddVote(testcommon.NewVoteHardcoded())
	qt.Assert(t, err, qt.IsNil)
	_, err = s.Vote(testutil.Hex2byte(t, "e9d5e8d791f51179e218c606f83f5967ab272292a6dbda887853d81f7a1d5105"),
		testutil.Hex2byte(t, "5592f1c18e2a15953f355c34b247d751da307338c994000b9a65db1dc14cc6c0"), false)
	qt.Assert(t, err, qt.IsNil)
	c, err := s.CountVotes(testutil.Hex2byte(t, "e9d5e8d791f51179e218c606f83f5967ab272292a6dbda887853d81f7a1d5105"), false)
	qt.Assert(t, err, qt.IsNil)
	qt.Assert(t, c, qt.Equals, uint64(1))
}

func TestGetEnvelopeList(t *testing.T) {
	t.Parallel()

	s := testcommon.NewVochainStateWithProcess(t)
	err := s.AddVote(testcommon.NewVoteHardcoded())
	qt.Assert(t, err, qt.IsNil)
	_, err = s.Vote(testutil.Hex2byte(t, "e9d5e8d791f51179e218c606f83f5967ab272292a6dbda887853d81f7a1d5105"),
		testutil.Hex2byte(t, "5592f1c18e2a15953f355c34b247d751da307338c994000b9a65db1dc14cc6c0"), false)
	qt.Assert(t, err, qt.IsNil)
	nullifiers := s.EnvelopeList(testutil.Hex2byte(t, "e9d5e8d791f51179e218c606f83f5967ab272292a6dbda887853d81f7a1d5105"), 0, 1, false)
	qt.Assert(t, string(nullifiers[0]), qt.Equals,
		string(testutil.Hex2byte(t, "5592f1c18e2a15953f355c34b247d751da307338c994000b9a65db1dc14cc6c0")))
}

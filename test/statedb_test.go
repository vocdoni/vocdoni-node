package test

import (
	"context"
	"fmt"
	"math/rand"
	"testing"

	"github.com/ethereum/go-ethereum/common"
	qt "github.com/frankban/quicktest"
	"github.com/tendermint/tendermint/privval"
	models "go.vocdoni.io/proto/build/go/models"

	"go.vocdoni.io/dvote/db"
	"go.vocdoni.io/dvote/test/testcommon"
	"go.vocdoni.io/dvote/test/testcommon/testutil"
	"go.vocdoni.io/dvote/util"
	"go.vocdoni.io/dvote/vochain"
)

func TestVochainState(t *testing.T) {
	t.Parallel()

	s, err := vochain.NewState(db.TypePebble, t.TempDir())
	qt.Assert(t, err, qt.IsNil)
	defer s.Close()

	// This used to panic due to nil *ImmutableTree fields.
	exists, err := s.EnvelopeExists(util.RandomBytes(32), util.RandomBytes(32), false)
	qt.Assert(t, err, qt.IsNil)
	qt.Assert(t, exists, qt.Equals, false)

	s.Tx.Add(vochain.ProcessesCfg.Key(), make([]byte, vochain.ProcessesCfg.HashFunc().Len()))
	for i := 0; i < 10; i++ {
		s.Tx.Add([]byte(fmt.Sprintf("%d", i)), []byte(fmt.Sprintf("number %d", i)))
		s.Tx.DeepAdd([]byte(fmt.Sprintf("%d", i+1)),
			[]byte(fmt.Sprintf("number %d", i+1)), vochain.ProcessesCfg)
	}
	s.Save()

	_, err = s.Store.Hash()
	qt.Assert(t, err, qt.IsNil)
}

func TestAddOracle(t *testing.T) {
	t.Parallel()
	s := testcommon.NewVochainStateWithOracles(t)
	err := s.AddOracle(common.HexToAddress("414896B0BC763b8762456DB00F9c76EBd49979C4"))
	qt.Assert(t, err, qt.IsNil)
}

func TestRemoveOracle(t *testing.T) {
	t.Parallel()
	s := testcommon.NewVochainStateWithOracles(t)
	err := s.RemoveOracle(testcommon.OracleListHardcoded[0])
	qt.Assert(t, err, qt.IsNil)
}

func TestGetOracles(t *testing.T) {
	t.Parallel()

	s := testcommon.NewVochainStateWithOracles(t)
	oracles, err := s.Oracles(false)
	qt.Assert(t, err, qt.IsNil)
	qt.Assert(t, oracles, qt.ContentEquals, testcommon.OracleListHardcoded)
}

func TestAddValidator(t *testing.T) {
	t.Parallel()

	s := testcommon.NewVochainStateWithValidators(t)
	rint := rand.Int()
	tmp := t.TempDir()
	val, err := privval.GenFilePV(
		fmt.Sprintf("%s/vochainBenchmark_keyfile%d", tmp, rint),
		fmt.Sprintf("%s/vochainBenchmark_statefile%d", tmp, rint),
		"ed25519",
	)

	qt.Assert(t, err, qt.IsNil)
	pubk, err := val.GetPubKey(context.Background())
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
	_, err = s.Envelope(testutil.Hex2byte(t, "e9d5e8d791f51179e218c606f83f5967ab272292a6dbda887853d81f7a1d5105"),
		testutil.Hex2byte(t, "5592f1c18e2a15953f355c34b247d751da307338c994000b9a65db1dc14cc6c0"), false)
	qt.Assert(t, err, qt.IsNil)
}

func TestCountVotes(t *testing.T) {
	t.Parallel()

	s := testcommon.NewVochainStateWithProcess(t)
	err := s.AddVote(testcommon.NewVoteHardcoded())
	qt.Assert(t, err, qt.IsNil)
	_, err = s.Envelope(testutil.Hex2byte(t, "e9d5e8d791f51179e218c606f83f5967ab272292a6dbda887853d81f7a1d5105"),
		testutil.Hex2byte(t, "5592f1c18e2a15953f355c34b247d751da307338c994000b9a65db1dc14cc6c0"), false)
	qt.Assert(t, err, qt.IsNil)
	c := s.CountVotes(testutil.Hex2byte(t, "e9d5e8d791f51179e218c606f83f5967ab272292a6dbda887853d81f7a1d5105"), false)
	qt.Assert(t, c, qt.Equals, uint32(1))
}

func TestGetEnvelopeList(t *testing.T) {
	t.Parallel()

	s := testcommon.NewVochainStateWithProcess(t)
	err := s.AddVote(testcommon.NewVoteHardcoded())
	qt.Assert(t, err, qt.IsNil)
	_, err = s.Envelope(testutil.Hex2byte(t, "e9d5e8d791f51179e218c606f83f5967ab272292a6dbda887853d81f7a1d5105"),
		testutil.Hex2byte(t, "5592f1c18e2a15953f355c34b247d751da307338c994000b9a65db1dc14cc6c0"), false)
	qt.Assert(t, err, qt.IsNil)
	nullifiers := s.EnvelopeList(testutil.Hex2byte(t, "e9d5e8d791f51179e218c606f83f5967ab272292a6dbda887853d81f7a1d5105"), 0, 1, false)
	qt.Assert(t, string(nullifiers[0]), qt.Equals,
		string(testutil.Hex2byte(t, "5592f1c18e2a15953f355c34b247d751da307338c994000b9a65db1dc14cc6c0")))
}

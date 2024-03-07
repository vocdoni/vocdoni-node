package testcommon

import (
	"encoding/base64"
	"math/rand"
	"strconv"
	"testing"
	"time"

	cometprivval "github.com/cometbft/cometbft/privval"

	"go.vocdoni.io/dvote/db"
	"go.vocdoni.io/dvote/test/testcommon/testutil"
	"go.vocdoni.io/dvote/vochain"
	"go.vocdoni.io/dvote/vochain/indexer"
	"go.vocdoni.io/dvote/vochain/state"
	"go.vocdoni.io/proto/build/go/models"
)

var (
	SignerPrivKey = "e0aa6db5a833531da4d259fb5df210bae481b276dc4c2ab6ab9771569375aed5"

	// NewVoteHardcoded needs to be a constructor, since multiple tests will
	// modify its value. We need a different pointer for each test.
	NewVoteHardcoded = func() *state.Vote {
		vp, _ := base64.StdEncoding.DecodeString("eyJ0eXBlIjoicG9sbC12b3RlIiwibm9uY2UiOiI1NTkyZjFjMThlMmExNTk1M2YzNTVjMzRiMjQ3ZDc1MWRhMzA3MzM4Yzk5NDAwMGI5YTY1ZGIxZGMxNGNjNmMwIiwidm90ZXMiOlsxLDIsMV19")
		return &state.Vote{
			ProcessID:   testutil.Hex2byte(nil, "e9d5e8d791f51179e218c606f83f5967ab272292a6dbda887853d81f7a1d5105"),
			Nullifier:   testutil.Hex2byte(nil, "5592f1c18e2a15953f355c34b247d751da307338c994000b9a65db1dc14cc6c0"), // nullifier and nonce are the same here
			VotePackage: vp,
		}
	}

	ProcessHardcoded = &models.Process{
		ProcessId:     testutil.Hex2byte(nil, "e9d5e8d791f51179e218c606f83f5967ab272292a6dbda887853d81f7a1d5105"),
		EntityId:      testutil.Hex2byte(nil, "180dd5765d9f7ecef810b565a2e5bd14a3ccd536c442b3de74867df552855e85"),
		CensusRoot:    testutil.Hex2byte(nil, "0a975f5cf517899e6116000fd366dc0feb34a2ea1b64e9b213278442dd9852fe"),
		CensusOrigin:  models.CensusOrigin_OFF_CHAIN_TREE,
		Duration:      uint32((time.Minute * 60).Seconds()),
		EnvelopeType:  &models.EnvelopeType{},
		Mode:          &models.ProcessMode{},
		Status:        models.ProcessStatus_READY,
		VoteOptions:   &models.ProcessVoteOptions{MaxCount: 16, MaxValue: 16},
		MaxCensusSize: 1000,
	}

	StateDBProcessHardcoded = &models.StateDBProcess{
		Process:   ProcessHardcoded,
		VotesRoot: make([]byte, 32),
	}
)

func NewVochainState(tb testing.TB) *state.State {
	s, err := state.New(db.TypePebble, tb.TempDir())
	if err != nil {
		tb.Fatal(err)
	}
	tb.Cleanup(func() { s.Close() })
	return s
}

func NewVochainStateWithValidators(tb testing.TB) *state.State {
	s := NewVochainState(tb)
	vals := make([]*cometprivval.FilePV, 2)
	rint := rand.Int()
	vals[0] = cometprivval.GenFilePV(
		"/tmp/"+strconv.Itoa(rint),
		"/tmp/"+strconv.Itoa(rint),
	)
	rint = rand.Int()
	vals[1] = cometprivval.GenFilePV(
		"/tmp/"+strconv.Itoa(rint),
		"/tmp/"+strconv.Itoa(rint),
	)
	validator0 := &models.Validator{
		Address: vals[0].Key.Address.Bytes(),
		PubKey:  vals[0].Key.PubKey.Bytes(),
		Power:   10,
	}
	validator1 := &models.Validator{
		Address: vals[0].Key.Address.Bytes(),
		PubKey:  vals[0].Key.PubKey.Bytes(),
		Power:   10,
	}
	if err := s.AddValidator(validator0); err != nil {
		tb.Fatal(err)
	}
	if err := s.AddValidator(validator1); err != nil {
		tb.Fatal(err)
	}
	return s
}

func NewVochainStateWithProcess(tb testing.TB) *state.State {
	s := NewVochainState(tb)
	p := StateDBProcessHardcoded.GetProcess()
	// add process
	if err := s.AddProcess(p); err != nil {
		tb.Fatal(err)
	}
	return s
}

func NewMockIndexer(tb testing.TB, vnode *vochain.BaseApplication) *indexer.Indexer {
	tb.Log("starting vochain indexer")
	sc, err := indexer.New(vnode, indexer.Options{DataDir: tb.TempDir()})
	if err != nil {
		tb.Fatal(err)
	}
	sc.AfterSyncBootstrap(true)
	return sc
}

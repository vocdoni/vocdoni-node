package testcommon

import (
	"encoding/base64"
	"math/rand"
	"strconv"
	"testing"

	secp "github.com/cometbft/cometbft/crypto/secp256k1"
	"github.com/cometbft/cometbft/privval"

	"go.vocdoni.io/dvote/db"
	"go.vocdoni.io/dvote/test/testcommon/testutil"
	"go.vocdoni.io/dvote/util"
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
		BlockCount:    1000,
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

	// privKey e0aa6db5a833531da4d259fb5df210bae481b276dc4c2ab6ab9771569375aed5 for address 06d0d2c41f4560f8ffea1285f44ce0ffa2e19ef0
	HardcodedNewProcessTx = &models.NewProcessTx{
		Txtype:  models.TxType_NEW_PROCESS,
		Process: ProcessHardcoded,
		Nonce:   0,
	}

	HardcodedNewVoteTx = &models.VoteEnvelope{
		Nonce:     util.RandomBytes(32),
		Nullifier: testutil.Hex2byte(nil, "5592f1c18e2a15953f355c34b247d751da307338c994000b9a65db1dc14cc6c0"),
		ProcessId: testutil.Hex2byte(nil, "e9d5e8d791f51179e218c606f83f5967ab272292a6dbda887853d81f7a1d5105"),
		Proof: &models.Proof{Payload: &models.Proof_Graviton{Graviton: &models.ProofGraviton{
			Siblings: testutil.Hex2byte(nil, "00030000000000000000000000000000000000000000000000000000000000070ab34471caaefc9bb249cb178335f367988c159f3907530ef7daa1e1bf0c9c7a218f981be7c0c46ffa345d291abb36a17c22722814fb0110240b8640fd1484a6268dc2f0fc2152bf83c06566fbf155f38b8293033d4779a63bba6c7157fd10c8"),
		}}},
		VotePackage: testutil.B642byte(nil, "eyJ0eXBlIjoicG9sbC12b3RlIiwibm9uY2UiOiI1NTkyZjFjMThlMmExNTk1M2YzNTVjMzRiMjQ3ZDc1MWRhMzA3MzM4Yzk5NDAwMGI5YTY1ZGIxZGMxNGNjNmMwIiwidm90ZXMiOlsxLDIsMV19"),
	}

	power                        = uint64(10)
	HardcodedAdminTxAddValidator = &models.AdminTx{
		Txtype:  models.TxType_ADD_VALIDATOR,
		Address: testutil.Hex2byte(nil, "5DC922017285EC24415F3E7ECD045665EADA8B5A"),
		Nonce:   0,
		Power:   &power,
	}

	HardcodedAdminTxRemoveValidator = &models.AdminTx{
		Txtype:  models.TxType_REMOVE_VALIDATOR,
		Address: testutil.Hex2byte(nil, "5DC922017285EC24415F3E7ECD045665EADA8B5A"),
		Nonce:   0,
	}
)

func NewVochainState(tb testing.TB) *state.State {
	s, err := state.NewState(db.TypePebble, tb.TempDir())
	if err != nil {
		tb.Fatal(err)
	}
	tb.Cleanup(func() { s.Close() })
	return s
}

func NewVochainStateWithValidators(tb testing.TB) *state.State {
	s := NewVochainState(tb)
	vals := make([]*privval.FilePV, 2)
	rint := rand.Int()
	var err error
	vals[0] = privval.NewFilePV(
		secp.GenPrivKey(),
		"/tmp/"+strconv.Itoa(rint),
		"/tmp/"+strconv.Itoa(rint),
	)
	if err != nil {
		tb.Fatal(err)
	}
	rint = rand.Int()
	vals[1] = privval.NewFilePV(
		secp.GenPrivKey(),
		"/tmp/"+strconv.Itoa(rint),
		"/tmp/"+strconv.Itoa(rint),
	)
	if err != nil {
		tb.Fatal(err)
	}
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
	sc, err := indexer.NewIndexer(tb.TempDir(), vnode, true)
	if err != nil {
		tb.Fatal(err)
	}
	sc.AfterSyncBootstrap(true)
	return sc
}

package indexer

import (
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	stdlog "log"
	"math/big"
	"sync"
	"testing"

	qt "github.com/frankban/quicktest"
	"github.com/pressly/goose/v3"
	"go.vocdoni.io/dvote/crypto/ethereum"
	"go.vocdoni.io/dvote/crypto/nacl"
	"go.vocdoni.io/dvote/log"
	"go.vocdoni.io/dvote/test/testcommon/testvoteproof"
	"go.vocdoni.io/dvote/types"
	"go.vocdoni.io/dvote/util"
	"go.vocdoni.io/dvote/vochain"
	"go.vocdoni.io/dvote/vochain/results"
	"go.vocdoni.io/dvote/vochain/state"
	"go.vocdoni.io/dvote/vochain/transaction/vochaintx"
	models "go.vocdoni.io/proto/build/go/models"
	"google.golang.org/protobuf/proto"
)

func init() {
	// keep the tests silent
	goose.SetLogger(stdlog.New(io.Discard, "", 0))
}

func newTestIndexer(tb testing.TB, app *vochain.BaseApplication, countLiveResults bool) *Indexer {
	idx, err := NewIndexer(tb.TempDir(), app, true)
	if err != nil {
		tb.Fatal(err)
	}
	idx.skipTargetHeightSleeps = true
	tb.Cleanup(func() {
		if err := idx.Close(); err != nil {
			tb.Error(err)
		}
	})
	return idx
}

func newTestIndexerNoCleanup(dataDir string, app *vochain.BaseApplication, countLiveResults bool) (*Indexer, error) {
	idx, err := NewIndexer(dataDir, app, true)
	if err != nil {
		return nil, err
	}
	idx.skipTargetHeightSleeps = true
	return idx, nil
}

func TestEntityList(t *testing.T) {
	for _, count := range []int{2, 100, 155} {
		t.Run(fmt.Sprintf("count=%03d", count), func(t *testing.T) {
			testEntityList(t, count)
		})
	}
}

func testEntityList(t *testing.T, entityCount int) {
	app := vochain.TestBaseApplication(t)
	idx := newTestIndexer(t, app, true)
	for i := 0; i < entityCount; i++ {
		pid := util.RandomBytes(32)
		eid := util.RandomBytes(20)
		if err := app.State.AddProcess(&models.Process{
			ProcessId:     pid,
			EntityId:      eid,
			BlockCount:    10,
			VoteOptions:   &models.ProcessVoteOptions{MaxCount: 8, MaxValue: 3},
			EnvelopeType:  &models.EnvelopeType{},
			MaxCensusSize: 1000,
		}); err != nil {
			t.Fatal(err)
		}
		if i%5 == 1 {
			app.AdvanceTestBlock()
		}
	}
	app.AdvanceTestBlock()

	qt.Assert(t, idx.EntityCount(), qt.Equals, uint64(entityCount))

	entitiesByID := make(map[string]bool)
	last := 0
	for len(entitiesByID) <= entityCount {
		list := idx.EntityList(10, last, "")
		if len(list) < 1 {
			t.Log("list is empty")
			break
		}
		for _, e := range list {
			if entitiesByID[e.String()] {
				t.Fatalf("found duplicated entity: %s", e)
			}
			entitiesByID[e.String()] = true
		}
		last += 10
	}
	if len(entitiesByID) < entityCount {
		t.Fatalf("expected %d entities, got %d", entityCount, len(entitiesByID))
	}
}

func TestEntitySearch(t *testing.T) {
	app := vochain.TestBaseApplication(t)
	idx := newTestIndexer(t, app, true)

	entityIds := []string{
		"1011d50537fa164b6fef261141797bbe4014526e",
		"2011d50537fa164b6fef261141797bbe4014526e",
		"3011d50537fa164b6fef261141797bbe4014526e",
		"4011d50537fa164b6fef261141797bbe4014526e",
		"5011d50537fa164b6fef261141797bbe4014526e",
		"6011d50537fa164b6fef261141797bbe4014526e",
		"7011d50537fa164b6fef261141797bbe4014526e",
		"8011d50537fa164b6fef261141797bbe4014526e",
		"9011d50537fa164b6fef261141797bbe4014526e",
	}
	// Add random entities before searchable ones
	for i := 0; i < 5; i++ {
		pid := util.RandomBytes(32)
		if err := app.State.AddProcess(&models.Process{
			ProcessId:     pid,
			EntityId:      util.RandomBytes(20),
			BlockCount:    10,
			VoteOptions:   &models.ProcessVoteOptions{MaxCount: 8, MaxValue: 3},
			EnvelopeType:  &models.EnvelopeType{},
			MaxCensusSize: 1000,
		}); err != nil {
			t.Fatal(err)
		}
		if i%5 == 1 {
			app.AdvanceTestBlock()
		}
	}
	for i, entity := range entityIds {
		pid := util.RandomBytes(32)
		entityId, err := hex.DecodeString(entity)
		if err != nil {
			t.Fatal(err)
		}
		if err := app.State.AddProcess(&models.Process{
			ProcessId:     pid,
			EntityId:      entityId,
			BlockCount:    10,
			VoteOptions:   &models.ProcessVoteOptions{MaxCount: 8, MaxValue: 3},
			EnvelopeType:  &models.EnvelopeType{},
			MaxCensusSize: 1000,
		}); err != nil {
			t.Fatal(err)
		}
		if i%5 == 1 {
			app.AdvanceTestBlock()
		}
	}
	// Add random entities after searchable ones
	for i := 0; i < 5; i++ {
		pid := util.RandomBytes(32)
		if err := app.State.AddProcess(&models.Process{
			ProcessId:     pid,
			EntityId:      util.RandomBytes(20),
			BlockCount:    10,
			VoteOptions:   &models.ProcessVoteOptions{MaxCount: 8, MaxValue: 3},
			EnvelopeType:  &models.EnvelopeType{},
			MaxCensusSize: 1000,
		}); err != nil {
			t.Fatal(err)
		}
		if i%5 == 1 {
			app.AdvanceTestBlock()
		}
	}
	app.AdvanceTestBlock()
	var list []types.HexBytes
	// Exact entity search
	list = idx.EntityList(10, 0, "4011d50537fa164b6fef261141797bbe4014526e")
	if len(list) < 1 {
		t.Fatalf("expected 1 entity, got %d", len(list))
	}
	// Search for nonexistent entity
	list = idx.EntityList(10, 0, "4011d50537fa164b6fef261141797bbe4014526f")
	if len(list) > 0 {
		t.Fatalf("expected 0 entities, got %d", len(list))
	}
	// Search containing part of all manually-defined entities
	list = idx.EntityList(10, 0, "011d50537fa164b6fef261141797bbe4014526e")
	log.Info(list)
	if len(list) < len(entityIds) {
		t.Fatalf("expected %d entities, got %d", len(entityIds), len(list))
	}
}

func TestProcessList(t *testing.T) {
	testProcessList(t, 10)
	testProcessList(t, 20)
	testProcessList(t, 155)
}

func testProcessList(t *testing.T, procsCount int) {
	app := vochain.TestBaseApplication(t)
	idx := newTestIndexer(t, app, true)

	// Add 10 entities and process for storing random content
	var eidOneProcess []byte // entity ID with one process
	for i := 0; i < 10; i++ {
		eid := util.RandomBytes(20)
		if i == 0 {
			eidOneProcess = eid
		}
		pid := util.RandomBytes(32)
		err := app.State.AddProcess(&models.Process{
			ProcessId:     pid,
			EntityId:      eid,
			VoteOptions:   &models.ProcessVoteOptions{MaxCount: 8, MaxValue: 3},
			EnvelopeType:  &models.EnvelopeType{},
			MaxCensusSize: 1000,
		})
		qt.Assert(t, err, qt.IsNil)
		if i%5 == 1 {
			app.AdvanceTestBlock()
		}

	}

	// For a entity, add entityCount processes (this will be the queried entity)
	eidProcsCount := util.RandomBytes(20) // entity ID with procsCount processes
	for i := 0; i < procsCount; i++ {
		pid := util.RandomBytes(32)
		err := app.State.AddProcess(&models.Process{
			ProcessId:     pid,
			EntityId:      eidProcsCount,
			VoteOptions:   &models.ProcessVoteOptions{MaxCount: 8, MaxValue: 3},
			EnvelopeType:  &models.EnvelopeType{},
			MaxCensusSize: 1000,
		})
		qt.Assert(t, err, qt.IsNil)
		if i%5 == 1 {
			app.AdvanceTestBlock()
		}
	}
	app.AdvanceTestBlock()

	procs := make(map[string]bool)
	last := 0
	for len(procs) < procsCount {
		list, err := idx.ProcessList(eidProcsCount, last, 10, "", 0, 0, "", false)
		if err != nil {
			t.Fatal(err)
		}
		if len(list) < 1 {
			t.Log("list is empty")
			break
		}
		for _, p := range list {
			if procs[string(p)] {
				t.Fatalf("found duplicated entity: %x", p)
			}
			procs[string(p)] = true
		}
		last += 10
	}
	qt.Assert(t, procs, qt.HasLen, procsCount)

	_, err := idx.ProcessList(nil, 0, 64, "", 0, 0, "", false)
	qt.Assert(t, err, qt.IsNil)

	qt.Assert(t, idx.ProcessCount(eidOneProcess), qt.Equals, uint64(1))
	qt.Assert(t, idx.ProcessCount(eidProcsCount), qt.Equals, uint64(procsCount))
	qt.Assert(t, idx.ProcessCount(nil), qt.Equals, uint64(10+procsCount))
	qt.Assert(t, idx.ProcessCount([]byte("not an entity id that exists")), qt.Equals, uint64(0))
}

func TestProcessSearch(t *testing.T) {
	app := vochain.TestBaseApplication(t)
	idx := newTestIndexer(t, app, true)

	// Add 10 entities and process for storing random content
	for i := 0; i < 10; i++ {
		pid := util.RandomBytes(32)
		t.Logf("random process ID: %x", pid)
		err := app.State.AddProcess(&models.Process{
			ProcessId:     pid,
			EntityId:      util.RandomBytes(20),
			VoteOptions:   &models.ProcessVoteOptions{MaxCount: 8, MaxValue: 3},
			EnvelopeType:  &models.EnvelopeType{},
			MaxCensusSize: 1000,
		})
		qt.Assert(t, err, qt.IsNil)
		if i%5 == 1 {
			app.AdvanceTestBlock()
		}
	}

	processIds := []string{
		"1011d50537fa164b6fef261141797bbe4014526e",
		"2011d50537fa164b6fef261141797bbe4014526e",
		"3011d50537fa164b6fef261141797bbe4014526e",
		"4011d50537fa164b6fef261141797bbe4014526e",
		"5011d50537fa164b6fef261141797bbe4014526e",
		"6011d50537fa164b6fef261141797bbe4014526e",
		"7011d50537fa164b6fef261141797bbe4014526e",
		"8011d50537fa164b6fef261141797bbe4014526e",
		"9011d50537fa164b6fef261141797bbe4014526e",
	}
	pidExact := processIds[3]
	pidExactEncrypted := processIds[5]
	// For a entity, add 25 processes (this will be the queried entity)
	eidTest := util.RandomBytes(20)
	for i, process := range processIds {
		pid, err := hex.DecodeString(process)
		if err != nil {
			t.Fatal(err)
		}
		encrypted := process == pidExactEncrypted
		if err := app.State.AddProcess(&models.Process{
			ProcessId:     pid,
			EntityId:      eidTest,
			BlockCount:    10,
			VoteOptions:   &models.ProcessVoteOptions{MaxCount: 8, MaxValue: 3},
			EnvelopeType:  &models.EnvelopeType{EncryptedVotes: encrypted},
			MaxCensusSize: 1000,
		}); err != nil {
			t.Fatal(err)
		}
		if i%5 == 1 {
			app.AdvanceTestBlock()
		}
	}

	endedPIDs := []string{
		"10c6ca22d2c175a1fbdd15d7595ae532bb1094b5",
		"20c6ca22d2c175a1fbdd15d7595ae532bb1094b5",
	}
	// For a entity, add 25 processes (this will be the queried entity)
	for i, process := range endedPIDs {
		pid, err := hex.DecodeString(process)
		if err != nil {
			t.Fatal(err)
		}
		if err := app.State.AddProcess(&models.Process{
			ProcessId:     pid,
			EntityId:      eidTest,
			BlockCount:    10,
			VoteOptions:   &models.ProcessVoteOptions{MaxCount: 8, MaxValue: 3},
			EnvelopeType:  &models.EnvelopeType{},
			Status:        models.ProcessStatus_ENDED,
			MaxCensusSize: 1000,
		}); err != nil {
			t.Fatal(err)
		}
		if i%5 == 1 {
			app.AdvanceTestBlock()
		}
	}
	app.AdvanceTestBlock()

	// Exact process search
	list, err := idx.ProcessList(eidTest, 0, 10, pidExact, 0, 0, "", false)
	if err != nil {
		t.Fatal(err)
	}
	if len(list) < 1 {
		t.Fatalf("expected 1 process, got %d", len(list))
	}
	// Exact process search, with it being encrypted.
	// This once caused a sqlite bug due to a mistake in the SQL query.
	list, err = idx.ProcessList(eidTest, 0, 10, pidExactEncrypted, 0, 0, "", false)
	if err != nil {
		t.Fatal(err)
	}
	if len(list) < 1 {
		t.Fatalf("expected 1 process, got %d", len(list))
	}
	// Search for nonexistent process
	list, err = idx.ProcessList(eidTest, 0, 10,
		"4011d50537fa164b6fef261141797bbe4014526f", 0, 0, "", false)
	if err != nil {
		t.Fatal(err)
	}
	if len(list) > 0 {
		t.Fatalf("expected 0 processes, got %d", len(list))
	}
	// Search containing part of all manually-defined processes
	list, err = idx.ProcessList(eidTest, 0, 10,
		"011d50537fa164b6fef261141797bbe4014526e", 0, 0, "", false)
	if err != nil {
		t.Fatal(err)
	}
	if len(list) < len(processIds) {
		t.Fatalf("expected %d processes, got %d", len(processIds), len(list))
	}

	list, err = idx.ProcessList(eidTest, 0, 100,
		"0c6ca22d2c175a1fbdd15d7595ae532bb1094b5", 0, 0, "ENDED", false)
	if err != nil {
		t.Fatal(err)
	}
	if len(list) < len(endedPIDs) {
		t.Fatalf("expected %d processes, got %d", len(endedPIDs), len(list))
	}

	// Search with an exact Entity ID, but starting with a null byte.
	// This can trip up sqlite, as it assumes TEXT strings are NUL-terminated.
	list, err = idx.ProcessList([]byte("\x00foobar"), 0, 100, "", 0, 0, "", false)
	if err != nil {
		t.Fatal(err)
	}
	if len(list) != 0 {
		t.Fatalf("expected zero processes, got %d", len(list))
	}

	// list all processes, with a max of 10
	list, err = idx.ProcessList(nil, 0, 10, "", 0, 0, "", false)
	qt.Assert(t, err, qt.IsNil)
	qt.Assert(t, list, qt.HasLen, 10)

	// list all processes, with a max of 1000
	list, err = idx.ProcessList(nil, 0, 1000, "", 0, 0, "", false)
	qt.Assert(t, err, qt.IsNil)
	qt.Assert(t, list, qt.HasLen, 21)
}

func TestProcessListWithNamespaceAndStatus(t *testing.T) {
	app := vochain.TestBaseApplication(t)

	idx := newTestIndexer(t, app, true)

	// Add 10 processes with different namespaces (from 10 to 20) and status ENDED
	for i := 0; i < 10; i++ {
		pid := util.RandomBytes(32)
		err := app.State.AddProcess(&models.Process{
			ProcessId:     pid,
			EntityId:      util.RandomBytes(20),
			VoteOptions:   &models.ProcessVoteOptions{MaxCount: 8, MaxValue: 3},
			EnvelopeType:  &models.EnvelopeType{},
			Namespace:     uint32(10 + i),
			Status:        models.ProcessStatus_ENDED,
			MaxCensusSize: 1000,
		})
		qt.Assert(t, err, qt.IsNil)
		if i%5 == 1 {
			app.AdvanceTestBlock()
		}
	}

	// For a entity, add 10 processes on namespace 123 and status READY
	eid20 := util.RandomBytes(20)
	for i := 0; i < 10; i++ {
		pid := util.RandomBytes(32)
		err := app.State.AddProcess(&models.Process{
			ProcessId:     pid,
			EntityId:      eid20,
			VoteOptions:   &models.ProcessVoteOptions{MaxCount: 8, MaxValue: 3},
			EnvelopeType:  &models.EnvelopeType{},
			Namespace:     123,
			Status:        models.ProcessStatus_READY,
			MaxCensusSize: 1000,
		})
		qt.Assert(t, err, qt.IsNil)
		if i%5 == 1 {
			app.AdvanceTestBlock()
		}
	}
	app.AdvanceTestBlock()

	// Get the process list for namespace 123
	list, err := idx.ProcessList(eid20, 0, 100, "", 123, 0, "", false)
	qt.Assert(t, err, qt.IsNil)
	// Check there are exactly 10
	qt.Assert(t, len(list), qt.CmpEquals(), 10)

	// Get the process list for all namespaces
	list, err = idx.ProcessList(nil, 0, 100, "", 0, 0, "", false)
	qt.Assert(t, err, qt.IsNil)
	// Check there are exactly 10 + 10
	qt.Assert(t, len(list), qt.CmpEquals(), 20)

	// Get the process list for namespace 10
	list, err = idx.ProcessList(nil, 0, 100, "", 10, 0, "", false)
	qt.Assert(t, err, qt.IsNil)
	// Check there is exactly 1
	qt.Assert(t, len(list), qt.CmpEquals(), 1)

	// Get the process list for namespace 10
	list, err = idx.ProcessList(nil, 0, 100, "", 0, 0, "READY", false)
	qt.Assert(t, err, qt.IsNil)
	// Check there is exactly 1
	qt.Assert(t, len(list), qt.CmpEquals(), 10)
}

func TestResults(t *testing.T) {
	app := vochain.TestBaseApplication(t)
	idx := newTestIndexer(t, app, true)

	keys, root, proofs := testvoteproof.CreateKeysAndBuildCensus(t, 30)
	pid := util.RandomBytes(32)
	err := app.State.AddProcess(&models.Process{
		ProcessId:             pid,
		EnvelopeType:          &models.EnvelopeType{EncryptedVotes: true},
		Status:                models.ProcessStatus_READY,
		Mode:                  &models.ProcessMode{AutoStart: true},
		BlockCount:            40,
		EncryptionPrivateKeys: make([]string, 16),
		EncryptionPublicKeys:  make([]string, 16),
		VoteOptions:           &models.ProcessVoteOptions{MaxCount: 4, MaxValue: 1},
		CensusOrigin:          models.CensusOrigin_OFF_CHAIN_TREE,
		CensusRoot:            root,
		MaxCensusSize:         1000,
	})
	qt.Assert(t, err, qt.IsNil)

	app.AdvanceTestBlock()

	priv, err := nacl.DecodePrivate(fmt.Sprintf("%x", ethereum.HashRaw(util.RandomBytes(32))))
	if err != nil {
		t.Fatalf("cannot generate encryption key: (%s)", err)
	}
	ki := uint32(1)
	err = app.State.AddProcessKeys(&models.AdminTx{
		Txtype:              models.TxType_ADD_PROCESS_KEYS,
		ProcessId:           pid,
		EncryptionPublicKey: priv.Public().Bytes(),
		KeyIndex:            &ki,
	})
	qt.Assert(t, err, qt.IsNil)

	vp, err := json.Marshal(vochain.VotePackage{
		Nonce: fmt.Sprintf("%x", util.RandomBytes(32)),
		Votes: []int{1, 1, 1, 1},
	})
	qt.Assert(t, err, qt.IsNil)
	vp, err = priv.Encrypt(vp, nil)
	qt.Assert(t, err, qt.IsNil)

	for i := int32(0); i < 30; i++ {
		idx.Rollback()
		vote := &models.VoteEnvelope{
			Nonce: util.RandomBytes(32),
			Proof: &models.Proof{Payload: &models.Proof_Arbo{
				Arbo: &models.ProofArbo{
					Type:     models.ProofArbo_BLAKE2B,
					Siblings: proofs[i],
					KeyType:  models.ProofArbo_ADDRESS,
				}}},
			ProcessId:            pid,
			VotePackage:          vp,
			Nullifier:            util.RandomBytes(32),
			EncryptionKeyIndexes: []uint32{1},
		}
		voteTx, err := proto.Marshal(&models.Tx{Payload: &models.Tx_Vote{Vote: vote}})
		qt.Assert(t, err, qt.IsNil)
		signature, err := keys[i].SignVocdoniTx(voteTx, app.ChainID())
		qt.Check(t, err, qt.IsNil)

		signedTx, err := proto.Marshal(&models.SignedTx{
			Tx:        voteTx,
			Signature: signature,
		})
		qt.Assert(t, err, qt.IsNil)
		_, err = app.SendTx(signedTx)
		qt.Assert(t, err, qt.IsNil)

		app.AdvanceTestBlock()
		qt.Assert(t, err, qt.IsNil)
	}

	// Reveal process encryption keys
	err = app.State.RevealProcessKeys(&models.AdminTx{
		Txtype:               models.TxType_ADD_PROCESS_KEYS,
		ProcessId:            pid,
		EncryptionPrivateKey: priv.Bytes(),
		KeyIndex:             &ki,
	})
	qt.Assert(t, err, qt.IsNil)
	err = idx.updateProcess(pid)
	qt.Assert(t, err, qt.IsNil)
	err = idx.setResultsHeight(pid, app.Height())
	qt.Assert(t, err, qt.IsNil)
	err = idx.ComputeResult(pid)
	qt.Assert(t, err, qt.IsNil)

	// GetEnvelopes with a limit
	envelopes, err := idx.GetEnvelopes(pid, 10, 0, "")
	qt.Assert(t, err, qt.IsNil)
	qt.Assert(t, envelopes, qt.HasLen, 10)
	qt.Assert(t, envelopes[0].Height, qt.Equals, uint32(1))
	qt.Assert(t, envelopes[4].Height, qt.Equals, uint32(5))
	qt.Assert(t, envelopes[9].Height, qt.Equals, uint32(10))

	matchNullifier := fmt.Sprintf("%x", envelopes[9].Nullifier)
	matchHeight := envelopes[9].Height

	// GetEnvelopes with a limit and offset
	envelopes, err = idx.GetEnvelopes(pid, 5, 27, "")
	qt.Assert(t, err, qt.IsNil)
	qt.Assert(t, envelopes, qt.HasLen, 3)
	qt.Assert(t, envelopes[0].Height, qt.Equals, uint32(28))
	qt.Assert(t, envelopes[2].Height, qt.Equals, uint32(30))

	// GetEnvelopes without a match
	envelopes, err = idx.GetEnvelopes(pid, 10, 0, fmt.Sprintf("%x", util.RandomBytes(32)))
	qt.Assert(t, err, qt.IsNil)
	qt.Assert(t, envelopes, qt.HasLen, 0)

	// GetEnvelopes with one match by full nullifier
	envelopes, err = idx.GetEnvelopes(pid, 10, 0, matchNullifier)
	qt.Assert(t, err, qt.IsNil)
	qt.Assert(t, envelopes, qt.HasLen, 1)
	qt.Assert(t, envelopes[0].Height, qt.Equals, matchHeight)

	// GetEnvelopes with one match by partial nullifier
	envelopes, err = idx.GetEnvelopes(pid, 10, 0, matchNullifier[:29])
	qt.Assert(t, err, qt.IsNil)
	qt.Assert(t, envelopes, qt.HasLen, 1)
	qt.Assert(t, envelopes[0].Height, qt.Equals, matchHeight)

	// Test results
	result, err := idx.GetResults(pid)
	qt.Assert(t, err, qt.IsNil)
	log.Infof("results: %s", GetFriendlyResults(result.Votes))
	v0 := big.NewInt(0)
	v30 := big.NewInt(30)
	var value *big.Int
	for q := range result.Votes {
		for qi := range result.Votes[q] {
			if qi > 3 {
				t.Fatalf("found more questions that expected")
			}
			value = result.Votes[q][qi].MathBigInt()
			if qi != 1 && value.Cmp(v0) != 0 {
				t.Fatalf("result is not correct, %d is not 0 as expected", value.Uint64())
			}
			if qi == 1 && value.Cmp(v30) != 0 {
				t.Fatalf("result is not correct, %d is not 30 as expected", value.Uint64())
			}
		}
	}
	for _, q := range GetFriendlyResults(result.Votes) {
		for qi, v1 := range q {
			if qi > 3 {
				t.Fatalf("found more questions that expected")
			}
			if qi != 1 && v1 != "0" {
				t.Fatalf("result is not correct, %s is not 0 as expected", v1)
			}
			if qi == 1 && v1 != "30" {
				t.Fatalf("result is not correct, %s is not 300 as expected", v1)
			}
		}
	}
}

func TestLiveResults(t *testing.T) {
	app := vochain.TestBaseApplication(t)
	idx := newTestIndexer(t, app, true)

	pid := util.RandomBytes(32)
	if err := app.State.AddProcess(&models.Process{
		ProcessId:     pid,
		EnvelopeType:  &models.EnvelopeType{EncryptedVotes: false},
		Status:        models.ProcessStatus_READY,
		BlockCount:    10,
		VoteOptions:   &models.ProcessVoteOptions{MaxCount: 3, MaxValue: 100},
		Mode:          &models.ProcessMode{AutoStart: true},
		MaxCensusSize: 1000,
	}); err != nil {
		t.Fatal(err)
	}
	app.AdvanceTestBlock()

	// Add 100 votes
	vp, err := json.Marshal(vochain.VotePackage{
		Nonce: fmt.Sprintf("%x", util.RandomHex(32)),
		Votes: []int{1, 1, 1},
	})
	qt.Assert(t, err, qt.IsNil)
	r := &results.Results{
		Votes:        results.NewEmptyVotes(3, 100),
		Weight:       new(types.BigInt).SetUint64(0),
		VoteOpts:     &models.ProcessVoteOptions{MaxCount: 3, MaxValue: 100},
		EnvelopeType: &models.EnvelopeType{},
	}
	idx.addProcessToLiveResults(pid)
	for i := 0; i < 100; i++ {
		qt.Assert(t, idx.addLiveVote(
			pid,
			vp,
			new(big.Int).SetUint64(1),
			r),
			qt.IsNil)
	}
	qt.Assert(t, idx.commitVotes(pid, r, nil, 1), qt.IsNil)

	if live, err := idx.isOpenProcess(pid); !live || err != nil {
		t.Fatal(fmt.Errorf("isLiveResultsProcess returned false: %v", err))
	}

	// Test results
	result, err := idx.GetResults(pid)
	qt.Assert(t, err, qt.IsNil)

	v0 := big.NewInt(0)
	v100 := big.NewInt(100)
	var value *big.Int
	for q := range result.Votes {
		for qi := range result.Votes[q] {
			if qi > 100 {
				t.Fatalf("found more questions that expected")
			}
			value = result.Votes[q][qi].MathBigInt()
			if qi == 0 && value.Cmp(v0) != 0 {
				t.Fatalf("result is not correct, %d is not 0 as expected", value.Uint64())
			}
			if qi == 1 && value.Cmp(v100) != 0 {
				t.Fatalf("result is not correct, %d is not 100 as expected", value.Uint64())
			}
		}
	}
}

func TestAddVote(t *testing.T) {
	app := vochain.TestBaseApplication(t)

	idx := newTestIndexer(t, app, true)

	options := &models.ProcessVoteOptions{
		MaxCount:     3,
		MaxValue:     3,
		MaxTotalCost: 6,
		CostExponent: 1,
	}

	pid := util.RandomBytes(32)
	if err := app.State.AddProcess(&models.Process{
		ProcessId:     pid,
		EnvelopeType:  &models.EnvelopeType{EncryptedVotes: false},
		Status:        models.ProcessStatus_READY,
		BlockCount:    10,
		VoteOptions:   options,
		Mode:          &models.ProcessMode{AutoStart: true},
		MaxCensusSize: 1000,
	}); err != nil {
		t.Fatal(err)
	}
	app.AdvanceTestBlock()

	pr, err := idx.GetResults(pid)
	qt.Assert(t, err, qt.IsNil)
	// Should be fine
	err = pr.AddVote([]int{1, 2, 3}, nil, nil)
	qt.Assert(t, err, qt.IsNil)

	// Overflows maxTotalCost
	err = pr.AddVote([]int{2, 2, 3}, nil, nil)
	qt.Assert(t, err, qt.ErrorMatches, "max total cost overflow.*")

	// Overflows maxValue
	err = pr.AddVote([]int{1, 1, 4}, nil, nil)
	qt.Assert(t, err, qt.ErrorMatches, "max value overflow.*")

	// Overflows maxCount
	err = pr.AddVote([]int{1, 1, 1, 1}, nil, nil)
	qt.Assert(t, err, qt.ErrorMatches, "max count overflow.*")

	// Quadratic voting, 10 credits to distribute among 3 options
	pr.VoteOpts = &models.ProcessVoteOptions{
		MaxCount:     3,
		MaxValue:     0,
		MaxTotalCost: 10,
		CostExponent: 2,
	}

	// Should be fine 2^2 + 2^2 + 1^2 = 9
	err = pr.AddVote([]int{2, 2, 1}, nil, nil)
	qt.Assert(t, err, qt.IsNil)

	// Should be fine 3^2 + 0 + 0 = 9
	err = pr.AddVote([]int{3, 0, 0}, nil, nil)
	qt.Assert(t, err, qt.IsNil)

	// Should fail since 2^2 + 2^2 + 2^2 = 12
	err = pr.AddVote([]int{2, 2, 2}, nil, nil)
	qt.Assert(t, err, qt.ErrorMatches, "max total cost overflow.*")

	// Should fail since 4^2 = 16
	err = pr.AddVote([]int{4, 0, 0}, nil, nil)
	qt.Assert(t, err, qt.ErrorMatches, "max total cost overflow.*")

	// Check unique values work
	pr.EnvelopeType.UniqueValues = true
	err = pr.AddVote([]int{2, 1, 1}, nil, nil)
	qt.Assert(t, err, qt.ErrorMatches, "values are not unique")
}

var vote = func(v []int, idx *Indexer, pid []byte, weight *big.Int) error {
	vp, err := json.Marshal(vochain.VotePackage{
		Nonce: fmt.Sprintf("%x", util.RandomHex(32)),
		Votes: v,
	})
	if err != nil {
		return err
	}
	max := 0
	for _, i := range v {
		if i > max {
			max = i
		}
	}
	proc, err := idx.ProcessInfo(pid)
	if err != nil {
		return err
	}
	r := &results.Results{
		ProcessID: pid,
		Votes: results.NewEmptyVotes(
			int(proc.VoteOpts.MaxCount), int(proc.VoteOpts.MaxValue)+1),
		Weight:       new(types.BigInt).SetUint64(0),
		Signatures:   []types.HexBytes{},
		VoteOpts:     proc.VoteOpts,
		EnvelopeType: proc.Envelope,
	}
	idx.addProcessToLiveResults(pid)
	if err := idx.addLiveVote(pid, vp, weight, r); err != nil {
		return err
	}
	return idx.commitVotes(pid, r, nil, 1)
}

func TestBallotProtocolRateProduct(t *testing.T) {
	// Rate a product from 0 to 4
	app := vochain.TestBaseApplication(t)

	idx := newTestIndexer(t, app, true)

	// Rate 2 products from 0 to 4
	pid := util.RandomBytes(32)
	if err := app.State.AddProcess(&models.Process{
		ProcessId:     pid,
		EnvelopeType:  &models.EnvelopeType{EncryptedVotes: false},
		Status:        models.ProcessStatus_READY,
		BlockCount:    10,
		Mode:          &models.ProcessMode{AutoStart: true},
		VoteOptions:   &models.ProcessVoteOptions{MaxCount: 2, MaxValue: 4},
		MaxCensusSize: 1000,
	}); err != nil {
		t.Fatal(err)
	}

	app.AdvanceTestBlock()

	// Rate a product, exepected result: [ [1,0,1,0,2], [0,0,2,0,2] ]
	qt.Assert(t, vote([]int{4, 2}, idx, pid, nil), qt.IsNil)
	qt.Assert(t, vote([]int{4, 2}, idx, pid, nil), qt.IsNil)
	qt.Assert(t, vote([]int{2, 4}, idx, pid, nil), qt.IsNil)
	qt.Assert(t, vote([]int{0, 4}, idx, pid, nil), qt.IsNil)
	qt.Assert(t, vote([]int{0, 5}, idx, pid, nil), qt.ErrorMatches, ".*overflow.*")
	qt.Assert(t, vote([]int{0, 0, 0}, idx, pid, nil), qt.ErrorMatches, ".*")

	result, err := idx.GetResults(pid)
	qt.Assert(t, err, qt.IsNil)
	votes := GetFriendlyResults(result.Votes)
	qt.Assert(t, votes[1], qt.DeepEquals, []string{"0", "0", "2", "0", "2"})
	qt.Assert(t, votes[0], qt.DeepEquals, []string{"1", "0", "1", "0", "2"})
}

func TestBallotProtocolQuadratic(t *testing.T) {
	// Rate a product from 0 to 4
	app := vochain.TestBaseApplication(t)

	idx := newTestIndexer(t, app, true)

	// Rate 2 products from 0 to 4
	pid := util.RandomBytes(32)
	if err := app.State.AddProcess(&models.Process{
		ProcessId:     pid,
		EnvelopeType:  &models.EnvelopeType{EncryptedVotes: false, CostFromWeight: true},
		Status:        models.ProcessStatus_READY,
		BlockCount:    10,
		Mode:          &models.ProcessMode{AutoStart: true},
		VoteOptions:   &models.ProcessVoteOptions{MaxCount: 2, MaxValue: 0, CostExponent: 2},
		MaxCensusSize: 1000,
	}); err != nil {
		t.Fatal(err)
	}

	app.AdvanceTestBlock()

	// Quadratic voting, exepected result: [ [100016], [100033] ]
	//
	//  weight: 1000, votes: 10^2 + 28^2
	//  weight: 50, votes: 5^2 + 5^2
	//  weight: 20000000000,  100000^2 + 100000^2
	//  weight: 1, votes 1^2 + 0^2

	//  weight: 25, votes 5^2 + 1 // wrong
	//  weight: 20000000000, 100000^2 + 112345^2 // wrong

	// Good
	qt.Assert(t, vote([]int{10, 28}, idx, pid, new(big.Int).SetUint64(1000)), qt.IsNil)
	qt.Assert(t, vote([]int{5, 5}, idx, pid, new(big.Int).SetUint64(50)), qt.IsNil)
	qt.Assert(t, vote([]int{100000, 100000}, idx, pid, new(big.Int).SetUint64(20000000000)), qt.IsNil)
	qt.Assert(t, vote([]int{1, 0}, idx, pid, new(big.Int).SetUint64(1)), qt.IsNil)
	// Wrong
	qt.Assert(t, vote([]int{5, 2}, idx, pid, new(big.Int).SetUint64(25)),
		qt.ErrorMatches, ".*overflow.*")
	qt.Assert(t, vote([]int{100000, 112345}, idx, pid, new(big.Int).SetUint64(20000000000)),
		qt.ErrorMatches, ".*overflow.*")

	result, err := idx.GetResults(pid)
	qt.Assert(t, err, qt.IsNil)
	votes := GetFriendlyResults(result.Votes)
	qt.Assert(t, votes[0], qt.DeepEquals, []string{"100016"})
	qt.Assert(t, votes[1], qt.DeepEquals, []string{"100033"})
}

func TestBallotProtocolMultiChoice(t *testing.T) {
	// Choose your 3 favorite colours out of 5

	app := vochain.TestBaseApplication(t)

	idx := newTestIndexer(t, app, true)

	// Rate 2 products from 0 to 4
	pid := util.RandomBytes(32)
	if err := app.State.AddProcess(&models.Process{
		ProcessId:    pid,
		EnvelopeType: &models.EnvelopeType{EncryptedVotes: false},
		Status:       models.ProcessStatus_READY,
		Mode:         &models.ProcessMode{AutoStart: true},
		BlockCount:   10,
		VoteOptions: &models.ProcessVoteOptions{
			MaxCount:     5,
			MaxValue:     1,
			MaxTotalCost: 3,
			CostExponent: 1,
		},
		MaxCensusSize: 1000,
	}); err != nil {
		t.Fatal(err)
	}

	app.AdvanceTestBlock()

	// Multichoice (choose 3 ouf of 5):
	// - Vote Envelope: `[1,1,1,0,0]` `[0,1,1,1,0]` `[1,1,0,0,0]`
	// - Results: `[ [1, 2], [0, 3], [1, 2], [2, 1], [3, 0] ]`
	qt.Assert(t, vote([]int{1, 1, 1, 0, 0}, idx, pid, nil), qt.IsNil)
	qt.Assert(t, vote([]int{0, 1, 1, 1, 0}, idx, pid, nil), qt.IsNil)
	qt.Assert(t, vote([]int{1, 1, 0, 0, 0}, idx, pid, nil), qt.IsNil)
	qt.Assert(t, vote([]int{2, 1, 0, 0, 0}, idx, pid, nil), qt.ErrorMatches, ".*overflow.*")
	qt.Assert(t, vote([]int{1, 1, 1, 1, 0}, idx, pid, nil), qt.ErrorMatches, ".*overflow.*")

	result, err := idx.GetResults(pid)
	qt.Assert(t, err, qt.IsNil)
	votes := GetFriendlyResults(result.Votes)
	qt.Assert(t, votes[0], qt.DeepEquals, []string{"1", "2"})
	qt.Assert(t, votes[1], qt.DeepEquals, []string{"0", "3"})
	qt.Assert(t, votes[2], qt.DeepEquals, []string{"1", "2"})
	qt.Assert(t, votes[3], qt.DeepEquals, []string{"2", "1"})
	qt.Assert(t, votes[4], qt.DeepEquals, []string{"3", "0"})
}

func TestAfterSyncBootStrap(t *testing.T) {
	app := vochain.TestBaseApplication(t)
	dataDir := t.TempDir()
	idx, err := newTestIndexerNoCleanup(dataDir, app, true)
	qt.Assert(t, err, qt.IsNil)
	pid := util.RandomBytes(32)
	qt.Assert(t, app.IsSynchronizing(), qt.Equals, false)

	err = app.State.AddProcess(&models.Process{
		ProcessId:     pid,
		EnvelopeType:  &models.EnvelopeType{EncryptedVotes: false},
		Status:        models.ProcessStatus_READY,
		Mode:          &models.ProcessMode{AutoStart: true},
		StartBlock:    1,
		BlockCount:    10,
		MaxCensusSize: 1000,
		VoteOptions: &models.ProcessVoteOptions{
			MaxCount: 5,
			MaxValue: 1,
		},
	})
	qt.Assert(t, err, qt.IsNil)
	app.AdvanceTestBlock() // block 1

	proc, err := idx.ProcessInfo(pid)
	qt.Assert(t, err, qt.IsNil)
	qt.Assert(t, proc.FinalResults, qt.IsFalse)

	// Stop the indexer
	qt.Assert(t, idx.Close(), qt.IsNil)
	app.State.CleanEventListeners()

	// Add 10 votes to the election
	vp, err := json.Marshal(vochain.VotePackage{
		Nonce: fmt.Sprintf("%x", util.RandomHex(32)),
		Votes: []int{1, 1, 1},
	})
	qt.Assert(t, err, qt.IsNil)
	for i := 0; i < 10; i++ {
		v := &state.Vote{ProcessID: pid, VotePackage: vp, Nullifier: util.RandomBytes(32)}
		qt.Assert(t, app.State.AddVote(v), qt.IsNil)
	}

	// Save the current state with the 10 new votes
	app.State.Save()

	// Start the indexer again
	idx, err = newTestIndexerNoCleanup(dataDir, app, true)
	qt.Assert(t, err, qt.IsNil)
	results, err := idx.GetResults(pid)
	qt.Assert(t, err, qt.IsNil)
	qt.Assert(t, results.EnvelopeHeight, qt.Equals, uint64(0))

	// Run the AfterSyncBootstrap, which should update the results
	idx.AfterSyncBootstrap()
	results, err = idx.GetResults(pid)
	qt.Assert(t, err, qt.IsNil)
	qt.Assert(t, results.EnvelopeHeight, qt.Equals, uint64(10))
}

func TestCountVotes(t *testing.T) {
	app := vochain.TestBaseApplication(t)
	idx := newTestIndexer(t, app, true)
	pid := util.RandomBytes(32)

	err := app.State.AddProcess(&models.Process{
		ProcessId:     pid,
		EnvelopeType:  &models.EnvelopeType{EncryptedVotes: false},
		Status:        models.ProcessStatus_READY,
		Mode:          &models.ProcessMode{AutoStart: true},
		BlockCount:    10,
		MaxCensusSize: 1000,
		VoteOptions: &models.ProcessVoteOptions{
			MaxCount:     5,
			MaxValue:     1,
			MaxTotalCost: 3,
			CostExponent: 1,
		},
	})
	qt.Assert(t, err, qt.IsNil)
	app.AdvanceTestBlock()

	// Add 100 votes
	vp, err := json.Marshal(vochain.VotePackage{
		Nonce: fmt.Sprintf("%x", util.RandomHex(32)),
		Votes: []int{1, 1, 1},
	})
	qt.Assert(t, err, qt.IsNil)
	for i := 0; i < 100; i++ {
		v := &state.Vote{ProcessID: pid, VotePackage: vp, Nullifier: util.RandomBytes(32)}
		// Add votes to votePool with i as txIndex
		qt.Assert(t, app.State.AddVote(v), qt.IsNil)
	}

	// Add last vote with known nullifier
	nullifier := util.RandomBytes(32)
	v := &state.Vote{ProcessID: pid, VotePackage: vp, Nullifier: nullifier}
	qt.Assert(t, app.State.AddVote(v), qt.IsNil)

	// save the block height for comparing later
	blockHeight := app.Height()
	app.AdvanceTestBlock()

	// Test envelope height for this PID
	height, err := idx.GetEnvelopeHeight(pid)
	qt.Assert(t, err, qt.IsNil)
	qt.Assert(t, height, qt.CmpEquals(), uint64(101))

	// Test global envelope height (number of votes)
	height, err = idx.GetEnvelopeHeight([]byte{})
	qt.Assert(t, err, qt.IsNil)
	qt.Assert(t, height, qt.CmpEquals(), uint64(101))

	ref, err := idx.GetEnvelopeReference(nullifier)
	qt.Assert(t, err, qt.IsNil)
	qt.Assert(t, ref.Height, qt.CmpEquals(), blockHeight)

	// Note that txIndex is 0, because the votes are added directly to the sate,
	// while the txCounter only increments on DeliverTx() execution.
}

func TestOverwriteVotes(t *testing.T) {
	app := vochain.TestBaseApplication(t)
	idx := newTestIndexer(t, app, true)
	pid := util.RandomBytes(32)
	keys, root, proofs := testvoteproof.CreateKeysAndBuildCensus(t, 10)

	err := app.State.AddProcess(&models.Process{
		CensusRoot:    root,
		CensusOrigin:  models.CensusOrigin_OFF_CHAIN_TREE,
		ProcessId:     pid,
		EnvelopeType:  &models.EnvelopeType{EncryptedVotes: false},
		Status:        models.ProcessStatus_READY,
		Mode:          &models.ProcessMode{AutoStart: true},
		BlockCount:    10,
		MaxCensusSize: 1000,
		VoteOptions: &models.ProcessVoteOptions{
			MaxCount:          3,
			MaxValue:          2,
			MaxTotalCost:      8,
			CostExponent:      1,
			MaxVoteOverwrites: 2,
		},
	})
	qt.Assert(t, err, qt.IsNil)
	app.AdvanceTestBlock()

	// Send the first vote
	vp, err := json.Marshal(vochain.VotePackage{
		Nonce: fmt.Sprintf("%x", util.RandomHex(32)),
		Votes: []int{1, 1, 1},
	})
	qt.Assert(t, err, qt.IsNil)
	vote := &models.VoteEnvelope{
		Nonce: util.RandomBytes(8),
		Proof: &models.Proof{Payload: &models.Proof_Arbo{
			Arbo: &models.ProofArbo{
				Type:     models.ProofArbo_BLAKE2B,
				Siblings: proofs[0],
				KeyType:  models.ProofArbo_ADDRESS,
			}}},
		ProcessId:   pid,
		VotePackage: vp,
	}
	voteTx, err := proto.Marshal(&models.Tx{Payload: &models.Tx_Vote{Vote: vote}})
	qt.Assert(t, err, qt.IsNil)
	signature, err := keys[0].SignVocdoniTx(voteTx, app.ChainID())
	qt.Check(t, err, qt.IsNil)

	signedTx, err := proto.Marshal(&models.SignedTx{
		Tx:        voteTx,
		Signature: signature,
	})
	qt.Assert(t, err, qt.IsNil)
	response, err := app.SendTx(signedTx)
	qt.Assert(t, err, qt.IsNil)
	nullifier := response.Data.Bytes()
	qt.Assert(t, nullifier, qt.DeepEquals, state.GenerateNullifier(keys[0].Address(), pid))

	app.AdvanceTestBlock()

	stateVote, err := app.State.Vote(pid, nullifier, true)
	qt.Assert(t, err, qt.IsNil)
	qt.Assert(t, stateVote.OverwriteCount, qt.IsNil)

	// Check the results actually changed
	results, err := idx.GetResults(pid)
	qt.Assert(t, err, qt.IsNil)
	qt.Assert(t, GetFriendlyResults(results.Votes), qt.DeepEquals, [][]string{
		{"0", "1", "0"},
		{"0", "1", "0"},
		{"0", "1", "0"},
	})

	// Send the second
	vp, err = json.Marshal(vochain.VotePackage{
		Nonce: fmt.Sprintf("%x", util.RandomHex(32)),
		Votes: []int{2, 2, 2},
	})
	qt.Assert(t, err, qt.IsNil)
	vote.VotePackage = vp
	vote.Nonce = util.RandomBytes(32)

	voteTx, err = proto.Marshal(&models.Tx{Payload: &models.Tx_Vote{Vote: vote}})
	qt.Assert(t, err, qt.IsNil)
	signature, err = keys[0].SignVocdoniTx(voteTx, app.ChainID())
	qt.Check(t, err, qt.IsNil)
	signedTx, err = proto.Marshal(&models.SignedTx{
		Tx:        voteTx,
		Signature: signature,
	})
	qt.Assert(t, err, qt.IsNil)
	response, err = app.SendTx(signedTx)
	qt.Assert(t, err, qt.IsNil)
	qt.Assert(t, response.Code, qt.Equals, uint32(0))

	app.AdvanceTestBlock()

	// check envelope height for this PID
	height, err := idx.GetEnvelopeHeight(pid)
	qt.Assert(t, err, qt.IsNil)
	qt.Assert(t, height, qt.CmpEquals(), uint64(1))

	// check overwrite count is correct
	ref, err := idx.GetEnvelopeReference(nullifier)
	qt.Assert(t, err, qt.IsNil)
	qt.Assert(t, ref.OverwriteCount, qt.CmpEquals(), uint32(1))

	// check vote package is updated
	envelope, err := idx.GetEnvelope(nullifier)
	qt.Assert(t, err, qt.IsNil)
	qt.Assert(t, envelope.VotePackage, qt.DeepEquals, vp)

	// check the results are correct (only the second vote should be counted)
	results, err = idx.GetResults(pid)
	qt.Assert(t, err, qt.IsNil)
	qt.Assert(t, GetFriendlyResults(results.Votes), qt.DeepEquals, [][]string{
		{"0", "0", "1"},
		{"0", "0", "1"},
		{"0", "0", "1"},
	})

	// Send a third vote from a different key
	vp, err = json.Marshal(vochain.VotePackage{
		Nonce: fmt.Sprintf("%x", util.RandomHex(32)),
		Votes: []int{2, 2, 2},
	})
	qt.Assert(t, err, qt.IsNil)
	vote2 := &models.VoteEnvelope{
		Nonce: util.RandomBytes(8),
		Proof: &models.Proof{Payload: &models.Proof_Arbo{
			Arbo: &models.ProofArbo{
				Type:     models.ProofArbo_BLAKE2B,
				Siblings: proofs[1],
				KeyType:  models.ProofArbo_ADDRESS,
			}}},
		ProcessId:   pid,
		VotePackage: vp,
	}
	voteTx, err = proto.Marshal(&models.Tx{Payload: &models.Tx_Vote{Vote: vote2}})
	qt.Assert(t, err, qt.IsNil)
	signature, err = keys[1].SignVocdoniTx(voteTx, app.ChainID())
	qt.Check(t, err, qt.IsNil)

	signedTx, err = proto.Marshal(&models.SignedTx{
		Tx:        voteTx,
		Signature: signature,
	})
	qt.Assert(t, err, qt.IsNil)
	response, err = app.SendTx(signedTx)
	qt.Assert(t, err, qt.IsNil)
	qt.Assert(t, response.Code, qt.Equals, uint32(0))

	app.AdvanceTestBlock()

	// check the results again
	results, err = idx.GetResults(pid)
	qt.Assert(t, err, qt.IsNil)
	qt.Assert(t, GetFriendlyResults(results.Votes), qt.DeepEquals, [][]string{
		{"0", "0", "2"},
		{"0", "0", "2"},
		{"0", "0", "2"},
	})

	// Send the initial vote again (for third time)
	vp, err = json.Marshal(vochain.VotePackage{
		Nonce: fmt.Sprintf("%x", util.RandomHex(32)),
		Votes: []int{0, 0, 0},
	})
	qt.Assert(t, err, qt.IsNil)
	vote.VotePackage = vp
	vote.Nonce = util.RandomBytes(32)

	voteTx, err = proto.Marshal(&models.Tx{Payload: &models.Tx_Vote{Vote: vote}})
	qt.Assert(t, err, qt.IsNil)
	signature, err = keys[0].SignVocdoniTx(voteTx, app.ChainID())
	qt.Check(t, err, qt.IsNil)
	signedTx, err = proto.Marshal(&models.SignedTx{
		Tx:        voteTx,
		Signature: signature,
	})
	qt.Assert(t, err, qt.IsNil)
	response, err = app.SendTx(signedTx)
	qt.Assert(t, err, qt.IsNil)
	qt.Assert(t, response.Code, qt.Equals, uint32(0))

	app.AdvanceTestBlock()

	// check the results again
	results, err = idx.GetResults(pid)
	qt.Assert(t, err, qt.IsNil)
	qt.Assert(t, GetFriendlyResults(results.Votes), qt.DeepEquals, [][]string{
		{"1", "0", "1"},
		{"1", "0", "1"},
		{"1", "0", "1"},
	})

	// check the weight is correct (should be 2 because of the overwrite)
	qt.Assert(t, results.Weight.MathBigInt().Int64(), qt.Equals, int64(2))
}

func TestTxIndexer(t *testing.T) {
	app := vochain.TestBaseApplication(t)
	idx := newTestIndexer(t, app, true)

	getTxID := func(i, j int) [32]byte {
		return [32]byte{byte(i), byte(j)}
	}

	const totalBlocks = 10
	const txsPerBlock = 10
	for i := 0; i < totalBlocks; i++ {
		for j := 0; j < txsPerBlock; j++ {
			idx.OnNewTx(&vochaintx.VochainTx{
				TxID:        getTxID(i, j),
				TxModelType: "setAccount",
			}, uint32(i), int32(j))
		}
	}
	qt.Assert(t, idx.Commit(0), qt.IsNil)
	idx.WaitIdle()

	count, err := idx.TransactionCount()
	qt.Assert(t, err, qt.IsNil)
	const totalTxs = totalBlocks * txsPerBlock
	qt.Assert(t, count, qt.Equals, uint64(totalTxs))

	for i := 0; i < totalBlocks; i++ {
		for j := 0; j < txsPerBlock; j++ {
			ref, err := idx.GetTxReference(uint64(i*txsPerBlock + j + 1))
			qt.Assert(t, err, qt.IsNil)
			qt.Assert(t, ref.BlockHeight, qt.Equals, uint32(i))
			qt.Assert(t, ref.TxBlockIndex, qt.Equals, int32(j))
			qt.Assert(t, ref.TxType, qt.Equals, "setAccount")
			h := make([]byte, 32)
			id := getTxID(i, j)
			copy(h, id[:])
			hashRef, err := idx.GetTxHashReference(h)
			qt.Assert(t, err, qt.IsNil)
			qt.Assert(t, hashRef.BlockHeight, qt.Equals, uint32(i))
			qt.Assert(t, hashRef.TxBlockIndex, qt.Equals, int32(j))
		}
	}

	txs, err := idx.GetLastTxReferences(15, 0)
	qt.Assert(t, err, qt.IsNil)
	for i, tx := range txs {
		// Index is between 1 and totalCount.
		qt.Assert(t, tx.Index, qt.Equals, uint64(totalTxs-i))
		// BlockIndex and TxBlockIndex start at 0, so subtract 1.
		qt.Assert(t, tx.BlockHeight, qt.Equals, uint32(totalTxs-i-1)/txsPerBlock)
		qt.Assert(t, tx.TxBlockIndex, qt.Equals, int32(totalTxs-i-1)%txsPerBlock)
		qt.Assert(t, tx.TxType, qt.Equals, "setAccount")
	}

	txs, err = idx.GetLastTxReferences(1, 5)
	qt.Assert(t, err, qt.IsNil)
	qt.Assert(t, txs, qt.HasLen, 1)
	qt.Assert(t, txs[0].Index, qt.Equals, uint64(95))
}

// Test that we can do concurrent reads and writes to sqlite without running
// into "database is locked" errors.
func TestIndexerConcurrentDB(t *testing.T) {
	app := vochain.TestBaseApplication(t)
	idx := newTestIndexer(t, app, true)

	pid := util.RandomBytes(32)
	if err := app.State.AddProcess(&models.Process{
		ProcessId:     pid,
		EntityId:      util.RandomBytes(20),
		BlockCount:    10,
		VoteOptions:   &models.ProcessVoteOptions{MaxCount: 8, MaxValue: 3},
		EnvelopeType:  &models.EnvelopeType{},
		MaxCensusSize: 1000,
	}); err != nil {
		t.Fatal(err)
	}
	app.AdvanceTestBlock()
	if err := idx.setResultsHeight(pid, 123); err != nil {
		t.Error(err)
	}

	const concurrency = 300
	var wg sync.WaitGroup

	wg.Add(concurrency)
	for i := 0; i < concurrency; i++ {
		i := i
		go func() {
			defer wg.Done()
			if err := idx.setResultsHeight(pid, 123); err != nil {
				t.Errorf("iteration %d: %v", i, err)
			}
			if _, err := idx.ProcessInfo(pid); err != nil {
				t.Errorf("iteration %d: %v", i, err)
			}
		}()
	}
	wg.Wait()
}

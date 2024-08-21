package indexer

import (
	"bytes"
	"context"
	"encoding/hex"
	"fmt"
	"io"
	stdlog "log"
	"math/big"
	"path/filepath"
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
	"go.vocdoni.io/proto/build/go/models"
	"google.golang.org/protobuf/proto"
)

func init() {
	// keep the tests silent
	goose.SetLogger(stdlog.New(io.Discard, "", 0))
}

func newTestIndexer(tb testing.TB, app *vochain.BaseApplication) *Indexer {
	idx, err := New(app, Options{DataDir: tb.TempDir()})
	if err != nil {
		tb.Fatal(err)
	}
	tb.Cleanup(func() {
		if err := idx.Close(); err != nil {
			tb.Error(err)
		}
	})
	return idx
}

func TestBackup(t *testing.T) {
	app := vochain.TestBaseApplication(t)

	idx, err := New(app, Options{DataDir: t.TempDir()})
	qt.Assert(t, err, qt.IsNil)

	wantTotalVotes := func(want uint64) {
		got, err := idx.CountTotalVotes()
		qt.Assert(t, err, qt.IsNil)
		qt.Assert(t, got, qt.Equals, want)
	}

	vp, err := state.NewVotePackage([]int{1, 1, 1}).Encode()
	qt.Assert(t, err, qt.IsNil)

	// A new indexer has no votes.
	wantTotalVotes(0)

	// Add 10 votes and check they are counted.
	pid := util.RandomBytes(32)
	err = app.State.AddProcess(&models.Process{
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
	for i := 0; i < 10; i++ {
		v := &state.Vote{ProcessID: pid, VotePackage: vp, Nullifier: util.RandomBytes(32)}
		qt.Assert(t, app.State.AddVote(v), qt.IsNil)
	}
	app.AdvanceTestBlock()
	wantTotalVotes(10)

	// Back up the database.
	backupPath := filepath.Join(t.TempDir(), "backup")
	err = idx.SaveBackup(context.TODO(), backupPath)
	qt.Assert(t, err, qt.IsNil)

	// Add another 5 votes which aren't in the backup.
	for i := 0; i < 5; i++ {
		v := &state.Vote{ProcessID: pid, VotePackage: vp, Nullifier: util.RandomBytes(32)}
		qt.Assert(t, app.State.AddVote(v), qt.IsNil)
	}
	app.AdvanceTestBlock()
	wantTotalVotes(15)

	// Starting a new database without the backup should see zero votes.
	idx.Close()
	idx, err = New(app, Options{DataDir: t.TempDir()})
	qt.Assert(t, err, qt.IsNil)
	wantTotalVotes(0)

	// Starting a new database with the backup should see the votes from the backup.
	idx.Close()
	idx, err = New(app, Options{DataDir: t.TempDir(), ExpectBackupRestore: true})
	qt.Assert(t, err, qt.IsNil)
	err = idx.RestoreBackup(backupPath)
	qt.Assert(t, err, qt.IsNil)
	wantTotalVotes(10)

	// Close the last indexer.
	idx.Close()
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
	idx := newTestIndexer(t, app)
	baseProcess := &models.Process{
		BlockCount:    10,
		VoteOptions:   &models.ProcessVoteOptions{MaxCount: 8, MaxValue: 3},
		EnvelopeType:  &models.EnvelopeType{},
		MaxCensusSize: 1000,
	}
	for i := 0; i < entityCount; i++ {
		pid := util.RandomBytes(32)
		eid := util.RandomBytes(20)
		baseProcess.ProcessId = pid
		baseProcess.EntityId = eid
		if err := app.State.AddProcess(baseProcess); err != nil {
			t.Fatal(err)
		}
		if i%5 == 1 {
			app.AdvanceTestBlock()
		}
	}
	// add 1 more process to an entity that already has one to test for duplicates
	twoProcessesEntity := baseProcess.EntityId
	baseProcess.ProcessId = util.RandomBytes(32)
	if err := app.State.AddProcess(baseProcess); err != nil {
		t.Fatal(err)
	}
	app.AdvanceTestBlock()

	qt.Assert(t, idx.CountTotalEntities(), qt.Equals, uint64(entityCount))

	entitiesByID := make(map[string]bool)
	last := 0
	for len(entitiesByID) <= entityCount {
		list := idx.EntityList(10, last, "")
		if len(list) < 1 {
			t.Log("list is empty")
			break
		}
		for _, e := range list {
			if entitiesByID[string(e.EntityID)] {
				t.Fatalf("found duplicated entity: %x", e.EntityID)
			}
			entitiesByID[string(e.EntityID)] = true
			if bytes.Equal(e.EntityID, twoProcessesEntity) {
				qt.Assert(t, e.ProcessCount, qt.Equals, int64(2))
			} else {
				qt.Assert(t, e.ProcessCount, qt.Equals, int64(1))
			}
		}
		last += 10
	}
	if len(entitiesByID) < entityCount {
		t.Fatalf("expected %d entities, got %d", entityCount, len(entitiesByID))
	}
}

func TestEntitySearch(t *testing.T) {
	app := vochain.TestBaseApplication(t)
	idx := newTestIndexer(t, app)

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
	// Exact entity search
	list := idx.EntityList(10, 0, "4011d50537fa164b6fef261141797bbe4014526e")
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
	idx := newTestIndexer(t, app)

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

	// For an entity, add entityCount processes (this will be the queried entity)
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

	qt.Assert(t, idx.CountTotalProcesses(), qt.Equals, uint64(10+procsCount))
	countEntityProcs := func(eid []byte) int64 {
		list := idx.EntityList(1, 0, fmt.Sprintf("%x", eid))
		if len(list) == 0 {
			return -1
		}
		return list[0].ProcessCount
	}
	qt.Assert(t, countEntityProcs(eidOneProcess), qt.Equals, int64(1))
	qt.Assert(t, countEntityProcs(eidProcsCount), qt.Equals, int64(procsCount))
	qt.Assert(t, countEntityProcs([]byte("not an entity id that exists")), qt.Equals, int64(-1))
}

func TestProcessSearch(t *testing.T) {
	app := vochain.TestBaseApplication(t)
	idx := newTestIndexer(t, app)

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
	// For an entity, add 25 processes (this will be the queried entity)
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
	// For an entity, add 25 processes (this will be the queried entity)
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

	idx := newTestIndexer(t, app)

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

	// For an entity, add 10 processes on namespace 123 and status READY
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
	idx := newTestIndexer(t, app)

	keys, root, proofs := testvoteproof.CreateKeysAndBuildCensus(t, 30)
	pid := util.RandomBytes(32)
	err := app.State.AddProcess(&models.Process{
		ProcessId:    pid,
		EnvelopeType: &models.EnvelopeType{EncryptedVotes: true},
		Status:       models.ProcessStatus_READY,
		Mode: &models.ProcessMode{
			AutoStart:     true,
			Interruptible: true},
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

	vp, err := state.NewVotePackage([]int{1, 1, 1, 1}).Encode()
	qt.Assert(t, err, qt.IsNil)
	vp, err = priv.Encrypt(vp, nil)
	qt.Assert(t, err, qt.IsNil)

	for i := int32(0); i < 30; i++ {
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

	// Compute and set results to the state
	r, err := results.ComputeResults(pid, app.State)
	qt.Assert(t, err, qt.IsNil)
	err = app.State.SetProcessStatus(pid, models.ProcessStatus_ENDED, true)
	qt.Assert(t, err, qt.IsNil)
	err = app.State.SetProcessResults(pid, results.ResultsToProto(r))
	qt.Assert(t, err, qt.IsNil)

	// Update the process
	app.AdvanceTestBlock()

	// GetEnvelopes with a limit
	envelopes, err := idx.GetEnvelopes(pid, 10, 0, "")
	qt.Assert(t, err, qt.IsNil)
	qt.Assert(t, envelopes, qt.HasLen, 10)
	qt.Assert(t, envelopes[0].Height, qt.Equals, uint32(30))
	qt.Assert(t, envelopes[4].Height, qt.Equals, uint32(26))
	qt.Assert(t, envelopes[9].Height, qt.Equals, uint32(21))

	matchNullifier := fmt.Sprintf("%x", envelopes[9].Nullifier)
	matchHeight := envelopes[9].Height

	// GetEnvelopes with a limit and offset
	envelopes, err = idx.GetEnvelopes(pid, 5, 27, "")
	qt.Assert(t, err, qt.IsNil)
	qt.Assert(t, envelopes, qt.HasLen, 3)
	qt.Assert(t, envelopes[0].Height, qt.Equals, uint32(3))
	qt.Assert(t, envelopes[2].Height, qt.Equals, uint32(1))

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
	proc, err := idx.ProcessInfo(pid)
	qt.Assert(t, err, qt.IsNil)
	log.Infof("results: %s", friendlyResults(proc.ResultsVotes))
	v0 := big.NewInt(0)
	v30 := big.NewInt(30)
	var value *big.Int
	for q := range proc.ResultsVotes {
		for qi := range proc.ResultsVotes[q] {
			if qi > 3 {
				t.Fatalf("found more questions that expected")
			}
			value = proc.ResultsVotes[q][qi].MathBigInt()
			if qi != 1 && value.Cmp(v0) != 0 {
				t.Fatalf("result is not correct, %d is not 0 as expected", value.Uint64())
			}
			if qi == 1 && value.Cmp(v30) != 0 {
				t.Fatalf("result is not correct, %d is not 30 as expected", value.Uint64())
			}
		}
	}
	for _, q := range friendlyResults(proc.ResultsVotes) {
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
	idx := newTestIndexer(t, app)

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
	vp, err := state.NewVotePackage([]int{1, 1, 1}).Encode()
	qt.Assert(t, err, qt.IsNil)

	for i := 0; i < 100; i++ {
		v := &state.Vote{ProcessID: pid, VotePackage: vp, Nullifier: util.RandomBytes(32)}
		qt.Assert(t, app.State.AddVote(v), qt.IsNil)
	}

	// Test results
	app.AdvanceTestBlock()
	proc, err := idx.ProcessInfo(pid)
	qt.Assert(t, err, qt.IsNil)

	v0 := big.NewInt(0)
	v100 := big.NewInt(100)
	var value *big.Int
	for q := range proc.ResultsVotes {
		for qi := range proc.ResultsVotes[q] {
			if qi > 100 {
				t.Fatalf("found more questions that expected")
			}
			value = proc.ResultsVotes[q][qi].MathBigInt()
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

	idx := newTestIndexer(t, app)

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

	proc, err := idx.ProcessInfo(pid)
	pr := proc.Results()
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

var addVote = func(t *testing.T, app *vochain.BaseApplication, pid []byte, votes []int, weight *big.Int) {
	t.Helper()
	vp, err := state.NewVotePackage(votes).Encode()
	qt.Assert(t, err, qt.IsNil)
	v := &state.Vote{
		ProcessID:   pid,
		VotePackage: vp,
		Nullifier:   util.RandomBytes(32),
		Weight:      weight,
	}
	err = app.State.AddVote(v)
	qt.Assert(t, err, qt.IsNil)
}

func TestBallotProtocolRateProduct(t *testing.T) {
	// Rate a product from 0 to 4
	app := vochain.TestBaseApplication(t)

	idx := newTestIndexer(t, app)

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

	// Rate a product, expected result: [ [1,0,1,0,2], [0,0,2,0,2] ]
	addVote(t, app, pid, []int{4, 2}, nil)
	addVote(t, app, pid, []int{4, 2}, nil)
	addVote(t, app, pid, []int{2, 4}, nil)
	addVote(t, app, pid, []int{0, 4}, nil)
	addVote(t, app, pid, []int{0, 5}, nil)    // error: overflow
	addVote(t, app, pid, []int{0, 0, 0}, nil) // error: too many votes

	app.AdvanceTestBlock()
	proc, err := idx.ProcessInfo(pid)
	qt.Assert(t, err, qt.IsNil)
	votes := friendlyResults(proc.ResultsVotes)
	qt.Assert(t, votes[1], qt.DeepEquals, []string{"0", "0", "2", "0", "2"})
	qt.Assert(t, votes[0], qt.DeepEquals, []string{"1", "0", "1", "0", "2"})
}

func TestBallotProtocolQuadratic(t *testing.T) {
	// Rate a product from 0 to 4
	app := vochain.TestBaseApplication(t)

	idx := newTestIndexer(t, app)

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
	addVote(t, app, pid, []int{10, 28}, new(big.Int).SetUint64(1000))
	addVote(t, app, pid, []int{5, 5}, new(big.Int).SetUint64(50))
	addVote(t, app, pid, []int{100000, 100000}, new(big.Int).SetUint64(20000000000))
	addVote(t, app, pid, []int{1, 0}, new(big.Int).SetUint64(1))
	// Wrong; overflows
	addVote(t, app, pid, []int{5, 2}, new(big.Int).SetUint64(25))
	addVote(t, app, pid, []int{100000, 112345}, new(big.Int).SetUint64(20000000000))

	app.AdvanceTestBlock()
	proc, err := idx.ProcessInfo(pid)
	qt.Assert(t, err, qt.IsNil)
	votes := friendlyResults(proc.ResultsVotes)
	qt.Assert(t, votes[0], qt.DeepEquals, []string{"100016"})
	qt.Assert(t, votes[1], qt.DeepEquals, []string{"100033"})
}

func TestBallotProtocolMultiChoice(t *testing.T) {
	// Choose your 3 favorite colours out of 5

	app := vochain.TestBaseApplication(t)

	idx := newTestIndexer(t, app)

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
	addVote(t, app, pid, []int{1, 1, 1, 0, 0}, nil)
	addVote(t, app, pid, []int{0, 1, 1, 1, 0}, nil)
	addVote(t, app, pid, []int{1, 1, 0, 0, 0}, nil)
	addVote(t, app, pid, []int{2, 1, 0, 0, 0}, nil)
	addVote(t, app, pid, []int{1, 1, 1, 1, 0}, nil)

	app.AdvanceTestBlock()
	proc, err := idx.ProcessInfo(pid)
	qt.Assert(t, err, qt.IsNil)
	votes := friendlyResults(proc.ResultsVotes)
	qt.Assert(t, votes[0], qt.DeepEquals, []string{"1", "2"})
	qt.Assert(t, votes[1], qt.DeepEquals, []string{"0", "3"})
	qt.Assert(t, votes[2], qt.DeepEquals, []string{"1", "2"})
	qt.Assert(t, votes[3], qt.DeepEquals, []string{"2", "1"})
	qt.Assert(t, votes[4], qt.DeepEquals, []string{"3", "0"})
}

func TestAfterSyncBootStrap(t *testing.T) {
	app := vochain.TestBaseApplication(t)
	idx := newTestIndexer(t, app)
	pid := util.RandomBytes(32)
	qt.Assert(t, app.IsSynced(), qt.Equals, true)

	err := app.State.AddProcess(&models.Process{
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

	// Stop the indexer from getting the state events.
	app.State.CleanEventListeners()

	// Add 10 votes to the election
	vp, err := state.NewVotePackage([]int{1, 1, 1}).Encode()
	qt.Assert(t, err, qt.IsNil)

	qt.Assert(t, err, qt.IsNil)
	for i := 0; i < 10; i++ {
		v := &state.Vote{ProcessID: pid, VotePackage: vp, Nullifier: util.RandomBytes(32)}
		qt.Assert(t, app.State.AddVote(v), qt.IsNil)
	}

	// Save the current state with the 10 new votes
	app.State.PrepareCommit()
	app.State.Save()

	// The results should not be up to date.
	proc, err = idx.ProcessInfo(pid)
	qt.Assert(t, err, qt.IsNil)
	qt.Assert(t, friendlyResults(proc.ResultsVotes), qt.DeepEquals, [][]string{
		{"0", "0"},
		{"0", "0"},
		{"0", "0"},
		{"0", "0"},
		{"0", "0"},
	})

	// Run the AfterSyncBootstrap, which should update the results.
	idx.AfterSyncBootstrap(true)

	proc, err = idx.ProcessInfo(pid)
	qt.Assert(t, err, qt.IsNil)
	qt.Assert(t, friendlyResults(proc.ResultsVotes), qt.DeepEquals, [][]string{
		{"0", "10"},
		{"0", "10"},
		{"0", "10"},
		{"0", "0"},
		{"0", "0"},
	})
}

func TestCountVotes(t *testing.T) {
	app := vochain.TestBaseApplication(t)
	idx := newTestIndexer(t, app)
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
	vp, err := state.NewVotePackage([]int{1, 1, 1}).Encode()
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
	proc, err := idx.ProcessInfo(pid)
	qt.Assert(t, err, qt.IsNil)
	qt.Assert(t, proc.VoteCount, qt.CmpEquals(), uint64(101))

	// Test global envelope total (number of votes)
	total, err := idx.CountTotalVotes()
	qt.Assert(t, err, qt.IsNil)
	qt.Assert(t, total, qt.CmpEquals(), uint64(101))

	ref, err := idx.GetEnvelope(nullifier)
	qt.Assert(t, err, qt.IsNil)
	qt.Assert(t, ref.Meta.Height, qt.CmpEquals(), blockHeight)

	// Note that txIndex is 0, because the votes are added directly to the sate,
	// while the txCounter only increments on DeliverTx() execution.
}

func TestOverwriteVotes(t *testing.T) {
	app := vochain.TestBaseApplication(t)
	idx := newTestIndexer(t, app)
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
	vp, err := state.NewVotePackage([]int{1, 1, 1}).Encode()
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
	app.AdvanceTestBlock()
	proc, err := idx.ProcessInfo(pid)
	qt.Assert(t, err, qt.IsNil)
	qt.Assert(t, friendlyResults(proc.ResultsVotes), qt.DeepEquals, [][]string{
		{"0", "1", "0"},
		{"0", "1", "0"},
		{"0", "1", "0"},
	})

	// Send the second
	vp, err = state.NewVotePackage([]int{2, 2, 2}).Encode()
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
	proc, err = idx.ProcessInfo(pid)
	qt.Assert(t, err, qt.IsNil)
	qt.Assert(t, proc.VoteCount, qt.CmpEquals(), uint64(1))

	// check overwrite count is correct
	ref, err := idx.GetEnvelope(nullifier)
	qt.Assert(t, err, qt.IsNil)
	qt.Assert(t, ref.OverwriteCount, qt.CmpEquals(), uint32(1))

	// check vote package is updated
	envelope, err := idx.GetEnvelope(nullifier)
	qt.Assert(t, err, qt.IsNil)
	qt.Assert(t, envelope.VotePackage, qt.DeepEquals, vp)

	// check the results are correct (only the second vote should be counted)
	qt.Assert(t, err, qt.IsNil)
	qt.Assert(t, friendlyResults(proc.ResultsVotes), qt.DeepEquals, [][]string{
		{"0", "0", "1"},
		{"0", "0", "1"},
		{"0", "0", "1"},
	})

	// Send a third vote from a different key
	vp, err = state.NewVotePackage([]int{2, 2, 2}).Encode()
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
	proc, err = idx.ProcessInfo(pid)
	qt.Assert(t, err, qt.IsNil)
	qt.Assert(t, friendlyResults(proc.ResultsVotes), qt.DeepEquals, [][]string{
		{"0", "0", "2"},
		{"0", "0", "2"},
		{"0", "0", "2"},
	})

	// Send the initial vote again (for third time)
	vp, err = state.NewVotePackage([]int{0, 0, 0}).Encode()
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
	proc, err = idx.ProcessInfo(pid)
	qt.Assert(t, err, qt.IsNil)
	qt.Assert(t, friendlyResults(proc.ResultsVotes), qt.DeepEquals, [][]string{
		{"1", "0", "1"},
		{"1", "0", "1"},
		{"1", "0", "1"},
	})

	// check the weight is correct (should be 2 because of the overwrite)
	qt.Assert(t, proc.ResultsWeight.MathBigInt().Int64(), qt.Equals, int64(2))
}

func TestTxIndexer(t *testing.T) {
	app := vochain.TestBaseApplication(t)
	idx := newTestIndexer(t, app)

	getTxID := func(i, j int) [32]byte {
		return [32]byte{byte(i), byte(j)}
	}

	const totalBlocks = 10
	const txsPerBlock = 10
	for i := 0; i < totalBlocks; i++ {
		for j := 0; j < txsPerBlock; j++ {
			idx.OnNewTx(&vochaintx.Tx{
				TxID:        getTxID(i, j),
				TxModelType: "setAccount",
			}, uint32(i), int32(j))
		}
	}
	qt.Assert(t, idx.Commit(0), qt.IsNil)

	count, err := idx.CountTotalTransactions()
	qt.Assert(t, err, qt.IsNil)
	const totalTxs = totalBlocks * txsPerBlock
	qt.Assert(t, count, qt.Equals, uint64(totalTxs))

	for i := 0; i < totalBlocks; i++ {
		for j := 0; j < txsPerBlock; j++ {
			ref, err := idx.GetTxReferenceByBlockHeightAndBlockIndex(int64(i), int64(j))
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

	txs, err := idx.GetLastTransactions(15, 0)
	qt.Assert(t, err, qt.IsNil)
	for i, tx := range txs {
		// BlockIndex and TxBlockIndex start at 0, so subtract 1.
		qt.Assert(t, tx.BlockHeight, qt.Equals, uint32(totalTxs-i-1)/txsPerBlock)
		qt.Assert(t, tx.TxBlockIndex, qt.Equals, int32(totalTxs-i-1)%txsPerBlock)
		qt.Assert(t, tx.TxType, qt.Equals, "setAccount")
	}

	txs, err = idx.GetLastTransactions(1, 5)
	qt.Assert(t, err, qt.IsNil)
	qt.Assert(t, txs, qt.HasLen, 1)
}

func TestCensusUpdate(t *testing.T) {
	app := vochain.TestBaseApplication(t)
	idx := newTestIndexer(t, app)

	originalCensusRoot := util.RandomBytes(32)
	originalCensusURI := new(string)
	*originalCensusURI = "ipfs://1234"
	pid := util.RandomBytes(32)

	if err := app.State.AddProcess(&models.Process{
		ProcessId:     pid,
		EnvelopeType:  &models.EnvelopeType{EncryptedVotes: false},
		Status:        models.ProcessStatus_READY,
		BlockCount:    10,
		VoteOptions:   &models.ProcessVoteOptions{MaxCount: 3, MaxValue: 100},
		Mode:          &models.ProcessMode{AutoStart: true, DynamicCensus: true},
		MaxCensusSize: 1000,
		CensusRoot:    originalCensusRoot,
		CensusURI:     originalCensusURI,
		CensusOrigin:  models.CensusOrigin_OFF_CHAIN_TREE,
	}); err != nil {
		t.Fatal(err)
	}
	app.AdvanceTestBlock()

	// get created process
	proc, err := idx.ProcessInfo(pid)
	qt.Assert(t, err, qt.IsNil)
	// check the census root is correct
	qt.Assert(t, proc.CensusRoot, qt.DeepEquals, types.HexBytes(originalCensusRoot))
	// check the census uri is correct
	qt.Assert(t, proc.CensusURI, qt.DeepEquals, *originalCensusURI)

	// send SET_PROCESS_CENSUS_UPDATE
	newCensusRoot := util.RandomBytes(32)
	newCensusURI := new(string)
	*newCensusURI = "ipfs://5678"
	if err := app.State.SetProcessCensus(pid, newCensusRoot, *newCensusURI, true); err != nil {
		t.Fatal(err)
	}

	// advance block
	app.AdvanceTestBlock()
	// get process again
	proc, err = idx.ProcessInfo(pid)
	qt.Assert(t, err, qt.IsNil)
	// check the census root is correct
	qt.Assert(t, proc.CensusRoot, qt.DeepEquals, types.HexBytes(newCensusRoot))
	// check the census uri is correct
	qt.Assert(t, proc.CensusURI, qt.DeepEquals, *newCensusURI)
}

func TestEndProcess(t *testing.T) {
	app := vochain.TestBaseApplication(t)
	idx := newTestIndexer(t, app)

	pid := util.RandomBytes(32)

	if err := app.State.AddProcess(&models.Process{
		ProcessId:     pid,
		EnvelopeType:  &models.EnvelopeType{EncryptedVotes: false},
		Status:        models.ProcessStatus_READY,
		StartTime:     0,
		Duration:      10,
		VoteOptions:   &models.ProcessVoteOptions{MaxCount: 3, MaxValue: 100},
		Mode:          &models.ProcessMode{AutoStart: true, Interruptible: true},
		MaxCensusSize: 1000,
		CensusRoot:    util.RandomBytes(32),
		CensusOrigin:  models.CensusOrigin_OFF_CHAIN_TREE,
	}); err != nil {
		t.Fatal(err)
	}
	app.AdvanceTestBlocksUntilTimestamp(5)

	// get created process
	proc, err := idx.ProcessInfo(pid)
	qt.Assert(t, err, qt.IsNil)
	// check the end date is correct
	qt.Assert(t, proc.EndDate.Unix(), qt.Equals, int64(10))
	// check the status is READY
	qt.Assert(t, proc.Status, qt.Equals, int32(models.ProcessStatus_READY))

	// advance block and finalize process
	app.AdvanceTestBlock()
	err = app.State.SetProcessStatus(pid, models.ProcessStatus_ENDED, true)
	qt.Assert(t, err, qt.IsNil)

	app.AdvanceTestBlock()

	// check the status is ENDED
	proc, err = idx.ProcessInfo(pid)
	qt.Assert(t, err, qt.IsNil)
	qt.Assert(t, proc.Status, qt.Equals, int32(models.ProcessStatus_ENDED))

	// check manually ended is true
	qt.Assert(t, err, qt.IsNil)
	qt.Assert(t, proc.ManuallyEnded, qt.Equals, true)
}

func TestAccountsList(t *testing.T) {
	app := vochain.TestBaseApplication(t)
	idx := newTestIndexer(t, app)

	keys := make([]*ethereum.SignKeys, 0)
	for i := 0; i < 25; i++ {
		key := &ethereum.SignKeys{}
		qt.Assert(t, key.Generate(), qt.IsNil)
		keys = append(keys, key)

		err := app.State.SetAccount(key.Address(), &state.Account{
			Account: models.Account{
				Balance: 500,
				Nonce:   uint32(0),
				InfoURI: "ipfs://vocdoni.io",
			},
		})
		qt.Assert(t, err, qt.IsNil)

		app.AdvanceTestBlock()
	}

	// Test total count accounts
	totalAccs, err := idx.CountTotalAccounts()
	qt.Assert(t, err, qt.IsNil)
	qt.Assert(t, totalAccs, qt.CmpEquals(), uint64(25))

	last := 0
	for i := 0; i < int(totalAccs); i++ {
		accts, err := idx.GetListAccounts(int32(last), 10)
		qt.Assert(t, err, qt.IsNil)

		for j, acc := range accts {
			qt.Assert(t, acc.Address.String(), qt.Equals, hex.EncodeToString(keys[j+last].Address().Bytes()))
			qt.Assert(t, acc.Nonce, qt.Equals, uint32(0))
			qt.Assert(t, acc.Balance, qt.Equals, uint64(500))
		}
		last += 10
	}

	// update an existing account
	err = app.State.SetAccount(keys[0].Address(), &state.Account{
		Account: models.Account{
			Balance: 600,
			Nonce:   uint32(1),
			InfoURI: "ipfs://vocdoni.io",
		},
	})
	qt.Assert(t, err, qt.IsNil)
	app.AdvanceTestBlock()

	// verify the updated balance and nonce
	accts, err := idx.GetListAccounts(int32(0), 5)
	qt.Assert(t, err, qt.IsNil)
	// the account in the position 0 must be the updated account balance due it has the major balance
	// indexer query has order BY balance DESC
	qt.Assert(t, accts[0].Address.String(), qt.Equals, hex.EncodeToString(keys[0].Address().Bytes()))
	qt.Assert(t, accts[0].Nonce, qt.Equals, uint32(1))
	qt.Assert(t, accts[0].Balance, qt.Equals, uint64(600))
}

func TestTokenTransfers(t *testing.T) {
	app := vochain.TestBaseApplication(t)
	idx := newTestIndexer(t, app)

	keys := make([]*ethereum.SignKeys, 0)
	// create 3 accounts
	for i := 0; i < 3; i++ {
		key := &ethereum.SignKeys{}
		qt.Assert(t, key.Generate(), qt.IsNil)
		keys = append(keys, key)

		err := app.State.SetAccount(key.Address(), &state.Account{
			Account: models.Account{
				Balance: 500,
				Nonce:   uint32(0),
				InfoURI: "ipfs://vocdoni.io",
			},
		})
		qt.Assert(t, err, qt.IsNil)

		app.AdvanceTestBlock()
	}

	err := app.State.TransferBalance(&vochaintx.TokenTransfer{
		FromAddress: keys[0].Address(),
		ToAddress:   keys[2].Address(),
		Amount:      18,
		TxHash:      util.RandomBytes(32),
	}, false)
	qt.Assert(t, err, qt.IsNil)

	app.AdvanceTestBlock()

	err = app.State.TransferBalance(&vochaintx.TokenTransfer{
		FromAddress: keys[1].Address(),
		ToAddress:   keys[2].Address(),
		Amount:      95,
		TxHash:      util.RandomBytes(32),
	}, false)
	qt.Assert(t, err, qt.IsNil)

	app.AdvanceTestBlock()

	err = app.State.TransferBalance(&vochaintx.TokenTransfer{
		FromAddress: keys[2].Address(),
		ToAddress:   keys[1].Address(),
		Amount:      5,
		TxHash:      util.RandomBytes(32),
	}, false)
	qt.Assert(t, err, qt.IsNil)

	app.AdvanceTestBlock()

	// acct 1 must have only one token transfer received
	acc1Tokentx, err := idx.GetTokenTransfersByToAccount(keys[1].Address().Bytes(), 0, 10)
	qt.Assert(t, err, qt.IsNil)
	qt.Assert(t, len(acc1Tokentx), qt.Equals, 1)
	qt.Assert(t, acc1Tokentx[0].Amount, qt.Equals, uint64(5))

	// acct 2 must two token transfers received
	acc2Tokentx, err := idx.GetTokenTransfersByToAccount(keys[2].Address().Bytes(), 0, 10)
	qt.Assert(t, err, qt.IsNil)
	qt.Assert(t, len(acc2Tokentx), qt.Equals, 2)
	qt.Assert(t, acc2Tokentx[0].Amount, qt.Equals, uint64(95))
	qt.Assert(t, acc2Tokentx[1].Amount, qt.Equals, uint64(18))

	// acct 0 must zero token transfers received
	acc0Tokentx, err := idx.GetTokenTransfersByToAccount(keys[0].Address().Bytes(), 0, 10)
	qt.Assert(t, err, qt.IsNil)
	qt.Assert(t, len(acc0Tokentx), qt.Equals, 0)
}

// friendlyResults translates votes into a matrix of strings
func friendlyResults(votes [][]*types.BigInt) [][]string {
	r := [][]string{}
	for i := range votes {
		r = append(r, []string{})
		for j := range votes[i] {
			n, err := votes[i][j].MarshalText()
			if err != nil {
				panic(err)
			}
			r[i] = append(r[i], string(n))
		}
	}
	return r
}

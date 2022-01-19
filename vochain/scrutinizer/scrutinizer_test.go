package scrutinizer

import (
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	stdlog "log"
	"math/big"
	"testing"
	"time"

	qt "github.com/frankban/quicktest"
	"github.com/pressly/goose/v3"
	"go.vocdoni.io/dvote/crypto/ethereum"
	"go.vocdoni.io/dvote/crypto/nacl"
	"go.vocdoni.io/dvote/log"
	"go.vocdoni.io/dvote/types"
	"go.vocdoni.io/dvote/util"
	"go.vocdoni.io/dvote/vochain"
	"go.vocdoni.io/dvote/vochain/scrutinizer/indexertypes"
	models "go.vocdoni.io/proto/build/go/models"
	"google.golang.org/protobuf/proto"
)

func init() {
	// keep the tests silent
	goose.SetLogger(stdlog.New(io.Discard, "", 0))
}

func TestEntityList(t *testing.T) {
	testEntityList(t, 2)
	testEntityList(t, 100)
	testEntityList(t, 155)
}

func testEntityList(t *testing.T, entityCount int) {
	app := vochain.TestBaseApplication(t)
	sc, err := NewScrutinizer(t.TempDir(), app, true)
	if err != nil {
		t.Fatal(err)
	}
	for i := 0; i < entityCount; i++ {
		pid := util.RandomBytes(32)
		if err := app.State.AddProcess(&models.Process{
			ProcessId:    pid,
			EntityId:     util.RandomBytes(20),
			BlockCount:   10,
			VoteOptions:  &models.ProcessVoteOptions{MaxCount: 8, MaxValue: 3},
			EnvelopeType: &models.EnvelopeType{},
		}); err != nil {
			t.Fatal(err)
		}
		if i%5 == 1 {
			app.AdvanceTestBlock()
		}
	}
	app.AdvanceTestBlock()
	entities := make(map[string]bool)
	if ec := sc.EntityCount(); ec != uint64(entityCount) {
		t.Fatalf("entity count is wrong, got %d expected %d", ec, entityCount)
	}
	var list []string
	last := 0
	for len(entities) <= entityCount {
		list = sc.EntityList(10, last, "")
		if len(list) < 1 {
			t.Log("list is empty")
			break
		}
		for _, e := range list {
			if entities[e] {
				t.Fatalf("found duplicated entity: %s", e)
			}
			entities[e] = true
		}
		last += 10
	}
	if len(entities) < entityCount {
		t.Fatalf("expected %d entityes, got %d", entityCount, len(entities))
	}
}

func TestEntitySearch(t *testing.T) {
	app := vochain.TestBaseApplication(t)
	sc, err := NewScrutinizer(t.TempDir(), app, true)
	if err != nil {
		t.Fatal(err)
	}

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
			ProcessId:    pid,
			EntityId:     util.RandomBytes(20),
			BlockCount:   10,
			VoteOptions:  &models.ProcessVoteOptions{MaxCount: 8, MaxValue: 3},
			EnvelopeType: &models.EnvelopeType{},
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
			ProcessId:    pid,
			EntityId:     entityId,
			BlockCount:   10,
			VoteOptions:  &models.ProcessVoteOptions{MaxCount: 8, MaxValue: 3},
			EnvelopeType: &models.EnvelopeType{},
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
			ProcessId:    pid,
			EntityId:     util.RandomBytes(20),
			BlockCount:   10,
			VoteOptions:  &models.ProcessVoteOptions{MaxCount: 8, MaxValue: 3},
			EnvelopeType: &models.EnvelopeType{},
		}); err != nil {
			t.Fatal(err)
		}
		if i%5 == 1 {
			app.AdvanceTestBlock()
		}
	}
	app.AdvanceTestBlock()
	var list []string
	// Exact entity search
	list = sc.EntityList(10, 0, "4011d50537fa164b6fef261141797bbe4014526e")
	if len(list) < 1 {
		t.Fatalf("expected 1 entity, got %d", len(list))
	}
	// Search for nonexistent entity
	list = sc.EntityList(10, 0, "4011d50537fa164b6fef261141797bbe4014526f")
	if len(list) > 0 {
		t.Fatalf("expected 0 entities, got %d", len(list))
	}
	// Search containing part of all manually-defined entities
	list = sc.EntityList(10, 0, "011d50537fa164b6fef261141797bbe4014526e")
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
	sc, err := NewScrutinizer(t.TempDir(), app, true)
	if err != nil {
		t.Fatal(err)
	}

	// Add 10 entities and process for storing random content
	for i := 0; i < 10; i++ {
		pid := util.RandomBytes(32)
		err := app.State.AddProcess(&models.Process{
			ProcessId:    pid,
			EntityId:     util.RandomBytes(20),
			VoteOptions:  &models.ProcessVoteOptions{MaxCount: 8, MaxValue: 3},
			EnvelopeType: &models.EnvelopeType{},
		})
		qt.Assert(t, err, qt.IsNil)
		if i%5 == 1 {
			app.AdvanceTestBlock()
		}

	}

	// For a entity, add 25 processes (this will be the queried entity)
	eidTest := util.RandomBytes(20)
	for i := 0; i < procsCount; i++ {
		pid := util.RandomBytes(32)
		err := app.State.AddProcess(&models.Process{
			ProcessId:    pid,
			EntityId:     eidTest,
			VoteOptions:  &models.ProcessVoteOptions{MaxCount: 8, MaxValue: 3},
			EnvelopeType: &models.EnvelopeType{},
		})
		qt.Assert(t, err, qt.IsNil)
		if i%5 == 1 {
			app.AdvanceTestBlock()
		}
	}
	app.AdvanceTestBlock()

	procs := make(map[string]bool)
	last := 0
	var list [][]byte
	for len(procs) < procsCount {
		list, err = sc.ProcessList(eidTest, last, 10, "", 0, "", "", false)
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
	if len(procs) != procsCount {
		t.Fatalf("expected %d processes, got %d", procsCount, len(procs))
	}

	_, err = sc.ProcessList(nil, 0, 64, "", 0, "", "", false)
	if err != nil {
		t.Fatal(err)
	}
}

func TestProcessSearch(t *testing.T) {
	app := vochain.TestBaseApplication(t)
	sc, err := NewScrutinizer(t.TempDir(), app, true)
	if err != nil {
		t.Fatal(err)
	}

	// Add 10 entities and process for storing random content
	for i := 0; i < 10; i++ {
		pid := util.RandomBytes(32)
		t.Logf("random process ID: %x", pid)
		err := app.State.AddProcess(&models.Process{
			ProcessId:    pid,
			EntityId:     util.RandomBytes(20),
			VoteOptions:  &models.ProcessVoteOptions{MaxCount: 8, MaxValue: 3},
			EnvelopeType: &models.EnvelopeType{},
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
	// For a entity, add 25 processes (this will be the queried entity)
	eidTest := util.RandomBytes(20)
	for i, process := range processIds {
		pid, err := hex.DecodeString(process)
		if err != nil {
			t.Fatal(err)
		}
		if err := app.State.AddProcess(&models.Process{
			ProcessId:    pid,
			EntityId:     eidTest,
			BlockCount:   10,
			VoteOptions:  &models.ProcessVoteOptions{MaxCount: 8, MaxValue: 3},
			EnvelopeType: &models.EnvelopeType{},
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
			ProcessId:    pid,
			EntityId:     eidTest,
			BlockCount:   10,
			VoteOptions:  &models.ProcessVoteOptions{MaxCount: 8, MaxValue: 3},
			EnvelopeType: &models.EnvelopeType{},
			Status:       models.ProcessStatus_ENDED,
		}); err != nil {
			t.Fatal(err)
		}
		if i%5 == 1 {
			app.AdvanceTestBlock()
		}
	}
	app.AdvanceTestBlock()

	// Exact process search
	list, err := sc.ProcessList(eidTest, 0, 10,
		"4011d50537fa164b6fef261141797bbe4014526e", 0, "", "", false)
	if err != nil {
		t.Fatal(err)
	}
	if len(list) < 1 {
		t.Fatalf("expected 1 process, got %d", len(list))
	}
	// Search for nonexistent process
	list, err = sc.ProcessList(eidTest, 0, 10,
		"4011d50537fa164b6fef261141797bbe4014526f", 0, "", "", false)
	if err != nil {
		t.Fatal(err)
	}
	if len(list) > 0 {
		t.Fatalf("expected 0 processes, got %d", len(list))
	}
	// Search containing part of all manually-defined processes
	list, err = sc.ProcessList(eidTest, 0, 10,
		"011d50537fa164b6fef261141797bbe4014526e", 0, "", "", false)
	if err != nil {
		t.Fatal(err)
	}
	if len(list) < len(processIds) {
		t.Fatalf("expected %d processes, got %d", len(processIds), len(list))
	}

	list, err = sc.ProcessList(eidTest, 0, 100,
		"0c6ca22d2c175a1fbdd15d7595ae532bb1094b5", 0, "", "ENDED", false)
	if err != nil {
		t.Fatal(err)
	}
	if len(list) < len(endedPIDs) {
		t.Fatalf("expected %d processes, got %d", len(endedPIDs), len(list))
	}

	// list all processes, with a max of 10
	list, err = sc.ProcessList(nil, 0, 10, "", 0, "", "", false)
	qt.Assert(t, err, qt.IsNil)
	qt.Assert(t, list, qt.HasLen, 10)

	// list all processes, with a max of 1000
	list, err = sc.ProcessList(nil, 0, 1000, "", 0, "", "", false)
	qt.Assert(t, err, qt.IsNil)
	qt.Assert(t, list, qt.HasLen, 21)
}

func TestProcessListWithNamespaceAndStatus(t *testing.T) {
	app := vochain.TestBaseApplication(t)

	sc, err := NewScrutinizer(t.TempDir(), app, true)
	if err != nil {
		t.Fatal(err)
	}

	// Add 10 processes with different namespaces (from 10 to 20) and status ENDED
	for i := 0; i < 10; i++ {
		pid := util.RandomBytes(32)
		err := app.State.AddProcess(&models.Process{
			ProcessId:    pid,
			EntityId:     util.RandomBytes(20),
			VoteOptions:  &models.ProcessVoteOptions{MaxCount: 8, MaxValue: 3},
			EnvelopeType: &models.EnvelopeType{},
			Namespace:    uint32(10 + i),
			Status:       models.ProcessStatus_ENDED,
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
			ProcessId:    pid,
			EntityId:     eid20,
			VoteOptions:  &models.ProcessVoteOptions{MaxCount: 8, MaxValue: 3},
			EnvelopeType: &models.EnvelopeType{},
			Namespace:    123,
			Status:       models.ProcessStatus_READY,
		})
		qt.Assert(t, err, qt.IsNil)
		if i%5 == 1 {
			app.AdvanceTestBlock()
		}
	}
	app.AdvanceTestBlock()

	// Get the process list for namespace 123
	list, err := sc.ProcessList(eid20, 0, 100, "", 123, "", "", false)
	qt.Assert(t, err, qt.IsNil)
	// Check there are exactly 10
	qt.Assert(t, len(list), qt.CmpEquals(), 10)

	// Get the process list for all namespaces
	list, err = sc.ProcessList(nil, 0, 100, "", 0, "", "", false)
	qt.Assert(t, err, qt.IsNil)
	// Check there are exactly 10 + 10
	qt.Assert(t, len(list), qt.CmpEquals(), 20)

	// Get the process list for namespace 10
	list, err = sc.ProcessList(nil, 0, 100, "", 10, "", "", false)
	qt.Assert(t, err, qt.IsNil)
	// Check there is exactly 1
	qt.Assert(t, len(list), qt.CmpEquals(), 1)

	// Get the process list for namespace 10
	list, err = sc.ProcessList(nil, 0, 100, "", 0, "", "READY", false)
	qt.Assert(t, err, qt.IsNil)
	// Check there is exactly 1
	qt.Assert(t, len(list), qt.CmpEquals(), 10)
}

func TestResults(t *testing.T) {
	app := vochain.TestBaseApplication(t)
	app.State.SetHeight(3)
	sc, err := NewScrutinizer(t.TempDir(), app, true)
	if err != nil {
		t.Fatal(err)
	}

	pid := util.RandomBytes(32)
	err = app.State.AddProcess(&models.Process{
		ProcessId:             pid,
		EnvelopeType:          &models.EnvelopeType{EncryptedVotes: true},
		Status:                models.ProcessStatus_READY,
		Mode:                  &models.ProcessMode{AutoStart: true},
		BlockCount:            10,
		EncryptionPrivateKeys: make([]string, 16),
		EncryptionPublicKeys:  make([]string, 16),
		VoteOptions:           &models.ProcessVoteOptions{MaxCount: 4, MaxValue: 1},
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

	// Add 100 votes
	vp, err := json.Marshal(vochain.VotePackage{
		Nonce: fmt.Sprintf("%x", util.RandomBytes(32)),
		Votes: []int{1, 1, 1, 1},
	})
	qt.Assert(t, err, qt.IsNil)
	vp, err = priv.Encrypt(vp, nil)
	qt.Assert(t, err, qt.IsNil)

	for i := int32(0); i < 300; i++ {
		sc.Rollback()
		vote := &models.VoteEnvelope{
			Nonce:                util.RandomBytes(32),
			ProcessId:            pid,
			VotePackage:          vp,
			Nullifier:            util.RandomBytes(32),
			EncryptionKeyIndexes: []uint32{1},
		}
		voteTx, err := proto.Marshal(&models.Tx{Payload: &models.Tx_Vote{Vote: vote}})
		qt.Assert(t, err, qt.IsNil)
		signedTx, err := proto.Marshal(&models.SignedTx{
			Tx:        voteTx,
			Signature: []byte{},
		})
		qt.Assert(t, err, qt.IsNil)
		_, err = app.SendTx(signedTx)
		qt.Assert(t, err, qt.IsNil)

		txRef := &VoteWithIndex{
			vote: &models.Vote{
				Nullifier: vote.Nullifier,
				ProcessId: pid,
				Weight:    big.NewInt(1).Bytes(),
			},
			txIndex: 0,
		}
		sc.voteIndexPool = append(sc.voteIndexPool, txRef)
		err = sc.Commit(uint32(i))
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
	err = sc.updateProcess(pid)
	qt.Assert(t, err, qt.IsNil)
	err = sc.setResultsHeight(pid, app.State.CurrentHeight())
	qt.Assert(t, err, qt.IsNil)
	err = sc.ComputeResult(pid)
	qt.Assert(t, err, qt.IsNil)

	// Test results
	result, err := sc.GetResults(pid)
	qt.Assert(t, err, qt.IsNil)
	log.Infof("results: %s", GetFriendlyResults(result.Votes))
	v0 := big.NewInt(0)
	v300 := big.NewInt(300)
	var value *big.Int
	for q := range result.Votes {
		for qi := range result.Votes[q] {
			if qi > 3 {
				t.Fatalf("found more questions that expected")
			}
			value = result.Votes[q][qi].ToInt()
			if qi != 1 && value.Cmp(v0) != 0 {
				t.Fatalf("result is not correct, %d is not 0 as expected", value.Uint64())
			}
			if qi == 1 && value.Cmp(v300) != 0 {
				t.Fatalf("result is not correct, %d is not 300 as expected", value.Uint64())
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
			if qi == 1 && v1 != "300" {
				t.Fatalf("result is not correct, %s is not 300 as expected", v1)
			}
		}
	}
}

func TestLiveResults(t *testing.T) {
	app := vochain.TestBaseApplication(t)

	sc, err := NewScrutinizer(t.TempDir(), app, true)
	if err != nil {
		t.Fatal(err)
	}

	pid := util.RandomBytes(32)
	if err := app.State.AddProcess(&models.Process{
		ProcessId:    pid,
		EnvelopeType: &models.EnvelopeType{EncryptedVotes: false},
		Status:       models.ProcessStatus_READY,
		BlockCount:   10,
		VoteOptions:  &models.ProcessVoteOptions{MaxCount: 3, MaxValue: 100},
		Mode:         &models.ProcessMode{AutoStart: true},
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
	r := &indexertypes.Results{
		Votes:        indexertypes.NewEmptyVotes(3, 100),
		Weight:       new(types.BigInt).SetUint64(0),
		VoteOpts:     &models.ProcessVoteOptions{MaxCount: 3, MaxValue: 100},
		EnvelopeType: &models.EnvelopeType{},
	}
	sc.addProcessToLiveResults(pid)
	for i := 0; i < 100; i++ {
		qt.Assert(t, sc.addLiveVote(
			pid,
			vp,
			new(big.Int).SetUint64(1),
			r),
			qt.IsNil)
	}
	qt.Assert(t, sc.commitVotes(pid, r, 1), qt.IsNil)

	if live, err := sc.isOpenProcess(pid); !live || err != nil {
		t.Fatal(fmt.Errorf("isLiveResultsProcess returned false: %v", err))
	}

	// Test results
	result, err := sc.GetResults(pid)
	qt.Assert(t, err, qt.IsNil)

	v0 := big.NewInt(0)
	v100 := big.NewInt(100)
	var value *big.Int
	for q := range result.Votes {
		for qi := range result.Votes[q] {
			if qi > 100 {
				t.Fatalf("found more questions that expected")
			}
			value = result.Votes[q][qi].ToInt()
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

	sc, err := NewScrutinizer(t.TempDir(), app, true)
	qt.Assert(t, err, qt.IsNil)

	options := &models.ProcessVoteOptions{
		MaxCount:     3,
		MaxValue:     3,
		MaxTotalCost: 6,
		CostExponent: 1,
	}

	pid := util.RandomBytes(32)
	if err := app.State.AddProcess(&models.Process{
		ProcessId:    pid,
		EnvelopeType: &models.EnvelopeType{EncryptedVotes: false},
		Status:       models.ProcessStatus_READY,
		BlockCount:   10,
		VoteOptions:  options,
		Mode:         &models.ProcessMode{AutoStart: true},
	}); err != nil {
		t.Fatal(err)
	}
	app.AdvanceTestBlock()

	pr, err := sc.GetResults(pid)
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

var vote = func(v []int, sc *Scrutinizer, pid []byte, weight *big.Int) error {
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
	proc, err := sc.ProcessInfo(pid)
	if err != nil {
		return err
	}
	r := &indexertypes.Results{
		ProcessID: pid,
		Votes: indexertypes.NewEmptyVotes(
			int(proc.VoteOpts.MaxCount), int(proc.VoteOpts.MaxValue)+1),
		Weight:       new(types.BigInt).SetUint64(0),
		Signatures:   []types.HexBytes{},
		VoteOpts:     proc.VoteOpts,
		EnvelopeType: proc.Envelope,
	}
	sc.addProcessToLiveResults(pid)
	if err := sc.addLiveVote(pid, vp, weight, r); err != nil {
		return err
	}
	return sc.commitVotes(pid, r, 1)
}

func TestBallotProtocolRateProduct(t *testing.T) {
	// Rate a product from 0 to 4
	app := vochain.TestBaseApplication(t)

	sc, err := NewScrutinizer(t.TempDir(), app, true)
	qt.Assert(t, err, qt.IsNil)

	// Rate 2 products from 0 to 4
	pid := util.RandomBytes(32)
	if err := app.State.AddProcess(&models.Process{
		ProcessId:    pid,
		EnvelopeType: &models.EnvelopeType{EncryptedVotes: false},
		Status:       models.ProcessStatus_READY,
		BlockCount:   10,
		Mode:         &models.ProcessMode{AutoStart: true},
		VoteOptions:  &models.ProcessVoteOptions{MaxCount: 2, MaxValue: 4},
	}); err != nil {
		t.Fatal(err)
	}

	app.AdvanceTestBlock()

	// Rate a product, exepected result: [ [1,0,1,0,2], [0,0,2,0,2] ]
	qt.Assert(t, vote([]int{4, 2}, sc, pid, nil), qt.IsNil)
	qt.Assert(t, vote([]int{4, 2}, sc, pid, nil), qt.IsNil)
	qt.Assert(t, vote([]int{2, 4}, sc, pid, nil), qt.IsNil)
	qt.Assert(t, vote([]int{0, 4}, sc, pid, nil), qt.IsNil)
	qt.Assert(t, vote([]int{0, 5}, sc, pid, nil), qt.ErrorMatches, ".*overflow.*")
	qt.Assert(t, vote([]int{0, 0, 0}, sc, pid, nil), qt.ErrorMatches, ".*")

	result, err := sc.GetResults(pid)
	qt.Assert(t, err, qt.IsNil)
	votes := GetFriendlyResults(result.Votes)
	qt.Assert(t, votes[1], qt.DeepEquals, []string{"0", "0", "2", "0", "2"})
	qt.Assert(t, votes[0], qt.DeepEquals, []string{"1", "0", "1", "0", "2"})
}

func TestBallotProtocolQuadratic(t *testing.T) {
	// Rate a product from 0 to 4
	app := vochain.TestBaseApplication(t)

	sc, err := NewScrutinizer(t.TempDir(), app, true)
	qt.Assert(t, err, qt.IsNil)

	// Rate 2 products from 0 to 4
	pid := util.RandomBytes(32)
	if err := app.State.AddProcess(&models.Process{
		ProcessId:    pid,
		EnvelopeType: &models.EnvelopeType{EncryptedVotes: false, CostFromWeight: true},
		Status:       models.ProcessStatus_READY,
		BlockCount:   10,
		Mode:         &models.ProcessMode{AutoStart: true},
		VoteOptions:  &models.ProcessVoteOptions{MaxCount: 2, MaxValue: 0, CostExponent: 2},
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
	qt.Assert(t, vote([]int{10, 28}, sc, pid, new(big.Int).SetUint64(1000)), qt.IsNil)
	qt.Assert(t, vote([]int{5, 5}, sc, pid, new(big.Int).SetUint64(50)), qt.IsNil)
	qt.Assert(t, vote([]int{100000, 100000}, sc, pid, new(big.Int).SetUint64(20000000000)), qt.IsNil)
	qt.Assert(t, vote([]int{1, 0}, sc, pid, new(big.Int).SetUint64(1)), qt.IsNil)
	// Wrong
	qt.Assert(t, vote([]int{5, 2}, sc, pid, new(big.Int).SetUint64(25)),
		qt.ErrorMatches, ".*overflow.*")
	qt.Assert(t, vote([]int{100000, 112345}, sc, pid, new(big.Int).SetUint64(20000000000)),
		qt.ErrorMatches, ".*overflow.*")

	result, err := sc.GetResults(pid)
	qt.Assert(t, err, qt.IsNil)
	votes := GetFriendlyResults(result.Votes)
	qt.Assert(t, votes[0], qt.DeepEquals, []string{"100016"})
	qt.Assert(t, votes[1], qt.DeepEquals, []string{"100033"})
}

func TestBallotProtocolMultiChoice(t *testing.T) {
	// Choose your 3 favorite colours out of 5

	app := vochain.TestBaseApplication(t)

	sc, err := NewScrutinizer(t.TempDir(), app, true)
	qt.Assert(t, err, qt.IsNil)

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
	}); err != nil {
		t.Fatal(err)
	}

	app.AdvanceTestBlock()

	// Multichoice (choose 3 ouf of 5):
	// - Vote Envelope: `[1,1,1,0,0]` `[0,1,1,1,0]` `[1,1,0,0,0]`
	// - Results: `[ [1, 2], [0, 3], [1, 2], [2, 1], [3, 0] ]`
	qt.Assert(t, vote([]int{1, 1, 1, 0, 0}, sc, pid, nil), qt.IsNil)
	qt.Assert(t, vote([]int{0, 1, 1, 1, 0}, sc, pid, nil), qt.IsNil)
	qt.Assert(t, vote([]int{1, 1, 0, 0, 0}, sc, pid, nil), qt.IsNil)
	qt.Assert(t, vote([]int{2, 1, 0, 0, 0}, sc, pid, nil), qt.ErrorMatches, ".*overflow.*")
	qt.Assert(t, vote([]int{1, 1, 1, 1, 0}, sc, pid, nil), qt.ErrorMatches, ".*overflow.*")

	result, err := sc.GetResults(pid)
	qt.Assert(t, err, qt.IsNil)
	votes := GetFriendlyResults(result.Votes)
	qt.Assert(t, votes[0], qt.DeepEquals, []string{"1", "2"})
	qt.Assert(t, votes[1], qt.DeepEquals, []string{"0", "3"})
	qt.Assert(t, votes[2], qt.DeepEquals, []string{"1", "2"})
	qt.Assert(t, votes[3], qt.DeepEquals, []string{"2", "1"})
	qt.Assert(t, votes[4], qt.DeepEquals, []string{"3", "0"})
}

func TestCountVotes(t *testing.T) {
	app := vochain.TestBaseApplication(t)
	sc, err := NewScrutinizer(t.TempDir(), app, true)
	if err != nil {
		t.Fatal(err)
	}
	pid := util.RandomBytes(32)

	err = app.State.AddProcess(&models.Process{
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
	})
	qt.Assert(t, err, qt.IsNil)
	app.AdvanceTestBlock()

	// Add 100 votes
	vp, err := json.Marshal(vochain.VotePackage{
		Nonce: fmt.Sprintf("%x", util.RandomHex(32)),
		Votes: []int{1, 1, 1},
	})
	qt.Assert(t, err, qt.IsNil)
	sc.Rollback()
	sc.addProcessToLiveResults(pid)
	for i := 0; i < 100; i++ {
		v := &models.Vote{ProcessId: pid, VotePackage: vp, Nullifier: util.RandomBytes(32)}
		// Add votes to votePool with i as txIndex
		sc.OnVote(v, int32(i))
	}
	nullifier := util.RandomBytes(32)
	v := &models.Vote{ProcessId: pid, VotePackage: vp, Nullifier: nullifier}
	// Add last vote with known nullifier
	txIndex := int32(100)
	sc.OnVote(v, txIndex)

	// Vote transactions are on imaginary 2000th block
	blockHeight := uint32(2000)
	err = sc.Commit(blockHeight)
	qt.Assert(t, err, qt.IsNil)

	// Test envelope height for this PID
	height, err := sc.GetEnvelopeHeight(pid)
	qt.Assert(t, err, qt.IsNil)
	qt.Assert(t, height, qt.CmpEquals(), uint64(101))
	// Test global envelope height
	height, err = sc.GetEnvelopeHeight([]byte{})
	qt.Assert(t, err, qt.IsNil)
	qt.Assert(t, height, qt.CmpEquals(), uint64(101))

	ref, err := sc.GetEnvelopeReference(nullifier)
	qt.Assert(t, err, qt.IsNil)
	qt.Assert(t, ref.Height, qt.CmpEquals(), blockHeight)
	qt.Assert(t, ref.TxIndex, qt.CmpEquals(), txIndex)
}

func TestTxIndexer(t *testing.T) {
	app := vochain.TestBaseApplication(t)

	sc, err := NewScrutinizer(t.TempDir(), app, true)
	qt.Assert(t, err, qt.IsNil)

	for i := 0; i < 10; i++ {
		for j := 0; j < 10; j++ {
			sc.OnNewTx([]byte(fmt.Sprintf("hash%d%d", i, j)), uint32(i), int32(j))
		}
	}
	qt.Assert(t, sc.Commit(0), qt.IsNil)
	time.Sleep(3 * time.Second)

	for i := 0; i < 10; i++ {
		for j := 0; j < 10; j++ {
			ref, err := sc.GetTxReference(uint64(i*10 + j + 1))
			qt.Assert(t, err, qt.IsNil)
			qt.Assert(t, ref.BlockHeight, qt.Equals, uint32(i))
			qt.Assert(t, ref.TxBlockIndex, qt.Equals, int32(j))

			hashRef, err := sc.GetTxHashReference([]byte(fmt.Sprintf("hash%d%d", i, j)))
			qt.Assert(t, err, qt.IsNil)
			qt.Assert(t, hashRef.BlockHeight, qt.Equals, uint32(i))
			qt.Assert(t, hashRef.TxBlockIndex, qt.Equals, int32(j))
		}
	}
}

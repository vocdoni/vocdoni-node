package scrutinizer

import (
	"encoding/json"
	"fmt"
	"math/big"
	"testing"

	qt "github.com/frankban/quicktest"
	"go.vocdoni.io/dvote/crypto/ethereum"
	"go.vocdoni.io/dvote/crypto/nacl"
	"go.vocdoni.io/dvote/log"
	"go.vocdoni.io/dvote/types"
	"go.vocdoni.io/dvote/util"
	"go.vocdoni.io/dvote/vochain"
	"go.vocdoni.io/proto/build/go/models"
)

func TestEntityList(t *testing.T) {
	testEntityList(t, 2)
	testEntityList(t, 100)
	testEntityList(t, 155)
}

func testEntityList(t *testing.T, entityCount int) {
	state, err := vochain.NewState(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}

	sc, err := NewScrutinizer(t.TempDir(), state)
	if err != nil {
		t.Fatal(err)
	}
	for i := 0; i < entityCount; i++ {
		pid := util.RandomBytes(32)
		if err := state.AddProcess(&models.Process{
			ProcessId:    pid,
			EntityId:     util.RandomBytes(20),
			BlockCount:   10,
			VoteOptions:  &models.ProcessVoteOptions{MaxCount: 8, MaxValue: 3},
			EnvelopeType: &models.EnvelopeType{},
		}); err != nil {
			t.Fatal(err)
		}
		if err := sc.newEmptyProcess(pid); err != nil {
			t.Fatal(err)
		}
	}
	entities := make(map[string]bool)
	if ec := sc.EntityCount(); ec != int64(entityCount) {
		t.Fatalf("entity count is wrong, got %d expected %d", ec, entityCount)
	}
	var list []string
	last := 0
	for len(entities) <= entityCount {
		list = sc.EntityList(10, last)
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

func TestProcessList(t *testing.T) {
	testProcessList(t, 10)
	testProcessList(t, 20)
	testProcessList(t, 155)
}

func testProcessList(t *testing.T, procsCount int) {
	state, err := vochain.NewState(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}

	sc, err := NewScrutinizer(t.TempDir(), state)
	if err != nil {
		t.Fatal(err)
	}

	// Add 10 entities and process for storing random content
	for i := 0; i < 10; i++ {
		pid := util.RandomBytes(32)
		err := state.AddProcess(&models.Process{
			ProcessId:    pid,
			EntityId:     util.RandomBytes(20),
			VoteOptions:  &models.ProcessVoteOptions{MaxCount: 8, MaxValue: 3},
			EnvelopeType: &models.EnvelopeType{},
		})
		qt.Assert(t, err, qt.IsNil)
		err = sc.newEmptyProcess(pid)
		qt.Assert(t, err, qt.IsNil)
	}

	// For a entity, add 25 processes (this will be the queried entity)
	eidTest := util.RandomBytes(20)
	for i := 0; i < procsCount; i++ {
		pid := util.RandomBytes(32)
		err := state.AddProcess(&models.Process{
			ProcessId:    pid,
			EntityId:     eidTest,
			VoteOptions:  &models.ProcessVoteOptions{MaxCount: 8, MaxValue: 3},
			EnvelopeType: &models.EnvelopeType{},
		})
		qt.Assert(t, err, qt.IsNil)
		err = sc.newEmptyProcess(pid)
		qt.Assert(t, err, qt.IsNil)
	}

	procs := make(map[string]bool)
	last := 0
	var list [][]byte
	for len(procs) < procsCount {
		list, err = sc.ProcessList(eidTest, 0, "", false, last, 10)
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
}

func TestProcessListWithNamespaceAndStatus(t *testing.T) {
	state, err := vochain.NewState(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}

	sc, err := NewScrutinizer(t.TempDir(), state)
	if err != nil {
		t.Fatal(err)
	}

	// Add 10 processes with different namespaces (from 10 to 20) and status ENDED
	for i := 0; i < 10; i++ {
		pid := util.RandomBytes(32)
		err := state.AddProcess(&models.Process{
			ProcessId:    pid,
			EntityId:     util.RandomBytes(20),
			VoteOptions:  &models.ProcessVoteOptions{MaxCount: 8, MaxValue: 3},
			EnvelopeType: &models.EnvelopeType{},
			Namespace:    uint32(10 + i),
			Status:       models.ProcessStatus_ENDED,
		})
		qt.Assert(t, err, qt.IsNil)
		err = sc.newEmptyProcess(pid)
		qt.Assert(t, err, qt.IsNil)
	}

	// For a entity, add 10 processes on namespace 123 and status READY
	eid20 := util.RandomBytes(20)
	for i := 0; i < 10; i++ {
		pid := util.RandomBytes(32)
		err := state.AddProcess(&models.Process{
			ProcessId:    pid,
			EntityId:     eid20,
			VoteOptions:  &models.ProcessVoteOptions{MaxCount: 8, MaxValue: 3},
			EnvelopeType: &models.EnvelopeType{},
			Namespace:    123,
			Status:       models.ProcessStatus_READY,
		})
		qt.Assert(t, err, qt.IsNil)
		err = sc.newEmptyProcess(pid)
		qt.Assert(t, err, qt.IsNil)
	}

	// Get the process list for namespace 123
	list, err := sc.ProcessList(eid20, 123, "", false, 0, 100)
	qt.Assert(t, err, qt.IsNil)
	// Check there are exactly 10
	qt.Assert(t, len(list), qt.CmpEquals(), 10)

	// Get the process list for all namespaces
	list, err = sc.ProcessList(nil, 0, "", false, 0, 100)
	qt.Assert(t, err, qt.IsNil)
	// Check there are exactly 10 + 10
	qt.Assert(t, len(list), qt.CmpEquals(), 20)

	// Get the process list for namespace 10
	list, err = sc.ProcessList(nil, 10, "", false, 0, 100)
	qt.Assert(t, err, qt.IsNil)
	// Check there is exactly 1
	qt.Assert(t, len(list), qt.CmpEquals(), 1)

	// Get the process list for namespace 10
	list, err = sc.ProcessList(nil, 0, "READY", false, 0, 100)
	qt.Assert(t, err, qt.IsNil)
	// Check there is exactly 1
	qt.Assert(t, len(list), qt.CmpEquals(), 10)
}

func TestResults(t *testing.T) {
	state, err := vochain.NewState(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}

	sc, err := NewScrutinizer(t.TempDir(), state)
	if err != nil {
		t.Fatal(err)
	}
	pid := util.RandomBytes(32)
	err = state.AddProcess(&models.Process{
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

	err = sc.newEmptyProcess(pid)
	qt.Assert(t, err, qt.IsNil)

	priv, err := nacl.DecodePrivate(fmt.Sprintf("%x", ethereum.HashRaw(util.RandomBytes(32))))
	if err != nil {
		t.Fatalf("cannot generate encryption key: (%s)", err)
	}
	ki := uint32(1)
	err = state.AddProcessKeys(&models.AdminTx{
		Txtype:              models.TxType_ADD_PROCESS_KEYS,
		ProcessId:           pid,
		EncryptionPublicKey: priv.Public().Bytes(),
		KeyIndex:            &ki,
	})
	qt.Assert(t, err, qt.IsNil)

	// Add 100 votes
	vp, err := json.Marshal(types.VotePackage{
		Nonce: fmt.Sprintf("%x", util.RandomBytes(32)),
		Votes: []int{1, 1, 1, 1},
	})
	qt.Assert(t, err, qt.IsNil)
	vp, err = priv.Encrypt(vp, nil)
	qt.Assert(t, err, qt.IsNil)

	sc.Rollback()
	for i := 0; i < 300; i++ {
		err := state.AddVote(&models.Vote{
			ProcessId:            pid,
			VotePackage:          vp,
			EncryptionKeyIndexes: []uint32{1},
			Nullifier:            util.RandomBytes(32),
			Weight:               new(big.Int).SetUint64(1).Bytes(),
		})
		qt.Assert(t, err, qt.IsNil)
	}

	// Reveal process encryption keys
	err = state.RevealProcessKeys(&models.AdminTx{
		Txtype:               models.TxType_ADD_PROCESS_KEYS,
		ProcessId:            pid,
		EncryptionPrivateKey: priv.Bytes(),
		KeyIndex:             &ki,
	})
	qt.Assert(t, err, qt.IsNil)
	err = sc.updateProcess(pid)
	qt.Assert(t, err, qt.IsNil)
	err = sc.setResultsHeight(pid, uint32(state.Header(false).GetHeight()))
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
			value = result.Votes[q][qi]
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
	state, err := vochain.NewState(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}

	sc, err := NewScrutinizer(t.TempDir(), state)
	if err != nil {
		t.Fatal(err)
	}
	pid := util.RandomBytes(32)
	if err := state.AddProcess(&models.Process{
		ProcessId:    pid,
		EnvelopeType: &models.EnvelopeType{EncryptedVotes: false},
		Status:       models.ProcessStatus_READY,
		BlockCount:   10,
		VoteOptions:  &models.ProcessVoteOptions{MaxCount: 3, MaxValue: 1},
		Mode:         &models.ProcessMode{AutoStart: true},
	}); err != nil {
		t.Fatal(err)
	}
	err = sc.newEmptyProcess(pid)
	qt.Assert(t, err, qt.IsNil)

	// Add 100 votes
	vp, err := json.Marshal(types.VotePackage{
		Nonce: fmt.Sprintf("%x", util.RandomHex(32)),
		Votes: []int{1, 1, 1},
	})
	qt.Assert(t, err, qt.IsNil)
	v := &models.Vote{ProcessId: pid, VotePackage: vp}
	r := &Results{
		Votes:  newEmptyVotes(3, 2),
		Weight: new(big.Int).SetUint64(0),
	}
	sc.addProcessToLiveResults(pid)
	for i := 0; i < 100; i++ {
		qt.Assert(t, sc.addLiveVote(v, r), qt.IsNil)
	}
	qt.Assert(t, sc.commitVotes(pid, r), qt.IsNil)

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
			if qi > 2 {
				t.Fatalf("found more questions that expected")
			}
			value = result.Votes[q][qi]
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
	state, err := vochain.NewState(t.TempDir())
	qt.Assert(t, err, qt.IsNil)

	sc, err := NewScrutinizer(t.TempDir(), state)
	qt.Assert(t, err, qt.IsNil)

	options := &models.ProcessVoteOptions{
		MaxCount:     3,
		MaxValue:     3,
		MaxTotalCost: 6,
		CostExponent: 1,
	}

	pid := util.RandomBytes(32)
	if err := state.AddProcess(&models.Process{
		ProcessId:    pid,
		EnvelopeType: &models.EnvelopeType{EncryptedVotes: false},
		Status:       models.ProcessStatus_READY,
		BlockCount:   10,
		VoteOptions:  options,
		Mode:         &models.ProcessMode{AutoStart: true},
	}); err != nil {
		t.Fatal(err)
	}
	err = sc.newEmptyProcess(pid)
	qt.Assert(t, err, qt.IsNil)

	envelopeType := &models.EnvelopeType{}

	pr, err := sc.GetResults(pid)
	qt.Assert(t, err, qt.IsNil)
	// Should be fine
	pr.Votes, err = addVote(pr.Votes, []int{1, 2, 3}, nil, options, envelopeType)
	qt.Assert(t, err, qt.IsNil)

	// Overflows maxTotalCost
	pr.Votes, err = addVote(pr.Votes, []int{2, 2, 3}, nil, options, envelopeType)
	qt.Assert(t, err, qt.ErrorMatches, "max total cost overflow.*")

	// Overflows maxValue
	pr.Votes, err = addVote(pr.Votes, []int{1, 1, 4}, nil, options, envelopeType)
	qt.Assert(t, err, qt.ErrorMatches, "max value overflow.*")

	// Overflows maxCount
	pr.Votes, err = addVote(pr.Votes, []int{1, 1, 1, 1}, nil, options, envelopeType)
	qt.Assert(t, err, qt.ErrorMatches, "max count overflow.*")

	// Quadratic voting, 10 credits to distribute among 3 options
	options = &models.ProcessVoteOptions{
		MaxCount:     3,
		MaxValue:     0,
		MaxTotalCost: 10,
		CostExponent: 2,
	}

	// Should be fine 2^2 + 2^2 + 1^2 = 9
	pr.Votes, err = addVote(pr.Votes, []int{2, 2, 1}, nil, options, envelopeType)
	qt.Assert(t, err, qt.IsNil)

	// Should be fine 3^2 + 0 + 0 = 9
	pr.Votes, err = addVote(pr.Votes, []int{3, 0, 0}, nil, options, envelopeType)
	qt.Assert(t, err, qt.IsNil)

	// Should fail since 2^2 + 2^2 + 2^2 = 12
	pr.Votes, err = addVote(pr.Votes, []int{2, 2, 2}, nil, options, envelopeType)
	qt.Assert(t, err, qt.ErrorMatches, "max total cost overflow.*")

	// Should fail since 4^2 = 16
	pr.Votes, err = addVote(pr.Votes, []int{4, 0, 0}, nil, options, envelopeType)
	qt.Assert(t, err, qt.ErrorMatches, "max total cost overflow.*")

	// Check unique values work
	envelopeType = &models.EnvelopeType{UniqueValues: true}
	pr.Votes, err = addVote(pr.Votes, []int{2, 1, 1}, nil, options, envelopeType)
	qt.Assert(t, err, qt.ErrorMatches, "values are not unique")
}

var vote = func(v []int, sc *Scrutinizer, pid []byte) error {
	vp, err := json.Marshal(types.VotePackage{
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
	r := &Results{
		Votes:  newEmptyVotes(len(v), max+1),
		Weight: new(big.Int).SetUint64(1),
	}
	sc.addProcessToLiveResults(pid)
	if err := sc.addLiveVote(
		&models.Vote{
			ProcessId:   pid,
			VotePackage: vp,
		}, r); err != nil {
		return err
	}
	return sc.commitVotes(pid, r)
}

func TestBallotProtocolRateProduct(t *testing.T) {
	// Rate a product from 0 to 4
	state, err := vochain.NewState(t.TempDir())
	qt.Assert(t, err, qt.IsNil)

	sc, err := NewScrutinizer(t.TempDir(), state)
	qt.Assert(t, err, qt.IsNil)

	// Rate 2 products from 0 to 4
	pid := util.RandomBytes(32)
	if err := state.AddProcess(&models.Process{
		ProcessId:    pid,
		EnvelopeType: &models.EnvelopeType{EncryptedVotes: false},
		Status:       models.ProcessStatus_READY,
		BlockCount:   10,
		Mode:         &models.ProcessMode{AutoStart: true},
		VoteOptions:  &models.ProcessVoteOptions{MaxCount: 2, MaxValue: 4},
	}); err != nil {
		t.Fatal(err)
	}

	qt.Assert(t, sc.newEmptyProcess(pid), qt.IsNil)

	// Rate a product, exepected result: [ [1,0,1,0,2], [0,0,2,0,2] ]
	qt.Assert(t, vote([]int{4, 2}, sc, pid), qt.IsNil)
	qt.Assert(t, vote([]int{4, 2}, sc, pid), qt.IsNil)
	qt.Assert(t, vote([]int{2, 4}, sc, pid), qt.IsNil)
	qt.Assert(t, vote([]int{0, 4}, sc, pid), qt.IsNil)
	qt.Assert(t, vote([]int{0, 5}, sc, pid), qt.ErrorMatches, ".*overflow.*")
	qt.Assert(t, vote([]int{0, 0, 0}, sc, pid), qt.ErrorMatches, ".*")

	result, err := sc.GetResults(pid)
	qt.Assert(t, err, qt.IsNil)
	votes := GetFriendlyResults(result.Votes)
	qt.Assert(t, votes[0], qt.DeepEquals, []string{"1", "0", "1", "0", "2"})
	qt.Assert(t, votes[1], qt.DeepEquals, []string{"0", "0", "2", "0", "2"})
}

func TestBallotProtocolMultiChoice(t *testing.T) {
	// Choose your 3 favorite colours out of 5

	state, err := vochain.NewState(t.TempDir())
	qt.Assert(t, err, qt.IsNil)

	sc, err := NewScrutinizer(t.TempDir(), state)
	qt.Assert(t, err, qt.IsNil)

	// Rate 2 products from 0 to 4
	pid := util.RandomBytes(32)
	if err := state.AddProcess(&models.Process{
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

	err = sc.newEmptyProcess(pid)
	qt.Assert(t, err, qt.IsNil)

	// Multichoice (choose 3 ouf of 5):
	// - Vote Envelope: `[1,1,1,0,0]` `[0,1,1,1,0]` `[1,1,0,0,0]`
	// - Results: `[ [1, 2], [0, 3], [1, 2], [2, 1], [3, 0] ]`
	qt.Assert(t, vote([]int{1, 1, 1, 0, 0}, sc, pid), qt.IsNil)
	qt.Assert(t, vote([]int{0, 1, 1, 1, 0}, sc, pid), qt.IsNil)
	qt.Assert(t, vote([]int{1, 1, 0, 0, 0}, sc, pid), qt.IsNil)
	qt.Assert(t, vote([]int{2, 1, 0, 0, 0}, sc, pid), qt.ErrorMatches, ".*overflow.*")
	qt.Assert(t, vote([]int{1, 1, 1, 1, 0}, sc, pid), qt.ErrorMatches, ".*overflow.*")

	result, err := sc.GetResults(pid)
	qt.Assert(t, err, qt.IsNil)
	votes := GetFriendlyResults(result.Votes)
	qt.Assert(t, votes[0], qt.DeepEquals, []string{"1", "2"})
	qt.Assert(t, votes[1], qt.DeepEquals, []string{"0", "3"})
	qt.Assert(t, votes[2], qt.DeepEquals, []string{"1", "2"})
	qt.Assert(t, votes[3], qt.DeepEquals, []string{"2", "1"})
	qt.Assert(t, votes[4], qt.DeepEquals, []string{"3", "0"})
}

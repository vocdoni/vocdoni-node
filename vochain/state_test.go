package vochain

import (
	"fmt"
	"math/rand"
	"testing"

	qt "github.com/frankban/quicktest"
	"go.vocdoni.io/dvote/log"
	models "go.vocdoni.io/proto/build/go/models"
)

type Random struct {
	rand *rand.Rand
}

func newRandom(seed int64) Random {
	return Random{
		rand: rand.New(rand.NewSource(seed)),
	}
}

func (r *Random) RandomBytes(n int) []byte {
	b := make([]byte, n)
	_, err := r.rand.Read(b)
	if err != nil {
		panic(err)
	}
	return b
}

func TestStateReopen(t *testing.T) {
	dir := t.TempDir()
	s, err := NewState(dir)
	qt.Assert(t, err, qt.IsNil)
	hash1Before, err := s.Save()
	qt.Assert(t, err, qt.IsNil)

	s.db.Close()

	s, err = NewState(dir)
	qt.Assert(t, err, qt.IsNil)
	hash1After, err := s.Store.Hash()
	qt.Assert(t, err, qt.IsNil)
	qt.Assert(t, hash1After, qt.DeepEquals, hash1Before)
}

func TestStateBasic(t *testing.T) {
	rng := newRandom(0)
	log.Init("info", "stdout")
	s, err := NewState(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}

	var pids [][]byte
	for i := 0; i < 100; i++ {
		pids = append(pids, rng.RandomBytes(32))
		censusURI := "ipfs://foobar"
		p := &models.Process{EntityId: rng.RandomBytes(32), CensusURI: &censusURI, ProcessId: pids[i]}
		if err := s.AddProcess(p); err != nil {
			t.Fatal(err)
		}

		for j := 0; j < 10; j++ {
			v := &models.Vote{
				ProcessId:   pids[i],
				Nullifier:   rng.RandomBytes(32),
				VotePackage: []byte(fmt.Sprintf("%d%d", i, j)),
			}
			if err := s.AddVote(v); err != nil {
				t.Error(err)
			}
		}
		totalVotes, err := s.VoteCount(false)
		qt.Assert(t, err, qt.IsNil)
		qt.Assert(t, totalVotes, qt.Equals, uint64(10*(i+1)))
	}
	s.Save()

	p, err := s.Process(pids[10], false)
	if err != nil {
		t.Error(err)
	}
	if len(p.EntityId) != 32 {
		t.Errorf("entityID is not correct")
	}

	_, err = s.Process(rng.RandomBytes(32), false)
	if err == nil {
		t.Errorf("process must not exist")
	}

	totalVotes, err := s.VoteCount(false)
	qt.Assert(t, err, qt.IsNil)
	qt.Assert(t, totalVotes, qt.Equals, uint64(100*10))
	totalVotes, err = s.VoteCount(true)
	qt.Assert(t, err, qt.IsNil)
	qt.Assert(t, totalVotes, qt.Equals, uint64(100*10))

	votes := s.CountVotes(pids[40], false)
	if votes != 10 {
		t.Errorf("missing votes for process %x (got %d expected %d)", pids[40], votes, 10)
	}
	nullifiers := s.EnvelopeList(pids[50], 0, 20, false)
	if len(nullifiers) != 10 {
		t.Errorf("missing vote nullifiers (got %d expected %d)", len(nullifiers), 10)
	}
	nullifiers = s.EnvelopeList(pids[50], 0, 5, false)
	if len(nullifiers) != 5 {
		t.Errorf("missing vote nullifiers (got %d expected %d)", len(nullifiers), 5)
	}
}

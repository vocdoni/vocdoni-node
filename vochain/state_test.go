package vochain

import (
	"fmt"
	"math/rand"
	"testing"

	qt "github.com/frankban/quicktest"
	"go.vocdoni.io/dvote/crypto/ethereum"
	"go.vocdoni.io/dvote/db"
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
	s, err := NewState(db.TypePebble, dir)
	qt.Assert(t, err, qt.IsNil)
	hash1Before, err := s.Save()
	qt.Assert(t, err, qt.IsNil)

	s.Close()

	s, err = NewState(db.TypePebble, dir)
	qt.Assert(t, err, qt.IsNil)
	hash1After, err := s.Store.Hash()
	qt.Assert(t, err, qt.IsNil)
	qt.Assert(t, hash1After, qt.DeepEquals, hash1Before)

	s.Close()
}

func TestStateBasic(t *testing.T) {
	rng := newRandom(0)
	log.Init("info", "stdout")
	s, err := NewState(db.TypePebble, t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()

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

func TestBalanceTransfer(t *testing.T) {
	log.Init("info", "stdout")
	s, err := NewState(db.TypePebble, t.TempDir())
	qt.Assert(t, err, qt.IsNil)
	defer s.Close()
	addr1 := ethereum.SignKeys{}
	addr1.Generate()
	addr2 := ethereum.SignKeys{}
	addr2.Generate()

	err = s.MintBalance(addr1.Address(), 50)
	qt.Assert(t, err, qt.IsNil)

	s.Save() // Save to test isQuery value on next call
	b1, err := s.GetBalance(addr1.Address(), true)
	qt.Assert(t, err, qt.IsNil)
	qt.Assert(t, b1.Amount, qt.Equals, uint64(50))
	qt.Assert(t, b1.Nonce, qt.Equals, uint32(0))

	err = s.TransferBalance(addr1.Address(), addr2.Address(), 20, 1, false)
	qt.Assert(t, err, qt.IsNil)

	b2, err := s.GetBalance(addr2.Address(), false)
	qt.Assert(t, err, qt.IsNil)
	qt.Assert(t, b2.Amount, qt.Equals, uint64(20))

	err = s.TransferBalance(addr1.Address(), addr2.Address(), 20, 1, false)
	qt.Assert(t, err, qt.IsNotNil)

	err = s.TransferBalance(addr1.Address(), addr2.Address(), 40, 2, false)
	qt.Assert(t, err, qt.IsNotNil)

	err = s.TransferBalance(addr2.Address(), addr1.Address(), 10, 1, false)
	qt.Assert(t, err, qt.IsNil)

	err = s.TransferBalance(addr2.Address(), addr1.Address(), 5, 2, false)
	qt.Assert(t, err, qt.IsNil)

	b1, err = s.GetBalance(addr1.Address(), false)
	qt.Assert(t, err, qt.IsNil)
	qt.Assert(t, b1.Amount, qt.Equals, uint64(45))
	qt.Assert(t, b1.Nonce, qt.Equals, uint32(1))

	s.Save()
	b2, err = s.GetBalance(addr2.Address(), true)
	qt.Assert(t, err, qt.IsNil)
	qt.Assert(t, b2.Amount, qt.Equals, uint64(5))
	qt.Assert(t, b2.Nonce, qt.Equals, uint32(2))
}

package state

import (
	"bytes"
	"fmt"
	"runtime"
	"testing"

	qt "github.com/frankban/quicktest"
	"go.vocdoni.io/dvote/crypto/ethereum"
	"go.vocdoni.io/dvote/db"
	"go.vocdoni.io/dvote/log"
	"go.vocdoni.io/dvote/test/testcommon/testutil"
	"go.vocdoni.io/dvote/vochain/transaction/vochaintx"
	"go.vocdoni.io/proto/build/go/models"
)

func testSaveState(t *testing.T, s *State) []byte {
	_, err := s.PrepareCommit()
	qt.Assert(t, err, qt.IsNil)
	hash, err := s.Save()
	qt.Assert(t, err, qt.IsNil)
	return hash
}

func TestStateReopen(t *testing.T) {
	dir := t.TempDir()
	s, err := New(db.TypePebble, dir)
	qt.Assert(t, err, qt.IsNil)
	hash1Before := testSaveState(t, s)
	qt.Assert(t, err, qt.IsNil)

	s.Close()

	s, err = New(db.TypePebble, dir)
	qt.Assert(t, err, qt.IsNil)
	hash1After, err := s.store.Hash()
	qt.Assert(t, err, qt.IsNil)
	qt.Assert(t, hash1After, qt.DeepEquals, hash1Before)

	s.Close()
}

func TestStateBasic(t *testing.T) {
	rng := testutil.NewRandom(0)
	log.Init("info", "stdout", nil)
	s, err := New(db.TypePebble, t.TempDir())
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
			vp, err := NewVotePackage([]int{i, j}).Encode()
			if err != nil {
				t.Error(err)
			}
			v := &Vote{
				ProcessID:   pids[i],
				Nullifier:   rng.RandomBytes(32),
				VotePackage: vp,
			}
			if err := s.AddVote(v); err != nil {
				t.Error(err)
			}
		}
		totalVotes, err := s.CountTotalVotes()
		qt.Assert(t, err, qt.IsNil)
		qt.Assert(t, totalVotes, qt.Equals, uint64(10*(i+1)))
	}
	testSaveState(t, s)

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

	totalVotes, err := s.CountTotalVotes()
	qt.Assert(t, err, qt.IsNil)
	qt.Assert(t, totalVotes, qt.Equals, uint64(100*10))
	totalVotes, err = s.CountTotalVotes()
	qt.Assert(t, err, qt.IsNil)
	qt.Assert(t, totalVotes, qt.Equals, uint64(100*10))

	votes, err := s.CountVotes(pids[40], false)
	qt.Assert(t, err, qt.IsNil)
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
	log.Init("info", "stdout", nil)
	s, err := New(db.TypePebble, t.TempDir())
	qt.Assert(t, err, qt.IsNil)
	defer s.Close()
	addr1 := ethereum.SignKeys{}
	addr1.Generate()
	addr2 := ethereum.SignKeys{}
	addr2.Generate()

	err = s.CreateAccount(addr1.Address(), "ipfs://", [][]byte{}, 50)
	qt.Assert(t, err, qt.IsNil)
	err = s.CreateAccount(addr2.Address(), "ipfs://", [][]byte{}, 0)
	qt.Assert(t, err, qt.IsNil)

	testSaveState(t, s) // Save to test committed value on next call
	b1, err := s.GetAccount(addr1.Address(), true)
	qt.Assert(t, err, qt.IsNil)
	qt.Assert(t, b1.Balance, qt.Equals, uint64(50))

	err = s.TransferBalance(&vochaintx.TokenTransfer{
		FromAddress: addr1.Address(),
		ToAddress:   addr2.Address(),
		Amount:      20,
	}, false)
	qt.Assert(t, err, qt.IsNil)

	b2, err := s.GetAccount(addr2.Address(), false)
	qt.Assert(t, err, qt.IsNil)
	qt.Assert(t, b2.Balance, qt.Equals, uint64(20))

	err = s.TransferBalance(&vochaintx.TokenTransfer{
		FromAddress: addr1.Address(),
		ToAddress:   addr2.Address(),
		Amount:      40,
	}, false)
	qt.Assert(t, err, qt.IsNotNil)

	err = s.TransferBalance(&vochaintx.TokenTransfer{
		FromAddress: addr2.Address(),
		ToAddress:   addr1.Address(),
		Amount:      10,
	}, false)
	qt.Assert(t, err, qt.IsNil)

	err = s.TransferBalance(&vochaintx.TokenTransfer{
		FromAddress: addr2.Address(),
		ToAddress:   addr1.Address(),
		Amount:      5,
	}, false)
	qt.Assert(t, err, qt.IsNil)

	b1, err = s.GetAccount(addr1.Address(), false)
	qt.Assert(t, err, qt.IsNil)
	qt.Assert(t, b1.Balance, qt.Equals, uint64(45))

	testSaveState(t, s)
	b2, err = s.GetAccount(addr2.Address(), true)
	qt.Assert(t, err, qt.IsNil)
	qt.Assert(t, b2.Balance, qt.Equals, uint64(5))

}

type Listener struct {
	processStart [][][]byte
}

func (*Listener) OnVote(_ *Vote, _ int32)                                         {}
func (*Listener) OnNewTx(_ *vochaintx.Tx, _ uint32, _ int32)                      {}
func (*Listener) OnBeginBlock(BeginBlock)                                         {}
func (*Listener) OnProcess(_, _ []byte, _, _ string, _ int32)                     {}
func (*Listener) OnProcessStatusChange(_ []byte, _ models.ProcessStatus, _ int32) {}
func (*Listener) OnCancel(_ []byte, _ int32)                                      {}
func (*Listener) OnProcessKeys(_ []byte, _ string, _ int32)                       {}
func (*Listener) OnRevealKeys(_ []byte, _ string, _ int32)                        {}
func (*Listener) OnProcessResults(_ []byte, _ *models.ProcessResult, _ int32)     {}
func (*Listener) OnCensusUpdate(_, _ []byte, _ string)                            {}
func (*Listener) OnSetAccount(_ []byte, _ *Account)                               {}
func (*Listener) OnTransferTokens(_ *vochaintx.TokenTransfer)                     {}
func (*Listener) OnSpendTokens(_ []byte, _ models.TxType, _ uint64, _ string)     {}
func (l *Listener) OnProcessesStart(pids [][]byte) {
	l.processStart = append(l.processStart, pids)
}
func (*Listener) Commit(_ uint32) (err error) {
	return nil
}
func (*Listener) Rollback() {}

func TestOnProcessStart(t *testing.T) {
	log.Init("info", "stdout", nil)
	s, err := New(db.TypePebble, t.TempDir())
	qt.Assert(t, err, qt.IsNil)
	defer s.Close()
	rng := testutil.NewRandom(0)
	listener := &Listener{}
	s.AddEventListener(listener)

	doBlock := func(height uint32, fn func()) {
		s.Rollback()
		s.SetHeight(height)
		fn()
		testSaveState(t, s)
	}

	pid := rng.RandomBytes(32)
	startBlock := uint32(4)
	doBlock(1, func() {
		censusURI := "ipfs://foobar"
		maxCensusSize := uint64(16)
		p := &models.Process{
			EntityId:   rng.RandomBytes(32),
			CensusURI:  &censusURI,
			ProcessId:  pid,
			StartBlock: startBlock,
			Mode: &models.ProcessMode{
				PreRegister: true,
			},
			EnvelopeType: &models.EnvelopeType{
				Anonymous: true,
			},
			MaxCensusSize: maxCensusSize,
		}
		qt.Assert(t, s.AddProcess(p), qt.IsNil)
	})
}

// TestBlockMemoryUsage prints the Heap usage by the number of votes in a
// block.  This is useful to analyze the memory taken by the underlying
// database transaction in the StateDB in a real scenario.
func TestBlockMemoryUsage(t *testing.T) {
	rng := testutil.NewRandom(0)
	log.Init("info", "stdout", nil)
	s, err := New(db.TypePebble, t.TempDir())
	qt.Assert(t, err, qt.IsNil)
	defer s.Close()

	var height uint32

	// block 1
	height = 1
	s.Rollback()
	s.SetHeight(height)

	pid := rng.RandomBytes(32)
	censusURI := "ipfs://foobar"
	p := &models.Process{
		EntityId:   rng.RandomBytes(32),
		CensusURI:  &censusURI,
		ProcessId:  pid,
		StartBlock: 2,
		Mode: &models.ProcessMode{
			PreRegister: false,
		},
		EnvelopeType: &models.EnvelopeType{
			Anonymous: false,
		},
	}
	qt.Assert(t, s.AddProcess(p), qt.IsNil)

	testSaveState(t, s)

	// block 2
	height = 2
	s.Rollback()
	s.SetHeight(height)

	var mem runtime.MemStats
	numVotes := 22_000
	for i := 0; i < numVotes; i++ {
		v := &Vote{
			ProcessID:   pid,
			Nullifier:   rng.RandomBytes(32),
			VotePackage: rng.RandomBytes(64),
		}
		qt.Assert(t, s.AddVote(v), qt.IsNil)

		if i%1_000 == 0 {
			runtime.GC()
			runtime.ReadMemStats(&mem)
			fmt.Printf("%v HeapAlloc: %v MiB, Heap:%v MiB\n",
				i, mem.HeapAlloc/1024/1024, (mem.HeapIdle+mem.HeapInuse)/1024/1024)
		}
	}

	testSaveState(t, s)
}

func TestStateSetGetTxCostByTxType(t *testing.T) {
	log.Init("info", "stdout", nil)
	s, err := New(db.TypePebble, t.TempDir())
	qt.Assert(t, err, qt.IsNil)
	defer s.Close()

	var height uint32 = 1
	s.Rollback()
	s.SetHeight(height)

	qt.Assert(t, s.SetTxBaseCost(models.TxType_SET_PROCESS_CENSUS, 100), qt.IsNil)
	cost, err := s.TxBaseCost(models.TxType_SET_PROCESS_CENSUS, false)
	qt.Assert(t, err, qt.IsNil)
	qt.Assert(t, cost, qt.Equals, uint64(100))
}

func TestNoState(t *testing.T) {
	s, err := New(db.TypePebble, t.TempDir())
	qt.Assert(t, err, qt.IsNil)
	defer func() {
		_ = s.Close()
	}()

	doBlock := func(height uint32, fn func()) []byte {
		s.Rollback()
		s.SetHeight(height)
		fn()
		return testSaveState(t, s)
	}

	ns := s.NoState(true)

	// commit first block
	hash1 := doBlock(1, func() {})

	// commit second block
	hash2 := doBlock(2, func() {
		err = ns.Set([]byte("foo"), []byte("bar"))
		qt.Assert(t, err, qt.IsNil)
	})

	// compare hash1 and hash 2, root hash should be the same
	qt.Assert(t, bytes.Equal(hash1, hash2), qt.IsTrue)

	// check that the is still in the nostate
	value, err := ns.Get([]byte("foo"))
	qt.Assert(t, err, qt.IsNil)
	qt.Assert(t, value, qt.CmpEquals(), []byte("bar"))

	// commit third block
	hash3 := doBlock(3, func() {
		err = ns.Delete([]byte("foo"))
		qt.Assert(t, err, qt.IsNil)
	})

	// compare hash2 and hash3, root hash should be the same
	qt.Assert(t, bytes.Equal(hash2, hash3), qt.IsTrue)

	// check that the value is not in the nostate
	_, err = ns.Get([]byte("foo"))
	qt.Assert(t, err, qt.Equals, db.ErrKeyNotFound)

}

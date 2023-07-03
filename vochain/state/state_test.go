package state

import (
	"fmt"
	"runtime"
	"testing"

	"github.com/ethereum/go-ethereum/common"
	qt "github.com/frankban/quicktest"
	"go.vocdoni.io/dvote/crypto/ethereum"
	"go.vocdoni.io/dvote/db"
	"go.vocdoni.io/dvote/log"
	"go.vocdoni.io/dvote/test/testcommon/testutil"
	"go.vocdoni.io/dvote/vochain/transaction/vochaintx"
	"go.vocdoni.io/proto/build/go/models"
)

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
	rng := testutil.NewRandom(0)
	log.Init("info", "stdout", nil)
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
		totalVotes, err := s.CountTotalVotes(false)
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

	totalVotes, err := s.CountTotalVotes(false)
	qt.Assert(t, err, qt.IsNil)
	qt.Assert(t, totalVotes, qt.Equals, uint64(100*10))
	totalVotes, err = s.CountTotalVotes(true)
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
	s, err := NewState(db.TypePebble, t.TempDir())
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

	s.Save() // Save to test committed value on next call
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

	s.Save()
	b2, err = s.GetAccount(addr2.Address(), true)
	qt.Assert(t, err, qt.IsNil)
	qt.Assert(t, b2.Balance, qt.Equals, uint64(5))

}

type Listener struct {
	processStart [][][]byte
}

func (l *Listener) OnVote(vote *Vote, txIndex int32)                                             {}
func (l *Listener) OnNewTx(tx *vochaintx.Tx, blockHeight uint32, txIndex int32)                  {}
func (l *Listener) OnBeginBlock(BeginBlock)                                                      {}
func (l *Listener) OnProcess(pid, eid []byte, censusRoot, censusURI string, txIndex int32)       {}
func (l *Listener) OnProcessStatusChange(pid []byte, status models.ProcessStatus, txIndex int32) {}
func (l *Listener) OnCancel(pid []byte, txIndex int32)                                           {}
func (l *Listener) OnProcessKeys(pid []byte, encryptionPub string, txIndex int32)                {}
func (l *Listener) OnRevealKeys(pid []byte, encryptionPriv string, txIndex int32)                {}
func (l *Listener) OnProcessResults(pid []byte, results *models.ProcessResult, txIndex int32) {
}
func (l *Listener) OnCensusUpdate(pid, censusRoot []byte, censusURI string) {}
func (l *Listener) OnSetAccount(addr []byte, account *Account) {
}
func (l *Listener) OnTransferTokens(tx *vochaintx.TokenTransfer) {
}
func (l *Listener) OnProcessesStart(pids [][]byte) {
	l.processStart = append(l.processStart, pids)
}
func (l *Listener) Commit(height uint32) (err error) {
	return nil
}
func (l *Listener) Rollback() {}

func TestOnProcessStart(t *testing.T) {
	log.Init("info", "stdout", nil)
	s, err := NewState(db.TypePebble, t.TempDir())
	qt.Assert(t, err, qt.IsNil)
	defer s.Close()
	rng := testutil.NewRandom(0)
	listener := &Listener{}
	s.AddEventListener(listener)

	doBlock := func(height uint32, fn func()) {
		s.Rollback()
		s.SetHeight(height)
		fn()
		_, err := s.Save()
		qt.Assert(t, err, qt.IsNil)
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

	for i := uint32(2); i < 6; i++ {
		doBlock(i, func() {
			if i < startBlock {
				key := rng.RandomInZKField()
				err := s.AddToRollingCensus(pid, key, nil)
				qt.Assert(t, err, qt.IsNil)
			}
		})
		if i >= startBlock {
			qt.Assert(t, listener.processStart, qt.DeepEquals, [][][]byte{{pid}})
		}
	}
}

// TestBlockMemoryUsage prints the Heap usage by the number of votes in a
// block.  This is useful to analyze the memory taken by the underlying
// database transaction in the StateDB in a real scenario.
func TestBlockMemoryUsage(t *testing.T) {
	rng := testutil.NewRandom(0)
	log.Init("info", "stdout", nil)
	s, err := NewState(db.TypePebble, t.TempDir())
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

	_, err = s.Save()
	qt.Assert(t, err, qt.IsNil)

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

	_, err = s.Save()
	qt.Assert(t, err, qt.IsNil)
}

func TestStateTreasurer(t *testing.T) {
	log.Init("info", "stdout", nil)
	s, err := NewState(db.TypePebble, t.TempDir())
	qt.Assert(t, err, qt.IsNil)
	defer s.Close()

	// block 1
	var height uint32 = 1
	s.Rollback()
	s.SetHeight(height)

	tAddr := common.HexToAddress("0x309Bd6959bf4289CDf9c7198cF9f4494e0244b7d")
	treasurer := &models.Treasurer{
		Address: tAddr.Bytes(),
		Nonce:   0,
	}
	qt.Assert(t, s.SetTreasurer(tAddr, 0), qt.IsNil)

	fetchedTreasurer, err := s.Treasurer(false)
	qt.Assert(t, err, qt.IsNil)
	qt.Assert(t, fetchedTreasurer.Address, qt.CmpEquals(), treasurer.Address)

	_, err = s.Treasurer(true)
	// key does not exist yet
	qt.Assert(t, err, qt.IsNotNil)

	_, err = s.Save()
	qt.Assert(t, err, qt.IsNil)

	fetchedTreasurer, err = s.Treasurer(true)
	qt.Assert(t, err, qt.IsNil)
	qt.Assert(t, fetchedTreasurer.Address, qt.CmpEquals(), treasurer.Address)
}

func TestStateIsTreasurer(t *testing.T) {
	log.Init("info", "stdout", nil)
	s, err := NewState(db.TypePebble, t.TempDir())
	qt.Assert(t, err, qt.IsNil)
	defer s.Close()

	var height uint32 = 1
	s.Rollback()
	s.SetHeight(height)

	tAddr := common.HexToAddress("0x309Bd6959bf4289CDf9c7198cF9f4494e0244b7d")
	qt.Assert(t, s.SetTreasurer(tAddr, 0), qt.IsNil)
	r, err := s.IsTreasurer(tAddr)
	qt.Assert(t, err, qt.IsNil)
	qt.Assert(t, r, qt.IsTrue)

	notTreasurer := "0x000000000000000000000000000000000000dead"
	r, err = s.IsTreasurer(common.HexToAddress(notTreasurer))
	qt.Assert(t, err, qt.IsNil)
	qt.Assert(t, r, qt.IsFalse)
}

func TestIncrementTreasurerNonce(t *testing.T) {
	log.Init("info", "stdout", nil)
	s, err := NewState(db.TypePebble, t.TempDir())
	qt.Assert(t, err, qt.IsNil)
	defer s.Close()

	var height uint32 = 1
	s.Rollback()
	s.SetHeight(height)

	tAddr := common.HexToAddress("0x309Bd6959bf4289CDf9c7198cF9f4494e0244b7d")
	qt.Assert(t, s.SetTreasurer(tAddr, 0), qt.IsNil)
	r, err := s.IsTreasurer(tAddr)
	qt.Assert(t, err, qt.IsNil)
	qt.Assert(t, r, qt.IsTrue)

	err = s.IncrementTreasurerNonce()
	qt.Assert(t, err, qt.IsNil)
	treasurer, err := s.Treasurer(false)
	qt.Assert(t, err, qt.IsNil)
	qt.Assert(t, treasurer.Nonce, qt.Equals, uint32(1))
}

func TestStateSetGetTxCostByTxType(t *testing.T) {
	log.Init("info", "stdout", nil)
	s, err := NewState(db.TypePebble, t.TempDir())
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

package indexer

import (
	"bytes"
	"context"
	"fmt"
	"math/big"
	"sync"
	"testing"
	"time"

	qt "github.com/frankban/quicktest"
	"go.vocdoni.io/dvote/crypto/ethereum"
	"go.vocdoni.io/dvote/log"
	"go.vocdoni.io/dvote/test/testcommon/testutil"
	"go.vocdoni.io/dvote/util"
	"go.vocdoni.io/dvote/vochain"
	indexerdb "go.vocdoni.io/dvote/vochain/indexer/db"
	"go.vocdoni.io/dvote/vochain/state"
	"go.vocdoni.io/dvote/vochain/transaction/vochaintx"
	"go.vocdoni.io/proto/build/go/models"
)

func BenchmarkIndexer(b *testing.B) {
	app := vochain.TestBaseApplication(b)

	idx := newTestIndexer(b, app)
	pid := util.RandomBytes(32)
	if err := app.State.AddProcess(&models.Process{
		ProcessId:    pid,
		EntityId:     util.RandomBytes(20),
		EnvelopeType: &models.EnvelopeType{EncryptedVotes: false},
		Status:       models.ProcessStatus_READY,
		BlockCount:   100000000,
		VoteOptions:  &models.ProcessVoteOptions{MaxCount: 3, MaxValue: 1},
		Mode:         &models.ProcessMode{AutoStart: true},
	}); err != nil {
		b.Fatal(err)
	}
	app.AdvanceTestBlock()

	s := new(ethereum.SignKeys)
	err := s.Generate()
	qt.Assert(b, err, qt.IsNil)

	vp, err := state.NewVotePackage([]int{1, 0, 1}).Encode()
	qt.Assert(b, err, qt.IsNil)

	rnd := testutil.NewRandom(1234)

	b.ReportAllocs()
	b.ResetTimer()

	var lastVotes []*state.Vote
	var lastTxs []*vochaintx.Tx

	// Note that we use qt's Check in the goroutines below,
	// since b.Fatal can only be called from the main goroutine.

	for i := 0; i < b.N; i++ {
		// Index $numInserts votes, and then do $numFetches across $concurrentReaders.
		// The read-only queries are done on the previous iteration, to ensure they are indexed.
		height := app.Height()
		const numInserts = 100
		const numFetches = 50
		const concurrentReaders = 5
		var curVotes []*state.Vote
		var curTxs []*vochaintx.Tx

		var wg sync.WaitGroup

		wg.Add(1)
		go func() {
			for j := 0; j < numInserts; j++ {
				txBlockIndex := int32(j)

				vote := &state.Vote{
					Height:      height,
					ProcessID:   pid,
					Nullifier:   rnd.RandomBytes(32),
					VotePackage: vp,
					Weight:      new(big.Int).SetUint64(1 + uint64(rnd.RandomIntn(9999))),
				}
				idx.OnVote(vote, txBlockIndex)
				curVotes = append(curVotes, vote)

				tx := &vochaintx.Tx{
					TxID:        rnd.Random32(),
					TxModelType: "vote",
					Tx:          &models.Tx{Payload: &models.Tx_Vote{}},
				}
				idx.OnNewTx(tx, height, txBlockIndex)
				curTxs = append(curTxs, tx)
			}
			app.AdvanceTestBlock()
			wg.Done()
		}()

		for reader := 0; reader < concurrentReaders; reader++ {
			if i == 0 {
				// lastVotes and lastTxs are empty at the beginning; nothing to fetch
				continue
			}
			wg.Add(1)
			go func() {
				numFetches := numFetches / concurrentReaders
				for j := 0; j < numFetches; j++ {
					vote := lastVotes[j%len(lastVotes)]
					tx := lastTxs[j%len(lastTxs)]

					voteRef, err := idx.GetEnvelope(vote.Nullifier)
					qt.Check(b, err, qt.IsNil)
					if err == nil {
						qt.Check(b, bytes.Equal(voteRef.Meta.Nullifier, vote.Nullifier), qt.IsTrue)
						qt.Check(b, bytes.Equal(voteRef.Meta.TxHash, tx.TxID[:]), qt.IsTrue)
					}

					txRef, err := idx.GetTxMetadataByHash(tx.TxID[:])
					qt.Check(b, err, qt.IsNil)
					if err == nil {
						qt.Check(b, txRef.BlockHeight, qt.Equals, vote.Height)
					}
				}
				wg.Done()
			}()
		}
		wg.Wait()

		lastVotes = curVotes
		lastTxs = curTxs
	}
}

func BenchmarkFetchTx(b *testing.B) {
	numTxs := 1000
	app := vochain.TestBaseApplication(b)

	idx := newTestIndexer(b, app)

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		for j := 0; j < numTxs; j++ {
			idx.OnNewTx(&vochaintx.Tx{
				TxID:        util.Random32(),
				TxModelType: "vote",
				Tx:          &models.Tx{Payload: &models.Tx_Vote{}},
			}, uint32(i), int32(j))
		}
		err := idx.Commit(uint32(i))
		qt.Assert(b, err, qt.IsNil)

		time.Sleep(time.Second * 2)

		startTime := time.Now()
		for j := 0; j < numTxs; j++ {
			_, err = idx.GetTransactionByHeightAndIndex(int64(i), int64(j))
			qt.Assert(b, err, qt.IsNil)
		}
		log.Infof("fetched %d transactions (out of %d total) by height+index, took %s",
			numTxs, (i+1)*numTxs, time.Since(startTime))
		startTime = time.Now()
		for j := 0; j < numTxs; j++ {
			_, err = idx.GetTxMetadataByHash([]byte(fmt.Sprintf("hash%d%d", i, j)))
			qt.Assert(b, err, qt.IsNil)
		}
		log.Infof("fetched %d transactions (out of %d total) by hash, took %s",
			numTxs, (i+1)*numTxs, time.Since(startTime))
	}
}

func BenchmarkNewProcess(b *testing.B) {
	app := vochain.TestBaseApplication(b)

	idx := newTestIndexer(b, app)
	_ = idx // used via the callbacks; we want to benchmark it too
	startTime := time.Now()
	numProcesses := b.N
	entityID := util.RandomBytes(20)

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < numProcesses; i++ {
		pid := util.RandomBytes(32)
		if err := app.State.AddProcess(&models.Process{
			ProcessId: pid,
			// EntityId:     util.RandomBytes(20),
			EntityId:     entityID,
			EnvelopeType: &models.EnvelopeType{EncryptedVotes: false},
			Status:       models.ProcessStatus_READY,
			BlockCount:   100000000,
			VoteOptions:  &models.ProcessVoteOptions{MaxCount: 3, MaxValue: 1},
			Mode:         &models.ProcessMode{AutoStart: true},
		}); err != nil {
			b.Fatal(err)
		}
	}
	app.AdvanceTestBlock()
	log.Infof("indexed %d new processes, took %s",
		numProcesses, time.Since(startTime))
}

func BenchmarkBlockList(b *testing.B) {
	app := vochain.TestBaseApplication(b)

	idx, err := New(app, Options{DataDir: b.TempDir()})
	qt.Assert(b, err, qt.IsNil)

	count := 100000

	createDummyBlocks(b, idx, count)

	b.ReportAllocs()
	b.ResetTimer()

	benchmarkBlockList := func(b *testing.B,
		limit int, offset int, chainID string, hash string, proposerAddress string,
	) {
		b.ReportAllocs()
		b.ResetTimer()

		for i := 0; i < b.N; i++ {
			blocks, total, err := idx.BlockList(limit, offset, chainID, hash, proposerAddress)
			qt.Assert(b, err, qt.IsNil)
			qt.Assert(b, blocks, qt.HasLen, limit, qt.Commentf("%+v", blocks))
			qt.Assert(b, blocks[0].TxCount, qt.Equals, int64(0))
			qt.Assert(b, total, qt.Equals, uint64(count))
		}
	}

	// Run sub-benchmarks with different limits and filters
	b.Run("BlockListLimit1", func(b *testing.B) {
		benchmarkBlockList(b, 1, 0, "", "", "")
	})

	b.Run("BlockListLimit10", func(b *testing.B) {
		benchmarkBlockList(b, 10, 0, "", "", "")
	})

	b.Run("BlockListLimit100", func(b *testing.B) {
		benchmarkBlockList(b, 100, 0, "", "", "")
	})

	b.Run("BlockListOffset", func(b *testing.B) {
		benchmarkBlockList(b, 10, count/2, "", "", "")
	})

	b.Run("BlockListWithChainID", func(b *testing.B) {
		benchmarkBlockList(b, 10, 0, "test", "", "")
	})

	b.Run("BlockListWithHashSubstr", func(b *testing.B) {
		benchmarkBlockList(b, 10, 0, "", "cafe", "")
	})
	b.Run("BlockListWithHashExact", func(b *testing.B) {
		benchmarkBlockList(b, 10, 0, "", "cafecafecafecafecafecafecafecafecafecafecafecafecafecafecafecafe", "")
	})
}

func createDummyBlocks(b *testing.B, idx *Indexer, n int) {
	idx.blockMu.Lock()
	defer idx.blockMu.Unlock()

	queries := idx.blockTxQueries()
	for h := 1; h <= n; h++ {
		_, err := queries.CreateBlock(context.TODO(), indexerdb.CreateBlockParams{
			ChainID: "test",
			Height:  int64(h),
			Time:    time.Now(),
			Hash: nonNullBytes([]byte{
				0xca, 0xfe, 0xca, 0xfe, 0xca, 0xfe, 0xca, 0xfe, 0xca, 0xfe, 0xca, 0xfe, 0xca, 0xfe, 0xca, 0xfe,
				0xca, 0xfe, 0xca, 0xfe, 0xca, 0xfe, 0xca, 0xfe, 0xca, 0xfe, 0xca, 0xfe, 0xca, 0xfe, 0xca, 0xfe,
			}),
			ProposerAddress: nonNullBytes([]byte{0xfe, 0xde}),
			LastBlockHash:   nonNullBytes([]byte{0xca, 0xfe}),
		},
		)
		qt.Assert(b, err, qt.IsNil)
	}
	err := idx.blockTx.Commit()
	qt.Assert(b, err, qt.IsNil)
}

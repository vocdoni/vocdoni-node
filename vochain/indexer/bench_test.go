package indexer

import (
	"fmt"
	"math/big"
	"testing"
	"time"

	qt "github.com/frankban/quicktest"
	"go.vocdoni.io/dvote/crypto/ethereum"
	"go.vocdoni.io/dvote/log"
	"go.vocdoni.io/dvote/test/testcommon/testutil"
	"go.vocdoni.io/dvote/util"
	"go.vocdoni.io/dvote/vochain"
	"go.vocdoni.io/dvote/vochain/state"
	"go.vocdoni.io/dvote/vochain/transaction/vochaintx"
	"go.vocdoni.io/proto/build/go/models"
)

func BenchmarkIndexVotes(b *testing.B) {
	app := vochain.TestBaseApplication(b)

	idx := newTestIndexer(b, app, true)
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
	for i := 0; i < b.N; i++ {
		// Index a number of votes, then run a couple of queries.
		height := uint32(500 + i)
		const totalVotes = 100
		var oneVote *state.Vote
		var oneTx *vochaintx.Tx
		for j := 0; j < totalVotes; j++ {
			txBlockIndex := int32(j)

			vote := &state.Vote{
				Height:      height,
				ProcessID:   pid,
				Nullifier:   rnd.RandomBytes(32),
				VotePackage: vp,
				Weight:      new(big.Int).SetUint64(1 + uint64(rnd.RandomIntn(9999))),
			}
			idx.OnVote(vote, txBlockIndex)

			tx := &vochaintx.Tx{
				TxID:        rnd.Random32(),
				TxModelType: "vote",
			}
			idx.OnNewTx(tx, height, txBlockIndex)
			if j == totalVotes/2 {
				oneVote = vote
				oneTx = tx
			}
		}
		app.AdvanceTestBlock()

		voteRef, err := idx.GetEnvelopeReference(oneVote.Nullifier)
		qt.Assert(b, err, qt.IsNil)
		qt.Assert(b, voteRef.Weight.MathBigInt().Cmp(oneVote.Weight), qt.Equals, 0)
		qt.Assert(b, []byte(voteRef.TxHash), qt.DeepEquals, oneTx.TxID[:])
	}
}

func BenchmarkFetchTx(b *testing.B) {
	numTxs := 1000
	app := vochain.TestBaseApplication(b)

	idx := newTestIndexer(b, app, true)

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		for j := 0; j < numTxs; j++ {
			idx.OnNewTx(&vochaintx.Tx{TxID: util.Random32()}, uint32(i), int32(j))
		}
		err := idx.Commit(uint32(i))
		qt.Assert(b, err, qt.IsNil)

		time.Sleep(time.Second * 2)

		startTime := time.Now()
		for j := 0; j < numTxs; j++ {
			_, err = idx.GetTxReference(uint64((i * numTxs) + j + 1))
			qt.Assert(b, err, qt.IsNil)
		}
		log.Infof("fetched %d transactions (out of %d total) by index, took %s",
			numTxs, (i+1)*numTxs, time.Since(startTime))
		startTime = time.Now()
		for j := 0; j < numTxs; j++ {
			_, err = idx.GetTxHashReference([]byte(fmt.Sprintf("hash%d%d", i, j)))
			qt.Assert(b, err, qt.IsNil)
		}
		log.Infof("fetched %d transactions (out of %d total) by hash, took %s",
			numTxs, (i+1)*numTxs, time.Since(startTime))
	}
}

func BenchmarkNewProcess(b *testing.B) {
	app := vochain.TestBaseApplication(b)

	idx := newTestIndexer(b, app, true)
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

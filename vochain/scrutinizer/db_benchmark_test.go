package scrutinizer

import (
	"math/big"
	"testing"
	"time"

	qt "github.com/frankban/quicktest"
	"go.vocdoni.io/dvote/crypto/ethereum"
	"go.vocdoni.io/dvote/log"
	"go.vocdoni.io/dvote/util"
	"go.vocdoni.io/dvote/vochain"
	models "go.vocdoni.io/proto/build/go/models"
)

// LOG_LEVEL=info go test -v -benchmem -run=- -bench=CheckTx -benchtime=20s
func BenchmarkCheckTx(b *testing.B) {
	b.ReportAllocs()
	b.ResetTimer()
	b.Run("indexTx", benchmarkIndexTx)
}

// LOG_LEVEL=info go test -v -benchmem -run=- -bench=NewProcess -benchtime=20s
func BenchmarkNewProcess(b *testing.B) {
	b.ReportAllocs()
	b.ResetTimer()
	b.Run("newProcess", benchmarkNewProcess)
}

func benchmarkIndexTx(b *testing.B) {
	app, err := vochain.NewBaseApplication(b.TempDir())
	if err != nil {
		b.Fatal(err)
	}

	sc, err := NewScrutinizer(b.TempDir(), app, true)
	if err != nil {
		b.Fatal(err)
	}
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
	err = sc.newEmptyProcess(pid)
	qt.Assert(b, err, qt.IsNil)

	s := new(ethereum.SignKeys)
	err = s.Generate()
	qt.Assert(b, err, qt.IsNil)

	for i := 0; i < b.N; i++ {
		sc.Rollback()
		for j := int32(0); j < 2000; j++ {
			vote := &models.Vote{
				Height:      uint32(util.RandomInt(10, 10000)),
				ProcessId:   pid,
				Nullifier:   util.RandomBytes(32),
				VotePackage: []byte("{[\"1\",\"2\",\"3\"]}"),
				Weight:      new(big.Int).SetUint64(uint64(util.RandomInt(1, 10000))).Bytes(),
			}
			sc.OnVote(vote, j)
		}
		err := sc.Commit(uint32(i))
		qt.Assert(b, err, qt.IsNil)
	}
}

func benchmarkNewProcess(b *testing.B) {
	app, err := vochain.NewBaseApplication(b.TempDir())
	if err != nil {
		b.Fatal(err)
	}

	sc, err := NewScrutinizer(b.TempDir(), app, true)
	if err != nil {
		b.Fatal(err)
	}
	startTime := time.Now()
	numProcesses := b.N
	entityID := util.RandomBytes(20)
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
		err = sc.newEmptyProcess(pid)
		qt.Assert(b, err, qt.IsNil)

	}
	log.Infof("indexed %d new processes, took %s",
		numProcesses, time.Since(startTime))
}

package scrutinizer

import (
	"math/big"
	"testing"

	qt "github.com/frankban/quicktest"
	"go.vocdoni.io/dvote/crypto/ethereum"
	"go.vocdoni.io/dvote/util"
	"go.vocdoni.io/dvote/vochain"
	models "go.vocdoni.io/proto/build/go/models"
)

// LOG_LEVEL=info go test -v -benchmem -run=- -bench=CheckTx -benchtime=20s
func BenchmarkCheckTx(b *testing.B) {
	b.ReportAllocs()
	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			benchmarkIndexTx(b)
		}
	})
}

func benchmarkIndexTx(b *testing.B) {
	app, err := vochain.NewBaseApplication(b.TempDir())
	if err != nil {
		b.Fatal(err)
	}

	sc, err := NewScrutinizer(b.TempDir(), app)
	if err != nil {
		b.Fatal(err)
	}
	pid := util.RandomBytes(32)
	if err := app.State.AddProcess(&models.Process{
		ProcessId:    pid,
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

	for i := uint32(0); i < 5; i++ {
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
		err := sc.Commit(i)
		qt.Assert(b, err, qt.IsNil)
	}
}

package test

import (
	"testing"
)

func BenchmarkVote(b *testing.B) {
	te := &testElection{}
	b.Run("NewTestElection", func(b *testing.B) {
		te = NewTestElection(b)
	})
	b.Run("GenerateVoters", func(b *testing.B) {
		te.GenerateVoters(b, b.N)
	})

	b.Run("CreateCensusAndElection", func(b *testing.B) {
		te.CreateCensusAndElection(b)
	})

	b.Run("AdvanceTestBlock2", func(b *testing.B) {
		// Block 2
		te.server.VochainAPP.AdvanceTestBlock()
		waitUntilHeight(b, te.c, 2)
	})

	b.ResetTimer()

	te.VoteAll(b)

	b.StopTimer()

	b.Run("AdvanceTestBlock3", func(b *testing.B) {
		// Block 3
		te.server.VochainAPP.AdvanceTestBlock()
		waitUntilHeight(b, te.c, 3)
	})

	te.VerifyVotes(b)
}

package test

import (
	"testing"
)

func BenchmarkVote(b *testing.B) {
	te := NewTestElection(b, b.N)
	te.CreateCensusAndElection(b)

	// Block 2
	te.server.VochainAPP.AdvanceTestBlock()
	waitUntilHeight(b, te.c, 2)

	b.ResetTimer()

	te.VoteAll(b)

	b.StopTimer()

	// Block 3
	te.server.VochainAPP.AdvanceTestBlock()
	waitUntilHeight(b, te.c, 3)

	te.VerifyVotes(b)
}

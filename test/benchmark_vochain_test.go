package test

import (
	"testing"
)

func BenchmarkCreateCensus(b *testing.B) {
	te := NewTestElection(b, b.TempDir())
	te.GenerateVoters(b, b.N)

	b.ResetTimer()
	te.CreateCensusAndElection(b)
	b.StopTimer()

	// Block 2
	te.server.VochainAPP.AdvanceTestBlock()
	waitUntilHeight(b, te.c, 2)

	te.VoteAll(b)

	// Block 3
	te.server.VochainAPP.AdvanceTestBlock()
	waitUntilHeight(b, te.c, 3)

	te.VerifyVotes(b)
}

func BenchmarkVote(b *testing.B) {
	te := NewTestElection(b, b.TempDir())
	te.GenerateVoters(b, b.N)

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

func BenchmarkVerifyVotes(b *testing.B) {
	te := NewTestElection(b, b.TempDir())
	te.GenerateVoters(b, b.N)

	te.CreateCensusAndElection(b)

	// Block 2
	te.server.VochainAPP.AdvanceTestBlock()
	waitUntilHeight(b, te.c, 2)

	te.VoteAll(b)

	// Block 3
	te.server.VochainAPP.AdvanceTestBlock()
	waitUntilHeight(b, te.c, 3)

	b.ResetTimer()
	te.VerifyVotes(b)
	b.StopTimer()
}

func BenchmarkVoteAndVerifyN(b *testing.B) {
	te := NewTestElection(b, b.TempDir())
	te.GenerateVoters(b, b.N)

	te.CreateCensusAndElection(b)

	// Block 2
	te.server.VochainAPP.AdvanceTestBlock()
	waitUntilHeight(b, te.c, 2)

	b.ResetTimer()
	te.VoteAll(b)

	// Block 3
	te.server.VochainAPP.AdvanceTestBlock()
	waitUntilHeight(b, te.c, 3)

	te.VerifyVotes(b)
	b.StopTimer()
}

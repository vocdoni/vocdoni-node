package test

import (
	"testing"
)

func BenchmarkCreateElectionAndVote10(b *testing.B) {
	for n := 0; n < b.N; n++ {
		CreateCensusAndElection(b, 10)
	}
}

func BenchmarkCreateElectionAndVote100(b *testing.B) {
	for n := 0; n < b.N; n++ {
		CreateCensusAndElection(b, 100)
	}
}

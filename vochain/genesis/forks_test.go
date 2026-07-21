package genesis

import (
	"testing"

	qt "github.com/frankban/quicktest"
)

func TestVoteMemoActive(t *testing.T) {
	c := qt.New(t)

	// Unknown chain: never active, at any height.
	c.Assert(VoteMemoActive("unknown/chain", 0), qt.IsFalse)
	c.Assert(VoteMemoActive("unknown/chain", 1_000_000), qt.IsFalse)

	// TEST chain: active from genesis (height 0 sentinel).
	c.Assert(VoteMemoActive("vocdoni/TEST/1", 0), qt.IsTrue)
	c.Assert(VoteMemoActive("vocdoni/TEST/1", 1_000), qt.IsTrue)

	// A chain scheduled at a future height: inactive below, active at/above.
	const chain = "vocdoni/UNITTEST/memo"
	voteMemoForkHeight[chain] = 100
	defer delete(voteMemoForkHeight, chain)
	c.Assert(VoteMemoActive(chain, 99), qt.IsFalse)
	c.Assert(VoteMemoActive(chain, 100), qt.IsTrue)
	c.Assert(VoteMemoActive(chain, 101), qt.IsTrue)

	// forkNever: inactive even at the maximum height.
	voteMemoForkHeight[chain] = forkNever
	c.Assert(VoteMemoActive(chain, forkNever), qt.IsFalse)
}

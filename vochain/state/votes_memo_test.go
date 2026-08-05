package state

import (
	"math/big"
	"testing"

	qt "github.com/frankban/quicktest"
	"go.vocdoni.io/dvote/db"
	"go.vocdoni.io/dvote/types"
	"go.vocdoni.io/proto/build/go/models"
	"google.golang.org/protobuf/proto"
)

func baseVote() *Vote {
	return &Vote{
		ProcessID:   types.HexBytes("processidprocessidprocessidproce"),
		Nullifier:   types.HexBytes("nullifiernullifiernullifiernull"),
		VotePackage: []byte(`{"votes":[1]}`),
		Weight:      big.NewInt(1),
	}
}

// newStateWithProcess returns a fresh state with a single process, for tests that
// need to store votes.
func newStateWithProcess(c *qt.C, pid []byte) *State {
	s, err := New(db.TypePebble, c.TempDir())
	c.Assert(err, qt.IsNil)
	c.Cleanup(func() { s.Close() })

	censusURI := "ipfs://foobar"
	c.Assert(s.AddProcess(&models.Process{
		EntityId:  []byte("entityidentityidentityidentityid"),
		CensusURI: &censusURI,
		ProcessId: pid,
	}), qt.IsNil)
	return s
}

func TestVoteHashMemo(t *testing.T) {
	c := qt.New(t)

	baseline := baseVote().Hash()

	// A non-empty memo must change the hash.
	v := baseVote()
	v.Memo = []byte("other: my open answer")
	c.Assert(v.Hash(), qt.Not(qt.DeepEquals), baseline)

	// The same memo hashes identically (determinism across validators).
	w := baseVote()
	w.Memo = []byte("other: my open answer")
	c.Assert(v.Hash(), qt.DeepEquals, w.Hash())

	// A different memo hashes differently.
	w.Memo = []byte("other: a different answer")
	c.Assert(v.Hash(), qt.Not(qt.DeepEquals), w.Hash())
}

func TestVoteDeepCopyMemo(t *testing.T) {
	c := qt.New(t)
	v := baseVote()
	v.Memo = []byte("keep me")

	cp := v.DeepCopy()
	c.Assert(string(cp.Memo), qt.Equals, "keep me")

	// Memo is a slice, so the copy must not alias the original.
	cp.Memo[0] = 'x'
	c.Assert(string(v.Memo), qt.Equals, "keep me")
}

// TestAddVoteMemoRoundTrip ensures the memo survives the StateDBVote
// marshal/store/unmarshal round-trip.
func TestAddVoteMemoRoundTrip(t *testing.T) {
	c := qt.New(t)
	pid := []byte("processidprocessidprocessidproce") // 32 bytes
	s := newStateWithProcess(c, pid)

	v := baseVote()
	v.ProcessID = pid
	v.Memo = []byte("other: my open answer")
	c.Assert(s.AddVote(v), qt.IsNil)

	got, err := s.Vote(pid, v.Nullifier, false)
	c.Assert(err, qt.IsNil)
	c.Assert(string(got.GetMemo()), qt.Equals, "other: my open answer")
}

// TestAddVoteMemoNilContract pins AddVote's contract for "no memo", which is
// consensus-relevant: memo is a proto3 optional, where a set empty value marshals
// as *present* and changes the StateDBVote bytes — and therefore the arbo leaf and
// the state hash — versus a vote cast without one.
//
// AddVote assigns the field verbatim, so callers must pass nil rather than an
// empty slice. transaction.checkVoteMemo does that normalization; the second half
// of this test asserts the difference is real, so that the requirement is a pinned
// fact rather than a silent trap for the next caller.
func TestAddVoteMemoNilContract(t *testing.T) {
	c := qt.New(t)
	pid := []byte("processidprocessidprocessidproce") // 32 bytes

	marshaledWith := func(memo []byte) []byte {
		s := newStateWithProcess(c, pid)
		v := baseVote()
		v.ProcessID = pid
		v.Memo = memo
		c.Assert(s.AddVote(v), qt.IsNil)

		got, err := s.Vote(pid, v.Nullifier, false)
		c.Assert(err, qt.IsNil)

		b, err := proto.Marshal(got)
		c.Assert(err, qt.IsNil)
		return b
	}

	// nil is stored as an absent field, byte-identical to a vote built without the
	// field at all. This is what keeps votes cast before the memo existed valid.
	noMemo := marshaledWith(nil)
	bare, err := proto.Marshal(&models.StateDBVote{
		VoteHash:    baseVote().Hash(),
		Nullifier:   baseVote().Nullifier,
		Weight:      baseVote().WeightBytes(),
		VotePackage: baseVote().VotePackage,
	})
	c.Assert(err, qt.IsNil)
	c.Assert(noMemo, qt.DeepEquals, bare)

	// An empty slice is NOT equivalent — it marshals as a present, zero-length
	// field. Hence the nil requirement above.
	c.Assert(marshaledWith([]byte{}), qt.Not(qt.DeepEquals), noMemo)
}

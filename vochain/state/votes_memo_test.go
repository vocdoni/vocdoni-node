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

// TestAddVoteEmptyMemoNotStored ensures a zero-length memo is stored as an absent
// field, so the marshaled StateDBVote — and therefore the arbo leaf and the state
// hash derived from it — is byte-identical to a vote cast with no memo at all.
// proto3 `optional` marshals a set empty value as present, so this is the
// property that keeps existing votes compatible; asserting got.Memo == nil alone
// would not catch a regression that stored an empty-but-present field.
func TestAddVoteEmptyMemoNotStored(t *testing.T) {
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
		c.Assert(got.Memo, qt.IsNil) // absent, not a present empty value

		b, err := proto.Marshal(got)
		c.Assert(err, qt.IsNil)
		return b
	}

	noMemo := marshaledWith(nil)
	emptyMemo := marshaledWith([]byte{})
	c.Assert(emptyMemo, qt.DeepEquals, noMemo)
}

// TestStateVotes checks the batch lookup against the single-vote path it replaces.
func TestStateVotes(t *testing.T) {
	c := qt.New(t)
	pid := []byte("processidprocessidprocessidproce") // 32 bytes
	s := newStateWithProcess(c, pid)

	nullifiers := [][]byte{
		[]byte("nullifier-one-nullifier-one-abcd"),
		[]byte("nullifier-two-nullifier-two-abcd"),
		[]byte("nullifier-three-nullifier-three-"),
	}
	for i, n := range nullifiers {
		v := baseVote()
		v.ProcessID = pid
		v.Nullifier = n
		v.Memo = []byte{byte('a' + i)}
		c.Assert(s.AddVote(v), qt.IsNil)
	}

	absent := []byte("nullifier-absent-nullifier-absen")
	got, err := s.Votes(pid, append(append([][]byte{}, nullifiers...), absent), false)
	c.Assert(err, qt.IsNil)

	// Positional: one entry per requested nullifier, nil where there is no vote.
	c.Assert(got, qt.HasLen, len(nullifiers)+1)
	c.Assert(got[len(nullifiers)], qt.IsNil)

	// Each entry must equal what the single-vote path returns.
	for i, n := range nullifiers {
		want, err := s.Vote(pid, n, false)
		c.Assert(err, qt.IsNil)
		c.Assert(got[i].GetMemo(), qt.DeepEquals, want.GetMemo())
		c.Assert(got[i].GetVoteHash(), qt.DeepEquals, want.GetVoteHash())
	}

	// An empty request is not an error and yields no entries.
	empty, err := s.Votes(pid, nil, false)
	c.Assert(err, qt.IsNil)
	c.Assert(empty, qt.HasLen, 0)

	// An unknown process yields no votes rather than an error, because DeepSubTree
	// creates the view lazily and the lookup simply misses. Assert that the batch
	// path agrees with the single-vote path, which reports ErrVoteNotFound here.
	unknownPID := []byte("unknownunknownunknownunknownunkn")
	_, err = s.Vote(unknownPID, nullifiers[0], false)
	c.Assert(err, qt.Equals, ErrVoteNotFound)

	gotUnknown, err := s.Votes(unknownPID, nullifiers, false)
	c.Assert(err, qt.IsNil)
	c.Assert(gotUnknown, qt.HasLen, len(nullifiers))
	for _, v := range gotUnknown {
		c.Assert(v, qt.IsNil)
	}
}

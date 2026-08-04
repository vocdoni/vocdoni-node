package state

import (
	"math/big"
	"testing"

	qt "github.com/frankban/quicktest"
	"go.vocdoni.io/dvote/db"
	"go.vocdoni.io/dvote/types"
	"go.vocdoni.io/proto/build/go/models"
)

func baseVote() *Vote {
	return &Vote{
		ProcessID:   types.HexBytes("processidprocessidprocessidproce"),
		Nullifier:   types.HexBytes("nullifiernullifiernullifiernull"),
		VotePackage: []byte(`{"votes":[1]}`),
		Weight:      big.NewInt(1),
	}
}

func TestVoteHashMemo(t *testing.T) {
	c := qt.New(t)

	baseline := baseVote().Hash()

	// No memo must not change the hash: votes cast before this field existed keep
	// hashing the same. This is the property that keeps the state tree compatible.
	v := baseVote()
	v.Memo = nil
	c.Assert(v.Hash(), qt.DeepEquals, baseline)

	// A zero-length memo must be indistinguishable from an absent one.
	v = baseVote()
	v.Memo = []byte{}
	c.Assert(v.Hash(), qt.DeepEquals, baseline)

	// A non-empty memo must change the hash.
	v = baseVote()
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
	s, err := New(db.TypePebble, t.TempDir())
	c.Assert(err, qt.IsNil)
	defer s.Close()

	pid := []byte("processidprocessidprocessidproce") // 32 bytes
	censusURI := "ipfs://foobar"
	c.Assert(s.AddProcess(&models.Process{
		EntityId:  []byte("entityidentityidentityidentityid"),
		CensusURI: &censusURI,
		ProcessId: pid,
	}), qt.IsNil)

	v := baseVote()
	v.ProcessID = pid
	v.Memo = []byte("other: my open answer")
	c.Assert(s.AddVote(v), qt.IsNil)

	got, err := s.Vote(pid, v.Nullifier, false)
	c.Assert(err, qt.IsNil)
	c.Assert(string(got.GetMemo()), qt.Equals, "other: my open answer")
}

// TestAddVoteEmptyMemoNotStored ensures an empty memo is stored as an absent
// (nil) field, so the marshaled StateDBVote — and thus the state hash — is
// identical to a vote cast without a memo at all.
func TestAddVoteEmptyMemoNotStored(t *testing.T) {
	c := qt.New(t)

	for name, memo := range map[string][]byte{"nil": nil, "empty": {}} {
		c.Run(name, func(c *qt.C) {
			s, err := New(db.TypePebble, t.TempDir())
			c.Assert(err, qt.IsNil)
			defer s.Close()

			pid := []byte("processidprocessidprocessidproce") // 32 bytes
			censusURI := "ipfs://foobar"
			c.Assert(s.AddProcess(&models.Process{
				EntityId:  []byte("entityidentityidentityidentityid"),
				CensusURI: &censusURI,
				ProcessId: pid,
			}), qt.IsNil)

			v := baseVote()
			v.ProcessID = pid
			v.Memo = memo
			c.Assert(s.AddVote(v), qt.IsNil)

			got, err := s.Vote(pid, v.Nullifier, false)
			c.Assert(err, qt.IsNil)
			c.Assert(got.Memo, qt.IsNil) // absent, not a present empty value
		})
	}
}

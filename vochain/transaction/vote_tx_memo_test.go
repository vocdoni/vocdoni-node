package transaction

import (
	"strings"
	"testing"

	qt "github.com/frankban/quicktest"
	"go.vocdoni.io/dvote/types"
	"go.vocdoni.io/proto/build/go/models"
)

func TestValidateVoteMemo(t *testing.T) {
	c := qt.New(t)
	invalidUTF8 := []byte{0xff, 0xfe}

	// Valid.
	m, err := validateVoteMemo([]byte("other: my answer"))
	c.Assert(err, qt.IsNil)
	c.Assert(string(m), qt.Equals, "other: my answer")

	// Absent and empty both yield nil, so the stored vote stays byte-identical to
	// one cast without the field. A zero-length slice would not: proto3 `optional`
	// marshals a set empty value as present.
	m, err = validateVoteMemo(nil)
	c.Assert(err, qt.IsNil)
	c.Assert(m, qt.IsNil)

	m, err = validateVoteMemo([]byte{})
	c.Assert(err, qt.IsNil)
	c.Assert(m, qt.IsNil)

	// Too long → rejected.
	_, err = validateVoteMemo([]byte(strings.Repeat("a", types.MaxVoteMemoSize+1)))
	c.Assert(err, qt.IsNotNil)

	// At the limit → accepted.
	m, err = validateVoteMemo([]byte(strings.Repeat("a", types.MaxVoteMemoSize)))
	c.Assert(err, qt.IsNil)
	c.Assert(len(m), qt.Equals, types.MaxVoteMemoSize)

	// The memo is opaque: bytes that are not valid UTF-8 pass through verbatim.
	// Only the length is the chain's business.
	m, err = validateVoteMemo(invalidUTF8)
	c.Assert(err, qt.IsNil)
	c.Assert(m, qt.DeepEquals, invalidUTF8)
}

func TestMemoAllowed(t *testing.T) {
	c := qt.New(t)

	// Signed votes bind the whole envelope through the tx signature.
	c.Assert(memoAllowed(&models.Process{
		CensusOrigin: models.CensusOrigin_OFF_CHAIN_TREE_WEIGHTED,
		EnvelopeType: &models.EnvelopeType{},
	}), qt.IsTrue)

	// Anonymous votes carry no signature and the proof does not cover the envelope.
	c.Assert(memoAllowed(&models.Process{
		CensusOrigin: models.CensusOrigin_OFF_CHAIN_TREE_WEIGHTED,
		EnvelopeType: &models.EnvelopeType{Anonymous: true},
	}), qt.IsFalse)

	// Farcaster signs the frame body, not the envelope.
	c.Assert(memoAllowed(&models.Process{
		CensusOrigin: models.CensusOrigin_FARCASTER_FRAME,
		EnvelopeType: &models.EnvelopeType{},
	}), qt.IsFalse)

	// A nil EnvelopeType must not panic and must not be treated as anonymous.
	c.Assert(memoAllowed(&models.Process{
		CensusOrigin: models.CensusOrigin_OFF_CHAIN_TREE_WEIGHTED,
	}), qt.IsTrue)
}

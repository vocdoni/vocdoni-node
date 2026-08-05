package transaction

import (
	"strings"
	"testing"

	qt "github.com/frankban/quicktest"
	"go.vocdoni.io/dvote/types"
	"go.vocdoni.io/proto/build/go/models"
)

// signedProcess is a process whose votes carry a transaction signature, which is
// the only kind that may carry a memo.
func signedProcess() *models.Process {
	return &models.Process{
		CensusOrigin: models.CensusOrigin_OFF_CHAIN_TREE_WEIGHTED,
		EnvelopeType: &models.EnvelopeType{},
	}
}

func TestCheckVoteMemo(t *testing.T) {
	c := qt.New(t)
	p := signedProcess()
	invalidUTF8 := []byte{0xff, 0xfe}

	// Valid.
	m, err := checkVoteMemo([]byte("other: my answer"), p)
	c.Assert(err, qt.IsNil)
	c.Assert(string(m), qt.Equals, "other: my answer")

	// Absent and empty both yield nil, never a zero-length slice: State.AddVote
	// assigns the value verbatim, and a set empty proto3 optional marshals as
	// present, which would change the stored bytes. See TestAddVoteMemoNilContract.
	m, err = checkVoteMemo(nil, p)
	c.Assert(err, qt.IsNil)
	c.Assert(m, qt.IsNil)

	m, err = checkVoteMemo([]byte{}, p)
	c.Assert(err, qt.IsNil)
	c.Assert(m, qt.IsNil)

	// Too long → rejected.
	_, err = checkVoteMemo([]byte(strings.Repeat("a", types.MaxVoteMemoSize+1)), p)
	c.Assert(err, qt.IsNotNil)

	// At the limit → accepted.
	m, err = checkVoteMemo([]byte(strings.Repeat("a", types.MaxVoteMemoSize)), p)
	c.Assert(err, qt.IsNil)
	c.Assert(len(m), qt.Equals, types.MaxVoteMemoSize)

	// The memo is opaque: bytes that are not valid UTF-8 pass through verbatim.
	// Only the length is the chain's business.
	m, err = checkVoteMemo(invalidUTF8, p)
	c.Assert(err, qt.IsNil)
	c.Assert(m, qt.DeepEquals, invalidUTF8)

	// A process that may not carry a memo rejects a non-empty one...
	anon := &models.Process{
		CensusOrigin: models.CensusOrigin_OFF_CHAIN_TREE_WEIGHTED,
		EnvelopeType: &models.EnvelopeType{Anonymous: true},
	}
	_, err = checkVoteMemo([]byte("attached by anyone"), anon)
	c.Assert(err, qt.IsNotNil)

	// ...but still accepts a vote that carries none, which is the common case.
	m, err = checkVoteMemo(nil, anon)
	c.Assert(err, qt.IsNil)
	c.Assert(m, qt.IsNil)
}

func TestMemoAllowed(t *testing.T) {
	c := qt.New(t)

	// Signed votes bind the whole envelope through the tx signature.
	c.Assert(memoAllowed(signedProcess()), qt.IsTrue)

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

	// Encrypted elections withhold the ballot until the keys are revealed; an
	// unencrypted memo served beside it would defeat that.
	c.Assert(memoAllowed(&models.Process{
		CensusOrigin: models.CensusOrigin_OFF_CHAIN_TREE_WEIGHTED,
		EnvelopeType: &models.EnvelopeType{EncryptedVotes: true},
	}), qt.IsFalse)

	// A nil EnvelopeType must not panic and must not be treated as anonymous.
	c.Assert(memoAllowed(&models.Process{
		CensusOrigin: models.CensusOrigin_OFF_CHAIN_TREE_WEIGHTED,
	}), qt.IsTrue)
}

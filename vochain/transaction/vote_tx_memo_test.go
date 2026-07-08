package transaction

import (
	"strings"
	"testing"

	qt "github.com/frankban/quicktest"
)

func TestResolveVoteMemo(t *testing.T) {
	c := qt.New(t)
	invalidUTF8 := []byte{0xff, 0xfe}

	// Inactive: any memo is ignored (no error, empty result) — keeps pre-fork state.
	m, err := resolveVoteMemo([]byte("hello"), false)
	c.Assert(err, qt.IsNil)
	c.Assert(m, qt.Equals, "")

	// Inactive with otherwise-invalid bytes: still ignored, no error (validation is gated).
	m, err = resolveVoteMemo(invalidUTF8, false)
	c.Assert(err, qt.IsNil)
	c.Assert(m, qt.Equals, "")

	// Active, valid.
	m, err = resolveVoteMemo([]byte("other: my answer"), true)
	c.Assert(err, qt.IsNil)
	c.Assert(m, qt.Equals, "other: my answer")

	// Active, empty.
	m, err = resolveVoteMemo(nil, true)
	c.Assert(err, qt.IsNil)
	c.Assert(m, qt.Equals, "")

	// Active, too long → rejected.
	_, err = resolveVoteMemo([]byte(strings.Repeat("a", MaxVoteMemoSize+1)), true)
	c.Assert(err, qt.IsNotNil)

	// Active, at the limit → accepted.
	m, err = resolveVoteMemo([]byte(strings.Repeat("a", MaxVoteMemoSize)), true)
	c.Assert(err, qt.IsNil)
	c.Assert(len(m), qt.Equals, MaxVoteMemoSize)

	// Active, invalid UTF-8 → rejected.
	_, err = resolveVoteMemo(invalidUTF8, true)
	c.Assert(err, qt.IsNotNil)
}

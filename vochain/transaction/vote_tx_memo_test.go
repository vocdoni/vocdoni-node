package transaction

import (
	"strings"
	"testing"

	qt "github.com/frankban/quicktest"
	"go.vocdoni.io/dvote/types"
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

	// Invalid UTF-8 → rejected.
	_, err = validateVoteMemo(invalidUTF8)
	c.Assert(err, qt.IsNotNil)
}

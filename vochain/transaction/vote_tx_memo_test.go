package transaction

import (
	"strings"
	"testing"

	qt "github.com/frankban/quicktest"
	"go.vocdoni.io/dvote/types"
)

func TestCheckVoteMemo(t *testing.T) {
	c := qt.New(t)
	invalidUTF8 := []byte{0xff, 0xfe}

	// Valid.
	m, err := checkVoteMemo([]byte("other: my answer"))
	c.Assert(err, qt.IsNil)
	c.Assert(string(m), qt.Equals, "other: my answer")

	// An absent memo decodes as nil and flows through untouched, so the common
	// memo-less vote stores an absent field. See TestAddVoteMemoNilContract.
	m, err = checkVoteMemo(nil)
	c.Assert(err, qt.IsNil)
	c.Assert(m, qt.IsNil)

	// Too long → rejected.
	_, err = checkVoteMemo([]byte(strings.Repeat("a", types.MaxVoteMemoSize+1)))
	c.Assert(err, qt.IsNotNil)

	// At the limit → accepted.
	m, err = checkVoteMemo([]byte(strings.Repeat("a", types.MaxVoteMemoSize)))
	c.Assert(err, qt.IsNil)
	c.Assert(len(m), qt.Equals, types.MaxVoteMemoSize)

	// The memo is opaque: bytes that are not valid UTF-8 pass through verbatim.
	// Only the length is the chain's business.
	m, err = checkVoteMemo(invalidUTF8)
	c.Assert(err, qt.IsNil)
	c.Assert(m, qt.DeepEquals, invalidUTF8)
}

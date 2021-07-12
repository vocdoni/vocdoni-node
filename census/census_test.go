package census

import (
	"strings"
	"testing"

	qt "github.com/frankban/quicktest"
)

func TestCompressor(t *testing.T) {
	t.Parallel()

	comp := newCompressor()
	input := []byte(strings.Repeat("foo bar baz", 10))

	// First, check that "decompressing" non-compressed bytes is a no-op,
	// for backwards compatibility with gateways, and to have a sane
	// fallback.
	qt.Assert(t, comp.decompressBytes(input), qt.DeepEquals, input)

	// Compressing should give a smaller size, at least by 50%.
	compressed := comp.compressBytes(input)
	qt.Assert(t, len(compressed) < len(input)/2, qt.IsTrue, qt.Commentf("expected size of 50%% at most, got %d out of %d", len(compressed), len(input)))

	// Decompressing should give us the original input back.
	qt.Assert(t, comp.decompressBytes(compressed), qt.DeepEquals, input)
}

func TestLoadTreeDefaultCensusType(t *testing.T) {
	m := &Manager{}
	err := m.Init(t.TempDir(), "")
	qt.Assert(t, err, qt.IsNil)

	// ensure that when TreeType is set to 0, uses the 'default' tree type
	tree, err := m.LoadTree("testtree", 0)
	qt.Assert(t, err, qt.IsNil)
	qt.Assert(t, censusDefaultType, qt.Equals, tree.Type())
}

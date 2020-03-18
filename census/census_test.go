package census

import (
	"bytes"
	"os"
	"strings"
	"testing"

	"gitlab.com/vocdoni/go-dvote/log"
)

func TestMain(m *testing.M) {
	log.InitLogger("error", "stdout")
	os.Exit(m.Run())
}

func TestCompressor(t *testing.T) {
	t.Parallel()

	comp := newCompressor()
	input := []byte(strings.Repeat("foo bar baz", 10))

	// First, check that "decompressing" non-compressed bytes is a no-op,
	// for backwards compatibility with gateways, and to have a sane
	// fallback.
	{
		got := comp.decompressBytes(input)
		if want := input; !bytes.Equal(got, want) {
			t.Fatalf("want %q, got %q", want, got)
		}
	}

	// Compressing should give a smaller size, at least by 50%.
	compressed := comp.compressBytes(input)
	if len(compressed) >= len(input)/2 {
		t.Fatalf("expected size of 50%% at most, got %d out of %d", len(compressed), len(input))
	}

	// Decompressing should give us the original input back.
	{
		got := comp.decompressBytes(compressed)
		if want := input; !bytes.Equal(got, want) {
			t.Fatalf("want %q, got %q", want, got)
		}
	}
}

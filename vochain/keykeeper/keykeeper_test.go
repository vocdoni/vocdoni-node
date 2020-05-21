package keykeeper

import (
	"crypto/rand"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"

	"gitlab.com/vocdoni/go-dvote/crypto/nacl"
)

func TestEncodeDecode(t *testing.T) {
	k1, err := nacl.Generate(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	k2, err := nacl.Generate(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}

	pk := processKeys{
		pubKey:        k1.Public(),
		privKey:       k1.Private(),
		revealKey:     k2.Public(),
		commitmentKey: k2.Private(),
		index:         5,
	}
	data := pk.Encode()
	t.Logf("encoded data: %x", data)
	var pk2 processKeys
	if err := pk2.Decode(data); err != nil {
		t.Fatal(err)
	}

	if diff := cmp.Diff(pk, pk2, cmpopts.IgnoreUnexported(processKeys{})); diff != "" {
		t.Fatalf("processKeys mismatch: %s", diff)
	}
}

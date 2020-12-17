package keykeeper

import (
	"crypto/rand"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"

	"go.vocdoni.io/dvote/crypto/nacl"
)

func TestEncodeDecode(t *testing.T) {
	priv1, err := nacl.Generate(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	priv2, err := nacl.Generate(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}

	pk := processKeys{
		pubKey:        priv1.Public().Bytes(),
		privKey:       priv1.Bytes(),
		revealKey:     priv2.Public().Bytes(),
		commitmentKey: priv2.Bytes(),
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

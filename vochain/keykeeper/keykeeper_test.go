package keykeeper

import (
	"crypto/rand"
	"testing"

	qt "github.com/frankban/quicktest"
	"github.com/google/go-cmp/cmp/cmpopts"

	"go.vocdoni.io/dvote/crypto/nacl"
)

func TestEncodeDecode(t *testing.T) {
	priv1, err := nacl.Generate(rand.Reader)
	qt.Assert(t, err, qt.IsNil)
	priv2, err := nacl.Generate(rand.Reader)
	qt.Assert(t, err, qt.IsNil)

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
	err = pk2.Decode(data)
	qt.Assert(t, err, qt.IsNil)

	qt.Assert(t, pk, qt.CmpEquals(cmpopts.IgnoreUnexported(processKeys{})), pk2)
}

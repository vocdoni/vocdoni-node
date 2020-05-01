package keykeeper

import (
	"crypto/rand"
	"testing"

	"gitlab.com/vocdoni/go-dvote/crypto/nacl"
)

func TestEncoding(t *testing.T) {
	k1, err := nacl.Generate(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	k2, err := nacl.Generate(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}

	pk := processKeys{
		pubKey:        k1.Public,
		privKey:       k1.Private,
		revealKey:     k2.Public,
		commitmentKey: k2.Private,
		index:         int8(5),
	}
	data := pk.Encode()
	t.Logf("encoded data: %x", data)
	var pk2 processKeys
	pk2.Decode(data)

	if string(pk.pubKey[:]) != string(pk2.pubKey[:]) {
		t.Errorf("data do not match: pubKey")
	}
	if string(pk.privKey[:]) != string(pk2.privKey[:]) {
		t.Errorf("data do not match: privKey")
	}
	if string(pk.revealKey[:]) != string(pk2.revealKey[:]) {
		t.Errorf("data do not match: revealKey")
	}
	if string(pk.commitmentKey[:]) != string(pk2.commitmentKey[:]) {
		t.Errorf("data do not match: commitmentKey")
	}
	if string(pk.index) != string(pk2.index) {
		t.Errorf("data do not match: index")
	}
}

package snarks

import (
	"encoding/base64"
	"encoding/hex"
	"testing"

	"gitlab.com/vocdoni/go-dvote/util"
)

func b64enc(b []byte) string {
	return base64.StdEncoding.EncodeToString(b)
}

func hexdec(t *testing.T, s string) []byte {
	t.Helper()
	s = util.TrimHex(s)
	b, err := hex.DecodeString(s)
	if err != nil {
		t.Fatal(err)
	}
	return b
}

func TestPoseidon(t *testing.T) {
	t.Parallel()

	// mnemonic := "fly cheap color olive setup rigid april forum over grief predict pipe toddler argue give"
	pubKey := "0x045a126cbbd3c66b6d542d40d91085e3f2b5db3bbc8cda0d59615deb08784e4f833e0bb082194790143c3d01cedb4a9663cb8c7bdaaad839cb794dd309213fcf30"
	expectedHash := "nGOYvS4aqqUVAT9YjWcUzA89DlHPWaooNpBTStOaHRA="

	hash := Poseidon.Hash(hexdec(t, pubKey))
	base64Hash := b64enc(hash)
	if base64Hash != expectedHash {
		t.Fatalf("%q should be %q", base64Hash, expectedHash)
	}

	// 2
	// mnemonic = "kangaroo improve enroll almost since stock travel grace improve welcome orbit decorate govern hospital select"
	pubKey = "0x049969c7741ade2e9f89f81d12080651038838e8089682158f3d892e57609b64e2137463c816e4d52f6688d490c35a0b8e524ac6d9722eed2616dbcaf676fc2578"
	expectedHash = "j7jJlnBN73ORKWbNbVCHG9WkoqSr+IEKDwjcsb6N4xw="
	hash = Poseidon.Hash(hexdec(t, pubKey))
	base64Hash = b64enc(hash)
	if base64Hash != expectedHash {
		t.Fatalf("%q should be %q", base64Hash, expectedHash)
	}

	// test hash with less than 32 bytes
	pubKey = "0x0409d240a33ca9c486c090135f06c5d801aceec6eaed94b8bef1c9763b6c39708819207786fe92b22c6661957e83923e24a5ba754755b181f82fdaed2ed3914453"
	expectedHash = "3/AaoqHPrz20tfLmhLz4ay5nrlKN5WiuvlDZkfZyfgA="

	hash = Poseidon.Hash(hexdec(t, pubKey))
	base64Hash = b64enc(hash)
	if base64Hash != expectedHash {
		t.Fatalf("%q should be %q", base64Hash, expectedHash)
	}
}

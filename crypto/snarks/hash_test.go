package snarks

import (
	"bytes"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"testing"

	"gitlab.com/vocdoni/go-dvote/crypto/internal/cryptotest"
	"gitlab.com/vocdoni/go-dvote/util"
)

func b64dec(t *testing.T, s string) []byte {
	t.Helper()
	b, err := base64.StdEncoding.DecodeString(s)
	if err != nil {
		t.Fatal(err)
	}
	return b
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

func TestHash(t *testing.T) { cryptotest.TestHash(t, Poseidon) }

func TestPoseidon(t *testing.T) {
	t.Parallel()

	tests := []struct {
		message  []byte
		wantHash []byte
	}{
		{
			// mnemonic: "fly cheap color olive setup rigid april forum over grief predict pipe toddler argue give"
			hexdec(t, "045a126cbbd3c66b6d542d40d91085e3f2b5db3bbc8cda0d59615deb08784e4f833e0bb082194790143c3d01cedb4a9663cb8c7bdaaad839cb794dd309213fcf30"),
			b64dec(t, "nGOYvS4aqqUVAT9YjWcUzA89DlHPWaooNpBTStOaHRA="),
		},
		{
			// mnemonic: "kangaroo improve enroll almost since stock travel grace improve welcome orbit decorate govern hospital select"
			hexdec(t, "049969c7741ade2e9f89f81d12080651038838e8089682158f3d892e57609b64e2137463c816e4d52f6688d490c35a0b8e524ac6d9722eed2616dbcaf676fc2578"),
			b64dec(t, "j7jJlnBN73ORKWbNbVCHG9WkoqSr+IEKDwjcsb6N4xw="),
		},
		{
			// test hash with less than 32 bytes
			hexdec(t, "0409d240a33ca9c486c090135f06c5d801aceec6eaed94b8bef1c9763b6c39708819207786fe92b22c6661957e83923e24a5ba754755b181f82fdaed2ed3914453"),
			b64dec(t, "3/AaoqHPrz20tfLmhLz4ay5nrlKN5WiuvlDZkfZyfgA="),
		},
	}
	for i, test := range tests {
		test := test
		t.Run(fmt.Sprintf("%02d", i), func(t *testing.T) {
			t.Parallel()

			got := Poseidon.Hash(test.message)
			if !bytes.Equal(got, test.wantHash) {
				t.Fatalf("got %x, want %x", got, test.wantHash)
			}
		})
	}
}

package cryptotest

import (
	"bytes"
	"testing"

	"gitlab.com/vocdoni/go-dvote/crypto"
)

var tests = []struct {
	name     string
	message  []byte
	jsCipher []byte
}{
	{
		name:    "Hello",
		message: []byte("hello world"),
	},
	{
		name:    "Empty",
		message: []byte(""),
	},
	{
		name:    "Accents",
		message: []byte("UTF-8-charsàèìòù"),
	},
	{
		name:    "NonText",
		message: []byte{0x01, 0x02, 0x03, 0x04},
	},
}

func testKeyPair(t *testing.T, keypair crypto.KeyPair) {
	t.Helper()

	pub := keypair.Public()
	priv := keypair.Private()

	if len(pub) == 0 || len(priv) == 0 || bytes.Equal(pub, priv) {
		t.Fatalf("invalid keypair: pub=%x priv=%x", pub, priv)
	}
}

func TestGenerateEncryptDecrypt(t *testing.T, gen func() (crypto.Cipher, error)) {
	t.Parallel()

	cipher1, err := gen()
	if err != nil {
		t.Fatal(err)
	}
	cipher2, err := gen()
	if err != nil {
		t.Fatal(err)
	}
	testKeyPair(t, cipher1)
	testKeyPair(t, cipher2)
	if bytes.Equal(cipher1.Private(), cipher2.Private()) {
		t.Fatal("cipher1 and cipher2 must be different")
	}

	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			enc, err := cipher1.Encrypt(test.message)
			if err != nil {
				t.Fatalf("Encrypt error: %v", err)
			}

			if _, err := cipher2.Decrypt(enc); err == nil {
				t.Fatalf("Decrypt with different keys should error")
			}

			dec, err := cipher1.Decrypt(enc)
			if err != nil {
				t.Fatalf("Decrypt error: %v", err)
			}

			if !bytes.Equal(dec, test.message) {
				t.Fatalf("encrypt-decrypt got %q, want %q", dec, test.message)
			}
		})
	}
}

func TestHash(t *testing.T, hash crypto.Hash) {
	t.Parallel()

	length := len(hash.Hash([]byte{0}))

	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			sum := hash.Hash(test.message)
			if len(sum) != length {
				t.Fatalf("hash length must be consistent; got %d and %d", length, len(sum))
			}

			if sum2 := hash.Hash(test.message); !bytes.Equal(sum, sum2) {
				t.Fatalf("Hash must always return the same; got %x and %x", sum, sum2)
			}
		})
	}
}

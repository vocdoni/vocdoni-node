package cryptotest

import (
	"bytes"
	"testing"

	"go.vocdoni.io/dvote/crypto"
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

func testPrivateKey(t *testing.T, key crypto.PrivateKey) {
	t.Helper()

	priv := key.Bytes()
	pub := key.Public().Bytes()

	if len(pub) == 0 || len(priv) == 0 || bytes.Equal(pub, priv) {
		t.Fatalf("invalid keypair: pub=%x priv=%x", pub, priv)
	}

	if pub2 := key.Public().Bytes(); !bytes.Equal(pub, pub2) {
		t.Fatalf("two Public calls got different keys: %x %x", pub, pub2)
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
	testPrivateKey(t, cipher1)
	testPrivateKey(t, cipher2)
	if bytes.Equal(cipher1.Bytes(), cipher2.Bytes()) {
		t.Fatal("cipher1 and cipher2 must be different")
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			enc1, err := cipher1.Encrypt(test.message, nil)
			if err != nil {
				t.Fatalf("Encrypt error: %v", err)
			}
			enc2, err := cipher1.Encrypt(test.message, cipher2.Public())
			if err != nil {
				t.Fatalf("Encrypt error: %v", err)
			}

			if _, err := cipher2.Decrypt(enc1); err == nil {
				t.Fatalf("Decrypt with different keys should error")
			}
			if _, err := cipher1.Decrypt(enc2); err == nil {
				t.Fatalf("Decrypt with different keys should error")
			}

			dec1, err := cipher1.Decrypt(enc1)
			if err != nil {
				t.Fatalf("Decrypt error: %v", err)
			}

			if !bytes.Equal(dec1, test.message) {
				t.Fatalf("encrypt-decrypt got %q, want %q", dec1, test.message)
			}

			dec2, err := cipher2.Decrypt(enc2)
			if err != nil {
				t.Fatalf("Decrypt error: %v", err)
			}

			if !bytes.Equal(dec2, test.message) {
				t.Fatalf("encrypt-decrypt got %q, want %q", dec2, test.message)
			}
		})
	}
}

func TestHash(t *testing.T, hash crypto.Hash) {
	t.Parallel()

	length := len(hash.Hash([]byte{0}))

	for _, test := range tests {
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

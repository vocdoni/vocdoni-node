// Package nacl implements encryption and decryption using anonymous sealed
// boxes, depending on golang.org/x/crypto/nacl/box.
package nacl

import (
	cryptorand "crypto/rand"
	"encoding/hex"
	"fmt"
	"io"

	"golang.org/x/crypto/curve25519"
	"golang.org/x/crypto/nacl/box"

	"gitlab.com/vocdoni/go-dvote/crypto"
)

const KeyLength = 32

// key represents a single nacl key, public or private. It contains common
// methods to represent the key as a byte slice or as a byte array, as well as
// decodig from hex.
type key [KeyLength]byte

func (k *key) Bytes() []byte { return k[:] }

func (k *key) array() *[KeyLength]byte { return (*[KeyLength]byte)(k) }

func (k *key) decode(hexkey string) error {
	b, err := hex.DecodeString(hexkey)
	if err != nil {
		return err
	}
	if len(b) != KeyLength {
		return fmt.Errorf("key length must be %d, not %d", KeyLength, len(b))
	}
	copy(k.Bytes(), b)
	return nil
}

// publicKey implements crypto.PublicKey.
type publicKey struct {
	key
}

// privateKey implements crypto.PrivateKey.
type privateKey struct {
	key

	pub publicKey
}

func (priv *privateKey) Public() crypto.PublicKey { return &priv.pub }

func (priv *privateKey) derivePublic() error {
	pub, err := curve25519.X25519(priv.Bytes(), curve25519.Basepoint)
	if err != nil {
		return err
	}
	copy(priv.pub.Bytes(), pub)
	return nil
}

// DecodePrivate decodes a private key from a hexadecimal string.
//
// Since this package supports encryption and decryption, we return a
// crypto.Cipher instead of just a crypto.PrivateKey.
func DecodePrivate(hexkey string) (crypto.Cipher, error) {
	var privkey privateKey
	if err := privkey.decode(hexkey); err != nil {
		return nil, err
	}
	if err := privkey.derivePublic(); err != nil {
		return nil, err
	}
	return &privkey, nil
}

// DecodePublic decodes a public key from a hexadecimal string.
func DecodePublic(hexkey string) (crypto.PublicKey, error) {
	var pubkey publicKey
	if err := pubkey.decode(hexkey); err != nil {
		return nil, err
	}
	return &pubkey, nil
}

// Generate creates a new random private key. If randReader is nil,
// crypto/rand.Reader is used.
//
// Since this package supports encryption and decryption, we return a
// crypto.Cipher instead of just a crypto.PrivateKey.
func Generate(randReader io.Reader) (crypto.Cipher, error) {
	if randReader == nil {
		randReader = cryptorand.Reader
	}
	var privkey privateKey
	if _, err := io.ReadFull(randReader, privkey.Bytes()); err != nil {
		return nil, err
	}

	if err := privkey.derivePublic(); err != nil {
		return nil, err
	}
	return &privkey, nil
}

// Encrypt is a standalone version of KeyPair.Encrypt, since the recipient's
// private key isn't needed to encrypt.
func Encrypt(message []byte, public *[KeyLength]byte) ([]byte, error) {
	return box.SealAnonymous(nil, message, public, cryptorand.Reader)
}

func (priv *privateKey) Encrypt(message []byte) ([]byte, error) {
	return Encrypt(message, priv.pub.array())
}

func (priv *privateKey) Decrypt(cipher []byte) ([]byte, error) {
	message, ok := box.OpenAnonymous(nil, cipher, priv.pub.array(), priv.array())
	if !ok {
		return nil, fmt.Errorf("could not open box")
	}
	return message, nil
}

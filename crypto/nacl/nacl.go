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

	"go.vocdoni.io/dvote/crypto"
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

// Anonymous is a convenience to encrypt anonymous sealed boxes for an explicit
// recipient public key, without having a private key at all.
var Anonymous crypto.Cipher = (*privateKey)(nil)

// Encrypt is a standalone version of KeyPair.Encrypt, since the recipient's
// private key isn't needed to encrypt.
func (priv *privateKey) Encrypt(message []byte, recipient crypto.PublicKey) ([]byte, error) {
	var pub *publicKey
	if recipient == nil {
		pub = &priv.pub
	} else {
		pub, _ = recipient.(*publicKey)
		if pub == nil {
			return nil, fmt.Errorf("invalid recipient key: %#v", recipient)
		}
	}
	return box.SealAnonymous(nil, message, pub.array(), cryptorand.Reader)
}

func (priv *privateKey) Decrypt(cipher []byte) ([]byte, error) {
	message, ok := box.OpenAnonymous(nil, cipher, priv.pub.array(), priv.array())
	if !ok {
		return nil, fmt.Errorf("could not open box")
	}
	return message, nil
}

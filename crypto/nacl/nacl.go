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
)

const keyLength = 32

type KeyPair struct {
	Public, Private [keyLength]byte
}

func Generate(randReader io.Reader) (*KeyPair, error) {
	if randReader == nil {
		randReader = cryptorand.Reader
	}
	pub, priv, err := box.GenerateKey(randReader)
	if err != nil {
		return nil, err
	}
	return &KeyPair{Public: *pub, Private: *priv}, nil
}

func FromHex(privHex string) (*KeyPair, error) {
	priv, err := hex.DecodeString(privHex)
	if err != nil {
		return nil, err
	}
	if len(priv) != keyLength {
		return nil, fmt.Errorf("private key length must be %d, not %d", keyLength, len(priv))
	}

	kp := &KeyPair{}
	copy(kp.Private[:], priv)
	pub, err := curve25519.X25519(priv, curve25519.Basepoint)
	if err != nil {
		return kp, err
	}
	copy(kp.Public[:], pub)

	return kp, nil
}

func (k *KeyPair) Hex() (string, string) {
	return hex.EncodeToString(k.Public[:]), hex.EncodeToString(k.Private[:])
}

func (k *KeyPair) Encrypt(message []byte) ([]byte, error) {
	return box.SealAnonymous(nil, message, &k.Public, cryptorand.Reader)
}

func (k *KeyPair) Decrypt(cipher []byte) ([]byte, error) {
	message, ok := box.OpenAnonymous(nil, cipher, &k.Public, &k.Private)
	if !ok {
		return nil, fmt.Errorf("could not open box")
	}
	return message, nil
}

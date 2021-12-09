package saltedkey

import (
	"crypto/ecdsa"
	"fmt"

	blind "github.com/arnaucube/go-blindsecp256k1"
	ethcrypto "github.com/ethereum/go-ethereum/crypto"
)

// SaltSize is the size (in bytes) of the salt word
const SaltSize = 20

// SaltBlindPubKey returns the salted blind public key of pubKey applying the salt.
func SaltBlindPubKey(pubKey *blind.PublicKey, salt []byte) (*blind.PublicKey, error) {
	if len(salt) < SaltSize {
		return nil, fmt.Errorf("provided salt is not large enough (need %d bytes)", SaltSize)
	}
	if pubKey == nil {
		return nil, fmt.Errorf("public key is nil")
	}
	var salt2 [SaltSize]byte
	copy(salt2[:], salt[:SaltSize])
	x, y := ethcrypto.S256().ScalarBaseMult(salt2[:])
	s := blind.Point{
		X: x,
		Y: y,
	}
	return (*blind.PublicKey)(pubKey.Point().Add(&s)), nil
}

// SaltECDSAPubKey returns the salted plain public key of pubKey applying the salt.
func SaltECDSAPubKey(pubKey *ecdsa.PublicKey, salt []byte) (*ecdsa.PublicKey, error) {
	if len(salt) < SaltSize {
		return nil, fmt.Errorf("provided salt is not large enough (need %d bytes)", SaltSize)
	}
	if pubKey == nil {
		return nil, fmt.Errorf("public key is nil")
	}
	var salt2 [SaltSize]byte
	copy(salt2[:], salt[:SaltSize])
	x, y := pubKey.Curve.ScalarBaseMult(salt2[:])
	pubKey.X, pubKey.Y = pubKey.Curve.Add(pubKey.X, pubKey.Y, x, y)
	return pubKey, nil
}

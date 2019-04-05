package signature

import (
	"crypto/ecdsa"
	hex "encoding/hex"
	"errors"

	crypto "github.com/ethereum/go-ethereum/crypto"
)

type SignKeys struct {
	Public  *ecdsa.PublicKey
	Private *ecdsa.PrivateKey
}

// generate new keys
func (k *SignKeys) Generate() error {
	var err error
	key, err := crypto.GenerateKey()
	if err != nil {
		return err
	}
	k.Public = &key.PublicKey
	k.Private = key
	return nil
}

// imports a private hex key
func (k *SignKeys) AddHexKey(privHex string) error {
	var err error
	k.Private, err = crypto.HexToECDSA(privHex)
	if err == nil {
		k.Public = &k.Private.PublicKey
	}
	return err
}

// returns the public and private keys as hex strings
func (k *SignKeys) HexString() (string, string) {
	pubHex := hex.EncodeToString(crypto.CompressPubkey(k.Public))
	privHex := hex.EncodeToString(crypto.FromECDSA(k.Private))
	return pubHex, privHex
}

// message is a normal string (no HexString)
func (k *SignKeys) Sign(message string) (string, error) {
	if k.Private == nil {
		return "", errors.New("No private key available")
	}
	hash := crypto.Keccak256([]byte(message))
	signature, err := crypto.Sign(hash, k.Private)
	signHex := hex.EncodeToString(signature)
	return signHex, err
}

// message is a normal string, signature and pubHex are HexStrings
func (k *SignKeys) Verify(message, signHex, pubHex string) (bool, error) {
	signature, err := hex.DecodeString(signHex)
	if err != nil {
		return false, err
	}
	pub, err := hex.DecodeString(pubHex)
	if err != nil {
		return false, err
	}
	hash := crypto.Keccak256([]byte(message))
	result := crypto.VerifySignature(pub, hash, signature[:64])
	return result, nil
}

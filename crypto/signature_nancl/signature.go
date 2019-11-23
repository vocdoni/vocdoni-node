package signature

import (
	rand "crypto/rand"
	hex "encoding/hex"
	"errors"
	"fmt"

	sign "golang.org/x/crypto/nacl/sign"
)

const signatureSize = 128

type SignKeys struct {
	Public  *[32]byte
	Private *[64]byte
}

func (k *SignKeys) Generate() error {
	var err error
	k.Public = new([32]byte)
	k.Private = new([64]byte)
	k.Public, k.Private, err = sign.GenerateKey(rand.Reader)
	return err
}

func (k *SignKeys) AddHexKeys(pubHex string, privHex string) error {
	if len(pubHex) < 32 || len(privHex) < 64 {
		return errors.New("Wrong key size, must be pub:32 priv:64 (bytes)")
	}
	pubKey, err := hex.DecodeString(pubHex)
	if err != nil {
		return err
	}
	privKey, err := hex.DecodeString(privHex)
	if err != nil {
		return err
	}
	k.Public = new([32]byte)
	k.Private = new([64]byte)
	copy(k.Public[:], pubKey[:32])
	copy(k.Private[:], privKey[:64])
	return nil
}

func (k *SignKeys) HexString() (string, string) {
	pubHex := hex.EncodeToString(k.Public[:])
	privHex := hex.EncodeToString(k.Private[:])
	return pubHex, privHex
}

// message is a normal string (no HexString)
func (k *SignKeys) Sign(message string) (string, error) {
	if k.Private == nil {
		return "", errors.New("No private key available")
	}
	signature := sign.Sign(nil, []byte(message), k.Private)
	signHexFull := hex.EncodeToString(signature)
	return signHexFull[:signatureSize], nil
}

// message is a normal string, signature and pubHex are HexStrings
func (k *SignKeys) Verify(message, signature, pubHex string) (bool, error) {
	msgHex := hex.EncodeToString([]byte(message))
	signatureAndText := fmt.Sprintf("%s%s", signature, msgHex)
	signatureToVerify, err := hex.DecodeString(signatureAndText)
	if err != nil {
		return false, err
	}
	pubKeyToVerify, err := hex.DecodeString(pubHex)
	if err != nil {
		return false, err
	}
	pubKey := new([32]byte)
	copy(pubKey[:], pubKeyToVerify[:32])
	_, result := sign.Open(nil, signatureToVerify, pubKey)
	return result, nil
}

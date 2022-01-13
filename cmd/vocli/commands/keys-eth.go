package commands

import (
	"crypto/ecdsa"
	"fmt"
	"io"

	ethkeystore "github.com/ethereum/go-ethereum/accounts/keystore"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/google/uuid"
)

func newKey(rand io.Reader) (*ethkeystore.Key, error) {
	privateKeyECDSA, err := ecdsa.GenerateKey(crypto.S256(), rand)
	if err != nil {
		return nil, err
	}
	return newKeyFromECDSA(privateKeyECDSA), nil
}

func newKeyFromECDSA(privateKeyECDSA *ecdsa.PrivateKey) *ethkeystore.Key {
	id, err := uuid.NewRandom()
	if err != nil {
		panic(fmt.Sprintf("Could not create random uuid: %v", err))
	}
	key := &ethkeystore.Key{
		Id:         id,
		Address:    crypto.PubkeyToAddress(privateKeyECDSA.PublicKey),
		PrivateKey: privateKeyECDSA,
	}
	return key
}

func storeNewKey(rand io.Reader, auth string) (*ethkeystore.Key, string, error) {
	key, err := newKey(rand)
	if err != nil {
		return nil, "", err
	}
	keyjson, err := ethkeystore.EncryptKey(key, auth, scryptN, scryptP)
	if err != nil {
		return nil, "", err
	}
	keyPath, err := generateKeyFilename(key.Address)
	if err != nil {
		return nil, "", err
	}
	err = writeKeyFile(keyPath, keyjson)
	if err != nil {
		return nil, "", err
	}
	return key, keyPath, err
}

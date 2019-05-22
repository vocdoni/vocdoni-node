package signature

import (
	"crypto/ecdsa"
	hex "encoding/hex"
	"errors"

	"github.com/ethereum/go-ethereum/common"
	crypto "github.com/ethereum/go-ethereum/crypto"
)

type SignKeys struct {
	Public     *ecdsa.PublicKey
	Private    *ecdsa.PrivateKey
	Authorized []common.Address
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

func (k *SignKeys) AddAuthKey(address string) error {
	if common.IsHexAddress(address) {
		addr := common.HexToAddress(address)
		k.Authorized = append(k.Authorized, addr)
		return nil
	} else {
		return errors.New("Invalid address hex")
	}
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

func (k *SignKeys) VerifySender(message, sig string) (bool, error) {
	sigHex, err := hex.DecodeString(sig)
	if err != nil {
		return false, err
	}
	msgHash := crypto.Keccak256Hash([]byte(message), sigHex)
	signerPKBytes, err := crypto.Ecrecover(msgHash.Bytes(), []byte(sig))
	if err != nil {
		return false, err
	}
	signerPK, err := crypto.UnmarshalPubkey(signerPKBytes)
	if err != nil {
		return false, err
	}
	clientAddr := crypto.PubkeyToAddress(*signerPK)
	for _, addr := range k.Authorized {
		if addr == clientAddr {
			return k.Verify(message, sig, string(signerPKBytes))
		}
	}
	return false, errors.New("Unknown error in verify sender")
}

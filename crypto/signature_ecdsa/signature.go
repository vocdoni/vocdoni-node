package signature

import (
	"crypto/ecdsa"
	hex "encoding/hex"
	"errors"
	"fmt"

	"github.com/ethereum/go-ethereum/accounts/keystore"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
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

func (k *SignKeys) AddKeyFromEncryptedJSON(keyJson []byte, passphrase string) error {
	key, err := keystore.DecryptKey(keyJson, passphrase)
	if err != nil {
		return err
	}
	k.Private = key.PrivateKey
	k.Public = &key.PrivateKey.PublicKey
	return nil
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

func signHash(data []byte) []byte {
	msg := fmt.Sprintf("\x19Ethereum Signed Message:\n%d%s", len(data), data)
	return crypto.Keccak256([]byte(msg))
}

func (k *SignKeys) VerifySender(msg, sigHex string) (bool, error) {
	sig := hexutil.MustDecode(sigHex)
	if sig[64] != 27 && sig[64] != 28 {
		return false, errors.New("Bad recovery hex")
	}
	sig[64] -= 27

	pubKey, err := crypto.SigToPub(signHash([]byte(msg)), sig)
	if err != nil {
		return false, errors.New("Bad sig")
	}

	recoveredAddr := crypto.PubkeyToAddress(*pubKey)
	for _, addr := range k.Authorized {
		if addr == recoveredAddr {
			return true, nil
		}
	}
	return false, nil
}

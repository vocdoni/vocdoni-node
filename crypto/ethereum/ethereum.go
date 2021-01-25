// Package ethereum provides cryptographic operations used in go-dvote related
// to ethereum.
package ethereum

import (
	"bytes"
	"crypto/ecdsa"
	"encoding/hex"
	"errors"
	"fmt"
	"sync"

	ethcommon "github.com/ethereum/go-ethereum/common"
	ethcrypto "github.com/ethereum/go-ethereum/crypto"

	"go.vocdoni.io/dvote/crypto"
)

// SignatureLength is the size of an ECDSA signature in hexString format
const SignatureLength = ethcrypto.SignatureLength

// PubKeyLength is the size of a Public Key
const PubKeyLength = 66

// PubKeyLengthUncompressed is the size of a uncompressed Public Key
const PubKeyLengthUncompressed = 130

// SigningPrefix is the prefix added when hashing
const SigningPrefix = "\u0019Ethereum Signed Message:\n"

// SignKeys represents an ECDSA pair of keys for signing.
// Authorized addresses is a list of Ethereum like addresses which are checked on Verify
type SignKeys struct {
	Public     ecdsa.PublicKey
	Private    ecdsa.PrivateKey
	Authorized map[ethcommon.Address]bool
	Lock       sync.RWMutex
}

// NewSignKeys creates an ECDSA pair of keys for signing
// and initializes the map for authorized keys
func NewSignKeys() *SignKeys {
	return &SignKeys{Authorized: make(map[ethcommon.Address]bool)}
}

// Generate generates new keys
func (k *SignKeys) Generate() error {
	key, err := ethcrypto.GenerateKey()
	if err != nil {
		return err
	}
	k.Private = *key
	k.Public = key.PublicKey
	return nil
}

// AddHexKey imports a private hex key
func (k *SignKeys) AddHexKey(privHex string) error {
	key, err := ethcrypto.HexToECDSA(trimHex(privHex))
	if err != nil {
		return err
	}
	k.Private = *key
	k.Public = key.PublicKey
	return nil
}

// AddAuthKey adds a new authorized address key
func (k *SignKeys) AddAuthKey(address ethcommon.Address) {
	k.Lock.Lock()
	k.Authorized[address] = true
	k.Lock.Unlock()
}

// HexString returns the public compressed and private keys as hex strings
func (k *SignKeys) HexString() (string, string) {
	pubHexComp := fmt.Sprintf("%x", ethcrypto.CompressPubkey(&k.Public))
	privHex := fmt.Sprintf("%x", ethcrypto.FromECDSA(&k.Private))
	return pubHexComp, privHex
}

// DecompressPubKey takes a hexString compressed public key and returns it descompressed. If already decompressed, returns the same key.
func DecompressPubKey(pubHexComp string) (string, error) {
	pubHexComp = trimHex(pubHexComp)
	if len(pubHexComp) > PubKeyLength {
		return pubHexComp, nil
	}
	pubBytes, err := hex.DecodeString(pubHexComp)
	if err != nil {
		return "", err
	}
	pub, err := ethcrypto.DecompressPubkey(pubBytes)
	if err != nil {
		return "", fmt.Errorf("decompress pubKey %w", err)
	}
	pubHex := fmt.Sprintf("%x", ethcrypto.FromECDSAPub(pub))
	return pubHex, nil
}

// CompressPubKey returns the compressed public key in hexString format
func CompressPubKey(pubHexDec string) (string, error) {
	pubHexDec = trimHex(pubHexDec)
	if len(pubHexDec) < PubKeyLengthUncompressed {
		return pubHexDec, nil
	}
	pubBytes, err := hex.DecodeString(pubHexDec)
	if err != nil {
		return "", err
	}
	pub, err := ethcrypto.UnmarshalPubkey(pubBytes)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("%x", ethcrypto.CompressPubkey(pub)), nil
}

// Address returns the SignKeys ethereum address
func (k *SignKeys) Address() ethcommon.Address {
	return ethcrypto.PubkeyToAddress(k.Public)
}

// AddressString returns the ethereum Address as string
func (k *SignKeys) AddressString() string { return ethcrypto.PubkeyToAddress(k.Public).String() }

// Sign signs a message. Message is a normal string (no HexString nor a Hash)
func (k *SignKeys) Sign(message []byte) ([]byte, error) {
	if k.Private.D == nil {
		return nil, errors.New("no private key available")
	}
	signature, err := ethcrypto.Sign(Hash(message), &k.Private)
	if err != nil {
		return nil, err
	}
	return signature, nil
}

// SignJSON signs a JSON message. Message is a struct interface
func (k *SignKeys) SignJSON(message interface{}) ([]byte, error) {
	encMsg, err := crypto.SortedMarshalJSON(message)
	if err != nil {
		return nil, errors.New("unable to marshal message to sign: %s")
	}
	return k.Sign(encMsg)
}

// Verify verifies a message. Signature is HexString
func (k *SignKeys) Verify(message, signature []byte) (bool, error) {
	pubHex, err := PubKeyFromSignature(message, signature)
	if err != nil {
		return false, err
	}
	pub, err := hex.DecodeString(pubHex)
	if err != nil {
		return false, err
	}
	hash := Hash(message)
	result := ethcrypto.VerifySignature(pub, hash, signature[:64])
	return result, nil
}

// VerifySender verifies if a message is sent by some Authorized address key
func (k *SignKeys) VerifySender(message, signature []byte) (bool, ethcommon.Address, error) {
	recoveredAddr, err := AddrFromSignature(message, signature)
	if err != nil {
		return false, ethcommon.Address{}, err
	}
	k.Lock.RLock()
	defer k.Lock.RUnlock()
	if k.Authorized[recoveredAddr] {
		return true, recoveredAddr, nil
	}
	return false, recoveredAddr, nil
}

// VerifyJSONsender verifies if a JSON message is sent by some Authorized address key
func (k *SignKeys) VerifyJSONsender(message interface{}, signature []byte) (bool, ethcommon.Address, error) {
	encMsg, err := crypto.SortedMarshalJSON(message)
	if err != nil {
		return false, ethcommon.Address{}, errors.New("unable to marshal message to sign: %s")
	}
	return k.VerifySender(encMsg, signature)
}

// Verify standalone function for verify a message
func Verify(message, signature, pub []byte) (bool, error) {
	// TODO(mvdan): pub is unused?
	sk := NewSignKeys()
	return sk.Verify(message, signature)
}

// AddrFromPublicKey standaolone function to obtain the Ethereum address from a ECDSA public key
func AddrFromPublicKey(pubHex string) (ethcommon.Address, error) {
	var pubHexDesc string
	var err error
	if len(pubHex) <= PubKeyLength {
		pubHexDesc, err = DecompressPubKey(pubHex)
		if err != nil {
			return ethcommon.Address{}, err
		}
	} else {
		pubHexDesc = pubHex
	}
	pubBytes, err := hex.DecodeString(pubHexDesc)
	if err != nil {
		return ethcommon.Address{}, err
	}
	pub, err := ethcrypto.UnmarshalPubkey(pubBytes)
	if err != nil {
		return ethcommon.Address{}, err
	}
	return ethcrypto.PubkeyToAddress(*pub), nil
}

// PubKeyFromPrivateKey returns the hex public key given a hex private key
func PubKeyFromPrivateKey(privHex string) (string, error) {
	s := NewSignKeys()
	if err := s.AddHexKey(privHex); err != nil {
		return "", err
	}
	pub, _ := s.HexString()
	return pub, nil
}

// PubKeyFromSignature recovers the ECDSA public key that created the signature of a message
// public key is hex encoded
func PubKeyFromSignature(message, signature []byte) (string, error) {
	if len(signature) < SignatureLength || len(signature) > SignatureLength+12 {
		return "", fmt.Errorf("signature length not correct (%d)", len(signature))
	}
	if signature[64] > 1 {
		signature[64] -= 27
	}
	if signature[64] > 1 {
		return "", errors.New("bad recover ID byte")
	}
	pubKey, err := ethcrypto.SigToPub(Hash(message), signature)
	if err != nil {
		return "", fmt.Errorf("sigToPub %w", err)
	}
	// Temporary until the client side changes to compressed keys
	return DecompressPubKey(fmt.Sprintf("%x", ethcrypto.CompressPubkey(pubKey)))
}

// AddrFromSignature recovers the Ethereum address that created the signature of a message
func AddrFromSignature(message, signature []byte) (ethcommon.Address, error) {
	pubHex, err := PubKeyFromSignature(message, signature)
	if err != nil {
		return ethcommon.Address{}, err
	}
	pub, err := hexToPubKey(pubHex)
	if err != nil {
		return ethcommon.Address{}, err
	}
	return ethcrypto.PubkeyToAddress(*pub), nil
}

// AddrFromJSONsignature recovers the Ethereum address that created the signature of a JSON message
func AddrFromJSONsignature(message interface{}, signature []byte) (ethcommon.Address, error) {
	encMsg, err := crypto.SortedMarshalJSON(message)
	if err != nil {
		return ethcommon.Address{}, errors.New("unable to marshal message to sign: %s")
	}
	return AddrFromSignature(encMsg, signature)
}

func hexToPubKey(pubHex string) (*ecdsa.PublicKey, error) {
	pubBytes, err := hex.DecodeString(trimHex(pubHex))
	if err != nil {
		return new(ecdsa.PublicKey), err
	}
	if len(pubHex) <= PubKeyLength {
		return ethcrypto.DecompressPubkey(pubBytes)
	}
	return ethcrypto.UnmarshalPubkey(pubBytes)
}

// Hash string data adding Ethereum prefix
func Hash(data []byte) []byte {
	var buf bytes.Buffer
	fmt.Fprintf(&buf, "%s%d%s", SigningPrefix, len(data), data)
	return HashRaw(buf.Bytes())
}

// HashRaw hashes a string with no prefix
func HashRaw(data []byte) []byte {
	return ethcrypto.Keccak256(data)
}

func trimHex(s string) string {
	if len(s) >= 2 && s[0] == '0' && (s[1] == 'x' || s[1] == 'X') {
		return s[2:]
	}
	return s
}

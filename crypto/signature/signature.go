package signature

import (
	"crypto/ecdsa"
	hex "encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/decred/dcrd/dcrec/secp256k1"
	crypto "github.com/ethereum/go-ethereum/crypto"
)

// AddressLength is the lenght of an Ethereum address
const AddressLength = 20

// SignatureLength is the size of an ECDSA signature in hexString format
const SignatureLength = 130

// PubKeyLength is the size of a Public Key
const PubKeyLength = 66

// SigningPrefix is the prefix added when hashing
const SigningPrefix = "\u0019Ethereum Signed Message:\n"

// SignKeys represents an ECDSA pair of keys for signing.
// Authorized addresses is a list of Ethereum like addresses which are checked on Verify
type SignKeys struct {
	Public     *ecdsa.PublicKey
	Private    *ecdsa.PrivateKey
	Authorized []Address
}

// Address is an Ethereum like adrress
type Address [AddressLength]byte

// Generate generates new keys
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

// AddHexKey imports a private hex key
func (k *SignKeys) AddHexKey(privHex string) error {
	var err error
	k.Private, err = crypto.HexToECDSA(sanitizeHex(privHex))
	if err == nil {
		k.Public = &k.Private.PublicKey
	}
	return err
}

// AddAuthKey adds a new authorized address key
func (k *SignKeys) AddAuthKey(address string) error {
	addrBytes, err := hex.DecodeString(sanitizeHex(address))
	if err != nil {
		return err
	}
	if len(addrBytes) == AddressLength {
		var addr Address
		copy(addr[:], addrBytes[:AddressLength])
		k.Authorized = append(k.Authorized, addr)
		return nil
	}
	return errors.New("invalid address lenght")
}

// HexString returns the public and private keys as hex strings
func (k *SignKeys) HexString() (string, string) {
	pubHex := fmt.Sprintf("%x", crypto.FromECDSAPub(k.Public))
	privHex := fmt.Sprintf("%x", crypto.FromECDSA(k.Private))
	return pubHex, privHex
}

// CompressPubKey returns the compressed public key in hexString format
func CompressPubKey(pubHex string) (string, error) {
	pubBytes, err := hex.DecodeString(sanitizeHex(pubHex))
	if err != nil {
		return "", err
	}
	pub, err := crypto.UnmarshalPubkey(pubBytes)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("%x", crypto.CompressPubkey(pub)), nil
}

// DecompressPubKey takes a hexString compressed public key and returns it descompressed
func DecompressPubKey(pubHexComp string) (string, error) {
	pubBytes, err := hex.DecodeString(pubHexComp)
	if err != nil {
		return "", err
	}
	pub, err := crypto.DecompressPubkey(pubBytes)
	if err != nil {
		return "", err
	}
	pubHex := fmt.Sprintf("%x", crypto.FromECDSAPub(pub))
	return pubHex, nil
}

// EthAddrString return the Ethereum address from the ECDSA public key
func (k *SignKeys) EthAddrString() string {
	recoveredAddr := crypto.PubkeyToAddress(*k.Public).Bytes()
	return fmt.Sprintf("%x", recoveredAddr)
}

// Sign signs a message. Message is a normal string (no HexString nor a Hash)
func (k *SignKeys) Sign(message string) (string, error) {
	if k.Private == nil {
		return "", errors.New("No private key available")
	}
	signature, err := crypto.Sign(Hash(message), k.Private)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("%x", signature), nil
}

// SignJSON signs a JSON message. Message is a struct interface
func (k *SignKeys) SignJSON(message interface{}) (string, error) {
	rawMsg, err := json.Marshal(message)
	if err != nil {
		return "", errors.New("unable to marshal message to sign: %s")
	}
	sig, err := k.Sign(string(rawMsg))
	if err != nil {
		return "", errors.New("error signing response body: %s")
	}
	prefixedSig := "0x" + sanitizeHex(sig)
	return prefixedSig, nil
}

// Verify verifies a message. Signature is HexString
func (k *SignKeys) Verify(message, signHex string) (bool, error) {
	pubHex, err := PubKeyFromSignature(message, signHex)
	if err != nil {
		return false, err
	}
	signature, err := hex.DecodeString(signHex)
	if err != nil {
		return false, err
	}
	pub, err := hex.DecodeString(pubHex)
	if err != nil {
		return false, err
	}
	hash := Hash(message)
	result := crypto.VerifySignature(pub, hash, signature[:64])
	return result, nil
}

// Verify verifies a JSON message. Signature is HexString
func (k *SignKeys) VerifyJSON(message interface{}, signHex string) (bool, error) {
	rawMsg, err := json.Marshal(message)
	if err != nil {
		return false, errors.New("unable to marshal message to sign: %s")
	}
	return k.Verify(string(rawMsg), signHex)
}

// VerifySender verifies if a message is sent by some Authorized address key
func (k *SignKeys) VerifySender(msg, sigHex string) (bool, string, error) {
	recoveredAddr, err := AddrFromSignature(msg, sigHex)
	if err != nil {
		return false, "", err
	}
	if len(k.Authorized) < 1 {
		return true, recoveredAddr, nil
	}
	for _, addr := range k.Authorized {
		if fmt.Sprintf("%x", addr) == recoveredAddr {
			return true, recoveredAddr, nil
		}
	}
	return false, recoveredAddr, nil
}

// VerifyJSONsender verifies if a JSON message is sent by some Authorized address key
func (k *SignKeys) VerifyJSONsender(msg interface{}, sigHex string) (bool, string, error) {
	rawMsg, err := json.Marshal(msg)
	if err != nil {
		return false, "", errors.New("unable to marshal message to sign: %s")
	}
	return k.VerifySender(string(rawMsg), sigHex)
}

// Standalone function for verify a message
func Verify(message, signHex, pubHex string) (bool, error) {
	sk := new(SignKeys)
	return sk.Verify(message, signHex)
}

// Standaolone function to obtain the Ethereum address from a ECDSA public key
func AddrFromPublicKey(pubHex string) (string, error) {
	pubBytes, err := hex.DecodeString(sanitizeHex(pubHex))
	if err != nil {
		return "", err
	}
	pub, err := crypto.UnmarshalPubkey(pubBytes)
	if err != nil {
		return "", err
	}
	recoveredAddr := [20]byte(crypto.PubkeyToAddress(*pub))
	return fmt.Sprintf("%x", recoveredAddr), nil
}

// PubKeyFromSignature recovers the ECDSA public key that created the signature of a message
func PubKeyFromSignature(msg, sigHex string) (string, error) {
	if len(sanitizeHex(sigHex)) != SignatureLength {
		return "", errors.New("signature length not correct")
	}
	sig, err := hex.DecodeString(sanitizeHex(sigHex))
	if err != nil {
		return "", err
	}
	if sig[64] > 1 {
		sig[64] -= 27
	}
	if sig[64] < 0 || sig[64] > 1 {
		return "", errors.New("Bad recover ID byte")
	}
	pubKey, err := crypto.SigToPub(Hash(msg), sig)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("%x", crypto.FromECDSAPub(pubKey)), nil
}

// AddrFromSignature recovers the Ethereum address that created the signature of a message
func AddrFromSignature(msg, sigHex string) (string, error) {
	pubHex, err := PubKeyFromSignature(msg, sigHex)
	if err != nil {
		return "", err
	}
	pub, err := hexToPubKey(pubHex)
	if err != nil {
		return "", err
	}
	addr := crypto.PubkeyToAddress(*pub)
	return fmt.Sprintf("%x", addr), nil
}

// AddrFromJSONsignature recovers the Ethereum address that created the signature of a JSON message
func AddrFromJSONsignature(msg interface{}, sigHex string) (string, error) {
	rawMsg, err := json.Marshal(msg)
	if err != nil {
		return "", errors.New("unable to marshal message to sign: %s")
	}
	return AddrFromSignature(string(rawMsg), sigHex)
}

func hexToPubKey(pubHex string) (*ecdsa.PublicKey, error) {
	pubBytes, err := hex.DecodeString(sanitizeHex(pubHex))
	if err != nil {
		return new(ecdsa.PublicKey), err
	}
	return crypto.UnmarshalPubkey(pubBytes)
}

// Hash string data adding Ethereum prefix
func Hash(data string) []byte {
	payloadToSign := fmt.Sprintf("%s%d%s", SigningPrefix, len(data), data)
	return crypto.Keccak256([]byte(payloadToSign))
}

// HashRaw hashes a string with no prefix
func HashRaw(data string) []byte {
	return crypto.Keccak256([]byte(data))
}

func sanitizeHex(hexStr string) string {
	if strings.HasPrefix(hexStr, "0x") {
		return fmt.Sprintf("%s", hexStr[2:])
	}
	return hexStr
}

// Encrypt uses secp256k1 standard from https://www.secg.org/sec2-v2.pdf to encrypt a message.
// The result is a Hexadecimal string
func (k *SignKeys) Encrypt(message string) (string, error) {
	pub, _ := k.HexString()
	pubBytes, err := hex.DecodeString(pub)
	if err != nil {
		return "", err
	}
	pubKey, err := secp256k1.ParsePubKey(pubBytes)
	if err != nil {
		return "", err
	}
	ciphertext, err := secp256k1.Encrypt(pubKey, []byte(message))
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("%x", ciphertext), nil
}

// Decrypt uses secp256k1 standard to decrypt a Hexadecimal string message
// The result is plain text (no hex encoded)
func (k *SignKeys) Decrypt(hexMessage string) (string, error) {
	_, priv := k.HexString()
	pkBytes, err := hex.DecodeString(priv)
	if err != nil {
		return "", err
	}
	privKey, _ := secp256k1.PrivKeyFromBytes(pkBytes)
	cipertext, err := hex.DecodeString(sanitizeHex(hexMessage))
	if err != nil {
		return "", err
	}
	plaintext, err := secp256k1.Decrypt(privKey, cipertext)
	if err != nil {
		return "", err
	}
	return string(plaintext), nil
}

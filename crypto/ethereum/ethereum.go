// Package ethereum provides cryptographic operations used in go-dvote related
// to ethereum.
package ethereum

import (
	"bytes"
	"crypto/ecdsa"
	"encoding/hex"
	"errors"
	"fmt"
	"math/big"
	"sync"

	ethcommon "github.com/ethereum/go-ethereum/common"
	ethcrypto "github.com/ethereum/go-ethereum/crypto"
	"github.com/iden3/go-iden3-crypto/poseidon"
	"go.vocdoni.io/dvote/crypto/zk"
	"go.vocdoni.io/dvote/tree/arbo"
	"go.vocdoni.io/dvote/types"
	"go.vocdoni.io/dvote/util"
)

const (
	// SignatureLength is the size of an ECDSA signature in hexString format
	SignatureLength = ethcrypto.SignatureLength
	// PubKeyLengthBytes is the size of a Public Key
	PubKeyLengthBytes = 33
	// PubKeyLengthBytesUncompressed is the size of a uncompressed Public Key
	PubKeyLengthBytesUncompressed = 65
	// SigningPrefix is the prefix added when hashing
	SigningPrefix = "\u0019Ethereum Signed Message:\n"
	// DefaultSikPayload conatains the default seed to sing during
	// SIK generation
	DefaultSikPayload = "vocdoni-sik-payload"
)

// SignKeys represents an ECDSA pair of keys for signing.
// Authorized addresses is a list of Ethereum like addresses which are checked on Verify
type SignKeys struct {
	Public     ecdsa.PublicKey
	Private    ecdsa.PrivateKey
	Authorized map[ethcommon.Address]bool
	Lock       sync.RWMutex
}

// Address represents an Ethereum like address
type Address ethcommon.Address

// NewSignKeys creates an ECDSA pair of keys for signing
// and initializes the map for authorized keys
func NewSignKeys() *SignKeys {
	return &SignKeys{
		Private:    ecdsa.PrivateKey{},
		Authorized: make(map[ethcommon.Address]bool),
	}
}

// NewSignKeysBatch creates a set of eth random signing keys
func NewSignKeysBatch(n int) []*SignKeys {
	s := make([]*SignKeys, n)
	for i := 0; i < n; i++ {
		s[i] = NewSignKeys()
		if err := s[i].Generate(); err != nil {
			panic(err)
		}
	}
	return s
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
	key, err := ethcrypto.HexToECDSA(util.TrimHex(privHex))
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

// PublicKey returns the compressed public key
func (k *SignKeys) PublicKey() types.HexBytes {
	return ethcrypto.CompressPubkey(&k.Public)
}

// PrivateKey returns the private key
func (k *SignKeys) PrivateKey() types.HexBytes {
	return ethcrypto.FromECDSA(&k.Private)
}

// DecompressPubKey takes a compressed public key and returns it descompressed. If already decompressed, returns the same key.
func DecompressPubKey(pubComp types.HexBytes) (types.HexBytes, error) {
	if len(pubComp) > PubKeyLengthBytes {
		return pubComp, nil
	}
	pub, err := ethcrypto.DecompressPubkey(pubComp)
	if err != nil {
		return nil, fmt.Errorf("decompress pubKey %w", err)
	}
	return ethcrypto.FromECDSAPub(pub), nil
}

// CompressPubKey returns the compressed public key in hexString format
func CompressPubKey(pubHexDec string) (string, error) {
	pubHexDec = util.TrimHex(pubHexDec)
	if len(pubHexDec) < PubKeyLengthBytesUncompressed*2 {
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
func (k *SignKeys) AddressString() string {
	return ethcrypto.PubkeyToAddress(k.Public).String()
}

// SignEthereum signs a message. Message is a normal string (no HexString nor a Hash)
func (k *SignKeys) SignEthereum(message []byte) ([]byte, error) {
	if k.Private.D == nil {
		return nil, errors.New("no private key available")
	}
	signature, err := ethcrypto.Sign(Hash(message), &k.Private)
	if err != nil {
		return nil, err
	}
	return signature, nil
}

// SignVocdoniTx signs a vocdoni transaction. TxData is the full transaction payload (no HexString nor a Hash)
func (k *SignKeys) SignVocdoniTx(txData []byte, chainID string) ([]byte, error) {
	if k.Private.D == nil {
		return nil, errors.New("no private key available")
	}
	signature, err := ethcrypto.Sign(Hash(BuildVocdoniTransaction(txData, chainID)), &k.Private)
	if err != nil {
		return nil, err
	}
	return signature, nil
}

// SignVocdoniMsg signs a vocdoni message. Message is the full payload (no HexString nor a Hash)
func (k *SignKeys) SignVocdoniMsg(message []byte) ([]byte, error) {
	if k.Private.D == nil {
		return nil, errors.New("no private key available")
	}
	signature, err := ethcrypto.Sign(Hash(BuildVocdoniMessage(message)), &k.Private)
	if err != nil {
		return nil, err
	}
	return signature, nil
}

// SignVocdoniSik signs the default vocdoni sik payload. It envolves the
// SignEthereum method, using the DefaultSikPayload and discarding the last
// byte of the signature (used for recovery), different that the same byte of a
// signature generated with javascript.
func (k *SignKeys) SignVocdoniSik() ([]byte, error) {
	sign, err := k.SignEthereum([]byte(DefaultSikPayload))
	if err != nil {
		return nil, err
	}
	return sign[:SignatureLength-1], nil
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

// CustomSik method generates the Secret Identity Key for the current SignKeys
// with the signature of the DefaultSikPayload and the user secret (if it is
// provided) following the definition:
//
//	SIK = poseidon(address, signature, secret)
//
// The secret could be nil.
func (k *SignKeys) CustomSik(secret []byte) ([]byte, error) {
	if secret == nil {
		return nil, fmt.Errorf("no secret provided")
	}
	sign, err := k.SignVocdoniSik()
	if err != nil {
		return nil, fmt.Errorf("error signing default sik seed: %w", err)
	}
	seed := []*big.Int{
		arbo.BytesToBigInt(k.Address().Bytes()),
		zk.BigToFF(new(big.Int).SetBytes(secret)),
		zk.BigToFF(new(big.Int).SetBytes(sign)),
	}
	hash, err := poseidon.Hash(seed)
	if err != nil {
		return nil, err
	}
	return arbo.BigIntToBytes(arbo.HashFunctionPoseidon.Len(), hash), nil
}

// Sik method returns the calculated SIK for the current SignKeys with any
// password or secret as input.
func (k *SignKeys) Sik() ([]byte, error) {
	return k.CustomSik([]byte{})
}

// Nullifier method composes the nullifier of the current SignKeys for the
// desired election id and the secret provided.
func (k *SignKeys) Nullifier(electionId, secret []byte) ([]byte, error) {
	// sign the default Secret Identity Key seed
	sign, err := k.SignVocdoniSik()
	if err != nil {
		return nil, fmt.Errorf("error signing default sik seed: %w", err)
	}
	// get the representation of the signature on the finite field and repeat
	// the same with the secret if it is provided, if not add a zero
	seed := []*big.Int{zk.BigToFF(new(big.Int).SetBytes(sign))}
	if secret != nil {
		seed = append(seed, zk.BigToFF(new(big.Int).SetBytes(secret)))
	} else {
		seed = append(seed, big.NewInt(0))
	}
	// encode the election id for circom and include it into the nullifier
	encElectionId := zk.BytesToArbo(electionId)
	seed = append(seed, encElectionId...)
	// calculate the poseidon image --> H(signature + secret + electionId)
	hash, err := poseidon.Hash(seed)
	if err != nil {
		return nil, err
	}
	return hash.Bytes(), nil
}

// AddrFromPublicKey standaolone function to obtain the Ethereum address from a ECDSA public key
func AddrFromPublicKey(pub []byte) (ethcommon.Address, error) {
	var err error
	if len(pub) <= PubKeyLengthBytes {
		pub, err = DecompressPubKey(pub)
		if err != nil {
			return ethcommon.Address{}, err
		}
	}
	pubkey, err := ethcrypto.UnmarshalPubkey(pub)
	if err != nil {
		return ethcommon.Address{}, err
	}
	return ethcrypto.PubkeyToAddress(*pubkey), nil
}

// AddrFromBytes returns the Ethereum address from a byte array
func AddrFromBytes(addr []byte) ethcommon.Address {
	return ethcommon.BytesToAddress(addr)
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
func PubKeyFromSignature(message, signature []byte) ([]byte, error) {
	if len(signature) < SignatureLength || len(signature) > SignatureLength+12 {
		// TODO: investigate the exact size (and if a marging is required)
		return nil, fmt.Errorf("signature length not correct (%d)", len(signature))
	}
	if signature[64] > 1 {
		signature[64] -= 27
	}
	if signature[64] > 1 {
		return nil, errors.New("bad recover ID byte")
	}
	pubKey, err := ethcrypto.SigToPub(Hash(message), signature)
	if err != nil {
		return nil, fmt.Errorf("sigToPub %w", err)
	}
	// Temporary until the client side changes to compressed keys
	return ethcrypto.CompressPubkey(pubKey), nil
}

// AddrFromSignature recovers the Ethereum address that created the signature of a message
func AddrFromSignature(message, signature []byte) (ethcommon.Address, error) {
	pub, err := PubKeyFromSignature(message, signature)
	if err != nil {
		return ethcommon.Address{}, err
	}
	return AddrFromPublicKey(pub)
}

// Hash data adding Ethereum prefix
func Hash(data []byte) []byte {
	var buf bytes.Buffer
	fmt.Fprintf(&buf, "%s%d%s", SigningPrefix, len(data), data)
	return HashRaw(buf.Bytes())
}

// BuildVocdoniTransaction builds the payload of a vochain transaction (txData)
// ready to be signed
func BuildVocdoniTransaction(txData []byte, chainID string) []byte {
	var buf bytes.Buffer
	fmt.Fprintf(&buf, "Vocdoni signed transaction:\n%s\n%x", chainID, HashRaw(txData))
	return buf.Bytes()
}

// BuildVocdoniMessage builds the payload of a vocdoni message
// ready to be signed
func BuildVocdoniMessage(message []byte) []byte {
	var buf bytes.Buffer
	fmt.Fprintf(&buf, "Vocdoni signed message:\n%x", HashRaw(message))
	return buf.Bytes()
}

// HashRaw hashes data with no prefix
func HashRaw(data []byte) []byte {
	return ethcrypto.Keccak256(data)
}

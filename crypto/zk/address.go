package zk

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"math/big"

	"github.com/iden3/go-iden3-crypto/babyjub"
	"github.com/iden3/go-iden3-crypto/poseidon"
	"go.vocdoni.io/dvote/crypto/ethereum"
	"go.vocdoni.io/dvote/tree/arbo"
	"go.vocdoni.io/dvote/util"
)

// defaultZkAddrLen contains the default size of a ZkAddress in bytes.
const defaultZkAddrLen = 20

// ZkAddress struct allow to create and encoding properly the zk-snark
// compatible address for the vochain. This address is calculated from a
// BabyJubJub key pair, based on a seed. The address has 20 bytes of size and
// it is the truncated version of the poseidon hash of the BabyJubJub publicKey.
type ZkAddress struct {
	// Privkey contains the big.Int version of the BabyJubJub private key
	PrivKey *big.Int
	// PubKey contains the big.Int poseidon hash of the BabyJubJub public key
	// (x and y coordinates of a point of the curve)
	PubKey *big.Int
	// addr contains the public key big.Int reduced to 20 bytes
	// (defaultZkAddrLen). It is a private attribute because the correct formats
	// are calulated throw other struct methods such as ZkAddress.String() or
	// ZkAddress.Bytes().
	addr *big.Int
}

// AddressFromBytes returns a new ZkAddress based on the seed provided. Using
// it, a BabyJubJub key pair is calculated and based on it. The ZkAddress
// components are:
//   - ZkAddress.PrivKey: The BabyJubJub private key big.Int value, following
//     the EdDSA standard, and using blake-512 hash.
//   - ZkAddress.PublicKey: The poseidon hash of the BabyJubJub public key
//     components (X and Y coordinates as big.Int).
//   - ZkAddress.addr: The truncated to 20 bytes (defaultZkAddrLen) little
//     endian version of the ZkAddress.PublicKey.
func AddressFromBytes(seed []byte) (*ZkAddress, error) {
	if len(seed) < 16 {
		return nil, fmt.Errorf("the seed provided does not have 32 bytes at least")
	}
	jubjubKey := babyjub.PrivateKey{}
	if _, err := hex.Decode(jubjubKey[:], seed); err != nil {
		return nil, fmt.Errorf("error generating babyjub key: %w", err)
	}
	pubKey, err := poseidon.Hash([]*big.Int{
		jubjubKey.Public().X,
		jubjubKey.Public().Y,
	})
	if err != nil {
		return nil, err
	}
	return &ZkAddress{
		PrivKey: babyjub.SkToBigInt(&jubjubKey),
		PubKey:  new(big.Int).Set(pubKey),
		addr:    LittleEndianToNBytes(pubKey, defaultZkAddrLen),
	}, nil
}

// AddressFromString wraps AddressFromBytes function transforming the seed from
// string to []byte.
func AddressFromString(seed string) (*ZkAddress, error) {
	return AddressFromBytes([]byte(seed))
}

// AddressFromSignKeys gets the private key from the ethereum.SignKeys provided
// and pass its string representation as []byte to AddressFromBytes function.
func AddressFromSignKeys(acc *ethereum.SignKeys) (*ZkAddress, error) {
	privKey := acc.PrivateKey()
	return AddressFromBytes([]byte(privKey.String()))
}

// NewRandAddress returns a ZkAddress based on a random BabyJubJub private key.
func NewRandAddress() (*ZkAddress, error) {
	return AddressFromBytes([]byte(util.RandomHex(32)))
}

// Bytes returns the current ZkAddress.addr transformed to types.HexBytes using
// the arbo.BigIntToBytes() function.
func (zk *ZkAddress) Bytes() []byte {
	return arbo.BigIntToBytes(defaultZkAddrLen, zk.addr)
}

// String function returns the current ZkAddress.Bytes() result transformed to
// string using the types.HexBytes.String() function.
func (zk *ZkAddress) String() string {
	addr := zk.Bytes()
	return hex.EncodeToString(addr)
}

// Nullifier returns ZkSnark ready vote nullifier based on the current
// ZkAddress.PrivKey and the electionId provided. The nullifier is calculated
// following this definition:
//
//	nullifier = poseidon(jubjubPrivKey, sha256(electionId))
func (zk *ZkAddress) Nullifier(electionId []byte) (*big.Int, error) {
	// Encode the electionId -> sha256(electionId)
	hashedElectionId := sha256.Sum256(electionId)
	intElectionId := []*big.Int{
		new(big.Int).SetBytes(arbo.SwapEndianness(hashedElectionId[:16])),
		new(big.Int).SetBytes(arbo.SwapEndianness(hashedElectionId[16:])),
	}
	// Calculate nullifier hash: poseidon(babyjubjub(privKey) + sha256(ElectionId))
	nullifier, err := poseidon.Hash([]*big.Int{
		zk.PrivKey,
		intElectionId[0],
		intElectionId[1],
	})
	if err != nil {
		return nil, fmt.Errorf("error generating nullifier: %w", err)
	}
	return nullifier, nil
}

package arbo

import (
	"crypto/sha256"
	"math/big"

	"github.com/iden3/go-iden3-crypto/poseidon"
	"golang.org/x/crypto/blake2b"
)

var (
	// TypeHashSha256 represents the label for the HashFunction of Sha256
	TypeHashSha256 = []byte("sha256")
	// TypeHashPoseidon represents the label for the HashFunction of
	// Poseidon
	TypeHashPoseidon = []byte("poseidon")
	// TypeHashBlake2b represents the label for the HashFunction of Blake2b
	TypeHashBlake2b = []byte("blake2b")

	// HashFunctionSha256 contains the HashSha256 struct which implements
	// the HashFunction interface
	HashFunctionSha256 HashSha256
	// HashFunctionPoseidon contains the HashPoseidon struct which implements
	// the HashFunction interface
	HashFunctionPoseidon HashPoseidon
	// HashFunctionBlake2b contains the HashBlake2b struct which implements
	// the HashFunction interface
	HashFunctionBlake2b HashBlake2b
)

// Once Generics are at Go, this will be updated (August 2021
// https://blog.golang.org/generics-next-step)

// HashFunction defines the interface that is expected for a hash function to be
// used in a generic way in the Tree.
type HashFunction interface {
	Type() []byte
	Len() int
	Hash(...[]byte) ([]byte, error)
	// CheckInputs checks if the inputs are valid without computing the hash
	// CheckInputs(...[]byte) error
}

// HashSha256 implements the HashFunction interface for the Sha256 hash
type HashSha256 struct{}

// Type returns the type of HashFunction for the HashSha256
func (f HashSha256) Type() []byte {
	return TypeHashSha256
}

// Len returns the length of the Hash output
func (f HashSha256) Len() int {
	return 32 //nolint:gomnd
}

// Hash implements the hash method for the HashFunction HashSha256
func (f HashSha256) Hash(b ...[]byte) ([]byte, error) {
	var toHash []byte
	for i := 0; i < len(b); i++ {
		toHash = append(toHash, b[i]...)
	}
	h := sha256.Sum256(toHash)
	return h[:], nil
}

// HashPoseidon implements the HashFunction interface for the Poseidon hash
type HashPoseidon struct{}

// Type returns the type of HashFunction for the HashPoseidon
func (f HashPoseidon) Type() []byte {
	return TypeHashPoseidon
}

// Len returns the length of the Hash output
func (f HashPoseidon) Len() int {
	return 32 //nolint:gomnd
}

// Hash implements the hash method for the HashFunction HashPoseidon
func (f HashPoseidon) Hash(b ...[]byte) ([]byte, error) {
	var toHash []*big.Int
	for i := 0; i < len(b); i++ {
		bi := BytesToBigInt(b[i])
		toHash = append(toHash, bi)
	}
	h, err := poseidon.Hash(toHash)
	if err != nil {
		return nil, err
	}
	hB := BigIntToBytes(h)
	return hB, nil
}

// HashBlake2b implements the HashFunction interface for the Blake2b hash
type HashBlake2b struct{}

// Type returns the type of HashFunction for the HashBlake2b
func (f HashBlake2b) Type() []byte {
	return TypeHashBlake2b
}

// Len returns the length of the Hash output
func (f HashBlake2b) Len() int {
	return 32 //nolint:gomnd
}

// Hash implements the hash method for the HashFunction HashBlake2b
func (f HashBlake2b) Hash(b ...[]byte) ([]byte, error) {
	hasher, err := blake2b.New256(nil)
	if err != nil {
		return nil, err
	}
	for i := 0; i < len(b); i++ {
		if _, err = hasher.Write(b[i]); err != nil {
			return nil, err
		}
	}
	return hasher.Sum(nil), nil
}

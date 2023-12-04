package util

import (
	"crypto/sha256"
	"math/big"

	"go.vocdoni.io/dvote/tree/arbo"
)

// bn254BaseField contains the Base Field of the twisted Edwards curve, whose
// base field os the scalar field on the curve BN254. It helps to represent
// a scalar number into the field.
var bn254BaseField, _ = new(big.Int).SetString("21888242871839275222246405745257275088548364400416034343698204186575808495617", 10)

// BigToFF function returns the finite field representation of the big.Int
// provided. It uses Euclidean Modulus and the BN254 curve scalar field to
// represent the provided number.
func BigToFF(iv *big.Int) *big.Int {
	z := big.NewInt(0)
	if c := iv.Cmp(bn254BaseField); c == 0 {
		return z
	} else if c != 1 && iv.Cmp(z) != -1 {
		return iv
	}
	return z.Mod(iv, bn254BaseField)
}

// BytesToArbo calculates the sha256 hash (32 bytes) of the slice of bytes
// provided. Then, splits the hash into a two parts of 16 bytes, swap the
// endianess of that parts and encodes they into a two big.Int's.
func BytesToArbo(input []byte) []*big.Int {
	hash := sha256.Sum256(input)
	return []*big.Int{
		new(big.Int).SetBytes(arbo.SwapEndianness(hash[:16])),
		new(big.Int).SetBytes(arbo.SwapEndianness(hash[16:])),
	}
}

// BytesToArboStr function wraps BytesToArbo to return the input as []string.
func BytesToArboStr(input []byte) []string {
	arboBytes := BytesToArbo(input)
	return []string{arboBytes[0].String(), arboBytes[1].String()}
}

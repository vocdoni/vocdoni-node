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

// BytesToArboSplit calculates the sha256 hash (32 bytes) of the slice of bytes
// provided. Then, splits the hash into a two parts of 16 bytes, swap the
// endianess of that parts and encodes they into a two big.Int's.
func BytesToArboSplit(input []byte) []*big.Int {
	hash := sha256.Sum256(input)
	return []*big.Int{
		new(big.Int).SetBytes(arbo.SwapEndianness(hash[:16])),
		new(big.Int).SetBytes(arbo.SwapEndianness(hash[16:])),
	}
}

// BytesToArboSplitStr function wraps BytesToArbo to return the input as []string.
func BytesToArboSplitStr(input []byte) []string {
	arboBytes := BytesToArboSplit(input)
	return []string{arboBytes[0].String(), arboBytes[1].String()}
}

// SplittedArboToBytes function receives a slice of big.Int's and returns the
func SplittedArboToBytes(input1, input2 *big.Int, swap, strict bool) []byte {
	// when the last bytes are 0, the SwapEndianness function removes them
	// so we need to add them back until we have 16 bytes in both parts
	b1 := input1.Bytes()
	b2 := input2.Bytes()
	if swap {
		b1 = arbo.SwapEndianness(b1)
		b2 = arbo.SwapEndianness(b2)
	}
	if strict {
		for len(b1) < 16 {
			b1 = append([]byte{0}, b1...)
		}
		for len(b2) < 16 {
			b2 = append([]byte{0}, b2...)
		}
	}
	return append(b1, b2...)
}

// SplittedArboStrToBytes function wraps SplittedArboToBytes to return the input as []byte
func SplittedArboStrToBytes(input1, input2 string, swap, strict bool) []byte {
	b1 := new(big.Int)
	b1.SetString(input1, 10)
	b2 := new(big.Int)
	b2.SetString(input2, 10)
	return SplittedArboToBytes(b1, b2, swap, strict)
}

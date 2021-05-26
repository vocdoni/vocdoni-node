package arbo

import "math/big"

// SwapEndianness swaps the order of the bytes in the byte slice.
func SwapEndianness(b []byte) []byte {
	o := make([]byte, len(b))
	for i := range b {
		o[len(b)-1-i] = b[i]
	}
	return o
}

// BigIntToBytes converts a *big.Int into a byte array in Little-Endian
func BigIntToBytes(blen int, bi *big.Int) []byte {
	// var b [blen]byte // TODO make the length depending on the tree.hashFunction.Len()
	b := make([]byte, blen)
	copy(b[:], SwapEndianness(bi.Bytes()))
	return b[:]
}

// BytesToBigInt converts a byte array in Little-Endian representation into
// *big.Int
func BytesToBigInt(b []byte) *big.Int {
	return new(big.Int).SetBytes(SwapEndianness(b))
}

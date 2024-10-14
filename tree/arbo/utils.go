package arbo

import (
	"fmt"
	"math"
	"math/big"

	"go.vocdoni.io/dvote/db"
)

// SwapEndianness swaps the order of the bytes in the byte slice.
func SwapEndianness(b []byte) []byte {
	o := make([]byte, len(b))
	for i := range b {
		o[len(b)-1-i] = b[i]
	}
	return o
}

// BigIntToBytes converts a *big.Int into a byte array in Little-Endian
//
// Deprecated: use BigIntToBytesLE or BigIntToBytesBE
func BigIntToBytes(blen int, bi *big.Int) []byte {
	return BigIntToBytesLE(blen, bi)
}

// BigIntToBytesLE converts a *big.Int into a byte array in Little-Endian
func BigIntToBytesLE(blen int, bi *big.Int) []byte {
	// TODO make the length depending on the tree.hashFunction.Len()
	b := make([]byte, blen)
	copy(b, SwapEndianness(bi.Bytes()))
	return b
}

// BigIntToBytesBE converts a *big.Int into a byte array in Big-Endian
func BigIntToBytesBE(blen int, bi *big.Int) []byte {
	// TODO make the length depending on the tree.hashFunction.Len()
	b := make([]byte, blen)
	copy(b, bi.Bytes())
	return b
}

// BytesToBigInt converts a byte array in Little-Endian representation into
// *big.Int
//
// Deprecated: use BytesLEToBigInt or BytesBEToBigInt
func BytesToBigInt(b []byte) *big.Int {
	return BytesLEToBigInt(b)
}

// BytesLEToBigInt converts a byte array in Little-Endian representation into
// *big.Int
func BytesLEToBigInt(b []byte) *big.Int {
	return new(big.Int).SetBytes(SwapEndianness(b))
}

// BytesBEToBigInt converts a byte array in Big-Endian representation into
// *big.Int
func BytesBEToBigInt(b []byte) *big.Int {
	return new(big.Int).SetBytes(b)
}

// newLeafValue takes a key & value from a leaf, and computes the leaf hash,
// which is used as the leaf key. And the value is the concatenation of the
// inputted key & value. The output of this function is used as key-value to
// store the leaf in the DB.
// [     1 byte   |     1 byte    | N bytes | M bytes ]
// [ type of node | length of key |   key   |  value  ]
func newLeafValue(hashFunc HashFunction, k, v []byte) ([]byte, []byte, error) {
	if err := checkKeyValueLen(k, v); err != nil {
		return nil, nil, err
	}
	leafKey, err := hashFunc.Hash(k, v, []byte{1})
	if err != nil {
		return nil, nil, err
	}
	var leafValue []byte
	leafValue = append(leafValue, byte(PrefixValueLeaf))
	leafValue = append(leafValue, byte(len(k)))
	leafValue = append(leafValue, k...)
	leafValue = append(leafValue, v...)
	return leafKey, leafValue, nil
}

// ReadLeafValue reads from a byte array the leaf key & value
func ReadLeafValue(b []byte) ([]byte, []byte) {
	if len(b) < PrefixValueLen {
		return []byte{}, []byte{}
	}

	kLen := b[1]
	if len(b) < PrefixValueLen+int(kLen) {
		return []byte{}, []byte{}
	}
	k := b[PrefixValueLen : PrefixValueLen+kLen]
	v := b[PrefixValueLen+kLen:]
	return k, v
}

// keyPathFromKey returns the keyPath and checks that the key is not bigger
// than maximum key length for the tree maxLevels size.
// This is because if the key bits length is bigger than the maxLevels of the
// tree, two different keys that their difference is at the end, will collision
// in the same leaf of the tree (at the max depth).
func keyPathFromKey(maxLevels int, k []byte) ([]byte, error) {
	maxKeyLen := int(math.Ceil(float64(maxLevels) / float64(8)))
	if len(k) > maxKeyLen {
		return nil, fmt.Errorf("len(k) can not be bigger than ceil(maxLevels/8), where"+
			" len(k): %d, maxLevels: %d, max key len=ceil(maxLevels/8): %d. Might need"+
			" a bigger tree depth (maxLevels>=%d) in order to input keys of length %d",
			len(k), maxLevels, maxKeyLen, len(k)*8, len(k))
	}
	keyPath := make([]byte, maxKeyLen)
	copy(keyPath, k)
	return keyPath, nil
}

// checkKeyValueLen checks the key length and value length. This method is used
// when adding single leafs and also when adding a batch. The limits of lengths
// used are derived from the encoding of tree dumps: 1 byte to define the
// length of the keys (2^8-1 bytes length)), and 2 bytes to define the length
// of the values (2^16-1 bytes length).
func checkKeyValueLen(k, v []byte) error {
	if len(k) > maxUint8 {
		return fmt.Errorf("len(k)=%v, can not be bigger than %v",
			len(k), maxUint8)
	}
	if len(v) > maxUint16 {
		return fmt.Errorf("len(v)=%v, can not be bigger than %v",
			len(v), maxUint16)
	}
	return nil
}

// deleteNodes removes the nodes in the keys slice from the database.
func deleteNodes(wTx db.WriteTx, keys [][]byte) error {
	for _, k := range keys {
		if err := wTx.Delete(k); err != nil {
			return err
		}
	}
	return nil
}

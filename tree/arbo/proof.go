package arbo

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"math"

	"go.vocdoni.io/dvote/db"
)

// GenProof generates a MerkleTree proof for the given key. The leaf value is
// returned, together with the packed siblings of the proof, and a boolean
// parameter that indicates if the proof is of existence (true) or not (false).
func (t *Tree) GenProof(k []byte) ([]byte, []byte, []byte, bool, error) {
	return t.GenProofWithTx(t.db, k)
}

// GenProofWithTx does the same than the GenProof method, but allowing to pass
// the db.ReadTx that is used.
func (t *Tree) GenProofWithTx(rTx db.Reader, k []byte) ([]byte, []byte, []byte, bool, error) {
	keyPath, err := keyPathFromKey(t.maxLevels, k)
	if err != nil {
		return nil, nil, nil, false, err
	}
	path := getPath(t.maxLevels, keyPath)

	root, err := t.RootWithTx(rTx)
	if err != nil {
		return nil, nil, nil, false, err
	}

	// go down to the leaf
	var siblings, intermediates [][]byte

	_, value, siblings, err := t.down(rTx, k, root, siblings, &intermediates, path, 0, true)
	if err != nil {
		return nil, nil, nil, false, err
	}

	s, err := PackSiblings(t.hashFunction, siblings)
	if err != nil {
		return nil, nil, nil, false, err
	}

	leafK, leafV := ReadLeafValue(value)
	if !bytes.Equal(k, leafK) {
		// key not in tree, proof of non-existence
		return leafK, leafV, s, false, nil
	}

	return leafK, leafV, s, true, nil
}

// PackSiblings packs the siblings into a byte array.
// [    2 byte   |     2 byte        | L bytes |      S * N bytes    ]
// [ full length | bitmap length (L) |  bitmap | N non-zero siblings ]
// Where the bitmap indicates if the sibling is 0 or a value from the siblings
// array. And S is the size of the output of the hash function used for the
// Tree. The 2 2-byte that define the full length and bitmap length, are
// encoded in little-endian.
func PackSiblings(hashFunc HashFunction, siblings [][]byte) ([]byte, error) {
	var b []byte
	var bitmap []bool
	emptySibling := make([]byte, hashFunc.Len())
	for i := 0; i < len(siblings); i++ {
		if bytes.Equal(siblings[i], emptySibling) {
			bitmap = append(bitmap, false)
		} else {
			bitmap = append(bitmap, true)
			b = append(b, siblings[i]...)
		}
	}

	bitmapBytes := bitmapToBytes(bitmap)
	l := len(bitmapBytes)
	if l > maxUint16 {
		return nil, fmt.Errorf("PackSiblings: bitmapBytes length > %v", maxUint16)
	}

	fullLen := 4 + l + len(b)
	if fullLen > maxUint16 {
		return nil, fmt.Errorf("PackSiblings: fullLen > %v", maxUint16)
	}
	res := make([]byte, fullLen)
	binary.LittleEndian.PutUint16(res[0:2], uint16(fullLen)) // set full length
	binary.LittleEndian.PutUint16(res[2:4], uint16(l))       // set the bitmapBytes length
	copy(res[4:4+l], bitmapBytes)
	copy(res[4+l:], b)
	return res, nil
}

// UnpackSiblings unpacks the siblings from a byte array.
func UnpackSiblings(hashFunc HashFunction, b []byte) ([][]byte, error) {
	// to prevent runtime slice out of bounds error check if the length of the
	// rest of the slice is at least equal to the encoded full length value
	if len(b) < 4 {
		return nil, fmt.Errorf("no packed siblings provided")
	}

	fullLen := binary.LittleEndian.Uint16(b[0:2])
	if len(b) != int(fullLen) {
		return nil, fmt.Errorf("expected len: %d, current len: %d", fullLen, len(b))
	}

	l := binary.LittleEndian.Uint16(b[2:4]) // bitmap bytes length
	// to prevent runtime slice out of bounds error check if the length of the
	// rest of the slice is at least equal to the encoded bitmap length value
	if len(b) < int(4+l) {
		return nil, fmt.Errorf("expected len: %d, current len: %d", 4+l, len(b))
	}
	bitmapBytes := b[4 : 4+l]
	bitmap := bytesToBitmap(bitmapBytes)
	siblingsBytes := b[4+l:]
	// to prevent a runtime slice out of bounds error, check if the length of
	// the siblings slice is a multiple of hashFunc length, because the
	// following loop will iterate over it in steps of that length.
	if len(siblingsBytes)%hashFunc.Len() != 0 {
		return nil, fmt.Errorf("bad formated siblings")
	}
	iSibl := 0
	emptySibl := make([]byte, hashFunc.Len())
	var siblings [][]byte
	for i := 0; i < len(bitmap); i++ {
		if iSibl >= len(siblingsBytes) {
			break
		}
		if bitmap[i] {
			siblings = append(siblings, siblingsBytes[iSibl:iSibl+hashFunc.Len()])
			iSibl += hashFunc.Len()
		} else {
			siblings = append(siblings, emptySibl)
		}
	}
	return siblings, nil
}

func bitmapToBytes(bitmap []bool) []byte {
	bitmapBytesLen := int(math.Ceil(float64(len(bitmap)) / 8))
	b := make([]byte, bitmapBytesLen)
	for i := 0; i < len(bitmap); i++ {
		if bitmap[i] {
			b[i/8] |= 1 << (i % 8)
		}
	}
	return b
}

func bytesToBitmap(b []byte) []bool {
	var bitmap []bool
	for i := 0; i < len(b); i++ {
		for j := 0; j < 8; j++ {
			bitmap = append(bitmap, b[i]&(1<<j) > 0)
		}
	}
	return bitmap
}

// CheckProof verifies the given proof. The proof verification depends on the
// HashFunction passed as parameter.
func CheckProof(hashFunc HashFunction, k, v, root, packedSiblings []byte) (bool, error) {
	siblings, err := UnpackSiblings(hashFunc, packedSiblings)
	if err != nil {
		return false, err
	}

	keyPath := make([]byte, int(math.Ceil(float64(len(siblings))/float64(8))))
	copy(keyPath, k)

	key, _, err := newLeafValue(hashFunc, k, v)
	if err != nil {
		return false, err
	}

	path := getPath(len(siblings), keyPath)
	for i := len(siblings) - 1; i >= 0; i-- {
		if path[i] {
			key, _, err = newIntermediate(hashFunc, siblings[i], key)
			if err != nil {
				return false, err
			}
		} else {
			key, _, err = newIntermediate(hashFunc, key, siblings[i])
			if err != nil {
				return false, err
			}
		}
	}
	if bytes.Equal(key, root) {
		return true, nil
	}
	return false, nil
}

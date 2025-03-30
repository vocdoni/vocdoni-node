package arbo

import (
	"bytes"
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"math"
	"slices"

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
// Returns nil if the proof is valid, or an error otherwise.
func CheckProof(hashFunc HashFunction, k, v, root, packedSiblings []byte) error {
	hashes, err := CalculateProofNodes(hashFunc, k, v, packedSiblings, nil, false)
	if err != nil {
		return err
	}
	if !bytes.Equal(hashes[0], root) {
		return fmt.Errorf("calculated vs expected root mismatch")
	}
	return nil
}

// CalculateProofNodes calculates the chain of hashes in the path of the given proof.
// In the returned list, first item is the root, and last item is the hash of the leaf.
func CalculateProofNodes(hashFunc HashFunction, k, v, packedSiblings, oldKey []byte, exclusion bool) ([][]byte, error) {
	siblings, err := UnpackSiblings(hashFunc, packedSiblings)
	if err != nil {
		return nil, err
	}

	keyPath := make([]byte, int(math.Ceil(float64(len(siblings))/float64(8))))
	copy(keyPath, k)
	path := getPath(len(siblings), keyPath)

	key := slices.Clone(k)

	if exclusion {
		if slices.Equal(k, oldKey) {
			return nil, fmt.Errorf("exclusion proof invalid, key and oldKey are equal")
		}
		// we'll prove the path to the existing key (passed as oldKey)
		key = slices.Clone(oldKey)
	}

	hash, _, err := newLeafValue(hashFunc, key, v)
	if err != nil {
		return nil, err
	}
	hashes := [][]byte{hash}
	for i, sibling := range slices.Backward(siblings) {
		if path[i] {
			hash, _, err = newIntermediate(hashFunc, sibling, hash)
		} else {
			hash, _, err = newIntermediate(hashFunc, hash, sibling)
		}
		if err != nil {
			return nil, err
		}
		hashes = append(hashes, hash)
	}
	slices.Reverse(hashes)
	return hashes, nil
}

// CheckProofBatch verifies a batch of N proofs pairs (old and new). The proof verification depends on the
// HashFunction passed as parameter.
// Returns nil if the batch is valid, or an error otherwise.
//
// TODO: doesn't support removing leaves (newProofs can only update or add new leaves)
func CheckProofBatch(hashFunc HashFunction, oldProofs, newProofs []*CircomVerifierProof) error {
	newBranches := make(map[string]int)
	newSiblings := make(map[string]int)

	if len(oldProofs) != len(newProofs) {
		return fmt.Errorf("batch of proofs incomplete")
	}

	if len(oldProofs) == 0 {
		return fmt.Errorf("empty batch")
	}

	for i := range oldProofs {
		// Map all old branches
		oldNodes, err := oldProofs[i].CalculateProofNodes(hashFunc)
		if err != nil {
			return fmt.Errorf("old proof invalid: %w", err)
		}
		// and check they are valid
		if !bytes.Equal(oldProofs[i].Root, oldNodes[0]) {
			return fmt.Errorf("old proof invalid: root doesn't match")
		}

		// Map all new branches
		newNodes, err := newProofs[i].CalculateProofNodes(hashFunc)
		if err != nil {
			return fmt.Errorf("new proof invalid: %w", err)
		}
		// and check they are valid
		if !bytes.Equal(newProofs[i].Root, newNodes[0]) {
			return fmt.Errorf("new proof invalid: root doesn't match")
		}

		for level, hash := range newNodes {
			newBranches[hex.EncodeToString(hash)] = level
		}

		for level := range newProofs[i].Siblings {
			if !slices.Equal(oldProofs[i].Siblings[level], newProofs[i].Siblings[level]) {
				// since in newBranch the root is level 0, we shift siblings to level + 1
				newSiblings[hex.EncodeToString(newProofs[i].Siblings[level])] = level + 1
			}
		}
	}

	for hash, level := range newSiblings {
		if newBranches[hash] != newSiblings[hash] {
			return fmt.Errorf("sibling %s (at level %d) changed but there's no proof why", hash, level)
		}
	}

	return nil
}

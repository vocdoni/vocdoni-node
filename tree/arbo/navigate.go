package arbo

import (
	"bytes"
	"encoding/hex"
	"fmt"

	"go.vocdoni.io/dvote/db"
)

// down goes down to the leaf recursively
func (t *Tree) down(rTx db.Reader, newKey, currKey []byte, siblings [][]byte, intermediates [][]byte,
	path []bool, currLvl int, getLeaf bool) ([]byte, []byte, [][]byte, [][]byte, error) {
	if currLvl > t.maxLevels {
		return nil, nil, nil, nil, ErrMaxLevel
	}

	var err error
	var currValue []byte
	if bytes.Equal(currKey, t.emptyHash) {
		// empty value
		return currKey, emptyValue, siblings, intermediates, nil
	}
	currValue, err = rTx.Get(currKey)
	if err != nil {
		return nil, nil, nil, nil, fmt.Errorf("could not get value for key %x: %w", currKey, err)
	}

	switch currValue[0] {
	case PrefixValueEmpty: // empty
		fmt.Printf("newKey: %s, currKey: %s, currLvl: %d, currValue: %s\n",
			hex.EncodeToString(newKey), hex.EncodeToString(currKey),
			currLvl, hex.EncodeToString(currValue))
		panic("This point should not be reached, as the 'if currKey==t.emptyHash'" +
			" above should avoid reaching this point. This panic is temporary" +
			" for reporting purposes, will be deleted in future versions." +
			" Please paste this log (including the previous log lines) in a" +
			" new issue: https://go.vocdoni.io/dvote/tree/arbo/issues/new") // TMP
	case PrefixValueLeaf: // leaf
		if !bytes.Equal(currValue, emptyValue) {
			if getLeaf {
				return currKey, currValue, siblings, intermediates, nil
			}
			oldLeafKey, _ := ReadLeafValue(currValue)
			if bytes.Equal(newKey, oldLeafKey) {
				return nil, nil, nil, nil, ErrKeyAlreadyExists
			}

			oldLeafKeyFull, err := keyPathFromKey(t.maxLevels, oldLeafKey)
			if err != nil {
				return nil, nil, nil, nil, err
			}

			// if currKey is already used, go down until paths diverge
			oldPath := getPath(t.maxLevels, oldLeafKeyFull)
			siblings, err = t.downVirtually(siblings, currKey, newKey, oldPath, path, currLvl)
			if err != nil {
				return nil, nil, nil, nil, err
			}
		}
		return currKey, currValue, siblings, intermediates, nil
	case PrefixValueIntermediate: // intermediate
		if len(currValue) != PrefixValueLen+t.hashFunction.Len()*2 {
			return nil, nil, nil, nil,
				fmt.Errorf("intermediate value invalid length (expected: %d, actual: %d)",
					PrefixValueLen+t.hashFunction.Len()*2, len(currValue))
		}
		intermediates[currLvl] = currKey

		// collect siblings while going down
		if path[currLvl] {
			// right
			lChild, rChild := ReadIntermediateChilds(currValue)
			siblings = append(siblings, lChild)
			return t.down(rTx, newKey, rChild, siblings, intermediates, path, currLvl+1, getLeaf)
		}
		// left
		lChild, rChild := ReadIntermediateChilds(currValue)
		siblings = append(siblings, rChild)
		return t.down(rTx, newKey, lChild, siblings, intermediates, path, currLvl+1, getLeaf)
	default:
		return nil, nil, nil, nil, ErrInvalidValuePrefix
	}
}

// up navigates back up the tree after a delete operation, updating
// the intermediate nodes and potentially removing nodes that are no longer needed.
func (t *Tree) up(wTx db.WriteTx, intermediates [][]byte, newKey []byte, siblings [][]byte, path []bool, currLvl, toLvl int) ([]byte, error) {
	if currLvl < 0 {
		return newKey, nil
	}

	var k, v []byte
	var err error
	if path[currLvl+toLvl] {
		k, v, err = t.newIntermediate(siblings[currLvl], newKey)
	} else {
		k, v, err = t.newIntermediate(newKey, siblings[currLvl])
	}

	if err != nil {
		return nil, fmt.Errorf("could not compute intermediary node: %w", err)
	}

	// If the new key is not the empty node, store it in the database
	// extra empty childs are stored. find the way to remove them
	if err = wTx.Set(k, v); err != nil {
		return nil, err
	}

	// If the node is modified, remove the old key
	oldKey := intermediates[currLvl]
	if !bytes.Equal(oldKey, k) && oldKey != nil {
		fmt.Printf("Removing old key: %s\n", hex.EncodeToString(oldKey))
		if err := wTx.Delete(oldKey); err != nil {
			return nil, fmt.Errorf("could not delete old key: %w", err)
		}
	}

	return t.up(wTx, intermediates, k, siblings, path, currLvl-1, toLvl)
}

// newIntermediate takes the left & right keys of a intermediate node, and
// computes its hash. Returns the hash of the node, which is the node key, and a
// byte array that contains the value (which contains the left & right child
// keys) to store in the DB.
// [     1 byte   |     1 byte         | N bytes  |  N bytes  ]
// [ type of node | length of left key | left key | right key ]
func newIntermediate(hashFunc HashFunction, l, r []byte) ([]byte, []byte, error) {
	b := make([]byte, PrefixValueLen+hashFunc.Len()*2)
	b[0] = PrefixValueIntermediate
	if len(l) > maxUint8 {
		return nil, nil, fmt.Errorf("newIntermediate: len(l) > %v", maxUint8)
	}
	b[1] = byte(len(l))
	copy(b[PrefixValueLen:PrefixValueLen+hashFunc.Len()], l)
	copy(b[PrefixValueLen+hashFunc.Len():], r)

	key, err := hashFunc.Hash(l, r)
	if err != nil {
		return nil, nil, err
	}

	return key, b, nil
}

// ReadIntermediateChilds reads from a byte array the two childs keys
func ReadIntermediateChilds(b []byte) ([]byte, []byte) {
	if len(b) < PrefixValueLen {
		return []byte{}, []byte{}
	}

	lLen := b[1]
	if len(b) < PrefixValueLen+int(lLen) {
		return []byte{}, []byte{}
	}
	l := b[PrefixValueLen : PrefixValueLen+lLen]
	r := b[PrefixValueLen+lLen:]
	return l, r
}

func getPath(numLevels int, k []byte) []bool {
	path := make([]bool, numLevels)
	for n := 0; n < numLevels; n++ {
		path[n] = k[n/8]&(1<<(n%8)) != 0
	}
	return path
}

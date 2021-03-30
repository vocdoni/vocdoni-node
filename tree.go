/*
Package arbo implements a Merkle Tree compatible with the circomlib
implementation of the MerkleTree (when using the Poseidon hash function),
following the specification from
https://docs.iden3.io/publications/pdfs/Merkle-Tree.pdf and
https://eprint.iacr.org/2018/955.

Also allows to define which hash function to use. So for example, when working
with zkSnarks the Poseidon hash function can be used, but when not, it can be
used the Blake3 hash function, which improves the computation time.
*/
package arbo

import (
	"bytes"
	"fmt"
	"sync/atomic"
	"time"

	"github.com/iden3/go-merkletree/db"
)

const (
	// PrefixValueLen defines the bytes-prefix length used for the Value
	// bytes representation stored in the db
	PrefixValueLen = 2

	// PrefixValueEmpty is used for the first byte of a Value to indicate
	// that is an Empty value
	PrefixValueEmpty = 0
	// PrefixValueLeaf is used for the first byte of a Value to indicate
	// that is a Leaf value
	PrefixValueLeaf = 1
	// PrefixValueIntermediate is used for the first byte of a Value to
	// indicate that is a Intermediate value
	PrefixValueIntermediate = 2
)

var (
	dbKeyRoot  = []byte("root")
	emptyValue = []byte{0}
)

// Tree defines the struct that implements the MerkleTree functionalities
type Tree struct {
	db         db.Storage
	lastAccess int64 // in unix time
	maxLevels  int
	root       []byte

	hashFunction HashFunction
}

// NewTree returns a new Tree, if there is a Tree still in the given storage, it
// will load it.
func NewTree(storage db.Storage, maxLevels int, hash HashFunction) (*Tree, error) {
	t := Tree{db: storage, maxLevels: maxLevels, hashFunction: hash}

	t.updateAccessTime()
	root, err := t.db.Get(dbKeyRoot)
	if err == db.ErrNotFound {
		// store new root 0
		tx, err := t.db.NewTx()
		if err != nil {
			return nil, err
		}
		t.root = make([]byte, t.hashFunction.Len()) // empty
		err = tx.Put(dbKeyRoot, t.root)
		if err != nil {
			return nil, err
		}
		err = tx.Commit()
		if err != nil {
			return nil, err
		}
		return &t, err
	} else if err != nil {
		return nil, err
	}
	t.root = root
	return &t, nil
}

func (t *Tree) updateAccessTime() {
	atomic.StoreInt64(&t.lastAccess, time.Now().Unix())
}

// LastAccess returns the last access timestamp in Unixtime
func (t *Tree) LastAccess() int64 {
	return atomic.LoadInt64(&t.lastAccess)
}

// Root returns the root of the Tree
func (t *Tree) Root() []byte {
	return t.root
}

// AddBatch adds a batch of key-values to the Tree. This method is optimized to
// do some internal parallelization. Returns an array containing the indexes of
// the keys failed to add.
func (t *Tree) AddBatch(keys, values [][]byte) ([]int, error) {
	return nil, fmt.Errorf("unimplemented")
}

// Add inserts the key-value into the Tree.
// If the inputs come from a *big.Int, is expected that are represented by a
// Little-Endian byte array (for circom compatibility).
func (t *Tree) Add(k, v []byte) error {
	// TODO check validity of key & value (for the Tree.HashFunction type)

	keyPath := make([]byte, t.hashFunction.Len())
	copy(keyPath[:], k)

	path := getPath(t.maxLevels, keyPath)
	// go down to the leaf
	var siblings [][]byte
	_, _, siblings, err := t.down(k, t.root, siblings, path, 0)
	if err != nil {
		return err
	}

	leafKey, leafValue, err := t.newLeafValue(k, v)
	if err != nil {
		return err
	}

	tx, err := t.db.NewTx()
	if err != nil {
		return err
	}
	if err := tx.Put(leafKey, leafValue); err != nil {
		return err
	}

	// go up to the root
	if len(siblings) == 0 {
		t.root = leafKey
		return tx.Commit()
	}
	root, err := t.up(tx, leafKey, siblings, path, len(siblings)-1)
	if err != nil {
		return err
	}

	t.root = root
	// store root to db
	return tx.Commit()
}

// down goes down to the leaf recursively
func (t *Tree) down(newKey, currKey []byte, siblings [][]byte, path []bool, l int) (
	[]byte, []byte, [][]byte, error) {
	if l > t.maxLevels-1 {
		return nil, nil, nil, fmt.Errorf("max level")
	}
	var err error
	var currValue []byte
	emptyKey := make([]byte, t.hashFunction.Len())
	if bytes.Equal(currKey, emptyKey) {
		// empty value
		return currKey, emptyValue, siblings, nil
	}
	currValue, err = t.db.Get(currKey)
	if err != nil {
		return nil, nil, nil, err
	}

	switch currValue[0] {
	case PrefixValueEmpty: // empty
		// TODO WIP WARNING should not be reached, as the 'if' above should avoid
		// reaching this point
		// return currKey, empty, siblings, nil
		panic("should not be reached, as the 'if' above should avoid reaching this point") // TMP
	case PrefixValueLeaf: // leaf
		if bytes.Equal(newKey, currKey) {
			return nil, nil, nil, fmt.Errorf("key already exists")
		}

		if !bytes.Equal(currValue, emptyValue) {
			oldLeafKey, _ := readLeafValue(currValue)
			oldLeafKeyFull := make([]byte, t.hashFunction.Len())
			copy(oldLeafKeyFull[:], oldLeafKey)

			// if currKey is already used, go down until paths diverge
			oldPath := getPath(t.maxLevels, oldLeafKeyFull)
			siblings, err = t.downVirtually(siblings, currKey, newKey, oldPath, path, l)
			if err != nil {
				return nil, nil, nil, err
			}
		}
		return currKey, currValue, siblings, nil
	case PrefixValueIntermediate: // intermediate
		if len(currValue) != PrefixValueLen+t.hashFunction.Len()*2 {
			return nil, nil, nil,
				fmt.Errorf("intermediate value invalid length (expected: %d, actual: %d)",
					PrefixValueLen+t.hashFunction.Len()*2, len(currValue))
		}
		// collect siblings while going down
		if path[l] {
			// right
			lChild, rChild := readIntermediateChilds(currValue)
			siblings = append(siblings, lChild)
			return t.down(newKey, rChild, siblings, path, l+1)
		}
		// left
		lChild, rChild := readIntermediateChilds(currValue)
		siblings = append(siblings, rChild)
		return t.down(newKey, lChild, siblings, path, l+1)
	default:
		return nil, nil, nil, fmt.Errorf("invalid value")
	}
}

// downVirtually is used when in a leaf already exists, and a new leaf which
// shares the path until the existing leaf is being added
func (t *Tree) downVirtually(siblings [][]byte, oldKey, newKey []byte, oldPath,
	newPath []bool, l int) ([][]byte, error) {
	var err error
	if l > t.maxLevels-1 {
		return nil, fmt.Errorf("max virtual level %d", l)
	}

	if oldPath[l] == newPath[l] {
		emptyKey := make([]byte, t.hashFunction.Len()) // empty
		siblings = append(siblings, emptyKey)

		siblings, err = t.downVirtually(siblings, oldKey, newKey, oldPath, newPath, l+1)
		if err != nil {
			return nil, err
		}
		return siblings, nil
	}
	// reached the divergence
	siblings = append(siblings, oldKey)

	return siblings, nil
}

// up goes up recursively updating the intermediate nodes
func (t *Tree) up(tx db.Tx, key []byte, siblings [][]byte, path []bool, l int) ([]byte, error) {
	var k, v []byte
	var err error
	if path[l] {
		k, v, err = t.newIntermediate(siblings[l], key)
		if err != nil {
			return nil, err
		}
	} else {
		k, v, err = t.newIntermediate(key, siblings[l])
		if err != nil {
			return nil, err
		}
	}
	// store k-v to db
	err = tx.Put(k, v)
	if err != nil {
		return nil, err
	}

	if l == 0 {
		// reached the root
		return k, nil
	}

	return t.up(tx, k, siblings, path, l-1)
}

func (t *Tree) newLeafValue(k, v []byte) ([]byte, []byte, error) {
	leafKey, err := t.hashFunction.Hash(k, v, []byte{1})
	if err != nil {
		return nil, nil, err
	}
	var leafValue []byte
	leafValue = append(leafValue, byte(1))
	leafValue = append(leafValue, byte(len(k)))
	leafValue = append(leafValue, k...)
	leafValue = append(leafValue, v...)
	return leafKey, leafValue, nil
}

func readLeafValue(b []byte) ([]byte, []byte) {
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

func (t *Tree) newIntermediate(l, r []byte) ([]byte, []byte, error) {
	b := make([]byte, PrefixValueLen+t.hashFunction.Len()*2)
	b[0] = 2
	b[1] = byte(len(l))
	copy(b[PrefixValueLen:PrefixValueLen+t.hashFunction.Len()], l)
	copy(b[PrefixValueLen+t.hashFunction.Len():], r)

	key, err := t.hashFunction.Hash(l, r)
	if err != nil {
		return nil, nil, err
	}

	return key, b, nil
}

func readIntermediateChilds(b []byte) ([]byte, []byte) {
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

// GenProof generates a MerkleTree proof for the given key. If the key exists in
// the Tree, the proof will be of existence, if the key does not exist in the
// tree, the proof will be of non-existence.
func (t *Tree) GenProof(k, v []byte) ([]byte, error) {
	// unimplemented
	return nil, fmt.Errorf("unimplemented")
}

// Get returns the value for a given key
func (t *Tree) Get(k []byte) ([]byte, []byte, error) {
	// unimplemented
	return nil, nil, fmt.Errorf("unimplemented")
}

// CheckProof verifies the given proof
func CheckProof(k, v, root, mproof []byte) (bool, error) {
	// unimplemented
	return false, fmt.Errorf("unimplemented")
}

// TODO method to export & import the full Tree without values

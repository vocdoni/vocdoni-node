/*
Package arbo implements a Merkle Tree compatible with the circomlib
implementation of the MerkleTree, following the specification from
https://docs.iden3.io/publications/pdfs/Merkle-Tree.pdf and
https://eprint.iacr.org/2018/955.

Allows to define which hash function to use. So for example, when working with
zkSnarks the Poseidon hash function can be used, but when not, it can be used
the Blake2b hash function, which has much faster computation time.
*/
package arbo

import (
	"bytes"
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"io"
	"math"
	"sync"

	"go.vocdoni.io/dvote/db"
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

	// nChars is used to crop the Graphviz nodes labels
	nChars = 4
)

var (
	dbKeyRoot   = []byte("root")
	dbKeyNLeafs = []byte("nleafs")
	emptyValue  = []byte{0}

	// ErrKeyNotFound is used when a key is not found in the db neither in
	// the current db Batch.
	ErrKeyNotFound = fmt.Errorf("key not found")
	// ErrKeyAlreadyExists is used when trying to add a key as leaf to the
	// tree that already exists.
	ErrKeyAlreadyExists = fmt.Errorf("key already exists")
	// ErrInvalidValuePrefix is used when going down into the tree, a value
	// is read from the db and has an unrecognized prefix.
	ErrInvalidValuePrefix = fmt.Errorf("invalid value prefix")
	// ErrDBNoTx is used when trying to use Tree.dbPut but Tree.dbBatch==nil
	ErrDBNoTx = fmt.Errorf("dbPut error: no db Batch")
	// ErrMaxLevel indicates when going down into the tree, the max level is
	// reached
	ErrMaxLevel = fmt.Errorf("max level reached")
	// ErrMaxVirtualLevel indicates when going down into the tree, the max
	// virtual level is reached
	ErrMaxVirtualLevel = fmt.Errorf("max virtual level reached")
	// ErrSnapshotNotEditable indicates when the tree is a snapshot, thus
	// can not be modified
	ErrSnapshotNotEditable = fmt.Errorf("snapshot tree can not be edited")
	// ErrTreeNotEmpty indicates when the tree was expected to be empty and
	// it is not
	ErrTreeNotEmpty = fmt.Errorf("tree is not empty")
)

// Tree defines the struct that implements the MerkleTree functionalities
type Tree struct {
	sync.RWMutex

	db           db.Database
	maxLevels    int
	snapshotRoot []byte

	hashFunction HashFunction
	// TODO in the methods that use it, check if emptyHash param is len>0
	// (check if it has been initialized)
	emptyHash []byte

	dbg *dbgStats
}

// NewTree returns a new Tree, if there is a Tree still in the given database, it
// will load it.
func NewTree(database db.Database, maxLevels int, hash HashFunction) (*Tree, error) {
	wTx := database.WriteTx()
	defer wTx.Discard()

	t, err := NewTreeWithTx(wTx, database, maxLevels, hash)
	if err != nil {
		return nil, err
	}

	if err = wTx.Commit(); err != nil {
		return nil, err
	}
	return t, nil
}

// NewTreeWithTx returns a new Tree using the given db.WriteTx, which will not
// be ccommited inside this method, if there is a Tree still in the given
// database, it will load it.
func NewTreeWithTx(wTx db.WriteTx, database db.Database,
	maxLevels int, hash HashFunction) (*Tree, error) {
	t := Tree{db: database, maxLevels: maxLevels, hashFunction: hash}
	t.emptyHash = make([]byte, t.hashFunction.Len()) // empty

	_, err := wTx.Get(dbKeyRoot)
	if err == db.ErrKeyNotFound {
		// store new root 0 (empty)
		if err = wTx.Set(dbKeyRoot, t.emptyHash); err != nil {
			return nil, err
		}
		if err = t.setNLeafs(wTx, 0); err != nil {
			return nil, err
		}
		return &t, nil
	} else if err != nil {
		return nil, err
	}
	return &t, nil
}

// Root returns the root of the Tree
func (t *Tree) Root() ([]byte, error) {
	rTx := t.db.ReadTx()
	defer rTx.Discard()
	return t.RootWithTx(rTx)
}

// RootWithTx returns the root of the Tree using the given db.ReadTx
func (t *Tree) RootWithTx(rTx db.ReadTx) ([]byte, error) {
	// if snapshotRoot is defined, means that the tree is a snapshot, and
	// the root is not obtained from the db, but from the snapshotRoot
	// parameter
	if t.snapshotRoot != nil {
		return t.snapshotRoot, nil
	}
	// get db root
	return rTx.Get(dbKeyRoot)
}

func (t *Tree) setRoot(wTx db.WriteTx, root []byte) error {
	return wTx.Set(dbKeyRoot, root)
}

// HashFunction returns Tree.hashFunction
func (t *Tree) HashFunction() HashFunction {
	return t.hashFunction
}

// editable returns true if the tree is editable, and false when is not
// editable (because is a snapshot tree)
func (t *Tree) editable() bool {
	return t.snapshotRoot == nil
}

// AddBatch adds a batch of key-values to the Tree. Returns an array containing
// the indexes of the keys failed to add. Supports empty values as input
// parameters, which is equivalent to 0 valued byte array.
func (t *Tree) AddBatch(keys, values [][]byte) ([]int, error) {
	wTx := t.db.WriteTx()
	defer wTx.Discard()

	invalids, err := t.AddBatchWithTx(wTx, keys, values)
	if err != nil {
		return invalids, err
	}
	return invalids, wTx.Commit()
}

// AddBatchWithTx does the same than the AddBatch method, but allowing to pass
// the db.WriteTx that is used. The db.WriteTx will not be committed inside
// this method.
func (t *Tree) AddBatchWithTx(wTx db.WriteTx, keys, values [][]byte) ([]int, error) {
	t.Lock()
	defer t.Unlock()

	if !t.editable() {
		return nil, ErrSnapshotNotEditable
	}

	vt, err := t.loadVT(wTx)
	if err != nil {
		return nil, err
	}

	e := []byte{}
	// equal the number of keys & values
	if len(keys) > len(values) {
		// add missing values
		for i := len(values); i < len(keys); i++ {
			values = append(values, e)
		}
	} else if len(keys) < len(values) {
		// crop extra values
		values = values[:len(keys)]
	}

	invalids, err := vt.addBatch(keys, values)
	if err != nil {
		return nil, err
	}

	// once the VirtualTree is build, compute the hashes
	pairs, err := vt.computeHashes()
	if err != nil {
		// currently invalids in computeHashes are not counted,
		// but should not be needed, as if there is an error there is
		// nothing stored in the db and the error is returned
		return nil, err
	}

	// store pairs in db
	for i := 0; i < len(pairs); i++ {
		if err := wTx.Set(pairs[i][0], pairs[i][1]); err != nil {
			return nil, err
		}
	}

	// store root (from the vt) to db
	if err := wTx.Set(dbKeyRoot, vt.root.h); err != nil {
		return nil, err
	}

	// update nLeafs
	if err := t.incNLeafs(wTx, len(keys)-len(invalids)); err != nil {
		return nil, err
	}

	return invalids, nil
}

// loadVT loads a new virtual tree (vt) from the current Tree, which contains
// the same leafs.
func (t *Tree) loadVT(rTx db.ReadTx) (vt, error) {
	vt := newVT(t.maxLevels, t.hashFunction)
	vt.params.dbg = t.dbg
	err := t.IterateWithTx(rTx, nil, func(k, v []byte) {
		if v[0] != PrefixValueLeaf {
			return
		}
		leafK, leafV := ReadLeafValue(v)
		if err := vt.add(0, leafK, leafV); err != nil {
			// TODO instead of panic, return this error
			panic(err)
		}
	})

	return vt, err
}

// Add inserts the key-value into the Tree.  If the inputs come from a
// *big.Int, is expected that are represented by a Little-Endian byte array
// (for circom compatibility).
func (t *Tree) Add(k, v []byte) error {
	wTx := t.db.WriteTx()
	defer wTx.Discard()

	if err := t.AddWithTx(wTx, k, v); err != nil {
		return err
	}

	return wTx.Commit()
}

// AddWithTx does the same than the Add method, but allowing to pass the
// db.WriteTx that is used. The db.WriteTx will not be committed inside this
// method.
func (t *Tree) AddWithTx(wTx db.WriteTx, k, v []byte) error {
	t.Lock()
	defer t.Unlock()

	if !t.editable() {
		return ErrSnapshotNotEditable
	}

	root, err := t.RootWithTx(wTx)
	if err != nil {
		return err
	}

	root, err = t.add(wTx, root, 0, k, v) // add from level 0
	if err != nil {
		return err
	}
	// store root to db
	if err := t.setRoot(wTx, root); err != nil {
		return err
	}
	// update nLeafs
	if err = t.incNLeafs(wTx, 1); err != nil {
		return err
	}
	return nil
}

func (t *Tree) add(wTx db.WriteTx, root []byte, fromLvl int, k, v []byte) ([]byte, error) {
	keyPath := make([]byte, t.hashFunction.Len())
	copy(keyPath[:], k)

	path := getPath(t.maxLevels, keyPath)
	// go down to the leaf
	var siblings [][]byte
	_, _, siblings, err := t.down(wTx, k, root, siblings, path, fromLvl, false)
	if err != nil {
		return nil, err
	}

	leafKey, leafValue, err := t.newLeafValue(k, v)
	if err != nil {
		return nil, err
	}

	if err := wTx.Set(leafKey, leafValue); err != nil {
		return nil, err
	}

	// go up to the root
	if len(siblings) == 0 {
		// return the leafKey as root
		return leafKey, nil
	}
	root, err = t.up(wTx, leafKey, siblings, path, len(siblings)-1, fromLvl)
	if err != nil {
		return nil, err
	}

	return root, nil
}

// down goes down to the leaf recursively
func (t *Tree) down(rTx db.ReadTx, newKey, currKey []byte, siblings [][]byte,
	path []bool, currLvl int, getLeaf bool) (
	[]byte, []byte, [][]byte, error) {
	if currLvl > t.maxLevels-1 {
		return nil, nil, nil, ErrMaxLevel
	}

	var err error
	var currValue []byte
	if bytes.Equal(currKey, t.emptyHash) {
		// empty value
		return currKey, emptyValue, siblings, nil
	}
	currValue, err = rTx.Get(currKey)
	if err != nil {
		return nil, nil, nil, err
	}

	switch currValue[0] {
	case PrefixValueEmpty: // empty
		fmt.Printf("newKey: %s, currKey: %s, currLvl: %d, currValue: %s\n",
			hex.EncodeToString(newKey), hex.EncodeToString(currKey),
			currLvl, hex.EncodeToString(currValue))
		panic("This point should not be reached, as the 'if' above" +
			" should avoid reaching this point. This panic is temporary" +
			" for reporting purposes, will be deleted in future versions." +
			" Please paste this log (including the previous lines) in a" +
			" new issue: https://github.com/vocdoni/arbo/issues/new") // TMP
	case PrefixValueLeaf: // leaf
		if !bytes.Equal(currValue, emptyValue) {
			if getLeaf {
				return currKey, currValue, siblings, nil
			}
			oldLeafKey, _ := ReadLeafValue(currValue)
			if bytes.Equal(newKey, oldLeafKey) {
				return nil, nil, nil, ErrKeyAlreadyExists
			}

			oldLeafKeyFull := make([]byte, t.hashFunction.Len())
			copy(oldLeafKeyFull[:], oldLeafKey)

			// if currKey is already used, go down until paths diverge
			oldPath := getPath(t.maxLevels, oldLeafKeyFull)
			siblings, err = t.downVirtually(siblings, currKey, newKey, oldPath, path, currLvl)
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
		if path[currLvl] {
			// right
			lChild, rChild := ReadIntermediateChilds(currValue)
			siblings = append(siblings, lChild)
			return t.down(rTx, newKey, rChild, siblings, path, currLvl+1, getLeaf)
		}
		// left
		lChild, rChild := ReadIntermediateChilds(currValue)
		siblings = append(siblings, rChild)
		return t.down(rTx, newKey, lChild, siblings, path, currLvl+1, getLeaf)
	default:
		return nil, nil, nil, ErrInvalidValuePrefix
	}
}

// downVirtually is used when in a leaf already exists, and a new leaf which
// shares the path until the existing leaf is being added
func (t *Tree) downVirtually(siblings [][]byte, oldKey, newKey []byte, oldPath,
	newPath []bool, currLvl int) ([][]byte, error) {
	var err error
	if currLvl > t.maxLevels-1 {
		return nil, ErrMaxVirtualLevel
	}

	if oldPath[currLvl] == newPath[currLvl] {
		siblings = append(siblings, t.emptyHash)

		siblings, err = t.downVirtually(siblings, oldKey, newKey, oldPath, newPath, currLvl+1)
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
func (t *Tree) up(wTx db.WriteTx, key []byte, siblings [][]byte, path []bool,
	currLvl, toLvl int) ([]byte, error) {
	var k, v []byte
	var err error
	if path[currLvl+toLvl] {
		k, v, err = t.newIntermediate(siblings[currLvl], key)
		if err != nil {
			return nil, err
		}
	} else {
		k, v, err = t.newIntermediate(key, siblings[currLvl])
		if err != nil {
			return nil, err
		}
	}
	// store k-v to db
	if err = wTx.Set(k, v); err != nil {
		return nil, err
	}

	if currLvl == 0 {
		// reached the root
		return k, nil
	}

	return t.up(wTx, k, siblings, path, currLvl-1, toLvl)
}

func (t *Tree) newLeafValue(k, v []byte) ([]byte, []byte, error) {
	t.dbg.incHash()
	return newLeafValue(t.hashFunction, k, v)
}

// newLeafValue takes a key & value from a leaf, and computes the leaf hash,
// which is used as the leaf key. And the value is the concatenation of the
// inputed key & value. The output of this function is used as key-value to
// store the leaf in the DB.
// [     1 byte   |     1 byte    | N bytes | M bytes ]
// [ type of node | length of key |   key   |  value  ]
func newLeafValue(hashFunc HashFunction, k, v []byte) ([]byte, []byte, error) {
	leafKey, err := hashFunc.Hash(k, v, []byte{1})
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

func (t *Tree) newIntermediate(l, r []byte) ([]byte, []byte, error) {
	t.dbg.incHash()
	return newIntermediate(t.hashFunction, l, r)
}

// newIntermediate takes the left & right keys of a intermediate node, and
// computes its hash. Returns the hash of the node, which is the node key, and a
// byte array that contains the value (which contains the left & right child
// keys) to store in the DB.
// [     1 byte   |     1 byte    | N bytes  |  N bytes  ]
// [ type of node | length of key | left key | right key ]
func newIntermediate(hashFunc HashFunction, l, r []byte) ([]byte, []byte, error) {
	b := make([]byte, PrefixValueLen+hashFunc.Len()*2)
	b[0] = 2
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

// Update updates the value for a given existing key. If the given key does not
// exist, returns an error.
func (t *Tree) Update(k, v []byte) error {
	wTx := t.db.WriteTx()
	defer wTx.Discard()

	if err := t.UpdateWithTx(wTx, k, v); err != nil {
		return err
	}
	return wTx.Commit()
}

// UpdateWithTx does the same than the Update method, but allowing to pass the
// db.WriteTx that is used. The db.WriteTx will not be committed inside this
// method.
func (t *Tree) UpdateWithTx(wTx db.WriteTx, k, v []byte) error {
	t.Lock()
	defer t.Unlock()

	if !t.editable() {
		return ErrSnapshotNotEditable
	}

	var err error

	keyPath := make([]byte, t.hashFunction.Len())
	copy(keyPath[:], k)
	path := getPath(t.maxLevels, keyPath)

	root, err := t.RootWithTx(wTx)
	if err != nil {
		return err
	}

	var siblings [][]byte
	_, valueAtBottom, siblings, err := t.down(wTx, k, root, siblings, path, 0, true)
	if err != nil {
		return err
	}
	oldKey, _ := ReadLeafValue(valueAtBottom)
	if !bytes.Equal(oldKey, k) {
		return ErrKeyNotFound
	}

	leafKey, leafValue, err := t.newLeafValue(k, v)
	if err != nil {
		return err
	}

	if err := wTx.Set(leafKey, leafValue); err != nil {
		return err
	}

	// go up to the root
	if len(siblings) == 0 {
		return t.setRoot(wTx, leafKey)
	}
	root, err = t.up(wTx, leafKey, siblings, path, len(siblings)-1, 0)
	if err != nil {
		return err
	}

	// store root to db
	if err := t.setRoot(wTx, root); err != nil {
		return err
	}
	return nil
}

// GenProof generates a MerkleTree proof for the given key. The leaf value is
// returned, together with the packed siblings of the proof, and a boolean
// parameter that indicates if the proof is of existence (true) or not (false).
func (t *Tree) GenProof(k []byte) ([]byte, []byte, []byte, bool, error) {
	rTx := t.db.ReadTx()
	defer rTx.Discard()

	return t.GenProofWithTx(rTx, k)
}

// GenProofWithTx does the same than the GenProof method, but allowing to pass
// the db.ReadTx that is used.
func (t *Tree) GenProofWithTx(rTx db.ReadTx, k []byte) ([]byte, []byte, []byte, bool, error) {
	keyPath := make([]byte, t.hashFunction.Len())
	copy(keyPath[:], k)

	root, err := t.RootWithTx(rTx)
	if err != nil {
		return nil, nil, nil, false, err
	}

	path := getPath(t.maxLevels, keyPath)
	// go down to the leaf
	var siblings [][]byte
	_, value, siblings, err := t.down(rTx, k, root, siblings, path, 0, true)
	if err != nil {
		return nil, nil, nil, false, err
	}

	s := PackSiblings(t.hashFunction, siblings)

	leafK, leafV := ReadLeafValue(value)
	if !bytes.Equal(k, leafK) {
		// key not in tree, proof of non-existence
		return leafK, leafV, s, false, nil
	}

	return leafK, leafV, s, true, nil
}

// PackSiblings packs the siblings into a byte array.
// [      1 byte        | L bytes |       S * N bytes     ]
// [  bitmap length (L) |  bitmap |  N non-zero siblings  ]
// Where the bitmap indicates if the sibling is 0 or a value from the siblings
// array. And S is the size of the output of the hash function used for the
// Tree.
func PackSiblings(hashFunc HashFunction, siblings [][]byte) []byte {
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

	res := make([]byte, l+1+len(b))
	res[0] = byte(l) // set the bitmapBytes length
	copy(res[1:1+l], bitmapBytes)
	copy(res[1+l:], b)
	return res
}

// UnpackSiblings unpacks the siblings from a byte array.
func UnpackSiblings(hashFunc HashFunction, b []byte) ([][]byte, error) {
	l := b[0]
	bitmapBytes := b[1 : 1+l]
	bitmap := bytesToBitmap(bitmapBytes)
	siblingsBytes := b[1+l:]
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
	bitmapBytesLen := int(math.Ceil(float64(len(bitmap)) / 8)) //nolint:gomnd
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

// Get returns the value for a given key. If the key is not found, will return
// the error ErrKeyNotFound, and in the leafK & leafV parameters will be placed
// the data found in the tree in the leaf that was on the path going to the
// input key.
func (t *Tree) Get(k []byte) ([]byte, []byte, error) {
	rTx := t.db.ReadTx()
	defer rTx.Discard()

	return t.GetWithTx(rTx, k)
}

// GetWithTx does the same than the Get method, but allowing to pass the
// db.ReadTx that is used. If the key is not found, will return the error
// ErrKeyNotFound, and in the leafK & leafV parameters will be placed the data
// found in the tree in the leaf that was on the path going to the input key.
func (t *Tree) GetWithTx(rTx db.ReadTx, k []byte) ([]byte, []byte, error) {
	keyPath := make([]byte, t.hashFunction.Len())
	copy(keyPath[:], k)

	root, err := t.RootWithTx(rTx)
	if err != nil {
		return nil, nil, err
	}

	path := getPath(t.maxLevels, keyPath)
	// go down to the leaf
	var siblings [][]byte
	_, value, _, err := t.down(rTx, k, root, siblings, path, 0, true)
	if err != nil {
		return nil, nil, err
	}
	leafK, leafV := ReadLeafValue(value)
	if !bytes.Equal(k, leafK) {
		return leafK, leafV, ErrKeyNotFound
	}

	return leafK, leafV, nil
}

// CheckProof verifies the given proof. The proof verification depends on the
// HashFunction passed as parameter.
func CheckProof(hashFunc HashFunction, k, v, root, packedSiblings []byte) (bool, error) {
	siblings, err := UnpackSiblings(hashFunc, packedSiblings)
	if err != nil {
		return false, err
	}

	keyPath := make([]byte, hashFunc.Len())
	copy(keyPath[:], k)

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
	if bytes.Equal(key[:], root) {
		return true, nil
	}
	return false, nil
}

func (t *Tree) incNLeafs(wTx db.WriteTx, nLeafs int) error {
	oldNLeafs, err := t.GetNLeafs()
	if err != nil {
		return err
	}
	newNLeafs := oldNLeafs + nLeafs
	return t.setNLeafs(wTx, newNLeafs)
}

func (t *Tree) setNLeafs(wTx db.WriteTx, nLeafs int) error {
	b := make([]byte, 8)
	binary.LittleEndian.PutUint64(b, uint64(nLeafs))
	if err := wTx.Set(dbKeyNLeafs, b); err != nil {
		return err
	}
	return nil
}

// GetNLeafs returns the number of Leafs of the Tree.
func (t *Tree) GetNLeafs() (int, error) {
	rTx := t.db.ReadTx()
	defer rTx.Discard()

	return t.GetNLeafsWithTx(rTx)
}

// GetNLeafsWithTx does the same than the GetNLeafs method, but allowing to
// pass the db.ReadTx that is used.
func (t *Tree) GetNLeafsWithTx(rTx db.ReadTx) (int, error) {
	b, err := rTx.Get(dbKeyNLeafs)
	if err != nil {
		return 0, err
	}
	nLeafs := binary.LittleEndian.Uint64(b)
	return int(nLeafs), nil
}

// Snapshot returns a read-only copy of the Tree from the given root
func (t *Tree) Snapshot(fromRoot []byte) (*Tree, error) {
	t.RLock()
	defer t.RUnlock()

	// allow to define which root to use
	if fromRoot == nil {
		var err error
		fromRoot, err = t.Root()
		if err != nil {
			return nil, err
		}
	}
	return &Tree{
		db:           t.db,
		maxLevels:    t.maxLevels,
		snapshotRoot: fromRoot,
		hashFunction: t.hashFunction,
		dbg:          t.dbg,
	}, nil
}

// Iterate iterates through the full Tree, executing the given function on each
// node of the Tree.
func (t *Tree) Iterate(fromRoot []byte, f func([]byte, []byte)) error {
	rTx := t.db.ReadTx()
	defer rTx.Discard()

	return t.IterateWithTx(rTx, fromRoot, f)
}

// IterateWithTx does the same than the Iterate method, but allowing to pass
// the db.ReadTx that is used.
func (t *Tree) IterateWithTx(rTx db.ReadTx, fromRoot []byte, f func([]byte, []byte)) error {
	// allow to define which root to use
	if fromRoot == nil {
		var err error
		fromRoot, err = t.RootWithTx(rTx)
		if err != nil {
			return err
		}
	}
	return t.iter(rTx, fromRoot, f)
}

// IterateWithStop does the same than Iterate, but with int for the current
// level, and a boolean parameter used by the passed function, is to indicate to
// stop iterating on the branch when the method returns 'true'.
func (t *Tree) IterateWithStop(fromRoot []byte, f func(int, []byte, []byte) bool) error {
	rTx := t.db.ReadTx()
	defer rTx.Discard()

	// allow to define which root to use
	if fromRoot == nil {
		var err error
		fromRoot, err = t.RootWithTx(rTx)
		if err != nil {
			return err
		}
	}
	return t.iterWithStop(rTx, fromRoot, 0, f)
}

// IterateWithStopWithTx does the same than the IterateWithStop method, but
// allowing to pass the db.ReadTx that is used.
func (t *Tree) IterateWithStopWithTx(rTx db.ReadTx, fromRoot []byte,
	f func(int, []byte, []byte) bool) error {
	// allow to define which root to use
	if fromRoot == nil {
		var err error
		fromRoot, err = t.RootWithTx(rTx)
		if err != nil {
			return err
		}
	}
	return t.iterWithStop(rTx, fromRoot, 0, f)
}

func (t *Tree) iterWithStop(rTx db.ReadTx, k []byte, currLevel int,
	f func(int, []byte, []byte) bool) error {
	var v []byte
	var err error
	if bytes.Equal(k, t.emptyHash) {
		v = t.emptyHash
	} else {
		v, err = rTx.Get(k)
		if err != nil {
			return err
		}
	}
	currLevel++

	switch v[0] {
	case PrefixValueEmpty:
		f(currLevel, k, v)
	case PrefixValueLeaf:
		f(currLevel, k, v)
	case PrefixValueIntermediate:
		stop := f(currLevel, k, v)
		if stop {
			return nil
		}
		l, r := ReadIntermediateChilds(v)
		if err = t.iterWithStop(rTx, l, currLevel, f); err != nil {
			return err
		}
		if err = t.iterWithStop(rTx, r, currLevel, f); err != nil {
			return err
		}
	default:
		return ErrInvalidValuePrefix
	}
	return nil
}

func (t *Tree) iter(rTx db.ReadTx, k []byte, f func([]byte, []byte)) error {
	f2 := func(currLvl int, k, v []byte) bool {
		f(k, v)
		return false
	}
	return t.iterWithStop(rTx, k, 0, f2)
}

// Dump exports all the Tree leafs in a byte array of length:
// [ N * (2+len(k+v)) ]. Where N is the number of key-values, and for each k+v:
// [ 1 byte | 1 byte | S bytes | len(v) bytes ]
// [ len(k) | len(v) |   key   |     value    ]
// Where S is the size of the output of the hash function used for the Tree.
func (t *Tree) Dump(fromRoot []byte) ([]byte, error) {
	// allow to define which root to use
	if fromRoot == nil {
		var err error
		fromRoot, err = t.Root()
		if err != nil {
			return nil, err
		}
	}

	// WARNING current encoding only supports key & values of 255 bytes each
	// (due using only 1 byte for the length headers).
	var b []byte
	err := t.Iterate(fromRoot, func(k, v []byte) {
		if v[0] != PrefixValueLeaf {
			return
		}
		leafK, leafV := ReadLeafValue(v)
		kv := make([]byte, 2+len(leafK)+len(leafV))
		kv[0] = byte(len(leafK))
		kv[1] = byte(len(leafV))
		copy(kv[2:2+len(leafK)], leafK)
		copy(kv[2+len(leafK):], leafV)
		b = append(b, kv...)
	})
	return b, err
}

// ImportDump imports the leafs (that have been exported with the Dump method)
// in the Tree.
func (t *Tree) ImportDump(b []byte) error {
	if !t.editable() {
		return ErrSnapshotNotEditable
	}
	root, err := t.Root()
	if err != nil {
		return err
	}
	if !bytes.Equal(root, t.emptyHash) {
		return ErrTreeNotEmpty
	}

	r := bytes.NewReader(b)
	var keys, values [][]byte
	for {
		l := make([]byte, 2)
		_, err = io.ReadFull(r, l)
		if err == io.EOF {
			break
		} else if err != nil {
			return err
		}
		k := make([]byte, l[0])
		_, err = io.ReadFull(r, k)
		if err != nil {
			return err
		}
		v := make([]byte, l[1])
		_, err = io.ReadFull(r, v)
		if err != nil {
			return err
		}
		keys = append(keys, k)
		values = append(values, v)
	}
	if _, err = t.AddBatch(keys, values); err != nil {
		return err
	}
	return nil
}

// Graphviz iterates across the full tree to generate a string Graphviz
// representation of the tree and writes it to w
func (t *Tree) Graphviz(w io.Writer, fromRoot []byte) error {
	return t.GraphvizFirstNLevels(w, fromRoot, t.maxLevels)
}

// GraphvizFirstNLevels iterates across the first NLevels of the tree to
// generate a string Graphviz representation of the first NLevels of the tree
// and writes it to w
func (t *Tree) GraphvizFirstNLevels(w io.Writer, fromRoot []byte, untilLvl int) error {
	fmt.Fprintf(w, `digraph hierarchy {
node [fontname=Monospace,fontsize=10,shape=box]
`)

	rTx := t.db.ReadTx()
	defer rTx.Discard()

	if fromRoot == nil {
		var err error
		fromRoot, err = t.RootWithTx(rTx)
		if err != nil {
			return err
		}
	}

	nEmpties := 0
	err := t.iterWithStop(rTx, fromRoot, 0, func(currLvl int, k, v []byte) bool {
		if currLvl == untilLvl {
			return true // to stop the iter from going down
		}
		switch v[0] {
		case PrefixValueEmpty:
		case PrefixValueLeaf:
			fmt.Fprintf(w, "\"%v\" [style=filled];\n", hex.EncodeToString(k[:nChars]))
			// key & value from the leaf
			kB, vB := ReadLeafValue(v)
			fmt.Fprintf(w, "\"%v\" -> {\"k:%v\\nv:%v\"}\n",
				hex.EncodeToString(k[:nChars]), hex.EncodeToString(kB[:nChars]),
				hex.EncodeToString(vB[:nChars]))
			fmt.Fprintf(w, "\"k:%v\\nv:%v\" [style=dashed]\n",
				hex.EncodeToString(kB[:nChars]), hex.EncodeToString(vB[:nChars]))
		case PrefixValueIntermediate:
			l, r := ReadIntermediateChilds(v)
			lStr := hex.EncodeToString(l[:nChars])
			rStr := hex.EncodeToString(r[:nChars])
			eStr := ""
			if bytes.Equal(l, t.emptyHash) {
				lStr = fmt.Sprintf("empty%v", nEmpties)
				eStr += fmt.Sprintf("\"%v\" [style=dashed,label=0];\n",
					lStr)
				nEmpties++
			}
			if bytes.Equal(r, t.emptyHash) {
				rStr = fmt.Sprintf("empty%v", nEmpties)
				eStr += fmt.Sprintf("\"%v\" [style=dashed,label=0];\n",
					rStr)
				nEmpties++
			}
			fmt.Fprintf(w, "\"%v\" -> {\"%v\" \"%v\"}\n", hex.EncodeToString(k[:nChars]),
				lStr, rStr)
			fmt.Fprint(w, eStr)
		default:
		}
		return false
	})
	fmt.Fprintf(w, "}\n")
	return err
}

// PrintGraphviz prints the output of Tree.Graphviz
func (t *Tree) PrintGraphviz(fromRoot []byte) error {
	if fromRoot == nil {
		var err error
		fromRoot, err = t.Root()
		if err != nil {
			return err
		}
	}
	return t.PrintGraphvizFirstNLevels(fromRoot, t.maxLevels)
}

// PrintGraphvizFirstNLevels prints the output of Tree.GraphvizFirstNLevels
func (t *Tree) PrintGraphvizFirstNLevels(fromRoot []byte, untilLvl int) error {
	if fromRoot == nil {
		var err error
		fromRoot, err = t.Root()
		if err != nil {
			return err
		}
	}
	w := bytes.NewBufferString("")
	fmt.Fprintf(w,
		"--------\nGraphviz of the Tree with Root "+hex.EncodeToString(fromRoot)+":\n")
	err := t.GraphvizFirstNLevels(w, fromRoot, untilLvl)
	if err != nil {
		fmt.Println(w)
		return err
	}
	fmt.Fprintf(w,
		"End of Graphviz of the Tree with Root "+hex.EncodeToString(fromRoot)+"\n--------\n")

	fmt.Println(w)
	return nil
}

// TODO circom proofs
// TODO data structure for proofs (including root, key, value, siblings,
// hashFunction) + method to verify that data structure

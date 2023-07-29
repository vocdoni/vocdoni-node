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
	"bufio"
	"bytes"
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"io"
	"math"
	"runtime"
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

	// maxUint8 is the max size of key length
	maxUint8 = int(^uint8(0)) // 2**8 -1
	// maxUint16 is the max size of value length
	maxUint16 = int(^uint16(0)) // 2**16 -1
)

var (
	// DefaultThresholdNLeafs defines the threshold number of leafs in the
	// tree that determines if AddBatch will work in memory or in disk.  It
	// is defined when calling NewTree, and if set to 0 it will work always
	// in disk.
	DefaultThresholdNLeafs = 65536

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
	// ErrSnapshotNotEditable indicates when the tree is a non writable
	// snapshot, thus can not be modified
	ErrSnapshotNotEditable = fmt.Errorf("snapshot tree can not be edited")
	// ErrTreeNotEmpty indicates when the tree was expected to be empty and
	// it is not
	ErrTreeNotEmpty = fmt.Errorf("tree is not empty")
)

// Tree defines the struct that implements the MerkleTree functionalities
type Tree struct {
	sync.Mutex

	db        db.Database
	maxLevels int
	// thresholdNLeafs defines the threshold number of leafs in the tree
	// that determines if AddBatch will work in memory or in disk.  It is
	// defined when calling NewTree, and if set to 0 it will work always in
	// disk.
	thresholdNLeafs int
	snapshotRoot    []byte

	hashFunction HashFunction
	// TODO in the methods that use it, check if emptyHash param is len>0
	// (check if it has been initialized)
	emptyHash []byte

	dbg *dbgStats
}

// Config defines the configuration for calling NewTree & NewTreeWithTx methods
type Config struct {
	Database        db.Database
	MaxLevels       int
	ThresholdNLeafs int
	HashFunction    HashFunction
}

// NewTree returns a new Tree, if there is a Tree still in the given database, it
// will load it.
func NewTree(cfg Config) (*Tree, error) {
	wTx := cfg.Database.WriteTx()
	defer wTx.Discard()

	t, err := NewTreeWithTx(wTx, cfg)
	if err != nil {
		return nil, err
	}

	if err = wTx.Commit(); err != nil {
		return nil, err
	}
	return t, nil
}

// NewTreeWithTx returns a new Tree using the given db.WriteTx, which will not
// be committed inside this method, if there is a Tree still in the given
// database, it will load it.
func NewTreeWithTx(wTx db.WriteTx, cfg Config) (*Tree, error) {
	// if thresholdNLeafs is set to 0, use the DefaultThresholdNLeafs
	if cfg.ThresholdNLeafs == 0 {
		cfg.ThresholdNLeafs = DefaultThresholdNLeafs
	}
	t := Tree{db: cfg.Database, maxLevels: cfg.MaxLevels,
		thresholdNLeafs: cfg.ThresholdNLeafs, hashFunction: cfg.HashFunction}
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
	return t.RootWithTx(t.db)
}

// RootWithTx returns the root of the Tree using the given db.ReadTx
func (t *Tree) RootWithTx(rTx db.Reader) ([]byte, error) {
	// if snapshotRoot is defined, means that the tree is a snapshot, and
	// the root is not obtained from the db, but from the snapshotRoot
	// parameter
	if t.snapshotRoot != nil {
		return t.snapshotRoot, nil
	}
	// get db root
	return rTx.Get(dbKeyRoot)
}

func (*Tree) setRoot(wTx db.WriteTx, root []byte) error {
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

// Invalid is used when a key-value can not be added trough AddBatch, and
// contains the index of the key-value and the error.
type Invalid struct {
	Index int
	Error error
}

// AddBatch adds a batch of key-values to the Tree. Returns an array containing
// the indexes of the keys failed to add. Supports empty values as input
// parameters, which is equivalent to 0 valued byte array.
func (t *Tree) AddBatch(keys, values [][]byte) ([]Invalid, error) {
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
func (t *Tree) AddBatchWithTx(wTx db.WriteTx, keys, values [][]byte) ([]Invalid, error) {
	t.Lock()
	defer t.Unlock()

	if !t.editable() {
		return nil, ErrSnapshotNotEditable
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

	nLeafs, err := t.GetNLeafsWithTx(wTx)
	if err != nil {
		return nil, err
	}
	if nLeafs > t.thresholdNLeafs {
		return t.addBatchInDisk(wTx, keys, values)
	}
	return t.addBatchInMemory(wTx, keys, values)
}

func (t *Tree) addBatchInDisk(wTx db.WriteTx, keys, values [][]byte) ([]Invalid, error) {
	nCPU := flp2(runtime.NumCPU())
	if nCPU == 1 || len(keys) < nCPU {
		var invalids []Invalid
		for i := 0; i < len(keys); i++ {
			if err := t.addWithTx(wTx, keys[i], values[i]); err != nil {
				invalids = append(invalids, Invalid{i, err})
			}
		}
		return invalids, nil
	}

	kvs, invalids, err := keysValuesToKvs(t.maxLevels, keys, values)
	if err != nil {
		return nil, err
	}

	buckets := splitInBuckets(kvs, nCPU)

	root, err := t.RootWithTx(wTx)
	if err != nil {
		return nil, err
	}

	l := int(math.Log2(float64(nCPU)))
	subRoots, err := t.getSubRootsAtLevel(wTx, root, l+1)
	if err != nil {
		return nil, err
	}
	if len(subRoots) != nCPU {
		// Already populated Tree but Unbalanced.

		// add one key at each bucket, and then continue with the flow
		for i := 0; i < len(buckets); i++ {
			// add one leaf of the bucket, if there is an error when
			// adding the k-v, try to add the next one of the bucket
			// (until one is added)
			inserted := -1
			for j := 0; j < len(buckets[i]); j++ {
				if newRoot, err := t.add(wTx, root, 0,
					buckets[i][j].k, buckets[i][j].v); err == nil {
					inserted = j
					root = newRoot
					break
				}
			}

			// remove the inserted element from buckets[i]
			if inserted != -1 {
				buckets[i] = append(buckets[i][:inserted], buckets[i][inserted+1:]...)
			}
		}
		subRoots, err = t.getSubRootsAtLevel(wTx, root, l+1)
		if err != nil {
			return nil, err
		}
	}

	if len(subRoots) != nCPU {
		return nil, fmt.Errorf("this error should not be reached."+
			" len(subRoots) != nCPU, len(subRoots)=%d, nCPU=%d."+
			" Please report it in a new issue:"+
			" https://go.vocdoni.io/dvote/tree/arbo/issues/new", len(subRoots), nCPU)
	}

	invalidsInBucket := make([][]Invalid, nCPU)
	txs := make([]db.WriteTx, nCPU)
	for i := 0; i < nCPU; i++ {
		txs[i] = t.db.WriteTx()
		err := txs[i].Apply(wTx)
		if err != nil {
			return nil, err
		}
	}

	var wg sync.WaitGroup
	wg.Add(nCPU)
	for i := 0; i < nCPU; i++ {
		go func(cpu int) {
			// use different wTx for each cpu, after once all
			// are done, iter over the cpuWTxs and copy their
			// content into the main wTx
			for j := 0; j < len(buckets[cpu]); j++ {
				newSubRoot, err := t.add(txs[cpu], subRoots[cpu],
					l, buckets[cpu][j].k, buckets[cpu][j].v)
				if err != nil {
					invalidsInBucket[cpu] = append(invalidsInBucket[cpu],
						Invalid{buckets[cpu][j].pos, err})
					continue
				}
				// if there has not been errors, set the new subRoots[cpu]
				subRoots[cpu] = newSubRoot
			}
			wg.Done()
		}(i)
	}
	wg.Wait()

	for i := 0; i < nCPU; i++ {
		if err := wTx.Apply(txs[i]); err != nil {
			return nil, err
		}
		txs[i].Discard()
	}

	for i := 0; i < len(invalidsInBucket); i++ {
		invalids = append(invalids, invalidsInBucket[i]...)
	}

	newRoot, err := t.upFromSubRoots(wTx, subRoots)
	if err != nil {
		return nil, err
	}

	// update dbKeyNLeafs
	if err := t.SetRootWithTx(wTx, newRoot); err != nil {
		return nil, err
	}

	// update nLeafs
	if err := t.incNLeafs(wTx, len(keys)-len(invalids)); err != nil {
		return nil, err
	}

	return invalids, nil
}

func (t *Tree) upFromSubRoots(wTx db.WriteTx, subRoots [][]byte) ([]byte, error) {
	// is a method of Tree just to get access to t.hashFunction and
	// t.emptyHash.

	// go up from subRoots to up, storing nodes in the given WriteTx
	// once up at the root, store it in the WriteTx using the dbKeyRoot
	if len(subRoots) == 1 {
		return subRoots[0], nil
	}
	// get the subRoots values to know the node types of each subRoot
	nodeTypes := make([]byte, len(subRoots))
	for i := 0; i < len(subRoots); i++ {
		if bytes.Equal(subRoots[i], t.emptyHash) {
			nodeTypes[i] = PrefixValueEmpty
			continue
		}
		v, err := wTx.Get(subRoots[i])
		if err != nil {
			return nil, err
		}
		nodeTypes[i] = v[0]
	}

	var newSubRoots [][]byte
	for i := 0; i < len(subRoots); i += 2 {
		if (bytes.Equal(subRoots[i], t.emptyHash) && bytes.Equal(subRoots[i+1], t.emptyHash)) ||
			(nodeTypes[i] == PrefixValueLeaf && bytes.Equal(subRoots[i+1], t.emptyHash)) {
			// when both sub nodes are empty, the parent is also empty
			// or
			// when 1st sub node is a leaf but the 2nd is empty, the
			// leaf is used as 'parent'

			newSubRoots = append(newSubRoots, subRoots[i])
			continue
		}
		if bytes.Equal(subRoots[i], t.emptyHash) && nodeTypes[i+1] == PrefixValueLeaf {
			// when 2nd sub node is a leaf but the 1st is empty,
			// the leaf is used as 'parent'
			newSubRoots = append(newSubRoots, subRoots[i+1])
			continue
		}

		k, v, err := t.newIntermediate(subRoots[i], subRoots[i+1])
		if err != nil {
			return nil, err
		}
		// store k-v to db
		if err = wTx.Set(k, v); err != nil {
			return nil, err
		}
		newSubRoots = append(newSubRoots, k)
	}

	return t.upFromSubRoots(wTx, newSubRoots)
}

func (t *Tree) getSubRootsAtLevel(rTx db.Reader, root []byte, l int) ([][]byte, error) {
	// go at level l and return each node key, where each node key is the
	// subRoot of the subTree that starts there

	var subRoots [][]byte
	err := t.iterWithStop(rTx, root, 0, func(currLvl int, k, v []byte) bool {
		if currLvl == l && !bytes.Equal(k, t.emptyHash) {
			subRoots = append(subRoots, k)
		}
		if currLvl >= l {
			return true // to stop the iter from going down
		}
		return false
	})

	return subRoots, err
}

func (t *Tree) addBatchInMemory(wTx db.WriteTx, keys, values [][]byte) ([]Invalid, error) {
	vt, err := t.loadVT(wTx)
	if err != nil {
		return nil, err
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
	if vt.root != nil {
		if err := wTx.Set(dbKeyRoot, vt.root.h); err != nil {
			return nil, err
		}
	}

	// update nLeafs
	if err := t.incNLeafs(wTx, len(keys)-len(invalids)); err != nil {
		return nil, err
	}

	return invalids, nil
}

// loadVT loads a new virtual tree (vt) from the current Tree, which contains
// the same leafs.
func (t *Tree) loadVT(rTx db.Reader) (vt, error) {
	vt := newVT(t.maxLevels, t.hashFunction)
	vt.params.dbg = t.dbg
	var callbackErr error
	err := t.IterateWithStopWithTx(rTx, nil, func(_ int, k, v []byte) bool {
		if v[0] != PrefixValueLeaf {
			return false
		}
		leafK, leafV := ReadLeafValue(v)
		if err := vt.add(0, leafK, leafV); err != nil {
			callbackErr = err
			return true
		}
		return false
	})
	if callbackErr != nil {
		return vt, callbackErr
	}

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
	return t.addWithTx(wTx, k, v)
}

// warning: addWithTx does not use the Tree mutex, the mutex is responsibility
// of the methods calling this method, and same with t.editable().
func (t *Tree) addWithTx(wTx db.WriteTx, k, v []byte) error {
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

func (t *Tree) add(wTx db.WriteTx, root []byte, fromLvl int, k, v []byte) ([]byte, error) {
	if err := checkKeyValueLen(k, v); err != nil {
		return nil, err
	}

	keyPath, err := keyPathFromKey(t.maxLevels, k)
	if err != nil {
		return nil, err
	}
	path := getPath(t.maxLevels, keyPath)

	// go down to the leaf
	var siblings [][]byte
	_, _, siblings, err = t.down(wTx, k, root, siblings, path, fromLvl, false)
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
func (t *Tree) down(rTx db.Reader, newKey, currKey []byte, siblings [][]byte,
	path []bool, currLvl int, getLeaf bool) (
	[]byte, []byte, [][]byte, error) {
	if currLvl > t.maxLevels {
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
		panic("This point should not be reached, as the 'if currKey==t.emptyHash'" +
			" above should avoid reaching this point. This panic is temporary" +
			" for reporting purposes, will be deleted in future versions." +
			" Please paste this log (including the previous log lines) in a" +
			" new issue: https://go.vocdoni.io/dvote/tree/arbo/issues/new") // TMP
	case PrefixValueLeaf: // leaf
		if !bytes.Equal(currValue, emptyValue) {
			if getLeaf {
				return currKey, currValue, siblings, nil
			}
			oldLeafKey, _ := ReadLeafValue(currValue)
			if bytes.Equal(newKey, oldLeafKey) {
				return nil, nil, nil, ErrKeyAlreadyExists
			}

			oldLeafKeyFull, err := keyPathFromKey(t.maxLevels, oldLeafKey)
			if err != nil {
				return nil, nil, nil, err
			}

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

func (t *Tree) newIntermediate(l, r []byte) ([]byte, []byte, error) {
	t.dbg.incHash()
	return newIntermediate(t.hashFunction, l, r)
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

	keyPath, err := keyPathFromKey(t.maxLevels, k)
	if err != nil {
		return err
	}
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
	var siblings [][]byte
	_, value, siblings, err := t.down(rTx, k, root, siblings, path, 0, true)
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
	// to prevent runtime slice out of bounds error check if the lenght of the
	// rest of the slice is at least equal to the encoded full length value
	if len(b) < 4 {
		return nil, fmt.Errorf("no enought size of packed siblings provided")
	}

	fullLen := binary.LittleEndian.Uint16(b[0:2])
	if len(b) != int(fullLen) {
		return nil, fmt.Errorf("expected len: %d, current len: %d", fullLen, len(b))
	}

	l := binary.LittleEndian.Uint16(b[2:4]) // bitmap bytes length
	// to prevent runtime slice out of bounds error check if the lenght of the
	// rest of the slice is at least equal to the encoded bitmap length value
	if len(b) < int(4+l) {
		return nil, fmt.Errorf("expected len: %d, current len: %d", 4+l, len(b))
	}
	bitmapBytes := b[4 : 4+l]
	bitmap := bytesToBitmap(bitmapBytes)
	siblingsBytes := b[4+l:]
	// to prevent a runtime slice out of bounds error, check if the lenght of
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

// Get returns the value in the Tree for a given key. If the key is not found,
// will return the error ErrKeyNotFound, and in the leafK & leafV parameters
// will be placed the data found in the tree in the leaf that was on the path
// going to the input key.
func (t *Tree) Get(k []byte) ([]byte, []byte, error) {
	return t.GetWithTx(t.db, k)
}

// GetWithTx does the same than the Get method, but allowing to pass the
// db.ReadTx that is used. If the key is not found, will return the error
// ErrKeyNotFound, and in the leafK & leafV parameters will be placed the data
// found in the tree in the leaf that was on the path going to the input key.
func (t *Tree) GetWithTx(rTx db.Reader, k []byte) ([]byte, []byte, error) {
	keyPath, err := keyPathFromKey(t.maxLevels, k)
	if err != nil {
		return nil, nil, err
	}
	path := getPath(t.maxLevels, keyPath)

	root, err := t.RootWithTx(rTx)
	if err != nil {
		return nil, nil, err
	}

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

func (t *Tree) incNLeafs(wTx db.WriteTx, nLeafs int) error {
	oldNLeafs, err := t.GetNLeafsWithTx(wTx)
	if err != nil {
		return err
	}
	newNLeafs := oldNLeafs + nLeafs
	return t.setNLeafs(wTx, newNLeafs)
}

func (*Tree) setNLeafs(wTx db.WriteTx, nLeafs int) error {
	b := make([]byte, 8)
	binary.LittleEndian.PutUint64(b, uint64(nLeafs))
	if err := wTx.Set(dbKeyNLeafs, b); err != nil {
		return err
	}
	return nil
}

// GetNLeafs returns the number of Leafs of the Tree.
func (t *Tree) GetNLeafs() (int, error) {
	return t.GetNLeafsWithTx(t.db)
}

// GetNLeafsWithTx does the same than the GetNLeafs method, but allowing to
// pass the db.ReadTx that is used.
func (*Tree) GetNLeafsWithTx(rTx db.Reader) (int, error) {
	b, err := rTx.Get(dbKeyNLeafs)
	if err != nil {
		return 0, err
	}
	nLeafs := binary.LittleEndian.Uint64(b)
	return int(nLeafs), nil
}

// SetRoot sets the root to the given root
func (t *Tree) SetRoot(root []byte) error {
	wTx := t.db.WriteTx()
	defer wTx.Discard()

	if err := t.SetRootWithTx(wTx, root); err != nil {
		return err
	}
	return wTx.Commit()
}

// SetRootWithTx sets the root to the given root using the given db.WriteTx
func (t *Tree) SetRootWithTx(wTx db.WriteTx, root []byte) error {
	if !t.editable() {
		return ErrSnapshotNotEditable
	}

	if root == nil {
		return fmt.Errorf("can not SetRoot with nil root")
	}

	// check that the root exists in the db
	if !bytes.Equal(root, t.emptyHash) {
		if _, err := wTx.Get(root); err == ErrKeyNotFound {
			return fmt.Errorf("can not SetRoot with root %x, as it"+
				" does not exist in the db", root)
		} else if err != nil {
			return err
		}
	}

	return wTx.Set(dbKeyRoot, root)
}

// Snapshot returns a read-only copy of the Tree from the given root
func (t *Tree) Snapshot(fromRoot []byte) (*Tree, error) {
	// allow to define which root to use
	if fromRoot == nil {
		var err error
		fromRoot, err = t.Root()
		if err != nil {
			return nil, err
		}
	}
	// check that the root exists in the db
	if !bytes.Equal(fromRoot, t.emptyHash) {
		if _, err := t.db.Get(fromRoot); err == ErrKeyNotFound {
			return nil,
				fmt.Errorf("can not do a Snapshot with root %x,"+
					" as it does not exist in the db", fromRoot)
		} else if err != nil {
			return nil, err
		}
	}

	return &Tree{
		db:           t.db,
		maxLevels:    t.maxLevels,
		snapshotRoot: fromRoot,
		emptyHash:    t.emptyHash,
		hashFunction: t.hashFunction,
		dbg:          t.dbg,
	}, nil
}

// Iterate iterates through the full Tree, executing the given function on each
// node of the Tree.
func (t *Tree) Iterate(fromRoot []byte, f func([]byte, []byte)) error {
	return t.IterateWithTx(t.db, fromRoot, f)
}

// IterateWithTx does the same than the Iterate method, but allowing to pass
// the db.ReadTx that is used.
func (t *Tree) IterateWithTx(rTx db.Reader, fromRoot []byte, f func([]byte, []byte)) error {
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
	// allow to define which root to use
	if fromRoot == nil {
		var err error
		fromRoot, err = t.RootWithTx(t.db)
		if err != nil {
			return err
		}
	}
	return t.iterWithStop(t.db, fromRoot, 0, f)
}

// IterateWithStopWithTx does the same than the IterateWithStop method, but
// allowing to pass the db.ReadTx that is used.
func (t *Tree) IterateWithStopWithTx(rTx db.Reader, fromRoot []byte,
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

func (t *Tree) iterWithStop(rTx db.Reader, k []byte, currLevel int,
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

func (t *Tree) iter(rTx db.Reader, k []byte, f func([]byte, []byte)) error {
	f2 := func(currLvl int, k, v []byte) bool {
		f(k, v)
		return false
	}
	return t.iterWithStop(rTx, k, 0, f2)
}

// Dump exports all the Tree leafs in a byte array
func (t *Tree) Dump(fromRoot []byte) ([]byte, error) {
	return t.dump(fromRoot, nil)
}

// DumpWriter exports all the Tree leafs writing the bytes in the given Writer
func (t *Tree) DumpWriter(fromRoot []byte, w io.Writer) error {
	_, err := t.dump(fromRoot, w)
	return err
}

// dump exports all the Tree leafs. If the given w is nil, it will return a
// byte array with the dump, if w contains a *bufio.Writer, it will write the
// dump in w.
// The format of the dump is the following:
// Dump length: [ N * (3+len(k+v)) ]. Where N is the number of key-values, and for each k+v:
// [ 1 byte | 2 byte | S bytes | len(v) bytes ]
// [ len(k) | len(v) |   key   |     value    ]
// Where S is the size of the output of the hash function used for the Tree.
func (t *Tree) dump(fromRoot []byte, w io.Writer) ([]byte, error) {
	// allow to define which root to use
	if fromRoot == nil {
		var err error
		fromRoot, err = t.Root()
		if err != nil {
			return nil, err
		}
	}

	// WARNING current encoding only supports keys of 255 bytes and values
	// of 65535 bytes each (due using only 1 and 2 bytes for the length
	// headers). These lengths are checked on leaf addition by the function
	// checkKeyValueLen.
	var b []byte
	var callbackErr error
	err := t.IterateWithStop(fromRoot, func(_ int, k, v []byte) bool {
		if v[0] != PrefixValueLeaf {
			return false
		}
		leafK, leafV := ReadLeafValue(v)
		kv := make([]byte, 3+len(leafK)+len(leafV))
		if len(leafK) > maxUint8 {
			callbackErr = fmt.Errorf("len(leafK) > %v", maxUint8)
			return true
		}
		kv[0] = byte(len(leafK))
		if len(leafV) > maxUint16 {
			callbackErr = fmt.Errorf("len(leafV) > %v", maxUint16)
			return true
		}
		binary.LittleEndian.PutUint16(kv[1:3], uint16(len(leafV)))
		copy(kv[3:3+len(leafK)], leafK)
		copy(kv[3+len(leafK):], leafV)

		if w == nil {
			b = append(b, kv...)
		} else {
			n, err := w.Write(kv)
			if err != nil {
				callbackErr = fmt.Errorf("dump: w.Write, %s", err)
				return true
			}
			if n != len(kv) {
				callbackErr = fmt.Errorf("dump: w.Write n!=len(kv), %s", err)
				return true
			}
		}
		return false
	})
	if callbackErr != nil {
		return nil, callbackErr
	}
	return b, err
}

// ImportDump imports the leafs (that have been exported with the Dump method)
// in the Tree, reading them from the given byte array.
func (t *Tree) ImportDump(b []byte) error {
	bytesReader := bytes.NewReader(b)
	r := bufio.NewReader(bytesReader)
	return t.ImportDumpReader(r)
}

// ImportDumpReader imports the leafs (that have been exported with the Dump
// method) in the Tree, reading them from the given reader.
func (t *Tree) ImportDumpReader(r io.Reader) error {
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

	var keys, values [][]byte
	for {
		l := make([]byte, 3)
		_, err = io.ReadFull(r, l)
		if err == io.EOF {
			break
		} else if err != nil {
			return err
		}
		lenK := int(l[0])
		k := make([]byte, lenK)
		_, err = io.ReadFull(r, k)
		if err != nil {
			return err
		}
		lenV := binary.LittleEndian.Uint16(l[1:3])
		v := make([]byte, lenV)
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

	if fromRoot == nil {
		var err error
		fromRoot, err = t.RootWithTx(t.db)
		if err != nil {
			return err
		}
	}

	nEmpties := 0
	err := t.iterWithStop(t.db, fromRoot, 0, func(currLvl int, k, v []byte) bool {
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

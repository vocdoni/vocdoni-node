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

	dbKeyRoot   = []byte("arbo/root/")
	dbKeyNLeafs = []byte("arbo/nleafs/")
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

	// emptyNode is the hash of an empty node (with both childs empty)
	emptyNode []byte

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

	t := Tree{
		db:              cfg.Database,
		maxLevels:       cfg.MaxLevels,
		thresholdNLeafs: cfg.ThresholdNLeafs,
		hashFunction:    cfg.HashFunction,
	}

	t.emptyHash = make([]byte, t.hashFunction.Len()) // empty
	var err error
	t.emptyNode, _, err = t.newIntermediate(t.emptyHash, t.emptyHash)
	if err != nil {
		return nil, err
	}

	_, err = wTx.Get(dbKeyRoot)
	if err == db.ErrKeyNotFound {
		// store new root 0 (empty)
		return &t, t.setToEmptyTree(wTx)
	} else if err != nil {
		return nil, fmt.Errorf("unknown database error: %w", err)
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
	hash, err := rTx.Get(dbKeyRoot)
	return hash, err
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

// Invalid is used when a key-value can not be added trough AddBatch, and
// contains the index of the key-value and the error.
type Invalid struct {
	Index int
	Error error
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
	return t.incNLeafs(wTx, 1)
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
	var siblings, intermediates [][]byte

	_, _, siblings, err = t.down(wTx, k, root, siblings, &intermediates, path, fromLvl, false)
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
		// if there are no siblings, means that the tree is empty.
		// We consider the leafKey as root
		return leafKey, nil
	}

	root, err = t.up(wTx, leafKey, siblings, path, len(siblings)-1, fromLvl)
	if err != nil {
		return nil, err
	}

	if err := deleteNodes(wTx, intermediates); err != nil {
		return root, fmt.Errorf("error deleting orphan intermediate nodes: %v", err)
	}

	return root, nil
}

func (t *Tree) newLeafValue(k, v []byte) ([]byte, []byte, error) {
	t.dbg.incHash()
	return newLeafValue(t.hashFunction, k, v)
}

func (t *Tree) newIntermediate(l, r []byte) ([]byte, []byte, error) {
	t.dbg.incHash()
	return newIntermediate(t.hashFunction, l, r)
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

	var siblings, intermediates [][]byte
	oldLeafKey, valueAtBottom, siblings, err := t.down(wTx, k, root, siblings, &intermediates, path, 0, true)
	if err != nil {
		return err
	}

	// check if the key is actually the same
	oldKey, _ := ReadLeafValue(valueAtBottom)
	if !bytes.Equal(oldKey, k) {
		return ErrKeyNotFound
	}

	// compute the new leaf key and value
	leafKey, leafValue, err := t.newLeafValue(k, v)
	if err != nil {
		return err
	}

	if bytes.Equal(oldLeafKey, leafKey) {
		// The key is the same, just return, nothing to do.
		// Note that if we don't exit here, the code below will delete
		// the valid intermediate nodes up to the root, and we don't want that.
		// (took me a while to figure out)
		return nil
	}

	// delete the old leaf key
	if err := wTx.Delete(oldLeafKey); err != nil {
		return fmt.Errorf("error deleting old leaf on update: %w", err)
	}

	// add the new leaf key
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

	// delete the old intermediate nodes
	if err := deleteNodes(wTx, intermediates); err != nil {
		return fmt.Errorf("error deleting orphan intermediate nodes: %v", err)
	}

	// store root to db
	return t.setRoot(wTx, root)
}

// Delete removes the key from the Tree.
func (t *Tree) Delete(k []byte) error {
	wTx := t.db.WriteTx()
	defer wTx.Discard()

	if err := t.DeleteWithTx(wTx, k); err != nil {
		return err
	}

	return wTx.Commit()
}

// DeleteWithTx does the same as the Delete method, but allows passing the
// db.WriteTx that is used. The db.WriteTx will not be committed inside this
// method.
func (t *Tree) DeleteWithTx(wTx db.WriteTx, k []byte) error {
	t.Lock()
	defer t.Unlock()

	if !t.editable() {
		return ErrSnapshotNotEditable
	}
	return t.deleteWithTx(wTx, k)
}

// setToEmptyTree sets the tree to an empty tree.
func (t *Tree) setToEmptyTree(wTx db.WriteTx) error {
	if err := wTx.Set(dbKeyRoot, t.emptyHash); err != nil {
		return err
	}
	return t.setNLeafs(wTx, 0)
}

// deleteWithTx deletes a key-value pair from the tree.
func (t *Tree) deleteWithTx(wTx db.WriteTx, k []byte) error {
	root, err := t.RootWithTx(wTx)
	if err != nil {
		return err
	}

	// Navigate down the tree to find the key and collect siblings.
	var siblings, intermediates [][]byte

	path := getPath(t.maxLevels, k)
	leafKey, _, siblings, err := t.down(wTx, k, root, siblings, &intermediates, path, 0, true)
	if err != nil {
		if err == ErrKeyNotFound {
			// Key not found, nothing to delete.
			return nil
		}
		return err
	}

	if len(siblings) == 0 {
		// if no siblings, means the tree is now empty,
		// just delete the leaf key and set the root to empty
		if err := wTx.Delete(leafKey); err != nil {
			return fmt.Errorf("error deleting leaf key %x: %w", leafKey, err)
		}
		return t.setToEmptyTree(wTx)
	}

	var neighbourKeys, neighbourValues [][]byte
	// if the neighbour is not empty, we set it to empty (instead of remove)
	if !bytes.Equal(siblings[len(siblings)-1], t.emptyHash) {
		neighbourKeys, neighbourValues, err = t.getLeavesFromSubPath(wTx, siblings[len(siblings)-1])
		if err != nil {
			return fmt.Errorf("error getting leafs from subpath on Delete: %w", err)
		}
		// if there are no neighbours, set the leaf key to the empty hash
		if len(neighbourKeys) == 0 {
			if err := wTx.Set(leafKey, t.emptyHash); err != nil {
				return fmt.Errorf("error setting leaf key %x to empty hash: %w", leafKey, err)
			}
		}
	} else {
		// else delete the leaf key
		if err := wTx.Delete(leafKey); err != nil {
			return fmt.Errorf("error deleting leaf key %x: %w", leafKey, err)
		}
	}

	// Navigate back up the tree, updating the intermediate nodes.
	newRoot, err := t.up(wTx, t.emptyHash, siblings, path, len(siblings)-1, 0)
	if err != nil {
		return err
	}

	// Delete the orphan intermediate nodes.
	if err := deleteNodes(wTx, intermediates); err != nil {
		return fmt.Errorf("error deleting orphan intermediate nodes: %v", err)
	}

	// Update the root of the tree.
	if err := t.setRoot(wTx, newRoot); err != nil {
		return err
	}

	// Delete the neighbour's childs and add them back to the tree in the right place.
	for i, k := range neighbourKeys {
		if err := t.deleteWithTx(wTx, k); err != nil {
			return fmt.Errorf("error deleting neighbour %d: %w", i, err)
		}
		if err := t.addWithTx(wTx, k, neighbourValues[i]); err != nil {
			return fmt.Errorf("error adding neighbour %d: %w", i, err)
		}
	}

	// Update the number of leaves.
	return t.decNLeafs(wTx, 1)
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
	t.Lock()
	defer t.Unlock()

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

	_, value, _, err := t.down(rTx, k, root, siblings, nil, path, 0, true)
	if err != nil {
		return nil, nil, err
	}
	leafK, leafV := ReadLeafValue(value)
	if !bytes.Equal(k, leafK) {
		return leafK, leafV, ErrKeyNotFound
	}

	return leafK, leafV, nil
}

// decNLeafs decreases the number of leaves in the tree.
func (t *Tree) decNLeafs(wTx db.WriteTx, n int) error {
	nLeafs, err := t.GetNLeafsWithTx(wTx)
	if err != nil {
		return err
	}
	return t.setNLeafs(wTx, nLeafs-n)
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
	return wTx.Set(dbKeyNLeafs, b)
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

// SetRoot sets the root to the given root. The root hash must be a valid existing
// intermediate node in the tree. The list of roots for a level can be obtained
// using tree.RootsFromLevel().
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
	t.Lock()
	defer t.Unlock()
	if !t.editable() {
		return ErrSnapshotNotEditable
	}
	if root == nil {
		return fmt.Errorf("can not SetRoot with nil root")
	}
	if _, err := wTx.Get(root); err != nil {
		return fmt.Errorf("could not SetRoot to %x,"+
			" as it does not exist in the db: %w", root, err)
	}
	return wTx.Set(dbKeyRoot, root)
}

// Snapshot returns a read-only copy of the Tree from the given root.
// If no root is given, the current root is used.
// The provided root must be a valid existing intermediate node in the tree.
// The list of roots for a level can be obtained using tree.RootsFromLevel().
func (t *Tree) Snapshot(fromRoot []byte) (*Tree, error) {
	t.Lock()
	defer t.Unlock()
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
// The iteration starts from the given root. If no root is given, the current
// root is used. The provided root must be a valid existing intermediate node in
// the tree.
func (t *Tree) Iterate(fromRoot []byte, f func([]byte, []byte)) error {
	return t.IterateWithTx(t.db, fromRoot, f)
}

// IterateWithTx does the same than the Iterate method, but allowing to pass
// the db.ReadTx that is used.
// The iteration starts from the given root. If no root is given, the current
// root is used. The provided root must be a valid existing intermediate node in
// the tree.
func (t *Tree) IterateWithTx(rTx db.Reader, fromRoot []byte, f func([]byte, []byte)) error {
	// allow to define which root to use
	if fromRoot == nil {
		var err error
		fromRoot, err = t.RootWithTx(rTx)
		if err != nil {
			return err
		}
	}
	t.Lock()
	defer t.Unlock()
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
func (t *Tree) IterateWithStopWithTx(rTx db.Reader, fromRoot []byte, f func(int, []byte, []byte) bool) error {
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

func (t *Tree) iterWithStop(rTx db.Reader, k []byte, currLevel int, f func(int, []byte, []byte) bool) error {
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

// Dump exports all the Tree leafs in a byte array.
// The provided root must be a valid existing intermediate node in the tree.
// Or nil to use the current root.
func (t *Tree) Dump(fromRoot []byte) ([]byte, error) {
	return t.dump(fromRoot, nil)
}

// DumpWriter exports all the Tree leafs writing the bytes in the given Writer.
// The provided root must be a valid existing intermediate node in the tree.
// Or nil to use the current root.
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
	wTx := t.db.WriteTx()
	defer wTx.Discard()

	if err := t.ImportDumpReaderWithTx(wTx, r); err != nil {
		return err
	}
	return wTx.Commit()
}

// ImportDumpReaderWithTx imports the leafs (that have been exported with the Dump
// method) in the Tree (using the given db.WriteTx), reading them from the given reader.
func (t *Tree) ImportDumpReaderWithTx(wTx db.WriteTx, r io.Reader) error {
	if !t.editable() {
		return ErrSnapshotNotEditable
	}

	// create the root node if it does not exist
	_, err := wTx.Get(dbKeyRoot)
	if err == db.ErrKeyNotFound {
		// store new root 0 (empty)
		if err := t.setToEmptyTree(wTx); err != nil {
			return fmt.Errorf("could not set empty tree: %w", err)
		}
	} else if err != nil {
		return fmt.Errorf("unknown database error: %w", err)
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

	invalid, err := t.AddBatchWithTx(wTx, keys, values)
	if err != nil {
		return fmt.Errorf("error adding batch: %w", err)
	}
	if len(invalid) > 0 {
		return fmt.Errorf("%d invalid keys found in batch", len(invalid))
	}
	return nil
}

// Graphviz iterates across the full tree to generate a string Graphviz
// representation of the tree and writes it to w
func (t *Tree) Graphviz(w io.Writer, fromRoot []byte) error {
	return t.GraphvizFirstNLevels(t.db, w, fromRoot, t.maxLevels)
}

// GraphvizFirstNLevels iterates across the first NLevels of the tree to
// generate a string Graphviz representation of the first NLevels of the tree
// and writes it to w
func (t *Tree) GraphvizFirstNLevels(rTx db.Reader, w io.Writer, fromRoot []byte, untilLvl int) error {
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
	err := t.iterWithStop(rTx, fromRoot, 0, func(currLvl int, k, v []byte) bool {
		if currLvl == untilLvl {
			return true // to stop the iter from going down
		}
		firstChars := func(b []byte) string {
			if len(b) > nChars {
				return hex.EncodeToString(b[:nChars])
			}
			return hex.EncodeToString(b)
		}
		switch v[0] {
		case PrefixValueEmpty:
		case PrefixValueLeaf:
			fmt.Fprintf(w, "\"%v\" [style=filled];\n", firstChars(k))
			// key & value from the leaf
			kB, vB := ReadLeafValue(v)
			fmt.Fprintf(w, "\"%v\" -> {\"k:%v\\nv:%v\"}\n",
				firstChars(k), firstChars(kB),
				firstChars(vB))
			fmt.Fprintf(w, "\"k:%v\\nv:%v\" [style=dashed]\n",
				firstChars(kB), firstChars(vB))
		case PrefixValueIntermediate:
			l, r := ReadIntermediateChilds(v)
			lStr := firstChars(l)
			rStr := firstChars(r)
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
			fmt.Fprintf(w, "\"%v\" -> {\"%v\" \"%v\"}\n", firstChars(k),
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
	return t.PrintGraphvizFirstNLevels(t.db, fromRoot, t.maxLevels)
}

// PrintGraphvizFirstNLevels prints the output of Tree.GraphvizFirstNLevels
func (t *Tree) PrintGraphvizFirstNLevels(rTx db.Reader, fromRoot []byte, untilLvl int) error {
	if fromRoot == nil {
		var err error
		fromRoot, err = t.RootWithTx(rTx)
		if err != nil {
			return err
		}
	}
	if untilLvl == 0 {
		untilLvl = t.maxLevels
	}
	w := bytes.NewBufferString("")
	fmt.Fprintf(w,
		"--------\nGraphviz of the Tree with Root "+hex.EncodeToString(fromRoot)+":\n")
	err := t.GraphvizFirstNLevels(rTx, w, fromRoot, untilLvl)
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

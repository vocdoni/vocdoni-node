/*
Package statedb contains the implementation of StateDB, a database backed
structure that holds the state of the blockchain indexed by version (each
version corresponding to the state at each block).  The StateDB holds a dynamic
hierarchy of linked merkle trees starting with the mainTree on top, with the
property that the keys and values of all merkle trees can be cryptographically
represented by a single hash, the StateDB.Hash (which corresponds to the
mainTree.Root).

Internally all subTrees of the StateDB use the same database (for views) and
the same transaction (for a block update).  Database prefixes are used to split
the storage of each subTree while avoiding collisions.  The structure of
prefixes is detailed here:
- subTree: `{KindID}{id}/`
  - arbo.Tree: `t/`
  - subTrees: `s/`
  - contains any number of subTree
  - nostate: `n/`
  - metadata: `m/`
  - versions: `v/` (only in mainTree)
  - currentVersion: `current` -> {currentVersion}
  - version to root: `{version}` -> `{root}`

Since the mainTree is unique and doesn't have a parent, the prefixes used in
the mainTree skip the first element of the path (`{KindID}{id}/`).
- Example:
  - mainTree arbo.Tree: `t/`
  - processTree arbo.Tree (a singleton under mainTree): `s/process/t/`
  - censusTree arbo.Tree (a non-singleton under processTree):
    `s/process/s/census{pID}/t/` (we are using pID, the processID as the id
    of the subTree)
  - voteTree arbo.Tree (a non-singleton under processTree):
    `s/process/s/vote{pID}/t/` (we are using pID, the processID as the id
    of the subTree)

Each tree has an associated database that can be accessed via the NoState
method.  These databases are auxiliary key-values that don't belong to the
blockchain state, and thus any value in the NoState databases won't be
reflected in the StateDB.Hash.  One of the usecases for the NoState database is
to store auxiliary mappings used to optimize the capacity usage of merkletrees
used for zkSNARKS, where the number of levels is of critical matter.  For
example, we may want to build a census tree of babyjubjub public keys that will
be used to prove ownership of the public key via a SNARK in order to vote.  If
we build the merkle tree using the public key as path, we will have an
unbalanced tree which requires more levels than strictly necessary.  On the
other hand, if we use a sequential index as path and set the value to the
public key, we achieve maximum balancing reducing the number of tree levels.
But then we can't easily query for the existence of a public key in the tree to
generate a proof, as it requires to know its index.  In such a case, we can
store the mapping of public key to index in the NoState database.
*/
package statedb

import (
	"bytes"
	"encoding/binary"
	"errors"
	"io"
	"path"
	"sync"

	"go.vocdoni.io/dvote/db"
	"go.vocdoni.io/dvote/db/prefixeddb"
	"go.vocdoni.io/dvote/tree"
	"go.vocdoni.io/dvote/tree/arbo"
)

const (
	subKeyTree    = "t"
	subKeyMeta    = "m"
	subKeyNoState = "n"
	subKeySubTree = "s"
)

const pathVersion = "v"

var keyCurVersion = []byte("current")

// uint32ToBytes encodes an uint32 as a little endian in []byte
func uint32ToBytes(v uint32) []byte {
	b := make([]byte, 4)
	binary.LittleEndian.PutUint32(b, v)
	return b
}

// subDB returns a db.Database prefixed with `path | '/'`
func subDB(db db.Database, path string) db.Database {
	return prefixeddb.NewPrefixedDatabase(db, []byte(path+"/"))
}

// subWriteTx returns a db.WriteTx prefixed with `path | '/'`
func subWriteTx(tx db.WriteTx, path string) db.WriteTx {
	return prefixeddb.NewPrefixedWriteTx(tx, []byte(path+"/"))
}

// subReader returns a db.ReadTx prefixed with `path | '/'`
func subReader(tx db.Reader, path string) db.Reader {
	return prefixeddb.NewPrefixedReader(tx, []byte(path+"/"))
}

// GetRootFn is a function type that takes a leaf value and returns the contained root.
type GetRootFn func(value []byte) ([]byte, error)

// SetRootFn is a function type that takes a leaf value and a root, updates the
// leaf value with the new root and returns it.
type SetRootFn func(value []byte, root []byte) ([]byte, error)

// TreeParams are the parameters used in the constructor for a tree configuration.
type TreeParams struct {
	// HashFunc is the hash function used in the merkle tree
	HashFunc arbo.HashFunction
	// KindID is a unique identifier that specifies what kind of tree this
	// is.  Different kind of trees under the same parent tree must have
	// different KindIDs, as this parameter is used to build a disjoint
	// database prefix for each subTree under the same parent tree.
	KindID string
	// MaxLevels is the maximum number of merkle tree levels allowed.
	MaxLevels int
	// ParentLeafGetRoot is the function that takes a leaf value of the
	// parent at which this tree hangs, and returns the root of this tree.
	ParentLeafGetRoot GetRootFn
	// ParentLeafSetRoot is the function that takes a leaf value of the
	// parent at which this tree hangs, and updates it (returning it) with
	// the new root of this tree.
	ParentLeafSetRoot SetRootFn
}

// TreeNonSingletonConfig contains the configuration used for a non-singleton subTree.
type TreeNonSingletonConfig struct {
	hashFunc          arbo.HashFunction
	kindID            string
	parentLeafGetRoot GetRootFn
	parentLeafSetRoot SetRootFn
	maxLevels         int
}

// NewTreeNonSingletonConfig creates a new configuration for a non-singleton subTree.
func NewTreeNonSingletonConfig(params TreeParams) *TreeNonSingletonConfig {
	return &TreeNonSingletonConfig{
		hashFunc:          params.HashFunc,
		kindID:            params.KindID,
		parentLeafGetRoot: params.ParentLeafGetRoot,
		parentLeafSetRoot: params.ParentLeafSetRoot,
		maxLevels:         params.MaxLevels,
	}
}

// HashFunc returns the hashFunc set for this SubTreeConfig
func (c *TreeNonSingletonConfig) HashFunc() arbo.HashFunction {
	return c.hashFunc
}

// WithKey returns a unified subTree configuration type for opening a singleton
// subTree that is identified by `key`.  `key` is the path in the parent tree
// to the leaf that contains the subTree root.
func (c *TreeNonSingletonConfig) WithKey(key []byte) TreeConfig {
	return TreeConfig{
		parentLeafKey:          key,
		prefix:                 c.kindID + string(key),
		TreeNonSingletonConfig: c,
	}
}

// TreeConfig contains the configuration used for a subTree.
type TreeConfig struct {
	parentLeafKey []byte
	prefix        string
	*TreeNonSingletonConfig
}

// NewTreeSingletonConfig creates a new configuration for a singleton subTree.
func NewTreeSingletonConfig(params TreeParams) TreeConfig {
	return TreeConfig{
		parentLeafKey:          []byte(params.KindID),
		prefix:                 params.KindID,
		TreeNonSingletonConfig: NewTreeNonSingletonConfig(params),
	}
}

// Key returns the key used in the parent tree in which the value that contains
// the subTree root is stored.  The key is the path of the parent leaf with the root.
func (c *TreeConfig) Key() []byte {
	return []byte(c.kindID)
}

// HashFunc returns the hashFunc set for this SubTreeSingleConfig
func (c *TreeConfig) HashFunc() arbo.HashFunction {
	return c.hashFunc
}

// MainTreeCfg is the subTree configuration of the mainTree.  It doesn't have a
// kindID because it's the top level tree.  For the same reason, it doesn't
// contain functions to work with the parent leaf: it doesn't have a parent.
var MainTreeCfg = NewTreeSingletonConfig(TreeParams{
	HashFunc:          arbo.HashFunctionSha256,
	KindID:            "",
	MaxLevels:         256,
	ParentLeafGetRoot: nil,
	ParentLeafSetRoot: nil,
})

// StateDB is a database backed structure that holds a dynamic hierarchy of
// linked merkle trees with the property that the keys and values of all merkle
// trees can be cryptographically represented by a single hash, the
// StateDB.Root (which corresponds to the mainTree.Root).
type StateDB struct {
	hashLen        int
	db             db.Database
	NoStateWriteTx db.WriteTx
	NoStateReadTx  db.Reader
}

// New returns an instance of the StateDB.
func New(db db.Database) *StateDB {
	return &StateDB{
		hashLen: MainTreeCfg.hashFunc.Len(),
		db:      db,
	}
}

// setVersionRoot is a helper function used to set the last version and its
// corresponding root from the top level tx.
func setVersionRoot(tx db.WriteTx, version uint32, root []byte) error {
	txMetaVer := subWriteTx(tx, path.Join(subKeyMeta, pathVersion))
	if err := txMetaVer.Set(uint32ToBytes(version), root); err != nil {
		return err
	}
	return txMetaVer.Set(keyCurVersion, uint32ToBytes(version))
}

// getVersionRoot is a helper function used get the last version from the top
// level tx.
func getVersion(tx db.Reader) (uint32, error) {
	versionLE, err := subReader(tx, path.Join(subKeyMeta, pathVersion)).Get(keyCurVersion)
	if errors.Is(err, db.ErrKeyNotFound) {
		return 0, nil
	} else if err != nil {
		return 0, err
	}
	return binary.LittleEndian.Uint32(versionLE), nil
}

// getVersionRoot is a helper function used get the root of version from the
// top level tx.
func (s *StateDB) getVersionRoot(tx db.Reader, version uint32) ([]byte, error) {
	root, err := subReader(tx, path.Join(subKeyMeta, pathVersion)).Get(uint32ToBytes(version))
	if errors.Is(err, db.ErrKeyNotFound) && version == 0 {
		return make([]byte, s.hashLen), nil
	} else if err != nil {
		return nil, err
	}
	return root, nil
}

// getRoot is a helper function used to get the last version's root from the
// top level tx.
func (s *StateDB) getRoot(tx db.Reader) ([]byte, error) {
	version, err := getVersion(tx)
	if err != nil {
		return nil, err
	}
	return s.getVersionRoot(tx, version)
}

// Version returns the last committed version.  Calling Version on a fresh
// StateDB will return 0.
func (s *StateDB) Version() (uint32, error) {
	return getVersion(s.db)
}

// VersionRoot returns the StateDB root corresponding to the version v.  A new
// StateDB always has the version 0 with root == emptyHash.
func (s *StateDB) VersionRoot(v uint32) ([]byte, error) {
	return s.getVersionRoot(s.db, v)
}

// Hash returns the cryptographic summary of the StateDB, which corresponds to
// the root of the mainTree.  This hash cryptographically represents the entire
// StateDB (except for the NoState databases).
func (s *StateDB) Hash() ([]byte, error) {
	return s.getRoot(s.db)
}

// BeginTx creates a new transaction for the StateDB to begin an update, with
// the mainTree opened for update wrapped in the returned TreeTx.  You must
// either call treeTx.Commit or treeTx.Discard if BeginTx doesn't return an
// error.  Calling treeTx.Discard after treeTx.Commit is ok.
func (s *StateDB) BeginTx() (treeTx *TreeTx, err error) {
	cfg := MainTreeCfg
	// NOTE(Edu): The introduction of Batched Txs here came from the fact
	// that Badger takes a lot of memory and as a preconfigured maximum
	// memory allocated for a Tx, which we were easily reaching.  But by
	// using Batched Txs we didn't have atomic updates for the StateDB.
	// Unfortunately I have been using the NoState assuming atomic updates,
	// and using NoState without assuming atomic updates is cumbersome, so
	// I'm switching back to a regular WriteTx now that we are using Pebble
	// which can grow the memory allocated to a Tx dynamically and uses
	// much less memory.  Based on the MemoryUsage results from the
	// `TestBlockMemoryUsage` in `vochain/state_test.go` this should be
	// reasonable.
	// tx := db.NewBatch(s.db)
	tx := s.db.WriteTx()
	defer func() {
		if err != nil {
			tx.Discard()
		}
	}()
	root, err := s.getRoot(tx)
	if err != nil {
		return nil, err
	}
	txTree := subWriteTx(tx, subKeyTree)
	tree, err := tree.New(txTree,
		tree.Options{DB: nil, MaxLevels: cfg.maxLevels, HashFunc: cfg.hashFunc})
	if err != nil {
		return nil, err
	}
	lastRoot, err := tree.Root(txTree)
	if err != nil {
		return nil, err
	}
	if !bytes.Equal(root, lastRoot) {
		if err := tree.SetRoot(txTree, root); err != nil {
			return nil, err
		}
	}
	s.NoStateReadTx = subReader(tx, subKeyNoState)
	s.NoStateWriteTx = subWriteTx(tx, subKeyNoState)
	return &TreeTx{
		sdb: s,
		TreeUpdate: TreeUpdate{
			tx: tx,
			tree: treeWithTx{
				Tree: tree,
				tx:   txTree,
			},
			cfg:      cfg,
			openSubs: sync.Map{},
		},
	}, nil
}

// readOnlyWriteTx is a wrapper over a db.ReadTx that implements the db.WriteTx
// methods but forbids them by returning errors.  This is used for functions
// that take a db.WriteTx but only write conditionally, and we want to detect a
// write attempt.
type readOnlyWriteTx struct {
	db.Reader
}

// ErrReadOnly is returned when a write operation is attempted on a db.WriteTx
// that we set up for read only.
var ErrReadOnly = errors.New("read only")

// ErrEmptyTree is returned when a tree is opened for read-only but hasn't been
// created yet.
var ErrEmptyTree = errors.New("empty tree")

// Set implements db.WriteTx.Set but returns error always.
func (*readOnlyWriteTx) Set(_ []byte, _ []byte) error {
	return ErrReadOnly
}

// Delete implements db.WriteTx.Delete but returns error always.
func (*readOnlyWriteTx) Delete(_ []byte) error {
	return ErrReadOnly
}

// Apply implements db.WriteTx.Apply but returns error always.
func (*readOnlyWriteTx) Apply(_ db.WriteTx) error {
	return ErrReadOnly
}

// Commit implements db.WriteTx.Commit but returns nil always.
func (*readOnlyWriteTx) Commit() error { return nil }

// Discard implements db.WriteTx.Discard as a no-op.
func (*readOnlyWriteTx) Discard() {}

// TreeView returns the mainTree opened at root as a TreeView for read-only.
// If root is nil, the last version's root is used.
func (s *StateDB) TreeView(root []byte) (*TreeView, error) {
	cfg := MainTreeCfg

	if root == nil {
		var err error
		if root, err = s.getRoot(s.db); err != nil {
			return nil, err
		}
	}

	txTree := subReader(s.db, subKeyTree)
	tree, err := tree.New(&readOnlyWriteTx{txTree},
		tree.Options{DB: subDB(s.db, subKeyTree), MaxLevels: cfg.maxLevels, HashFunc: cfg.hashFunc})
	if errors.Is(err, ErrReadOnly) {
		return nil, ErrEmptyTree
	} else if err != nil {
		return nil, err
	}
	tree, err = tree.FromRoot(root)
	if err != nil {
		return nil, err
	}
	return &TreeView{
		db:   s.db,
		tree: tree,
		cfg:  MainTreeCfg,
	}, nil
}

// Import imports the contents from r into a state tree
func (s *StateDB) Import(cfg TreeConfig, parent *TreeConfig, r io.Reader) (err error) {
	dbtx := s.db.WriteTx()
	defer func() {
		if err != nil {
			dbtx.Discard()
		}
	}()

	var txTree db.WriteTx
	if cfg.kindID == "" { // TreeMain
		txTree = subWriteTx(dbtx, subKeyTree)
	} else if parent != nil && parent.prefix != "" { // ChildTree (for example Votes)
		txParent := subWriteTx(dbtx, path.Join(subKeySubTree, parent.prefix))
		txChild := subWriteTx(txParent, path.Join(subKeySubTree, cfg.prefix))
		txTree = subWriteTx(txChild, subKeyTree)
	} else {
		tx := subWriteTx(dbtx, path.Join(subKeySubTree, cfg.prefix))
		txTree = subWriteTx(tx, subKeyTree)
	}
	tree, err := tree.New(txTree,
		tree.Options{DB: nil, MaxLevels: cfg.maxLevels, HashFunc: cfg.hashFunc})
	if err != nil {
		return err
	}
	if err := tree.ImportDumpReaderWithTx(txTree, r); err != nil {
		return err
	}

	return txTree.Commit()
}

package statedb

import (
	"path"

	"go.vocdoni.io/dvote/db"
	"go.vocdoni.io/dvote/tree"
)

// Viewer is an interface for a read-only key-value database.  This is similar
// to db.ReadTx, but without the Discard method.
type Viewer interface {
	Get(key []byte) ([]byte, error)
}

// databaseViewer is a wrapper over db.Database that implements Viewer.  We
// implement this viewer for db.Database to offer a Get method for easy queries
// without having to deal with the db.ReadTx externally.
type databaseViewer struct {
	db db.Database
}

// Get the key from the database.  Internally uses a new read transaction.
func (v *databaseViewer) Get(key []byte) ([]byte, error) {
	tx := v.db.ReadTx()
	defer tx.Discard()
	return tx.Get(key)
}

// TreeView is an opened tree that can only be read.
type TreeView struct {
	// db is db.Database where the contents of this TreeView are stored.
	// The db is already prefixed to avoid collision with parent and
	// sibling TreeUpdates.  From this db, the following prefixes are
	// available:
	// - arbo.Tree (used in TreeView.tree): `t/`
	// - subTrees (used in TreeView.subTree): `s/`
	// - nostate (used in TreeView.NoState): `n/`
	// - metadata: `m/`
	db db.Database
	// tree is the Arbo merkle tree in this TreeView.
	tree *tree.Tree
	// cfg points to this TreeView configuration.
	cfg *treeConfig
}

// NoState returns a read-only key-value database associated with this tree
// that doesn't affect the cryptographic integrity of the StateDB.
func (v *TreeView) NoState() Viewer {
	return &databaseViewer{
		db: subDB(v.db, subKeyNoState),
	}
}

// Get returns the value at key in this tree.  `key` is the path of the leaf,
// and the returned value is the leaf's value.
func (v *TreeView) Get(key []byte) ([]byte, error) {
	return v.tree.Get(nil, key)
}

// Iterate iterates over all nodes of this tree.
func (v *TreeView) Iterate(callback func(key, value []byte) bool) error {
	return v.tree.Iterate(nil, callback)
}

// Root returns the root of the tree, which cryptographically summarises the
// state of the tree.
func (v *TreeView) Root() ([]byte, error) {
	return v.tree.Root(nil)
}

// Size returns the number of leafs (key-values) that this tree contains.
func (v *TreeView) Size() (uint64, error) {
	// NOTE: Tree.Size is currently unimplemented
	return v.tree.Size(nil), nil
}

// GenProof generates a proof of existence of the given key for this tree.  The
// returned values are the leaf value and the proof itself.
func (v *TreeView) GenProof(key []byte) ([]byte, []byte, error) {
	return v.tree.GenProof(nil, key)
}

// Dump exports all the tree leafs in a byte array.
func (v *TreeView) Dump() ([]byte, error) {
	return v.tree.Dump()
}

// subTree is an internal function used to open the subTree (singleton and
// non-singleton) as a TreeView.  The treeView.db is created from
// v.db appending the prefix `subKeySubTree | cfg.prefix`.  In turn
// the treeView.db uses the db.Database from treeView.db appending the
// prefix `'/' | subKeyTree`.  The treeView.tree is opened as a snapshot from
// the root found in its parent leaf
func (v *TreeView) subTree(cfg *treeConfig) (treeView *TreeView, err error) {
	parentLeaf, err := v.tree.Get(nil, cfg.parentLeafKey)
	if err != nil {
		return nil, err
	}
	root, err := cfg.parentLeafGetRoot(parentLeaf)
	if err != nil {
		return nil, err
	}

	db := subDB(v.db, path.Join(subKeySubTree, cfg.prefix))
	tx := db.ReadTx()
	defer tx.Discard()
	txTree := subReadTx(tx, subKeyTree)
	defer txTree.Discard()
	tree, err := tree.New(&readOnlyWriteTx{txTree},
		tree.Options{DB: subDB(db, subKeyTree), MaxLevels: cfg.maxLevels, HashFunc: cfg.hashFunc})
	if err == ErrReadOnly {
		return nil, ErrEmptyTree
	} else if err != nil {
		return nil, err
	}
	tree, err = tree.FromRoot(root)
	if err != nil {
		return nil, err
	}
	return &TreeView{
		db:   db,
		tree: tree,
		cfg:  mainTreeCfg,
	}, nil
}

// SubTreeSingle returns a TreeView of a singleton SubTree whose root is stored
// in the leaf with `cfg.Key()`, and is parametrized by `cfg`.
func (v *TreeView) SubTreeSingle(c *SubTreeSingleConfig) (*TreeView, error) {
	return v.subTree(c.treeConfig())
}

// SubTree returns a TreeView of a non-singleton SubTree whose root is stored
// in the leaf with `key`, and is parametrized by `cfg`.
func (v *TreeView) SubTree(key []byte, c *SubTreeConfig) (*TreeView, error) {
	return v.subTree(c.treeConfig(key))
}

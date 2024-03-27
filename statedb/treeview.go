package statedb

import (
	"errors"
	"fmt"
	"io"
	"path"

	"go.vocdoni.io/dvote/db"
	"go.vocdoni.io/dvote/tree"
)

// TreeViewer groups the read-only methods that can be made on a subTree.
type TreeViewer interface {
	// Get returns the value at key in this tree.  `key` is the path of the leaf,
	// and the returned value is the leaf's value.
	Get(key []byte) ([]byte, error)
	// DeepGet allows performing a Get on a nested subTree by passing the list
	// of tree configurations and the key to get at the last subTree.
	DeepGet(key []byte, cfgs ...TreeConfig) ([]byte, error)
	// Iterate iterates over all leafs of this tree.  When callback returns true,
	// the iteration is stopped and this function returns.
	Iterate(callback func(key, value []byte) bool) error
	// Dump exports all the tree leafs into a writer buffer. The leaves can be read
	// again via Import.
	Dump(w io.Writer) error
	// Import reads exported tree leaves from r.
	Import(r io.Reader) error
	/// Root returns the root of the tree, which cryptographically summarises the
	// state of the tree.
	Root() ([]byte, error)
	// Size returns the number of leafs (key-values) that this tree contains.
	Size() (uint64, error)
	// GenProof generates a proof of existence of the given key for this tree.  The
	// returned values are the leaf value and the proof itself.
	GenProof(key []byte) ([]byte, []byte, error)
	// SubTree is used to open the subTree (singleton and non-singleton) as a
	// TreeView.
	SubTree(c TreeConfig) (TreeViewer, error)
	// DeepSubTree allows opening a nested subTree by passing the list of tree
	// configurations.
	DeepSubTree(cfgs ...TreeConfig) (TreeViewer, error)
	PrintGraphviz() error
}

var _ TreeViewer = (*TreeView)(nil)

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
	cfg TreeConfig
}

// noState returns a read-only key-value database associated with this tree
// that doesn't affect the cryptographic integrity of the StateDB.
func (v *TreeView) noState() db.Reader {
	return subReader(v.db, subKeyNoState)
}

// Get returns the value at key in this tree.  `key` is the path of the leaf,
// and the returned value is the leaf's value.
func (v *TreeView) Get(key []byte) ([]byte, error) {
	return v.tree.Get(nil, key)
}

// IterateNodes iterates over all nodes of this tree.  The key and value are
// the Arbo database representation of a node, and don't match the key value
// used in Get, Add and Set.  When callback returns true, the iteration is
// stopped and this function returns.
func (v *TreeView) IterateNodes(callback func(key, value []byte) bool) error {
	return v.tree.Iterate(nil, callback)
}

func (v *TreeView) PrintGraphviz() error {
	return v.tree.PrintGraphviz(v.db)
}

// Iterate iterates over all leafs of this tree.  When callback returns true,
// the iteration is stopped and this function returns.
func (v *TreeView) Iterate(callback func(key, value []byte) bool) error {
	return v.tree.IterateLeaves(nil, callback)
}

// Root returns the root of the tree, which cryptographically summarises the
// state of the tree.
func (v *TreeView) Root() ([]byte, error) {
	return v.tree.Root(nil)
}

// Size returns the number of leafs (key-values) that this tree contains.
func (v *TreeView) Size() (uint64, error) {
	return v.tree.Size(nil)
}

// GenProof generates a proof of existence of the given key for this tree.  The
// returned values are the leaf value and the proof itself.
func (v *TreeView) GenProof(key []byte) ([]byte, []byte, error) {
	return v.tree.GenProof(nil, key)
}

// Dump exports all the tree leafs.
func (v *TreeView) Dump(w io.Writer) error {
	return v.tree.DumpWriter(w)
}

// Import does nothing.
func (*TreeView) Import(_ io.Reader) error {
	return fmt.Errorf("tree is not writable")
}

// SubTree is used to open the subTree (singleton and non-singleton) as a
// TreeView.  The treeView.db is created from v.db appending the prefix
// `subKeySubTree | cfg.prefix`.  In turn the treeView.db uses the db.Database
// from treeView.db appending the prefix `'/' | subKeyTree`.  The treeView.tree
// is opened as a snapshot from the root found in its parent leaf
func (v *TreeView) SubTree(cfg TreeConfig) (treeView TreeViewer, err error) {
	parentLeaf, err := v.tree.Get(nil, cfg.parentLeafKey)
	if err != nil {
		return nil, err
	}
	root, err := cfg.parentLeafGetRoot(parentLeaf)
	if err != nil {
		return nil, err
	}

	db := subDB(v.db, path.Join(subKeySubTree, cfg.prefix))
	txTree := subReader(db, subKeyTree)
	tree, err := tree.New(&readOnlyWriteTx{txTree},
		tree.Options{DB: subDB(db, subKeyTree), MaxLevels: cfg.maxLevels, HashFunc: cfg.hashFunc})
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
		db:   db,
		tree: tree,
		cfg:  MainTreeCfg,
	}, nil
}

// DeepSubTree allows opening a nested subTree by passing the list of tree
// configurations.
func (v *TreeView) DeepSubTree(cfgs ...TreeConfig) (treeUpdate TreeViewer, err error) {
	var tree TreeViewer = v
	for _, cfg := range cfgs {
		if tree, err = tree.SubTree(cfg); err != nil {
			return nil, err
		}
	}
	return tree, nil
}

// DeepGet allows performing a Get on a nested subTree by passing the list
// of tree configurations and the key to get at the last subTree.
func (v *TreeView) DeepGet(key []byte, cfgs ...TreeConfig) ([]byte, error) {
	tree, err := v.DeepSubTree(cfgs...)
	if err != nil {
		return nil, err
	}
	return tree.Get(key)
}

package statedb

import (
	"path"

	"go.vocdoni.io/dvote/db"
	"go.vocdoni.io/dvote/tree"
)

// Updater is an interface for a read-write key-value database.  It extends
// Viewer with a Set method.  This is simmilar to db.WriteTx but without the
// Commit and Discard methods.  Any db.WriteTx implements Updater.
type Updater interface {
	Viewer
	Set(key, value []byte) error
}

// treeWithTx is an Arbo merkle tree with the db.WriteTx used for updating it.
type treeWithTx struct {
	*tree.Tree
	tx db.WriteTx
}

// TreeUpdate is an opened tree that can be updated.  All updates are stored in
// an internal transaction (shared with all the opened subTrees) and will only
// be commited with the TreeTx.Commit function is called.  The TreeUpdate is
// not safe for concurrent use.
type TreeUpdate struct {
	// tx is db.WriteTx where all the changes done to this TreeUpdate will
	// be applied.  The tx is already prefixed to avoid collision with
	// parent and sibling TreeUpdates.  From this tx, the following prefixes are available:
	// - arbo.Tree (used in TreeUpdate.tree): `t/`
	// - subTrees (used in TreeUpdate.subTree): `s/`
	// - nostate (used in TreeUpdate.NoState): `n/`
	// - metadata: `m/`
	tx db.WriteTx
	// dirtyTree is set to true when the TreeUpdate.tree has been modified.
	// If true, during the TreeTx.Commit operation, the root of this tree
	// will be propagated upwards to update the parent trees.
	dirtyTree bool
	// tree is the Arbo merkle tree (with the db.WriteTx) in this
	// TreeUpdate.
	tree treeWithTx
	// openSubs is a map of opened subTrees in a particular TreeTx whose
	// parent is this TreeUpdate.  The key is the subtree prefix.  This map
	// is used in the TreeTx.Commit to traverse all opened subTrees in a
	// TreeTx in order to propagate the roots upwards to update the
	// corresponding parent leafs up to the mainTree.
	openSubs map[string]*TreeUpdate
	// cfg points to this TreeUpdate configuration.
	cfg *treeConfig
}

// Get returns the value at key in this tree.  `key` is the path of the leaf,
// and the returned value is the leaf's value.
func (u *TreeUpdate) Get(key []byte) ([]byte, error) {
	return u.tree.Get(u.tree.tx, key)
}

// Iterate iterates over all nodes of this tree.
func (u *TreeUpdate) Iterate(callback func(key, value []byte) bool) error {
	return u.tree.Iterate(u.tree.tx, callback)
}

// Root returns the root of the tree, which cryptographically summarises the
// state of the tree.
func (u *TreeUpdate) Root() ([]byte, error) {
	return u.tree.Root(u.tree.tx)
}

// Size returns the number of leafs (key-values) that this tree contains.
func (u *TreeUpdate) Size() (uint64, error) {
	// NOTE: Tree.Size is currently unimplemented
	return u.tree.Size(u.tree.tx), nil
}

// GenProof generates a proof of existence of the given key for this tree.  The
// returned values are the leaf value and the proof itself.
func (u *TreeUpdate) GenProof(key []byte) ([]byte, []byte, error) {
	return u.tree.GenProof(u.tree.tx, key)
}

// Unimplemented because arbo.Tree.Dump doesn't take db.ReadTx as input.
// func (u *TreeUpdate) Dump() ([]byte, error) {
// 	panic("TODO")
// }

// NoState returns a key-value database associated with this tree that doesn't
// affect the cryptographic integrity of the StateDB.  Writing to this database
// won't change the StateDB.Root.
func (u *TreeUpdate) NoState() Updater {
	return subWriteTx(u.tx, subKeyNoState)
}

// Add a new key-value to this tree.  `key` is the path of the leaf, and
// `value` is the content of the leaf.
func (u *TreeUpdate) Add(key, value []byte) error {
	u.dirtyTree = true
	return u.tree.Add(u.tree.tx, key, value)
}

// Set adds or updates a key-value in this tree.  `key` is the path of the
// leaf, and `value` is the content of the leaf.
func (u *TreeUpdate) Set(key, value []byte) error {
	u.dirtyTree = true
	return u.tree.Set(u.tree.tx, key, value)
}

// subTree is an internal function used to open the subTree (singleton and
// non-singleton) as a TreeUpdate.  The treeUpdate.tx is created from
// u.tx appending the prefix `subKeySubTree | cfg.prefix`.  In turn
// the treeUpdate.tree uses the db.WriteTx from treeUpdate.tx appending the
// prefix `'/' | subKeyTree`.
func (u *TreeUpdate) subTree(cfg *treeConfig) (treeUpdate *TreeUpdate, err error) {
	if treeUpdate, ok := u.openSubs[string(cfg.prefix)]; ok {
		return treeUpdate, nil
	}
	tx := subWriteTx(u.tx, path.Join(subKeySubTree, cfg.prefix))
	defer func() {
		if err != nil {
			tx.Discard()
		}
	}()
	txTree := subWriteTx(tx, subKeyTree)
	tree, err := tree.New(txTree,
		tree.Options{DB: nil, MaxLevels: cfg.maxLevels, HashFunc: cfg.hashFunc})
	if err != nil {
		return nil, err
	}
	treeUpdate = &TreeUpdate{
		tx: tx,
		tree: treeWithTx{
			Tree: tree,
			tx:   txTree,
		},
		openSubs: make(map[string]*TreeUpdate),
		cfg:      cfg,
	}
	u.openSubs[string(cfg.prefix)] = treeUpdate
	return treeUpdate, nil
}

// SubTreeSingle returns a TreeUpdate of a singleton SubTree whose root is stored
// in the leaf with `cfg.Key()`, and is parametrized by `cfg`.
func (u *TreeUpdate) SubTreeSingle(cfg *SubTreeSingleConfig) (*TreeUpdate, error) {
	return u.subTree(cfg.treeConfig())
}

// SubTree returns a TreeUpdate of a non-singleton SubTree whose root is stored
// in the leaf with `key`, and is parametrized by `cfg`.
func (u *TreeUpdate) SubTree(key []byte, cfg *SubTreeConfig) (*TreeUpdate, error) {
	return u.subTree(cfg.treeConfig(key))
}

// TreeTx is a wrapper over TreeUpdate that includes the Commit and Discard
// methods to control the transaction used to update the StateDB.  It contains
// the mainTree opened in the wrapped TreeUpdate.  The TreeTx is not safe for
// concurent use.
type TreeTx struct {
	sdb *StateDB
	// TreeUpdate contains the mainTree opened for updates.
	TreeUpdate
}

// update is a helper struct used to collect subTree updates that need to
// update the parent's corresponding leaf with the new root.
type update struct {
	setRoot SetRootFn
	root    []byte
}

// propagateRoot performs a Depth-First Search on the opened subTrees,
// propagating the roots and updating the parent leaves when the trees are
// dirty.  Only when the treeUpdate.tree is not dirty, and no open subTrees
// (recursively) are dirty we can skip propagating (and thus skip updating the
// treeUpdate.tree, avoiding recalculating hashes unnecessarily).
func propagateRoot(treeUpdate *TreeUpdate) ([]byte, error) {
	// End of recursion
	if len(treeUpdate.openSubs) == 0 {
		// If tree is not dirty, there's nothing to propagate
		if !treeUpdate.dirtyTree {
			return nil, nil
		} else {
			return treeUpdate.tree.Root(treeUpdate.tree.tx)
		}
	}
	// Gather all the updates that need to be applied to the
	// treeUpdate.tree leaves, by leaf key
	updatesByKey := make(map[string][]update)
	for _, sub := range treeUpdate.openSubs {
		root, err := propagateRoot(sub)
		if err != nil {
			return nil, err
		}
		if root == nil {
			continue
		}
		key := sub.cfg.parentLeafKey
		updatesByKey[string(key)] = append(updatesByKey[string(key)], update{
			// key:     key,
			setRoot: sub.cfg.parentLeafSetRoot,
			root:    root,
		})
	}
	// If there are no updates for treeUpdate.tree leaves, and treeUpdate
	// is not dirty, there's nothing to propagate
	if len(updatesByKey) == 0 && !treeUpdate.dirtyTree {
		return nil, nil
	}
	// Apply the updates to treeUpdate.tree leaves.  There can be multiple
	// updates for a single leaf, so we apply them all and then update the
	// leaf once.
	for key, updates := range updatesByKey {
		leaf, err := treeUpdate.tree.Get(treeUpdate.tree.tx, []byte(key))
		if err != nil {
			return nil, err
		}
		for _, update := range updates {
			leaf, err = update.setRoot(leaf, update.root)
			if err != nil {
				return nil, err
			}
		}
		if err := treeUpdate.tree.Set(treeUpdate.tree.tx, []byte(key), leaf); err != nil {
			return nil, err
		}
	}
	// We either updated leaves here, or treeUpdate.tree was dirty, so we
	// propagate the root to trigger updates in the parent tree.
	return treeUpdate.tree.Root(treeUpdate.tree.tx)
}

// Commit will write all the changes made from the TreeTx into the database,
// propagating the roots of dirtiy subTrees up to the mainTree so that a new
// Hash/Root (mainTree.Root == StateDB.Root) is calculated representing the
// state.
func (t *TreeTx) Commit() error {
	root, err := propagateRoot(&t.TreeUpdate)
	if err != nil {
		return err
	}
	t.openSubs = make(map[string]*TreeUpdate)
	version, err := getVersion(t.tx)
	if err != nil {
		return err
	}
	// If root is nil, it means that there were no updates to the StateDB,
	// so the next version root is the current version root.
	if root == nil {
		if root, err = t.sdb.getVersionRoot(t.tx, version); err != nil {
			return err
		}
	}
	if err := setVersionRoot(t.tx, version+1, root); err != nil {
		return err
	}
	return t.tx.Commit()
}

// Discard all the changes that have been made from the TreeTx.  After calling
// Discard, the TreeTx shouldn't no longer be used.
func (t *TreeTx) Discard() {
	t.tx.Discard()
}

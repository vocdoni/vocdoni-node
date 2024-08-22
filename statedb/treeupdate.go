package statedb

import (
	"fmt"
	"io"
	"path"
	"sync"

	"go.vocdoni.io/dvote/db"
	"go.vocdoni.io/dvote/tree"
)

// Updater is an interface for a read-write key-value database.  It extends
// Viewer with a Set method.  This is similar to db.WriteTx but without the
// Commit and Discard methods.  Any db.WriteTx implements Updater.
type Updater interface {
	db.Reader
	Set(key, value []byte) error
	Delete(key []byte) error
}

// treeWithTx is an Arbo merkle tree with the db.WriteTx used for updating it.
type treeWithTx struct {
	*tree.Tree
	tx db.WriteTx
}

// TreeUpdate is an opened tree that can be updated.  All updates are stored in
// an internal transaction (shared with all the opened subTrees) and will only
// be committed with the TreeTx.Commit function is called.  The TreeUpdate is
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
	// openSubs map[string]*TreeUpdate
	openSubs sync.Map
	// cfg points to this TreeUpdate configuration.
	cfg TreeConfig
}

// Get returns the value at key in this tree.  `key` is the path of the leaf,
// and the returned value is the leaf's value.
func (u *TreeUpdate) Get(key []byte) ([]byte, error) {
	return u.tree.Get(u.tree.tx, key)
}

// IterateNodes iterates over all nodes of this tree.  The key and value are
// the Arbo database representation of a node, and don't match the key value
// used in Get, Add and Set.
func (u *TreeUpdate) IterateNodes(callback func(key, value []byte) bool) error {
	return u.tree.Iterate(u.tree.tx, callback)
}

// Iterate iterates over all leafs of this tree.  When callback returns true,
// the iteration is stopped and this function returns.
func (u *TreeUpdate) Iterate(callback func(key, value []byte) bool) error {
	return u.tree.IterateLeaves(u.tree.tx, callback)
}

// Root returns the root of the tree, which cryptographically summarises the
// state of the tree.
func (u *TreeUpdate) Root() ([]byte, error) {
	return u.tree.Root(u.tree.tx)
}

// Size returns the number of leafs (key-values) that this tree contains.
func (u *TreeUpdate) Size() (uint64, error) {
	// NOTE: Tree.Size is currently unimplemented
	return u.tree.Size(u.tree.tx)
}

// GenProof generates a proof of existence of the given key for this tree.  The
// returned values are the leaf value and the proof itself.
func (u *TreeUpdate) GenProof(key []byte) ([]byte, []byte, error) {
	return u.tree.GenProof(u.tree.tx, key)
}

// Dump exports all the tree leafs.
// Unimplemented because arbo.Tree.Dump doesn't take db.ReadTx as input.
func (u *TreeUpdate) Dump(w io.Writer) error {
	return u.tree.DumpWriter(w)
}

// Import writes the content exported with Dump.
func (u *TreeUpdate) Import(r io.Reader) error {
	return u.tree.ImportDumpReaderWithTx(u.tx, r)
}

// noState returns a key-value database associated with this tree that doesn't
// affect the cryptographic integrity of the StateDB.  Writing to this database
// won't change the StateDB.Root.
func (u *TreeUpdate) noState() Updater {
	return subWriteTx(u.tx, subKeyNoState)
}

// MarkDirty sets dirtyTree = true
func (u *TreeUpdate) MarkDirty() {
	u.dirtyTree = true
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

// Del removes a key-value from this tree.  `key` is the path of the leaf.
func (u *TreeUpdate) Del(key []byte) error {
	u.dirtyTree = true
	return u.tree.Del(u.tree.tx, key)
}

// PrintGraphviz prints the tree in Graphviz format.
func (u *TreeUpdate) PrintGraphviz() error {
	return u.tree.PrintGraphviz(u.tree.tx)
}

// SubTree is used to open the subTree (singleton and non-singleton) as a
// TreeUpdate.  The treeUpdate.tx is created from u.tx appending the prefix
// `subKeySubTree | cfg.prefix`.  In turn the treeUpdate.tree uses the
// db.WriteTx from treeUpdate.tx appending the prefix `'/' | subKeyTree`.
func (u *TreeUpdate) SubTree(cfg TreeConfig) (treeUpdate *TreeUpdate, err error) {
	if treeUpdate, ok := u.openSubs.Load(cfg.prefix); ok {
		return treeUpdate.(*TreeUpdate), nil
	}
	tx := subWriteTx(u.tx, path.Join(subKeySubTree, cfg.prefix))
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
		openSubs: sync.Map{},
		cfg:      cfg,
	}
	u.openSubs.Store(cfg.prefix, treeUpdate)
	return treeUpdate, nil
}

// DeepSubTree allows opening a nested subTree by passing the list of tree
// configurations.
func (u *TreeUpdate) DeepSubTree(cfgs ...TreeConfig) (treeUpdate *TreeUpdate, err error) {
	tree := u
	for _, cfg := range cfgs {
		if tree, err = tree.SubTree(cfg); err != nil {
			return nil, err
		}
	}
	return tree, nil
}

// DeepGet allows performing a Get on a nested subTree by passing the list
// of tree configurations and the key to get at the last subTree.
func (u *TreeUpdate) DeepGet(key []byte, cfgs ...TreeConfig) ([]byte, error) {
	tree, err := u.DeepSubTree(cfgs...)
	if err != nil {
		return nil, err
	}
	return tree.Get(key)
}

// DeepAdd allows performing an Add on a nested subTree by passing the list
// of tree configurations and the key, value to add on the last subTree.
func (u *TreeUpdate) DeepAdd(key, value []byte, cfgs ...TreeConfig) error {
	tree, err := u.DeepSubTree(cfgs...)
	if err != nil {
		return err
	}
	return tree.Add(key, value)
}

// DeepSet allows performing a Set on a nested subTree by passing the list
// of tree configurations and the key, value to set on the last subTree.
func (u *TreeUpdate) DeepSet(key, value []byte, cfgs ...TreeConfig) error {
	tree, err := u.DeepSubTree(cfgs...)
	if err != nil {
		return err
	}
	return tree.Set(key, value)
}

// TreeTx is a wrapper over TreeUpdate that includes the Commit and Discard
// methods to control the transaction used to update the StateDB.  It contains
// the mainTree opened in the wrapped TreeUpdate.  The TreeTx is not safe for
// concurrent use.
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
	emptySubs := true
	treeUpdate.openSubs.Range(func(k, value any) bool {
		emptySubs = false
		return false
	})
	if emptySubs {
		// If tree is not dirty, there's nothing to propagate
		if !treeUpdate.dirtyTree {
			return nil, nil
		}
		return treeUpdate.tree.Root(treeUpdate.tree.tx)
	}
	// Gather all the updates that need to be applied to the
	// treeUpdate.tree leaves, by leaf key
	updatesByKey := make(map[string][]update)
	var gerr error
	treeUpdate.openSubs.Range(func(k, v any) bool {
		sub := v.(*TreeUpdate)
		root, err := propagateRoot(sub)
		if err != nil {
			gerr = err
			return false
		}
		if root == nil {
			gerr = err
			return true
		}
		key := sub.cfg.parentLeafKey
		updatesByKey[string(key)] = append(updatesByKey[string(key)], update{
			// key:     key,
			setRoot: sub.cfg.parentLeafSetRoot,
			root:    root,
		})
		return true
	})
	if gerr != nil {
		return nil, gerr
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
			return nil, fmt.Errorf("failed to get leaf %x: %w", key, err)
		}
		for _, update := range updates {
			leaf, err = update.setRoot(leaf, update.root)
			if err != nil {
				return nil, fmt.Errorf("failed to set root for leaf %x: %w", key, err)
			}
		}
		if err := treeUpdate.tree.Set(treeUpdate.tree.tx, []byte(key), leaf); err != nil {
			return nil, fmt.Errorf("failed to set leaf %x: %w", key, err)
		}
	}
	// We either updated leaves here, or treeUpdate.tree was dirty, so we
	// propagate the root to trigger updates in the parent tree.
	return treeUpdate.tree.Root(treeUpdate.tree.tx)
}

// Commit will write all the changes made from the TreeTx into the database,
// propagating the roots of dirtiy subTrees up to the mainTree so that a new
// Hash/Root (mainTree.Root == StateDB.Root) is calculated representing the
// state.  Parameter version sets the version used to index this update
// (identified by the mainTree root).  The specified version will be stored as
// the last version of the StateDB.  In general, Commits should use sequential
// version numbers, but overwriting an existing version can be useful in some
// cases (for example, overwriting version 0 to setup a genesis state).
func (t *TreeTx) Commit(version uint32) error {
	if err := t.CommitOnTx(version); err != nil {
		return err
	}
	return t.tx.Commit()
}

// CommitOnTx do as Commit but without committing the transaction to database.
// After CommitOnTx(), caller should call SaveWithoutCommit() to commit the transaction.
func (t *TreeTx) CommitOnTx(version uint32) error {
	root, err := propagateRoot(&t.TreeUpdate)
	if err != nil {
		return fmt.Errorf("could not propagate root: %w", err)
	}
	t.openSubs = sync.Map{}
	curVersion, err := getVersion(t.tx)
	if err != nil {
		return fmt.Errorf("could not get current version: %w", err)
	}
	// If root is nil, it means that there were no updates to the StateDB,
	// so the next version root is the current version root.
	if root == nil {
		if root, err = t.sdb.getVersionRoot(t.tx, curVersion); err != nil {
			return fmt.Errorf("could not get current version root: %w", err)
		}
	}
	if err := setVersionRoot(t.tx, version, root); err != nil {
		return fmt.Errorf("could not set version root: %w", err)
	}
	return nil
}

// Discard all the changes that have been made from the TreeTx.  After calling
// Discard, the TreeTx shouldn't no longer be used.
func (t *TreeTx) Discard() {
	t.tx.Discard()
}

// SaveWithoutCommit saves the changes made from the TreeTx without committing a new state.
// This is useful for testing purposes and for committing only nostate changes.
func (t *TreeTx) SaveWithoutCommit() error {
	return t.tx.Commit()
}

// treeUpdateView is a wrapper over TreeUpdate that fulfills the TreeViewer
// interface.
type treeUpdateView TreeUpdate

// SubTree implements the TreeViewer.SubTree method.
func (v *treeUpdateView) SubTree(c TreeConfig) (TreeViewer, error) {
	tu, err := (*TreeUpdate)(v).SubTree(c)
	return (*treeUpdateView)(tu), err
}

// DeepSubTree implements the TreeViewer.DeepSubTree method.
func (v *treeUpdateView) DeepSubTree(cfgs ...TreeConfig) (TreeViewer, error) {
	tu, err := (*TreeUpdate)(v).DeepSubTree(cfgs...)
	return (*treeUpdateView)(tu), err
}

// Get implements the TreeViewer.Get method.
func (v *treeUpdateView) Get(key []byte) ([]byte, error) { return (*TreeUpdate)(v).Get(key) }

// DeepGet implements the TreeViewer.DeepGet method.
func (v *treeUpdateView) DeepGet(key []byte, cfgs ...TreeConfig) ([]byte, error) {
	return (*TreeUpdate)(v).DeepGet(key, cfgs...)
}

// Iterate implements the TreeViewer.Iterate method.
func (v *treeUpdateView) Iterate(callback func(key, value []byte) bool) error {
	return (*TreeUpdate)(v).Iterate(callback)
}

// Root implements the TreeViewer.Root method.
func (v *treeUpdateView) Root() ([]byte, error) { return (*TreeUpdate)(v).Root() }

// Size implements the TreeViewer.Size method.
func (v *treeUpdateView) Size() (uint64, error) { return (*TreeUpdate)(v).Size() }

// GenProof implements the TreeViewer.GenProof method.
func (v *treeUpdateView) GenProof(key []byte) ([]byte, []byte, error) {
	return (*TreeUpdate)(v).GenProof(key)
}

// Dump exports all the tree leafs.
func (v *treeUpdateView) Dump(w io.Writer) error {
	return (*TreeUpdate)(v).Dump(w)
}

// Import writes all the tree leafs.
func (v *treeUpdateView) Import(r io.Reader) error {
	return (*TreeUpdate)(v).Import(r)
}

func (v *treeUpdateView) PrintGraphviz() error {
	return v.tree.PrintGraphviz(v.tx)
}

// verify that treeUpdateView fulfills the TreeViewer interface.
var _ TreeViewer = (*treeUpdateView)(nil)

// AsTreeView returns a read-only view of this subTree that fulfills the
// TreeViewer interface.
func (u *TreeUpdate) AsTreeView() TreeViewer {
	return (*treeUpdateView)(u)
}

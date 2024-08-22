package statedb

import (
	"encoding/binary"
	"fmt"
	"strings"
	"testing"

	qt "github.com/frankban/quicktest"
	"go.vocdoni.io/dvote/db"
	"go.vocdoni.io/dvote/db/metadb"
	"go.vocdoni.io/dvote/tree"
	"go.vocdoni.io/dvote/tree/arbo"
)

var emptyHash = make([]byte, 32)

func TestVersion(t *testing.T) {
	sdb := New(metadb.NewTest(t))
	version, err := sdb.Version()
	qt.Assert(t, err, qt.IsNil)
	qt.Assert(t, version, qt.Equals, uint32(0))

	root := make([]byte, 32)
	root[0] = 0x41
	wTx := sdb.db.WriteTx()
	qt.Assert(t, setVersionRoot(wTx, 1, root), qt.IsNil)
	qt.Assert(t, wTx.Commit(), qt.IsNil)

	version, err = sdb.Version()
	qt.Assert(t, err, qt.IsNil)
	qt.Assert(t, version, qt.Equals, uint32(1))

	root1, err := sdb.VersionRoot(1)
	qt.Assert(t, err, qt.IsNil)
	qt.Assert(t, root1, qt.DeepEquals, root)

	root1, err = sdb.Hash()
	qt.Assert(t, err, qt.IsNil)
	qt.Assert(t, root1, qt.DeepEquals, root)

	// dumpPrint(sdb.db)
}

func TestStateDB(t *testing.T) {
	sdb := New(metadb.NewTest(t))
	version, err := sdb.Version()
	qt.Assert(t, err, qt.IsNil)
	qt.Assert(t, version, qt.Equals, uint32(0))

	// Can't view because the tree doesn't exist yet
	{
		_, err := sdb.TreeView(nil)
		qt.Assert(t, err, qt.Equals, ErrEmptyTree)
	}

	mainTree, err := sdb.BeginTx()
	qt.Assert(t, err, qt.IsNil)

	// Initial root is emptyHash
	root0, err := mainTree.Root()
	qt.Assert(t, err, qt.IsNil)
	qt.Assert(t, root0, qt.DeepEquals, emptyHash)

	keys := [][]byte{[]byte("key0"), []byte("key1"), []byte("key2"), []byte("key3")}
	vals := [][]byte{[]byte("val0"), []byte("val1"), []byte("val2"), []byte("val3")}

	qt.Assert(t, mainTree.Add(keys[0], vals[0]), qt.IsNil)
	qt.Assert(t, mainTree.Add(keys[1], vals[1]), qt.IsNil)

	// Current root is != emptyHash
	root1, err := mainTree.Root()
	qt.Assert(t, err, qt.IsNil)
	qt.Assert(t, root1, qt.Not(qt.DeepEquals), emptyHash)

	qt.Assert(t, mainTree.Commit(1), qt.IsNil)

	// statedb.Root == mainTree.Root
	rootStateDB, err := sdb.Hash()
	qt.Assert(t, err, qt.IsNil)
	qt.Assert(t, rootStateDB, qt.DeepEquals, root1)

	// mainTreeView.Root = statedb.Root
	{
		mainTreeView, err := sdb.TreeView(nil)
		qt.Assert(t, err, qt.IsNil)
		rootView, err := mainTreeView.Root()
		qt.Assert(t, err, qt.IsNil)
		qt.Assert(t, rootView, qt.DeepEquals, root1)

		value, err := mainTreeView.Get(keys[0])
		qt.Assert(t, err, qt.IsNil)
		qt.Assert(t, value, qt.DeepEquals, vals[0])
	}

	// mainTreeView from root at version 0
	{
		mainTreeView, err := sdb.TreeView(root0)
		qt.Assert(t, err, qt.IsNil)
		rootView, err := mainTreeView.Root()
		qt.Assert(t, err, qt.IsNil)
		qt.Assert(t, rootView, qt.DeepEquals, root0)

		_, err = mainTreeView.Get(keys[0])
		qt.Assert(t, err, qt.Equals, arbo.ErrKeyNotFound)
	}

	// dumpPrint(sdb.db)

	// Begin another Tx
	mainTree, err = sdb.BeginTx()
	qt.Assert(t, err, qt.IsNil)

	root2, err := mainTree.Root()
	qt.Assert(t, err, qt.IsNil)
	qt.Assert(t, root2, qt.DeepEquals, root1)

	// Insert a new key-value
	qt.Assert(t, mainTree.Add(keys[2], vals[2]), qt.IsNil)

	// Uncommitted changes are not available in the View
	{
		mainTreeView, err := sdb.TreeView(nil)
		qt.Assert(t, err, qt.IsNil)

		_, err = mainTreeView.Get(keys[2])
		qt.Assert(t, err, qt.Equals, arbo.ErrKeyNotFound)
	}

	mainTree.Discard()

	// Expect Commit with no changes to work, StateDB.Root is not changed
	mainTree, err = sdb.BeginTx()
	qt.Assert(t, err, qt.IsNil)
	qt.Assert(t, mainTree.Commit(2), qt.IsNil)
	version, err = sdb.Version()
	qt.Assert(t, err, qt.IsNil)
	qt.Assert(t, version, qt.Equals, uint32(2))
	root2, err = sdb.Hash()
	qt.Assert(t, err, qt.IsNil)
	qt.Assert(t, root2, qt.DeepEquals, root1)
}

// singleCfg is a test configuration for a singleton subTree
var singleCfg = NewTreeSingletonConfig(TreeParams{
	HashFunc:  arbo.HashFunctionSha256,
	KindID:    "single",
	MaxLevels: 256,
	ParentLeafGetRoot: func(value []byte) ([]byte, error) {
		if len(value) != 32 {
			return nil, fmt.Errorf("len(value) = %v != 32", len(value))
		}
		return value, nil
	},
	ParentLeafSetRoot: func(value []byte, root []byte) ([]byte, error) {
		if len(value) != 32 {
			return nil, fmt.Errorf("len(value) = %v != 32", len(value))
		}
		return root, nil
	},
})

// multiACfg is a test configuration for a non-singleton subtTree whose root is
// stored in a leaf alongside another root (multiB)
var multiACfg = NewTreeNonSingletonConfig(TreeParams{
	HashFunc:  arbo.HashFunctionSha256,
	KindID:    "multia",
	MaxLevels: 256,
	ParentLeafGetRoot: func(value []byte) ([]byte, error) {
		if len(value) != 64 {
			return nil, fmt.Errorf("len(value) = %v != 64", len(value))
		}
		return value[:32], nil
	},
	ParentLeafSetRoot: func(value []byte, root []byte) ([]byte, error) {
		if len(value) != 64 {
			return nil, fmt.Errorf("len(value) = %v != 64", len(value))
		}
		copy(value[:32], root)
		return value, nil
	},
})

// multiBCfg is a test configuration for a non-singleton subtTree whose root is
// stored in a leaf alongside another root (multiA)
var multiBCfg = NewTreeNonSingletonConfig(TreeParams{
	HashFunc:  arbo.HashFunctionPoseidon,
	KindID:    "multib",
	MaxLevels: 256,
	ParentLeafGetRoot: func(value []byte) ([]byte, error) {
		if len(value) != 64 {
			return nil, fmt.Errorf("len(value) = %v != 64", len(value))
		}
		return value[32:], nil
	},
	ParentLeafSetRoot: func(value []byte, root []byte) ([]byte, error) {
		if len(value) != 64 {
			return nil, fmt.Errorf("len(value) = %v != 64", len(value))
		}
		copy(value[32:], root)
		return value, nil
	},
})

func TestSubTree(t *testing.T) {
	// In this test we have:
	// - a singleton subTree (singleCfg) at path "single".  The leaf of the
	//   mainTree at path "single" == singleTree.Root.
	// - an instance of two non-singleton subTrees (multiACfg, multiBCfg)
	//   in a single leaf at path `id`.  The leaf of the mainTree at path
	//   `id` == multiA_id.Root | multiB_id.Root
	sdb := New(metadb.NewTest(t))

	mainTree, err := sdb.BeginTx()
	qt.Assert(t, err, qt.IsNil)

	// Example of a singleton subtree which root is the leaf of the main tree.
	// Crete the leaf in mainTree which contains the root of the single
	qt.Assert(t, mainTree.Add(singleCfg.Key(), emptyHash), qt.IsNil)
	// treePrint(mainTree.tree, mainTree.txTree, "main")
	single, err := mainTree.SubTree(singleCfg)
	qt.Assert(t, err, qt.IsNil)
	// We add two key-values to the single tree
	qt.Assert(t, single.Add([]byte("key0"), []byte("value0")), qt.IsNil)
	qt.Assert(t, single.Add([]byte("key1"), []byte("value1")), qt.IsNil)
	// treePrint(single.tree, single.txTree, "single")

	// Example of two (multi) subtrees which roots are in a leaf of the main tree
	id := []byte("01234567")
	// Create one leaf at `id` in the mainTree which contains the roots of
	// (multiA_id, multiB_id)
	qt.Assert(t, mainTree.Add(id, make([]byte, 32*2)), qt.IsNil)
	// treePrint(mainTree.tree, mainTree.txTree, "main")
	multiA, err := mainTree.SubTree(multiACfg.WithKey(id))
	qt.Assert(t, err, qt.IsNil)
	// We add one key-value to multiA
	qt.Assert(t, multiA.Add([]byte("key2"), []byte("value2")), qt.IsNil)
	multiB, err := mainTree.SubTree(multiBCfg.WithKey(id))
	qt.Assert(t, err, qt.IsNil)
	// We add one key-value to multiB
	qt.Assert(t, multiB.Add([]byte("key3"), []byte("value3")), qt.IsNil)
	// treePrint(multiA.tree, multiA.txTree, "multiA")

	qt.Assert(t, mainTree.Commit(1), qt.IsNil)

	// dumpPrint(sdb.db)

	// Check that values in each subtree is as expected
	{
		mainTreeView, err := sdb.TreeView(nil)
		qt.Assert(t, err, qt.IsNil)

		// Expect the two key-values in single
		single, err := mainTreeView.SubTree(singleCfg)
		qt.Assert(t, err, qt.IsNil)
		v0, err := single.Get([]byte("key0"))
		qt.Assert(t, err, qt.IsNil)
		qt.Assert(t, v0, qt.DeepEquals, []byte("value0"))
		v1, err := single.Get([]byte("key1"))
		qt.Assert(t, err, qt.IsNil)
		qt.Assert(t, v1, qt.DeepEquals, []byte("value1"))

		// Expect the key-value in multiA
		multiA, err := mainTreeView.SubTree(multiACfg.WithKey(id))
		qt.Assert(t, err, qt.IsNil)
		// dumpPrint(multiA.tree.DB())
		// treePrint(multiA.tree, nil, "multiA")
		v2, err := multiA.Get([]byte("key2"))
		qt.Assert(t, err, qt.IsNil)
		qt.Assert(t, v2, qt.DeepEquals, []byte("value2"))

		// Expect the key-value in multiB
		multiB, err := mainTreeView.SubTree(multiBCfg.WithKey(id))
		qt.Assert(t, err, qt.IsNil)
		v3, err := multiB.Get([]byte("key3"))
		qt.Assert(t, err, qt.IsNil)
		qt.Assert(t, v3, qt.DeepEquals, []byte("value3"))
	}

	// Check the roots in the leafs are as expected
	{
		// Get the leafs that contain the roots of the subTrees
		mainTreeView, err := sdb.TreeView(nil)
		qt.Assert(t, err, qt.IsNil)
		singleParentLeaf, err := mainTreeView.Get(singleCfg.Key())
		qt.Assert(t, err, qt.IsNil)
		multiParentLeaf, err := mainTreeView.Get(id)
		qt.Assert(t, err, qt.IsNil)

		// Expect that mainTree's leaf for single == single.Root
		single, err := mainTreeView.SubTree(singleCfg)
		qt.Assert(t, err, qt.IsNil)
		singleRoot, err := single.Root()
		qt.Assert(t, err, qt.IsNil)
		qt.Assert(t, singleParentLeaf, qt.DeepEquals, singleRoot)

		// Expect that mainTree's leaf for id == multiA.Root | multiB.Root
		multiA, err := mainTreeView.SubTree(multiACfg.WithKey(id))
		qt.Assert(t, err, qt.IsNil)
		multiARoot, err := multiA.Root()
		qt.Assert(t, err, qt.IsNil)
		qt.Assert(t, multiParentLeaf[:32], qt.DeepEquals, multiARoot)

		multiB, err := mainTreeView.SubTree(multiBCfg.WithKey(id))
		qt.Assert(t, err, qt.IsNil)
		multiBRoot, err := multiB.Root()
		qt.Assert(t, err, qt.IsNil)
		qt.Assert(t, multiParentLeaf[32:], qt.DeepEquals, multiBRoot)
	}
}

func TestNoState(t *testing.T) {
	sdb := New(metadb.NewTest(t))

	mainTree, err := sdb.BeginTx()
	qt.Assert(t, err, qt.IsNil)

	// Set key-value in mainTree.NoState
	mainTreeNoState := mainTree.noState()
	qt.Assert(t, mainTreeNoState.Set([]byte("key0"), []byte("value0")), qt.IsNil)
	v0, err := mainTreeNoState.Get([]byte("key0"))
	qt.Assert(t, err, qt.IsNil)
	qt.Assert(t, v0, qt.DeepEquals, []byte("value0"))

	// Example of a singleton subtree which root is the leaf of the main tree
	qt.Assert(t, mainTree.Add(singleCfg.Key(), emptyHash), qt.IsNil)
	mainTreeRoot, err := mainTree.Root()
	qt.Assert(t, err, qt.IsNil)

	single, err := mainTree.SubTree(singleCfg)
	qt.Assert(t, err, qt.IsNil)
	singleRoot, err := single.Root()
	qt.Assert(t, err, qt.IsNil)

	// Set key-value in single.NoState
	singleNoState := single.noState()
	qt.Assert(t, singleNoState.Set([]byte("key1"), []byte("value1")), qt.IsNil)
	v1, err := singleNoState.Get([]byte("key1"))
	qt.Assert(t, err, qt.IsNil)
	qt.Assert(t, v1, qt.DeepEquals, []byte("value1"))

	qt.Assert(t, mainTree.Commit(1), qt.IsNil)

	// Expect the NoState values in each treeView.  Expect that roots
	// haven't changed after Commit.
	{
		// mainTree root hasn't changed
		mainTreeView, err := sdb.TreeView(nil)
		qt.Assert(t, err, qt.IsNil)
		mainTreeRoot1, err := mainTreeView.Root()
		qt.Assert(t, err, qt.IsNil)
		qt.Assert(t, mainTreeRoot1, qt.DeepEquals, mainTreeRoot)

		// Expect the value in mainTree.NoState
		mainTreeNoState := mainTreeView.noState()
		v0, err := mainTreeNoState.Get([]byte("key0"))
		qt.Assert(t, err, qt.IsNil)
		qt.Assert(t, v0, qt.DeepEquals, []byte("value0"))

		// single root hasn't changed
		single, err := mainTreeView.SubTree(singleCfg)
		qt.Assert(t, err, qt.IsNil)
		singleRoot1, err := single.Root()
		qt.Assert(t, err, qt.IsNil)
		qt.Assert(t, singleRoot1, qt.DeepEquals, singleRoot)
	}

	// dumpPrint(sdb.db)
}

//lint:ignore U1000 debug function
func toString(v []byte) string {
	elems := strings.Split(string(v), "/")
	elemsClean := make([]string, len(elems))
	for i, elem := range elems {
		asciiPrint := true
		for _, b := range []byte(elem) {
			if b < 0x20 || 0x7e < b {
				asciiPrint = false
				break
			}
		}
		var elemClean string
		if asciiPrint {
			elemClean = elem
		} else {
			elemClean = fmt.Sprintf("\\x%x", elem)
		}
		if len(elemClean) > 10 {
			elemClean = fmt.Sprintf("%v..", elemClean[:10])
		}
		elemsClean[i] = elemClean
	}
	return strings.Join(elemsClean, "/")
}

// test function useful for debugging.  Prints a dump of the database with path
// separator awareness (best effort), and allows showing mix of ascii and
// non-ascii content (again, best effort).
//
//lint:ignore U1000 debug function
func dumpPrint(db db.Database) {
	fmt.Printf("--- DB Print ---\n")
	db.Iterate(nil, func(key, value []byte) bool {
		fmt.Printf("%v -> %v\n", toString(key), toString(value))
		return true
	})
}

// test function useful for debugging.  Prints the paths and leaf contents of
// the tree.
//
//lint:ignore U1000 debug function
func treePrint(t *tree.Tree, tx db.Reader, name string) {
	fmt.Printf("--- Tree Print (%s)---\n", name)
	if err := t.Iterate(tx, func(key, value []byte) bool {
		if value[0] != arbo.PrefixValueLeaf {
			return true
		}
		leafK, leafV := arbo.ReadLeafValue(value)
		if len(leafK) > 10 {
			fmt.Printf("%x..", leafK[:10])
		} else {
			fmt.Printf("%x", leafK)
		}
		fmt.Printf(" -> ")
		if len(leafV) > 10 {
			fmt.Printf("%x..", leafV[:10])
		} else {
			fmt.Printf("%x", leafV)
		}
		fmt.Printf("\n")
		return true
	}); err != nil {
		panic(err)
	}
}

func TestTree(t *testing.T) {
	db := metadb.NewTest(t)

	tx := db.WriteTx()
	txTree := subWriteTx(tx, subKeyTree)
	tree, err := tree.New(txTree,
		tree.Options{DB: nil, MaxLevels: 256, HashFunc: arbo.HashFunctionSha256})
	qt.Assert(t, err, qt.IsNil)
	qt.Assert(t, tree.Add(txTree, []byte("key0"), []byte("value0")), qt.IsNil)
	tx.Commit()
	// dumpPrint(db)
}

func TestBigUpdate(t *testing.T) {
	sdb := New(metadb.NewTest(t))

	mainTree, err := sdb.BeginTx()
	qt.Assert(t, err, qt.IsNil)

	const n = 20_000
	var key [4]byte
	var value [4]byte
	for i := 0; i < n; i++ {
		binary.LittleEndian.PutUint32(key[:], uint32(i))
		binary.LittleEndian.PutUint32(value[:], uint32(i))
		qt.Assert(t, mainTree.Add(key[:], value[:]), qt.IsNil)
	}
	qt.Assert(t, mainTree.Commit(1), qt.IsNil)
}

func TestBigUpdateDiscard(t *testing.T) {
	sdb := New(metadb.NewTest(t))

	mainTree, err := sdb.BeginTx()
	qt.Assert(t, err, qt.IsNil)

	root, err := mainTree.Root()
	qt.Assert(t, err, qt.IsNil)
	qt.Assert(t, root, qt.DeepEquals, emptyHash)

	// Do a big update in the mainTree.  Although internally there will be
	// Database transactions, we shouldn't observe a change in the tree
	// because we discard.
	const n = 20_000
	var key [4]byte
	var value [4]byte
	for i := 0; i < n; i++ {
		binary.LittleEndian.PutUint32(key[:], uint32(i))
		binary.LittleEndian.PutUint32(value[:], uint32(i))
		qt.Assert(t, mainTree.Add(key[:], value[:]), qt.IsNil)
	}
	mainTree.Discard()

	mainTree, err = sdb.BeginTx()
	qt.Assert(t, err, qt.IsNil)
	root, err = mainTree.Root()
	qt.Assert(t, err, qt.IsNil)
	qt.Assert(t, root, qt.DeepEquals, emptyHash)
	mainTree.Discard()

	// Now we do the same for a subTree.
	mainTree, err = sdb.BeginTx()
	qt.Assert(t, err, qt.IsNil)
	qt.Assert(t, mainTree.Add(singleCfg.Key(), emptyHash), qt.IsNil)
	qt.Assert(t, mainTree.Commit(1), qt.IsNil)

	mainTree, err = sdb.BeginTx()
	qt.Assert(t, err, qt.IsNil)
	single, err := mainTree.SubTree(singleCfg)
	qt.Assert(t, err, qt.IsNil)
	singleRoot, err := single.Root()
	qt.Assert(t, err, qt.IsNil)
	qt.Assert(t, singleRoot, qt.DeepEquals, emptyHash)

	for i := 0; i < n; i++ {
		binary.LittleEndian.PutUint32(key[:], uint32(i))
		binary.LittleEndian.PutUint32(value[:], uint32(i))
		qt.Assert(t, single.Add(key[:], value[:]), qt.IsNil)
	}
	mainTree.Discard()

	mainTree, err = sdb.BeginTx()
	qt.Assert(t, err, qt.IsNil)
	single, err = mainTree.SubTree(singleCfg)
	qt.Assert(t, err, qt.IsNil)
	singleRoot, err = single.Root()
	qt.Assert(t, err, qt.IsNil)
	qt.Assert(t, singleRoot, qt.DeepEquals, emptyHash)
	mainTree.Discard()
}

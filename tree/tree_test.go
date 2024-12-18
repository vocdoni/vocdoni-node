package tree

import (
	"strconv"
	"testing"

	qt "github.com/frankban/quicktest"
	"go.vocdoni.io/dvote/db/metadb"
	"go.vocdoni.io/dvote/tree/arbo"
)

// NOTE: most of the methods of Tree are just wrappers over
// https://go.vocdoni.io/dvote/tree/arbo.  The proper tests are in arbo's repo, here
// there are tests that check the added code in the Tree wrapper

func TestSet(t *testing.T) {
	database := metadb.NewTest(t)

	tree, err := New(nil, Options{DB: database, MaxLevels: 100, HashFunc: arbo.HashFunctionBlake2b})
	qt.Assert(t, err, qt.IsNil)

	wTx := tree.DB().WriteTx()

	// set, expecting an internal add
	err = tree.Set(wTx, []byte("key0"), []byte("value0"))
	qt.Assert(t, err, qt.IsNil)

	v, err := tree.Get(wTx, []byte("key0"))
	qt.Assert(t, err, qt.IsNil)
	qt.Assert(t, v, qt.DeepEquals, []byte("value0"))

	// set with the same value (update)
	err = tree.Set(wTx, []byte("key0"), []byte("value0"))
	qt.Assert(t, err, qt.IsNil)

	v, err = tree.Get(wTx, []byte("key0"))
	qt.Assert(t, err, qt.IsNil)
	qt.Assert(t, v, qt.DeepEquals, []byte("value0"))

	// set with a new value (expecting an update)
	err = tree.Set(wTx, []byte("key0"), []byte("value1"))
	qt.Assert(t, err, qt.IsNil)

	v, err = tree.Get(wTx, []byte("key0"))
	qt.Assert(t, err, qt.IsNil)
	qt.Assert(t, v, qt.DeepEquals, []byte("value1"))

	// try to do an Add for the same key, expecting an error
	err = tree.Add(wTx, []byte("key0"), []byte("value0"))
	qt.Assert(t, err, qt.Equals, arbo.ErrKeyAlreadyExists)

	err = tree.Add(wTx, []byte("key1"), []byte("value1"))
	qt.Assert(t, err, qt.IsNil)

	v, err = tree.Get(wTx, []byte("key1"))
	qt.Assert(t, err, qt.IsNil)
	qt.Assert(t, v, qt.DeepEquals, []byte("value1"))

	_, err = tree.Get(wTx, []byte("key2"))
	qt.Assert(t, err, qt.Equals, arbo.ErrKeyNotFound)

	err = wTx.Commit()
	qt.Assert(t, err, qt.IsNil)
}

func TestGenProof(t *testing.T) {
	database := metadb.NewTest(t)

	tree, err := New(nil, Options{DB: database, MaxLevels: 100, HashFunc: arbo.HashFunctionBlake2b})
	qt.Assert(t, err, qt.IsNil)

	wTx := tree.DB().WriteTx()
	for i := 0; i < 10; i++ {
		k := []byte("key" + strconv.Itoa(i))
		v := []byte("value" + strconv.Itoa(i))
		err := tree.Add(wTx, k, v)
		qt.Assert(t, err, qt.IsNil)
	}
	k := []byte("key3")
	v, proof, err := tree.GenProof(wTx, k)
	qt.Assert(t, err, qt.IsNil)
	qt.Assert(t, v, qt.DeepEquals, []byte("value3"))

	root, err := tree.Root(wTx)
	qt.Assert(t, err, qt.IsNil)

	err = tree.VerifyProof(k, v, proof, root)
	qt.Assert(t, err, qt.IsNil)

	err = wTx.Commit()
	qt.Assert(t, err, qt.IsNil)
}

func TestFromRoot(t *testing.T) {
	database := metadb.NewTest(t)

	tree, err := New(nil, Options{DB: database, MaxLevels: 100, HashFunc: arbo.HashFunctionBlake2b})
	qt.Assert(t, err, qt.IsNil)

	wTx := tree.DB().WriteTx()
	for i := 0; i < 10; i++ {
		k := []byte("key" + strconv.Itoa(i))
		v := []byte("value" + strconv.Itoa(i))
		err := tree.Add(wTx, k, v)
		qt.Assert(t, err, qt.IsNil)
	}
	err = wTx.Commit()
	qt.Assert(t, err, qt.IsNil)

	wTx = tree.DB().WriteTx()

	// do a snapshot
	tree2, err := tree.FromRoot(nil)
	qt.Assert(t, err, qt.IsNil)

	oldRoot, err := tree.Root(wTx)
	qt.Assert(t, err, qt.IsNil)

	// try to add a key-value to the snapshot tree, expecting the error
	err = tree2.Add(wTx, []byte("key10"), []byte("value10"))
	qt.Assert(t, err, qt.Equals, arbo.ErrSnapshotNotEditable)

	// add the key-value to the original tree, and check that the snapshot
	// tree still has the old root
	err = tree.Add(wTx, []byte("key10"), []byte("value10"))
	qt.Assert(t, err, qt.IsNil)
	// expect the snapshot tree to have the same root that before the Add
	oldRoot2, err := tree2.Root(wTx)
	qt.Assert(t, err, qt.IsNil)
	qt.Assert(t, oldRoot2, qt.DeepEquals, oldRoot)
	// expect the original tree to have a new root different than the old
	// one
	newRoot, err := tree.Root(wTx)
	qt.Assert(t, err, qt.IsNil)
	qt.Assert(t, newRoot, qt.Not(qt.DeepEquals), oldRoot)

	err = wTx.Commit()
	qt.Assert(t, err, qt.IsNil)
}

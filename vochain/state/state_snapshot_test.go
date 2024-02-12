package state

import (
	"io"
	"path/filepath"
	"testing"

	qt "github.com/frankban/quicktest"
	"go.vocdoni.io/dvote/db"
	"go.vocdoni.io/dvote/db/metadb"
	"go.vocdoni.io/dvote/test/testcommon/testutil"
	"go.vocdoni.io/dvote/tree"
	"go.vocdoni.io/dvote/tree/arbo"
	"go.vocdoni.io/proto/build/go/models"
)

func TestSnapshot(t *testing.T) {
	st := newStateForTest(t)

	err := st.AddValidator(&models.Validator{
		Address: []byte("validator1"),
		Power:   10,
	})
	qt.Assert(t, err, qt.IsNil)
	err = st.AddValidator(&models.Validator{
		Address: []byte("validator2"),
		Power:   20,
	})
	qt.Assert(t, err, qt.IsNil)

	nostate := st.NoState(true)
	err = nostate.Set([]byte("nostate1"), []byte("value1"))
	qt.Assert(t, err, qt.IsNil)
	err = nostate.Set([]byte("nostate2"), []byte("value2"))
	qt.Assert(t, err, qt.IsNil)

	_, err = st.PrepareCommit()
	qt.Assert(t, err, qt.IsNil)
	hash, err := st.Save()
	qt.Assert(t, err, qt.IsNil)
	qt.Assert(t, hash, qt.Not(qt.IsNil))
}

func TestTreeExportImport(t *testing.T) {
	rnd := testutil.NewRandom(10)
	mainRoot := rnd.RandomBytes(32)

	var snap StateSnapshot
	err := snap.Create(filepath.Join(t.TempDir(), "snapshot1"))
	qt.Assert(t, err, qt.IsNil)
	snap.SetMainRoot(mainRoot)

	tree1 := newTreeForTest(t, 0)
	root1, err := tree1.Root(nil)
	qt.Assert(t, err, qt.IsNil)
	snap.AddTree("Tree1", "", root1)
	err = tree1.DumpWriter(&snap)
	qt.Assert(t, err, qt.IsNil)
	snap.EndTree()

	tree2 := newTreeForTest(t, 1)
	root2, err := tree2.Root(nil)
	qt.Assert(t, err, qt.IsNil)
	snap.AddTree("Tree2", "", root2)
	err = tree2.DumpWriter(&snap)
	qt.Assert(t, err, qt.IsNil)
	snap.EndTree()

	tree3 := newTreeForTest(t, 2)
	root3, err := tree3.Root(nil)
	qt.Assert(t, err, qt.IsNil)
	snap.AddTree("Tree3", "Tree1", root3)
	err = tree3.DumpWriter(&snap)
	qt.Assert(t, err, qt.IsNil)
	snap.EndTree()

	err = snap.Save()
	qt.Assert(t, err, qt.IsNil)

	var snap2 StateSnapshot
	err = snap2.Open(snap.Path())
	qt.Assert(t, err, qt.IsNil)

	// tree1
	treeImp := newEmptyTreeForTest(t)
	b, err := snap2.ReadAll()
	qt.Assert(t, err, qt.IsNil)
	qt.Assert(t, len(b), qt.Not(qt.Equals), 0)
	_, err = snap2.ReadAll()
	qt.Assert(t, err, qt.ErrorIs, io.EOF)
	err = treeImp.ImportDump(b)
	qt.Assert(t, err, qt.IsNil)
	_, err = snap2.ReadAll()
	qt.Assert(t, err, qt.ErrorIs, io.EOF)
	root, err := treeImp.Root(nil)
	qt.Assert(t, err, qt.IsNil)
	qt.Assert(t, root1, qt.DeepEquals, root)

	// tree2
	err = snap2.FetchNextTree()
	qt.Assert(t, err, qt.IsNil)
	treeImp = newEmptyTreeForTest(t)
	b, err = snap2.ReadAll()
	qt.Assert(t, err, qt.IsNil)
	err = treeImp.ImportDump(b)
	qt.Assert(t, err, qt.IsNil)
	root, err = treeImp.Root(nil)
	qt.Assert(t, err, qt.IsNil)
	qt.Assert(t, root2, qt.DeepEquals, root)

	// tree3
	err = snap2.FetchNextTree()
	qt.Assert(t, err, qt.IsNil)
	treeImp = newEmptyTreeForTest(t)
	b, err = snap2.ReadAll()
	qt.Assert(t, err, qt.IsNil)
	err = treeImp.ImportDump(b)
	qt.Assert(t, err, qt.IsNil)
	root, err = treeImp.Root(nil)
	qt.Assert(t, err, qt.IsNil)
	qt.Assert(t, root3, qt.DeepEquals, root)

	// end of trees
	err = snap2.FetchNextTree()
	qt.Assert(t, err, qt.ErrorIs, io.EOF)

}

func newTreeForTest(t *testing.T, rndGenerator int64) *tree.Tree {
	tree := newEmptyTreeForTest(t)
	rnd := testutil.NewRandom(rndGenerator)

	// Add 3 values to the tree
	wtx := tree.DB().WriteTx()
	err := tree.Add(wtx, rnd.RandomBytes(32), rnd.RandomBytes(32))
	qt.Assert(t, err, qt.IsNil)
	err = tree.Add(wtx, rnd.RandomBytes(32), rnd.RandomBytes(32))
	qt.Assert(t, err, qt.IsNil)
	err = tree.Add(wtx, rnd.RandomBytes(32), rnd.RandomBytes(32))
	qt.Assert(t, err, qt.IsNil)
	err = tree.Add(wtx, rnd.RandomBytes(32), rnd.RandomBytes(32))
	qt.Assert(t, err, qt.IsNil)
	err = wtx.Commit()
	qt.Assert(t, err, qt.IsNil)

	return tree
}

func newEmptyTreeForTest(t *testing.T) *tree.Tree {
	db, err := metadb.New(db.TypePebble, t.TempDir())
	qt.Assert(t, err, qt.IsNil)
	tr, err := tree.New(nil, tree.Options{DB: db, MaxLevels: 256, HashFunc: arbo.HashFunctionBlake2b})
	qt.Assert(t, err, qt.IsNil)
	return tr
}

func newStateForTest(t *testing.T) *State {
	state, err := New(db.TypePebble, t.TempDir())
	qt.Assert(t, err, qt.IsNil)
	return state
}

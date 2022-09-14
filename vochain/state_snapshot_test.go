package vochain

import (
	"io"
	"path/filepath"
	"testing"

	qt "github.com/frankban/quicktest"
	"github.com/vocdoni/arbo"
	"go.vocdoni.io/dvote/db"
	"go.vocdoni.io/dvote/db/metadb"
	"go.vocdoni.io/dvote/test/testcommon/testutil"
	"go.vocdoni.io/dvote/tree"
)

func TestStateSnapshot(t *testing.T) {
	rnd := testutil.NewRandom(10)
	mainRoot := rnd.RandomBytes(32)

	var snap StateSnapshot
	err := snap.Create(filepath.Join(t.TempDir(), "snapshot1"))
	qt.Assert(t, err, qt.IsNil)
	snap.SetMainRoot(mainRoot)

	tree1 := newTreeForTest(t, 0)
	root1, err := tree1.Root(tree1.DB().ReadTx())
	qt.Assert(t, err, qt.IsNil)
	snap.AddTree("Tree1", "", root1)
	err = tree1.DumpWriter(&snap)
	qt.Assert(t, err, qt.IsNil)
	snap.EndTree()

	tree2 := newTreeForTest(t, 1)
	root2, err := tree2.Root(tree2.DB().ReadTx())
	qt.Assert(t, err, qt.IsNil)
	snap.AddTree("Tree2", "", root2)
	err = tree2.DumpWriter(&snap)
	qt.Assert(t, err, qt.IsNil)
	snap.EndTree()

	tree3 := newTreeForTest(t, 2)
	root3, err := tree3.Root(tree3.DB().ReadTx())
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
	err = treeImp.ImportDump(b)
	qt.Assert(t, err, qt.IsNil)
	_, err = snap2.ReadAll()
	qt.Assert(t, err, qt.ErrorIs, io.EOF)
	root, err := treeImp.Root(treeImp.DB().ReadTx())
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
	root, err = treeImp.Root(treeImp.DB().ReadTx())
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
	root, err = treeImp.Root(treeImp.DB().ReadTx())
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

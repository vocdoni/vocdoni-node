package censustree

import (
	"testing"

	qt "github.com/frankban/quicktest"
	"github.com/vocdoni/arbo"
	"go.vocdoni.io/dvote/db/badgerdb"
	"go.vocdoni.io/dvote/tree"
)

// NOTE: most of the methods of CensusTree are just wrappers over tree.Tree.
// The proper tests are in tree package, here there are tests that check the
// added code in the CensusTree wrapper.

func TestPublish(t *testing.T) {
	database, err := badgerdb.New(badgerdb.Options{Path: t.TempDir()})
	qt.Assert(t, err, qt.IsNil)

	censusTree, err := New(nil, tree.Options{DB: database, MaxLevels: 100, HashFunc: arbo.HashFunctionBlake2b})
	qt.Assert(t, err, qt.IsNil)

	qt.Assert(t, censusTree.IsPublic(), qt.IsFalse)

	censusTree.Publish()
	qt.Assert(t, censusTree.IsPublic(), qt.IsTrue)

	censusTree.Unpublish()
	qt.Assert(t, censusTree.IsPublic(), qt.IsFalse)

}

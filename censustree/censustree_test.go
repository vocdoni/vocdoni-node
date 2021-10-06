package censustree

import (
	"testing"

	qt "github.com/frankban/quicktest"
	"go.vocdoni.io/dvote/db/pebbledb"
	"go.vocdoni.io/proto/build/go/models"
)

// NOTE: most of the methods of CensusTree are just wrappers over tree.Tree.
// The proper tests are in tree package, here there are tests that check the
// added code in the CensusTree wrapper.

func TestPublish(t *testing.T) {
	db, err := pebbledb.New(pebbledb.Options{Path: t.TempDir()})
	qt.Assert(t, err, qt.IsNil)

	censusTree, err := New(Options{Name: "test", ParentDB: db, MaxLevels: 256,
		CensusType: models.Census_ARBO_BLAKE2B})
	qt.Assert(t, err, qt.IsNil)

	qt.Assert(t, censusTree.IsPublic(), qt.IsFalse)

	censusTree.Publish()
	qt.Assert(t, censusTree.IsPublic(), qt.IsTrue)

	censusTree.Unpublish()
	qt.Assert(t, censusTree.IsPublic(), qt.IsFalse)

}

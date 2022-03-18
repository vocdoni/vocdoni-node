package censustree

import (
	"math/big"
	"strconv"
	"testing"

	qt "github.com/frankban/quicktest"
	"go.vocdoni.io/dvote/db/metadb"
	"go.vocdoni.io/proto/build/go/models"
)

// NOTE: most of the methods of CensusTree are just wrappers over tree.Tree.
// The proper tests are in tree package, here there are tests that check the
// added code in the CensusTree wrapper.

func TestPublish(t *testing.T) {
	db := metadb.NewTest(t)
	censusTree, err := New(Options{Name: "test", ParentDB: db, MaxLevels: 256,
		CensusType: models.Census_ARBO_BLAKE2B})
	qt.Assert(t, err, qt.IsNil)

	qt.Assert(t, censusTree.IsPublic(), qt.IsFalse)

	censusTree.Publish()
	qt.Assert(t, censusTree.IsPublic(), qt.IsTrue)

	censusTree.Unpublish()
	qt.Assert(t, censusTree.IsPublic(), qt.IsFalse)
}

func TestGetCensusWeight(t *testing.T) {
	db := metadb.NewTest(t)
	tree, err := New(Options{Name: "test", ParentDB: db, MaxLevels: 256,
		CensusType: models.Census_ARBO_BLAKE2B})
	qt.Assert(t, err, qt.IsNil)

	w, err := tree.GetCensusWeight()
	qt.Assert(t, err, qt.IsNil)
	qt.Assert(t, w.String(), qt.Equals, "0")

	weight := tree.BigIntToBytes(big.NewInt(17))
	err = tree.Add([]byte("key"), weight)
	qt.Assert(t, err, qt.IsNil)

	w, err = tree.GetCensusWeight()
	qt.Assert(t, err, qt.IsNil)
	qt.Assert(t, w.String(), qt.Equals, "17")

	// test CensusWeight after doing a loop of censustree.Add
	for i := 0; i < 100; i++ {
		weight := tree.BigIntToBytes(big.NewInt(int64(i)))
		err = tree.Add([]byte{byte(i)}, weight)
		qt.Assert(t, err, qt.IsNil)
	}

	w, err = tree.GetCensusWeight()
	qt.Assert(t, err, qt.IsNil)
	qt.Assert(t, w.String(), qt.Equals, "4967") // = 17 + (0+1+2+...+99)

	// test CensusWeight after using censustree.AddBatch
	// prepare inputs
	var keys, values [][]byte
	for i := 0; i < 100; i++ {
		weight := tree.BigIntToBytes(big.NewInt(int64(i)))
		// use 100+i, as the first 99 keys are already used
		keys = append(keys, []byte{byte(100 + i)})
		values = append(values, weight)
	}

	invalids, err := tree.AddBatch(keys, values)
	qt.Assert(t, err, qt.IsNil)
	qt.Assert(t, len(invalids), qt.Equals, 0)

	w, err = tree.GetCensusWeight()
	qt.Assert(t, err, qt.IsNil)
	qt.Assert(t, w.String(), qt.Equals, "9917") // = 4967 + (0+1+2+...+99)

	// test CensusWeight after using censustree.AddBatch, but this time
	// with some invalid keys, which weight should not be counted
	keys = [][]byte{}
	values = [][]byte{}
	for i := 0; i < 100; i++ {
		weight := tree.BigIntToBytes(big.NewInt(int64(i)))
		// use 200+i, in order to get 56 correct keys, and 44
		// incorrect, as on 256 the byte used in the key will overflow
		// and go back to 0, and the keys from 0 to 198 are already
		// used by previous additions to the tree, so when trying to
		// add them will get an invalid code.
		keys = append(keys, []byte{byte(200 + i)})
		values = append(values, weight)
	}

	invalids, err = tree.AddBatch(keys, values)
	qt.Assert(t, err, qt.IsNil)
	qt.Assert(t, len(invalids), qt.Equals, 44)

	w, err = tree.GetCensusWeight()
	qt.Assert(t, err, qt.IsNil)
	qt.Assert(t, w.String(), qt.Equals, "11457") // = 9917 + (0+1+2+...+56) = 9917 + 1540

	// try to add keys with empty values
	keys = [][]byte{}
	for i := 0; i < 100; i++ {
		keys = append(keys, []byte("keysWithoutWeight"+strconv.Itoa(i)))
	}

	invalids, err = tree.AddBatch(keys, nil)
	qt.Assert(t, err, qt.IsNil)
	qt.Assert(t, len(invalids), qt.Equals, 0)

	w, err = tree.GetCensusWeight()
	qt.Assert(t, err, qt.IsNil)
	qt.Assert(t, w.String(), qt.Equals, "11457") // = 9917 + (0+1+2+...+56) = 9917 + 1540

	// dump the leaves & import them into a new empty tree, and check that
	// the censusWeight is correctly recomputed
	db2 := metadb.NewTest(t)
	tree2, err := New(Options{Name: "test2", ParentDB: db2, MaxLevels: 256,
		CensusType: models.Census_ARBO_BLAKE2B})
	qt.Assert(t, err, qt.IsNil)

	dump, err := tree.Dump()
	qt.Assert(t, err, qt.IsNil)

	err = tree2.ImportDump(dump)
	qt.Assert(t, err, qt.IsNil)

	w, err = tree2.GetCensusWeight()
	qt.Assert(t, err, qt.IsNil)
	qt.Assert(t, w.String(), qt.Equals, "11457") // same than in the original tree
}

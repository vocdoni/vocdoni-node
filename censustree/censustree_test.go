package censustree

import (
	"math/big"
	"strconv"
	"testing"

	qt "github.com/frankban/quicktest"
	"github.com/vocdoni/arbo"
	"go.vocdoni.io/dvote/db/metadb"
	"go.vocdoni.io/dvote/test/testcommon/testutil"
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
	rnd := testutil.NewRandom(0)
	key, value := rnd.RandomBytes(32), rnd.RandomBytes(32)
	qt.Assert(t, censusTree.Add(key, value), qt.IsNil)

	qt.Assert(t, censusTree.IsPublic(), qt.IsFalse)

	censusTree.Publish()
	qt.Assert(t, censusTree.IsPublic(), qt.IsTrue)

	recValue, _, err := censusTree.GenProof(key)
	qt.Assert(t, err, qt.IsNil)
	qt.Assert(t, value, qt.DeepEquals, recValue)

	censusTree.Unpublish()
	qt.Assert(t, censusTree.IsPublic(), qt.IsFalse)
}

func TestImportWeighted(t *testing.T) {
	db := metadb.NewTest(t)
	censusTree, err := New(Options{Name: "test", ParentDB: db, MaxLevels: 256,
		CensusType: models.Census_ARBO_BLAKE2B})
	qt.Assert(t, err, qt.IsNil)

	rnd := testutil.NewRandom(0)
	totalWeight := big.NewInt(0)

	// add a bunch of keys and values (weights)
	for i := 1; i < 11; i++ {
		h, err := arbo.HashFunctionBlake2b.Hash(rnd.RandomBytes(32))
		qt.Assert(t, err, qt.IsNil)
		qt.Assert(t, h, qt.Not(qt.HasLen), 0)
		censusTree.Add(h, censusTree.BigIntToBytes(new(big.Int).SetInt64(int64(i))))
		qt.Assert(t, err, qt.IsNil)
		totalWeight.Add(totalWeight, big.NewInt(int64(i)))
	}

	// check the total weight is correctly calculated
	weight, err := censusTree.GetCensusWeight()
	qt.Assert(t, err, qt.IsNil)
	qt.Assert(t, weight.Cmp(totalWeight), qt.Equals, 0)

	// dump the tree
	dump, err := censusTree.Dump()
	qt.Assert(t, err, qt.IsNil)

	// import into a new tree
	censusTree2, err := New(Options{Name: "test2", ParentDB: db, MaxLevels: 256,
		CensusType: models.Census_ARBO_BLAKE2B})
	qt.Assert(t, err, qt.IsNil)

	err = censusTree2.ImportDump(dump)
	qt.Assert(t, err, qt.IsNil)

	// check the weight is still the same
	weight, err = censusTree2.GetCensusWeight()
	qt.Assert(t, err, qt.IsNil)
	qt.Assert(t, weight.Cmp(totalWeight), qt.Equals, 0)

	// check root is the same after adding a new leaf
	k, v := rnd.RandomBytes(32), rnd.RandomBytes(32)
	err = censusTree.Add(k, v)
	qt.Assert(t, err, qt.IsNil)
	err = censusTree2.Add(k, v)
	qt.Assert(t, err, qt.IsNil)

	r1, err := censusTree.Root()
	qt.Assert(t, err, qt.IsNil)
	r2, err := censusTree2.Root()
	qt.Assert(t, err, qt.IsNil)
	qt.Assert(t, r1, qt.DeepEquals, r2)
}
func TestImportIndexed(t *testing.T) {
	db := metadb.NewTest(t)
	censusTree, err := New(Options{Name: "test", ParentDB: db, MaxLevels: 256,
		CensusType: models.Census_ARBO_BLAKE2B, IndexAsKeysCensus: true})
	qt.Assert(t, err, qt.IsNil)

	rnd := testutil.NewRandom(0)
	index := uint32(0)

	// add a bunch of keys
	for i := 1; i < 11; i++ {
		h, err := arbo.HashFunctionBlake2b.Hash(rnd.RandomBytes(32))
		qt.Assert(t, err, qt.IsNil)
		qt.Assert(t, h, qt.Not(qt.HasLen), 0)
		err = censusTree.Add(h, nil)
		qt.Assert(t, err, qt.IsNil)
		index++
	}

	// check the total inex is correctly calculated
	censusIndex, err := censusTree.GetCensusIndex()
	qt.Assert(t, err, qt.IsNil)
	qt.Assert(t, index, qt.Equals, censusIndex)

	// dump the tree
	dump, err := censusTree.Dump()
	qt.Assert(t, err, qt.IsNil)

	// import into a new tree
	censusTree2, err := New(Options{Name: "test2", ParentDB: db, MaxLevels: 256,
		CensusType: models.Census_ARBO_BLAKE2B, IndexAsKeysCensus: true})
	qt.Assert(t, err, qt.IsNil)

	err = censusTree2.ImportDump(dump)
	qt.Assert(t, err, qt.IsNil)

	// check the index is still the same
	censusIndex, err = censusTree2.GetCensusIndex()
	qt.Assert(t, err, qt.IsNil)
	qt.Assert(t, index, qt.Equals, censusIndex)

	// check root is the same after adding a new leaf
	k := rnd.RandomBytes(32)
	err = censusTree.Add(k, nil)
	qt.Assert(t, err, qt.IsNil)
	err = censusTree2.Add(k, nil)
	qt.Assert(t, err, qt.IsNil)

	r1, err := censusTree.Root()
	qt.Assert(t, err, qt.IsNil)
	r2, err := censusTree2.Root()
	qt.Assert(t, err, qt.IsNil)
	qt.Assert(t, r1, qt.DeepEquals, r2)
}

func TestWeightedProof(t *testing.T) {
	db := metadb.NewTest(t)
	censusTree, err := New(Options{Name: "test", ParentDB: db, MaxLevels: 256,
		CensusType: models.Census_ARBO_POSEIDON, IndexAsKeysCensus: false})
	qt.Assert(t, err, qt.IsNil)

	rnd := testutil.NewRandom(0)

	// add a bunch of keys
	for i := 1; i < 11; i++ {
		err = censusTree.Add(
			censusTree.BigIntToBytes(big.NewInt(int64(rnd.RandomIntn(1000000)))),
			censusTree.BigIntToBytes(big.NewInt(int64(rnd.RandomIntn(1000000)))),
		)
		qt.Assert(t, err, qt.IsNil)
	}

	// add the last key (we will use if for testing the proof)
	userKey := censusTree.BigIntToBytes(big.NewInt(int64(rnd.RandomIntn(100000))))
	userWeight := censusTree.BigIntToBytes(big.NewInt(int64(rnd.RandomIntn(100000))))

	err = censusTree.Add(userKey, userWeight)
	qt.Assert(t, err, qt.IsNil)

	censusTree.Publish()

	// generate and test the proof
	value, siblings, err := censusTree.GenProof(userKey)
	qt.Assert(t, err, qt.IsNil)
	qt.Assert(t, userWeight, qt.DeepEquals, value)

	root, err := censusTree.Root()
	qt.Assert(t, err, qt.IsNil)

	verified, err := censusTree.VerifyProof(userKey, value, siblings, root)
	qt.Assert(t, err, qt.IsNil)
	qt.Assert(t, verified, qt.IsTrue)
}

func TestIndexProof(t *testing.T) {
	db := metadb.NewTest(t)
	censusTree, err := New(Options{Name: "test", ParentDB: db, MaxLevels: 256,
		CensusType: models.Census_ARBO_POSEIDON, IndexAsKeysCensus: true})
	qt.Assert(t, err, qt.IsNil)

	rnd := testutil.NewRandom(0)

	// add a bunch of keys
	for i := 1; i < 11; i++ {
		err = censusTree.Add(
			censusTree.BigIntToBytes(big.NewInt(int64(rnd.RandomIntn(1000000)))),
			nil,
		)
		qt.Assert(t, err, qt.IsNil)
	}

	// add the last key (we will use if for testing the proof)
	userKey := censusTree.BigIntToBytes(big.NewInt(int64(rnd.RandomIntn(1000))))
	qt.Assert(t, err, qt.IsNil)

	err = censusTree.Add(userKey, nil)
	qt.Assert(t, err, qt.IsNil)

	censusTree.Publish()

	// generate and test the proof
	value, siblings, err := censusTree.GenProof(userKey)
	qt.Assert(t, err, qt.IsNil)
	qt.Assert(t, userKey, qt.DeepEquals, value)

	verified, err := censusTree.VerifyProof(userKey, value, siblings, nil)
	qt.Assert(t, err, qt.IsNil)
	qt.Assert(t, verified, qt.IsTrue)
}

func TestAddBatchIndexed(t *testing.T) {
	db := metadb.NewTest(t)
	censusTree, err := New(Options{Name: "test", ParentDB: db, MaxLevels: 256,
		CensusType: models.Census_ARBO_POSEIDON, IndexAsKeysCensus: true})
	qt.Assert(t, err, qt.IsNil)

	rnd := testutil.NewRandom(0)
	keys := [][]byte{}
	// add a bunch of keys
	for i := 1; i < 11; i++ {
		keys = append(keys,
			censusTree.BigIntToBytes(big.NewInt(int64(rnd.RandomIntn(1000000)))),
		)
		qt.Assert(t, err, qt.IsNil)
	}
	userKey := censusTree.BigIntToBytes(big.NewInt(int64(rnd.RandomIntn(1000))))

	invalid, err := censusTree.AddBatch(append(keys, userKey), nil)
	qt.Assert(t, err, qt.IsNil)
	qt.Assert(t, invalid, qt.HasLen, 0)

	censusTree.Publish()

	lent, err := censusTree.Size()
	qt.Assert(t, err, qt.IsNil)
	qt.Assert(t, lent, qt.Equals, uint64(len(keys)+1))

	index, err := censusTree.GetCensusIndex()
	qt.Assert(t, err, qt.IsNil)
	qt.Assert(t, lent, qt.Equals, uint64(index))

	// generate and test the proof
	value, siblings, err := censusTree.GenProof(userKey)
	qt.Assert(t, err, qt.IsNil)
	qt.Assert(t, userKey, qt.DeepEquals, value)

	verified, err := censusTree.VerifyProof(userKey, value, siblings, nil)
	qt.Assert(t, err, qt.IsNil)
	qt.Assert(t, verified, qt.IsTrue)
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

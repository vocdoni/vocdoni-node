package arbo

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"math/big"
	"os"
	"runtime"
	"sort"
	"testing"
	"time"

	qt "github.com/frankban/quicktest"
	"go.vocdoni.io/dvote/db/metadb"
)

var debug = os.Getenv("LOG_LEVEL") == "debug"

func printTestContext(prefix string, nLeafs int, hashName, dbName string) {
	if debug {
		fmt.Printf("%snCPU: %d, nLeafs: %d, hash: %s, db: %s\n",
			prefix, runtime.NumCPU(), nLeafs, hashName, dbName)
	}
}

func printRes(name string, value any) {
	if debug {
		fmt.Printf("%s:	%s \n", name, value)
	}
}

func debugTime(descr string, time1, time2 time.Duration) {
	if debug {
		fmt.Printf("%s was %.02fx times faster than without AddBatch\n",
			descr, float64(time1)/float64(time2))
	}
}

func testInit(c *qt.C, n int) (*Tree, *Tree) {
	database1 := metadb.NewTest(c)
	tree1, err := NewTree(Config{
		Database: database1, MaxLevels: 256,
		HashFunction: HashFunctionPoseidon,
	})
	c.Assert(err, qt.IsNil)

	database2 := metadb.NewTest(c)
	tree2, err := NewTree(Config{
		Database: database2, MaxLevels: 256,
		HashFunction: HashFunctionPoseidon,
	})
	c.Assert(err, qt.IsNil)

	bLen := HashFunctionPoseidon.Len()
	// add the initial leafs to fill a bit the trees before calling the
	// AddBatch method
	for i := 0; i < n; i++ {
		k := BigIntToBytesLE(bLen, big.NewInt(int64(i)))
		v := BigIntToBytesLE(bLen, big.NewInt(int64(i*2)))
		if err := tree1.Add(k, v); err != nil {
			c.Fatal(err)
		}
		if err := tree2.Add(k, v); err != nil {
			c.Fatal(err)
		}
	}
	return tree1, tree2
}

func TestAddBatchTreeEmpty(t *testing.T) {
	c := qt.New(t)

	nLeafs := 1024

	database := metadb.NewTest(t)
	tree, err := NewTree(Config{database, 256, DefaultThresholdNLeafs, HashFunctionPoseidon})
	c.Assert(err, qt.IsNil)

	bLen := 32
	var keys, values [][]byte
	for i := 0; i < nLeafs; i++ {
		k := BigIntToBytesLE(bLen, big.NewInt(int64(i)))
		v := BigIntToBytesLE(bLen, big.NewInt(int64(i*2)))
		keys = append(keys, k)
		values = append(values, v)
	}

	start := time.Now()
	for i := 0; i < nLeafs; i++ {
		if err := tree.Add(keys[i], values[i]); err != nil {
			t.Fatal(err)
		}
	}
	time1 := time.Since(start)

	database2 := metadb.NewTest(t)
	tree2, err := NewTree(Config{database2, 256, DefaultThresholdNLeafs, HashFunctionPoseidon})
	c.Assert(err, qt.IsNil)
	tree2.dbgInit()

	start = time.Now()
	invalids, err := tree2.AddBatch(keys, values)
	c.Assert(err, qt.IsNil)
	time2 := time.Since(start)
	if debug {
		debugTime("Case tree empty, AddBatch", time1, time2)
		printTestContext("	", nLeafs, "Poseidon", "memory")
		tree2.dbg.print("	")
	}
	c.Check(len(invalids), qt.Equals, 0)

	// check that both trees roots are equal
	checkRoots(c, tree, tree2)
}

func TestAddBatchTreeEmptyNotPowerOf2(t *testing.T) {
	c := qt.New(t)

	nLeafs := 1027

	database := metadb.NewTest(t)
	tree, err := NewTree(Config{
		database, 256, DefaultThresholdNLeafs,
		HashFunctionPoseidon,
	})
	c.Assert(err, qt.IsNil)

	bLen := 32
	for i := 0; i < nLeafs; i++ {
		k := BigIntToBytesLE(bLen, big.NewInt(int64(i)))
		v := BigIntToBytesLE(bLen, big.NewInt(int64(i*2)))
		if err := tree.Add(k, v); err != nil {
			t.Fatal(err)
		}
	}

	database2 := metadb.NewTest(t)
	tree2, err := NewTree(Config{
		database2, 256, DefaultThresholdNLeafs,
		HashFunctionPoseidon,
	})
	c.Assert(err, qt.IsNil)

	var keys, values [][]byte
	for i := 0; i < nLeafs; i++ {
		k := BigIntToBytesLE(bLen, big.NewInt(int64(i)))
		v := BigIntToBytesLE(bLen, big.NewInt(int64(i*2)))
		keys = append(keys, k)
		values = append(values, v)
	}
	invalids, err := tree2.AddBatch(keys, values)
	c.Assert(err, qt.IsNil)
	c.Check(len(invalids), qt.Equals, 0)

	// check that both trees roots are equal
	checkRoots(c, tree, tree2)
}

func randomBytes(n int) []byte {
	b := make([]byte, n)
	_, err := rand.Read(b)
	if err != nil {
		panic(err)
	}
	return b
}

func TestAddBatchTestVector1(t *testing.T) {
	c := qt.New(t)
	database1 := metadb.NewTest(t)
	tree1, err := NewTree(Config{
		database1, 256, DefaultThresholdNLeafs,
		HashFunctionBlake2b,
	})
	c.Assert(err, qt.IsNil)

	database2 := metadb.NewTest(t)
	tree2, err := NewTree(Config{
		database2, 256, DefaultThresholdNLeafs,
		HashFunctionBlake2b,
	})
	c.Assert(err, qt.IsNil)

	// leafs in 2nd level subtrees: [ 6, 0, 1, 1]
	testvectorKeys := []string{
		"1c7c2265e368314ca58ed2e1f33a326f1220e234a566d55c3605439dbe411642",
		"2c9f0a578afff5bfa4e0992a43066460faaab9e8e500db0b16647c701cdb16bf",
		"1c45cb31f2fa39ec7b9ebf0fad40e0b8296016b5ce8844ae06ff77226379d9a5",
		"d8af98bbbb585129798ae54d5eabbc9d0561d583faf1663b3a3724d15bda4ec7",
	}
	var keys, values [][]byte
	for i := 0; i < len(testvectorKeys); i++ {
		key, err := hex.DecodeString(testvectorKeys[i])
		c.Assert(err, qt.IsNil)
		keys = append(keys, key)
		values = append(values, []byte{0})
	}

	for i := 0; i < len(keys); i++ {
		if err := tree1.Add(keys[i], values[i]); err != nil {
			t.Fatal(err)
		}
	}

	invalids, err := tree2.AddBatch(keys, values)
	c.Assert(err, qt.IsNil)
	c.Check(len(invalids), qt.Equals, 0)
	// check that both trees roots are equal
	checkRoots(c, tree1, tree2)

	// 2nd test vectors
	database1 = metadb.NewTest(t)
	tree1, err = NewTree(Config{
		database1, 256, DefaultThresholdNLeafs,
		HashFunctionBlake2b,
	})
	c.Assert(err, qt.IsNil)

	database2 = metadb.NewTest(t)
	tree2, err = NewTree(Config{
		database2, 256, DefaultThresholdNLeafs,
		HashFunctionBlake2b,
	})
	c.Assert(err, qt.IsNil)

	testvectorKeys = []string{
		"1c7c2265e368314ca58ed2e1f33a326f1220e234a566d55c3605439dbe411642",
		"2c9f0a578afff5bfa4e0992a43066460faaab9e8e500db0b16647c701cdb16bf",
		"9cb87ec67e875c61390edcd1ab517f443591047709a4d4e45b0f9ed980857b8e",
		"9b4e9e92e974a589f426ceeb4cb291dc24893513fecf8e8460992dcf52621d4d",
		"1c45cb31f2fa39ec7b9ebf0fad40e0b8296016b5ce8844ae06ff77226379d9a5",
		"d8af98bbbb585129798ae54d5eabbc9d0561d583faf1663b3a3724d15bda4ec7",
		"3cd55dbfb8f975f20a0925dfbdabe79fa2d51dd0268afbb8ba6b01de9dfcdd3c",
		"5d0a9d6d9f197c091bf054fac9cb60e11ec723d6610ed8578e617b4d46cb43d5",
	}
	keys = [][]byte{}
	values = [][]byte{}
	for i := 0; i < len(testvectorKeys); i++ {
		key, err := hex.DecodeString(testvectorKeys[i])
		c.Assert(err, qt.IsNil)
		keys = append(keys, key)
		values = append(values, []byte{0})
	}

	for i := 0; i < len(keys); i++ {
		if err := tree1.Add(keys[i], values[i]); err != nil {
			t.Fatal(err)
		}
	}

	invalids, err = tree2.AddBatch(keys, values)
	c.Assert(err, qt.IsNil)
	c.Check(len(invalids), qt.Equals, 0)
	// check that both trees roots are equal
	checkRoots(c, tree1, tree2)
}

func TestAddBatchTestVector2(t *testing.T) {
	// test vector with unbalanced tree
	c := qt.New(t)

	database := metadb.NewTest(t)
	tree1, err := NewTree(Config{
		database, 256, DefaultThresholdNLeafs,
		HashFunctionPoseidon,
	})
	c.Assert(err, qt.IsNil)

	database2 := metadb.NewTest(t)
	tree2, err := NewTree(Config{
		database2, 256, DefaultThresholdNLeafs,
		HashFunctionPoseidon,
	})
	c.Assert(err, qt.IsNil)

	bLen := tree1.HashFunction().Len()
	var keys, values [][]byte
	// 1
	keys = append(keys, BigIntToBytesLE(bLen, big.NewInt(int64(1))))
	values = append(values, BigIntToBytesLE(bLen, big.NewInt(int64(1))))
	// 2
	keys = append(keys, BigIntToBytesLE(bLen, big.NewInt(int64(2))))
	values = append(values, BigIntToBytesLE(bLen, big.NewInt(int64(2))))
	// 3
	keys = append(keys, BigIntToBytesLE(bLen, big.NewInt(int64(3))))
	values = append(values, BigIntToBytesLE(bLen, big.NewInt(int64(3))))
	// 5
	keys = append(keys, BigIntToBytesLE(bLen, big.NewInt(int64(5))))
	values = append(values, BigIntToBytesLE(bLen, big.NewInt(int64(5))))

	for i := 0; i < len(keys); i++ {
		if err := tree1.Add(keys[i], values[i]); err != nil {
			t.Fatal(err)
		}
	}

	invalids, err := tree2.AddBatch(keys, values)
	c.Assert(err, qt.IsNil)
	c.Check(len(invalids), qt.Equals, 0)

	// check that both trees roots are equal
	checkRoots(c, tree1, tree2)
}

func TestAddBatchTestVector3(t *testing.T) {
	// test vector with unbalanced tree
	c := qt.New(t)

	database := metadb.NewTest(t)
	tree1, err := NewTree(Config{
		database, 256, DefaultThresholdNLeafs,
		HashFunctionPoseidon,
	})
	c.Assert(err, qt.IsNil)

	database2 := metadb.NewTest(t)
	tree2, err := NewTree(Config{
		database2, 256, DefaultThresholdNLeafs,
		HashFunctionPoseidon,
	})
	c.Assert(err, qt.IsNil)

	bLen := tree1.HashFunction().Len()
	var keys, values [][]byte
	// 0
	keys = append(keys, BigIntToBytesLE(bLen, big.NewInt(int64(0))))
	values = append(values, BigIntToBytesLE(bLen, big.NewInt(int64(0))))
	// 3
	keys = append(keys, BigIntToBytesLE(bLen, big.NewInt(int64(3))))
	values = append(values, BigIntToBytesLE(bLen, big.NewInt(int64(3))))
	// 7
	keys = append(keys, BigIntToBytesLE(bLen, big.NewInt(int64(7))))
	values = append(values, BigIntToBytesLE(bLen, big.NewInt(int64(7))))
	// 135
	keys = append(keys, BigIntToBytesLE(bLen, big.NewInt(int64(135))))
	values = append(values, BigIntToBytesLE(bLen, big.NewInt(int64(135))))

	for i := 0; i < len(keys); i++ {
		if err := tree1.Add(keys[i], values[i]); err != nil {
			t.Fatal(err)
		}
	}

	invalids, err := tree2.AddBatch(keys, values)
	c.Assert(err, qt.IsNil)
	c.Check(len(invalids), qt.Equals, 0)

	// check that both trees roots are equal
	checkRoots(c, tree1, tree2)
	//
	// tree1.PrintGraphvizFirstNLevels(nil, 100)
	// tree2.PrintGraphvizFirstNLevels(nil, 100)
}

func TestAddBatchTreeEmptyRandomKeys(t *testing.T) {
	c := qt.New(t)

	nLeafs := 8

	database1 := metadb.NewTest(t)
	tree1, err := NewTree(Config{
		database1, 256, DefaultThresholdNLeafs,
		HashFunctionBlake2b,
	})
	c.Assert(err, qt.IsNil)

	database2 := metadb.NewTest(t)
	tree2, err := NewTree(Config{
		database2, 256, DefaultThresholdNLeafs,
		HashFunctionBlake2b,
	})
	c.Assert(err, qt.IsNil)

	var keys, values [][]byte
	for i := 0; i < nLeafs; i++ {
		keys = append(keys, randomBytes(32))
		values = append(values, randomBytes(32))
	}

	for i := 0; i < len(keys); i++ {
		if err := tree1.Add(keys[i], values[i]); err != nil {
			t.Fatal(err)
		}
	}

	invalids, err := tree2.AddBatch(keys, values)
	c.Assert(err, qt.IsNil)
	c.Check(len(invalids), qt.Equals, 0)
	// check that both trees roots are equal
	checkRoots(c, tree1, tree2)
}

func TestAddBatchTreeNotEmptyFewLeafs(t *testing.T) {
	c := qt.New(t)

	nLeafs := 1024
	initialNLeafs := 99

	tree1, tree2 := testInit(c, initialNLeafs)
	tree2.dbgInit()

	bLen := tree1.HashFunction().Len()
	start := time.Now()
	for i := initialNLeafs; i < nLeafs; i++ {
		k := BigIntToBytesLE(bLen, big.NewInt(int64(i)))
		v := BigIntToBytesLE(bLen, big.NewInt(int64(i*2)))
		if err := tree1.Add(k, v); err != nil {
			t.Fatal(err)
		}
	}
	time1 := time.Since(start)

	// prepare the key-values to be added
	var keys, values [][]byte
	for i := initialNLeafs; i < nLeafs; i++ {
		k := BigIntToBytesLE(bLen, big.NewInt(int64(i)))
		v := BigIntToBytesLE(bLen, big.NewInt(int64(i*2)))
		keys = append(keys, k)
		values = append(values, v)
	}
	start = time.Now()
	invalids, err := tree2.AddBatch(keys, values)
	c.Assert(err, qt.IsNil)
	time2 := time.Since(start)
	if debug {
		debugTime("Case tree not empty w/ few leafs, AddBatch", time1, time2)
		printTestContext("	", nLeafs, "Poseidon", "memory")
		tree2.dbg.print("	")
	}
	c.Check(len(invalids), qt.Equals, 0)

	// check that both trees roots are equal
	checkRoots(c, tree1, tree2)
}

func TestAddBatchTreeNotEmptyEnoughLeafs(t *testing.T) {
	c := qt.New(t)

	nLeafs := 1024
	initialNLeafs := 500

	tree1, tree2 := testInit(c, initialNLeafs)
	tree2.dbgInit()

	bLen := tree1.HashFunction().Len()
	start := time.Now()
	for i := initialNLeafs; i < nLeafs; i++ {
		k := BigIntToBytesLE(bLen, big.NewInt(int64(i)))
		v := BigIntToBytesLE(bLen, big.NewInt(int64(i*2)))
		if err := tree1.Add(k, v); err != nil {
			t.Fatal(err)
		}
	}
	time1 := time.Since(start)

	// prepare the key-values to be added
	var keys, values [][]byte
	for i := initialNLeafs; i < nLeafs; i++ {
		k := BigIntToBytesLE(bLen, big.NewInt(int64(i)))
		v := BigIntToBytesLE(bLen, big.NewInt(int64(i*2)))
		keys = append(keys, k)
		values = append(values, v)
	}
	start = time.Now()
	invalids, err := tree2.AddBatch(keys, values)
	c.Assert(err, qt.IsNil)
	time2 := time.Since(start)
	if debug {
		debugTime("Case tree not empty w/ enough leafs, AddBatch", time1, time2)
		printTestContext("	", nLeafs, "Poseidon", "memory")
		tree2.dbg.print("	")
	}
	c.Check(len(invalids), qt.Equals, 0)
	// check that both trees roots are equal
	checkRoots(c, tree1, tree2)
}

func TestAddBatchTreeEmptyRepeatedLeafs(t *testing.T) {
	c := qt.New(t)

	nLeafs := 1024
	nRepeatedKeys := 99

	tree1, tree2 := testInit(c, 0)

	bLen := tree1.HashFunction().Len()
	// prepare the key-values to be added
	var keys, values [][]byte
	for i := 0; i < nLeafs; i++ {
		k := BigIntToBytesLE(bLen, big.NewInt(int64(i)))
		v := BigIntToBytesLE(bLen, big.NewInt(int64(i*2)))
		keys = append(keys, k)
		values = append(values, v)
	}
	// add repeated key-values
	for i := 0; i < nRepeatedKeys; i++ {
		k := BigIntToBytesLE(bLen, big.NewInt(int64(i)))
		v := BigIntToBytesLE(bLen, big.NewInt(int64(i*2)))
		keys = append(keys, k)
		values = append(values, v)
	}

	// add the non-repeated key-values in tree1 with .Add loop
	for i := 0; i < nLeafs; i++ {
		if err := tree1.Add(keys[i], values[i]); err != nil {
			t.Fatal(err)
		}
	}

	invalids, err := tree2.AddBatch(keys, values)
	c.Assert(err, qt.IsNil)
	c.Check(len(invalids), qt.Equals, nRepeatedKeys)
	// check that both trees roots are equal
	checkRoots(c, tree1, tree2)
}

func TestAddBatchTreeNotEmptyFewLeafsRepeatedLeafs(t *testing.T) {
	c := qt.New(t)

	nLeafs := 1024
	initialNLeafs := 99

	tree1, tree2 := testInit(c, initialNLeafs)

	bLen := tree1.HashFunction().Len()
	// prepare the key-values to be added
	var keys, values [][]byte
	for i := 0; i < nLeafs; i++ {
		k := BigIntToBytesLE(bLen, big.NewInt(int64(i)))
		v := BigIntToBytesLE(bLen, big.NewInt(int64(i*2)))
		keys = append(keys, k)
		values = append(values, v)
	}

	// add the keys that will be existing when AddBatch is called
	for i := initialNLeafs; i < nLeafs; i++ {
		if err := tree1.Add(keys[i], values[i]); err != nil {
			t.Fatal(err)
		}
	}

	invalids, err := tree2.AddBatch(keys, values)
	c.Assert(err, qt.IsNil)
	c.Assert(len(invalids), qt.Equals, initialNLeafs)
	// check that both trees roots are equal
	checkRoots(c, tree1, tree2)
}

func TestSplitInBuckets(t *testing.T) {
	c := qt.New(t)

	bLen := HashFunctionPoseidon.Len()
	nLeafs := 16
	kvs := make([]kv, nLeafs)
	for i := 0; i < nLeafs; i++ {
		k := BigIntToBytesLE(bLen, big.NewInt(int64(i)))
		v := BigIntToBytesLE(bLen, big.NewInt(int64(i*2)))
		keyPath := make([]byte, 32)
		copy(keyPath, k)
		kvs[i].pos = i
		kvs[i].keyPath = k
		kvs[i].k = k
		kvs[i].v = v
	}

	// check keyToBucket results for 4 buckets & 8 keys
	c.Assert(keyToBucket(kvs[0].k, 4), qt.Equals, 0)
	c.Assert(keyToBucket(kvs[1].k, 4), qt.Equals, 2)
	c.Assert(keyToBucket(kvs[2].k, 4), qt.Equals, 1)
	c.Assert(keyToBucket(kvs[3].k, 4), qt.Equals, 3)
	c.Assert(keyToBucket(kvs[4].k, 4), qt.Equals, 0)
	c.Assert(keyToBucket(kvs[5].k, 4), qt.Equals, 2)
	c.Assert(keyToBucket(kvs[6].k, 4), qt.Equals, 1)
	c.Assert(keyToBucket(kvs[7].k, 4), qt.Equals, 3)

	// check keyToBucket results for 8 buckets & 8 keys
	c.Assert(keyToBucket(kvs[0].k, 8), qt.Equals, 0)
	c.Assert(keyToBucket(kvs[1].k, 8), qt.Equals, 4)
	c.Assert(keyToBucket(kvs[2].k, 8), qt.Equals, 2)
	c.Assert(keyToBucket(kvs[3].k, 8), qt.Equals, 6)
	c.Assert(keyToBucket(kvs[4].k, 8), qt.Equals, 1)
	c.Assert(keyToBucket(kvs[5].k, 8), qt.Equals, 5)
	c.Assert(keyToBucket(kvs[6].k, 8), qt.Equals, 3)
	c.Assert(keyToBucket(kvs[7].k, 8), qt.Equals, 7)

	buckets := splitInBuckets(kvs, 4)

	expected := [][]string{
		{
			"00000000", // bucket 0
			"08000000",
			"04000000",
			"0c000000",
		},
		{
			"02000000", // bucket 1
			"0a000000",
			"06000000",
			"0e000000",
		},
		{
			"01000000", // bucket 2
			"09000000",
			"05000000",
			"0d000000",
		},
		{
			"03000000", // bucket 3
			"0b000000",
			"07000000",
			"0f000000",
		},
	}

	for i := 0; i < len(buckets); i++ {
		sortKvs(buckets[i])
		c.Assert(len(buckets[i]), qt.Equals, len(expected[i]))
		for j := 0; j < len(buckets[i]); j++ {
			c.Check(hex.EncodeToString(buckets[i][j].k[:4]),
				qt.Equals, expected[i][j])
		}
	}
}

// compareBytes compares byte slices where the bytes are compared from left to
// right and each byte is compared by bit from right to left
func compareBytes(a, b []byte) bool {
	// WIP
	for i := 0; i < len(a); i++ {
		for j := 0; j < 8; j++ {
			aBit := a[i] & (1 << j)
			bBit := b[i] & (1 << j)
			if aBit > bBit {
				return false
			} else if aBit < bBit {
				return true
			}
		}
	}
	return false
}

// sortKvs sorts the kv by path
func sortKvs(kvs []kv) {
	sort.Slice(kvs, func(i, j int) bool {
		return compareBytes(kvs[i].keyPath, kvs[j].keyPath)
	})
}

func TestAddBatchTreeNotEmpty(t *testing.T) {
	c := qt.New(t)

	nLeafs := 4096
	initialNLeafs := 900

	tree1, tree2 := testInit(c, initialNLeafs)
	tree2.dbgInit()

	bLen := tree1.HashFunction().Len()
	start := time.Now()
	for i := initialNLeafs; i < nLeafs; i++ {
		k := BigIntToBytesLE(bLen, big.NewInt(int64(i)))
		v := BigIntToBytesLE(bLen, big.NewInt(int64(i*2)))
		if err := tree1.Add(k, v); err != nil {
			t.Fatal(err)
		}
	}
	time1 := time.Since(start)

	// prepare the key-values to be added
	var keys, values [][]byte
	for i := initialNLeafs; i < nLeafs; i++ {
		k := BigIntToBytesLE(bLen, big.NewInt(int64(i)))
		v := BigIntToBytesLE(bLen, big.NewInt(int64(i*2)))
		keys = append(keys, k)
		values = append(values, v)
	}
	start = time.Now()
	invalids, err := tree2.AddBatch(keys, values)
	c.Assert(err, qt.IsNil)
	time2 := time.Since(start)
	if debug {
		debugTime("Case tree not empty, AddBatch", time1, time2)
		printTestContext("	", nLeafs, "Poseidon", "memory")
		tree2.dbg.print("	")
	}
	c.Check(len(invalids), qt.Equals, 0)

	// check that both trees roots are equal
	checkRoots(c, tree1, tree2)
}

func TestAddBatchNotEmptyUnbalanced(t *testing.T) {
	c := qt.New(t)

	nLeafs := 4096
	initialNLeafs := 900

	tree1, _ := testInit(c, initialNLeafs)
	bLen := tree1.HashFunction().Len()

	start := time.Now()
	for i := initialNLeafs; i < nLeafs; i++ {
		k := BigIntToBytesLE(bLen, big.NewInt(int64(i)))
		v := BigIntToBytesLE(bLen, big.NewInt(int64(i*2)))
		if err := tree1.Add(k, v); err != nil {
			t.Fatal(err)
		}
	}
	time1 := time.Since(start)

	database2 := metadb.NewTest(t)
	tree2, err := NewTree(Config{
		database2, 256, DefaultThresholdNLeafs,
		HashFunctionPoseidon,
	})
	c.Assert(err, qt.IsNil)
	tree2.dbgInit()

	var keys, values [][]byte
	// add the initial leafs to fill a bit the tree before calling the
	// AddBatch method
	for i := 0; i < initialNLeafs; i++ {
		k := BigIntToBytesLE(bLen, big.NewInt(int64(i)))
		v := BigIntToBytesLE(bLen, big.NewInt(int64(i*2)))
		// use only the keys of one bucket, store the not used ones for
		// later
		if i%4 != 0 {
			keys = append(keys, k)
			values = append(values, v)
			continue
		}
		if err := tree2.Add(k, v); err != nil {
			t.Fatal(err)
		}
	}

	for i := initialNLeafs; i < nLeafs; i++ {
		k := BigIntToBytesLE(bLen, big.NewInt(int64(i)))
		v := BigIntToBytesLE(bLen, big.NewInt(int64(i*2)))
		keys = append(keys, k)
		values = append(values, v)
	}
	start = time.Now()
	invalids, err := tree2.AddBatch(keys, values)
	c.Assert(err, qt.IsNil)
	time2 := time.Since(start)
	if debug {
		debugTime("Case tree not empty & unbalanced, AddBatch", time1, time2)
		printTestContext("	", nLeafs, "Poseidon", "memory")
		tree2.dbg.print("	")
	}
	c.Check(len(invalids), qt.Equals, 0)

	// check that both trees roots are equal
	checkRoots(c, tree1, tree2)
}

func TestFlp2(t *testing.T) {
	c := qt.New(t)
	c.Assert(flp2(31), qt.Equals, 16)
	c.Assert(flp2(32), qt.Equals, 32)
	c.Assert(flp2(33), qt.Equals, 32)
	c.Assert(flp2(63), qt.Equals, 32)
	c.Assert(flp2(64), qt.Equals, 64)
	c.Assert(flp2(9000), qt.Equals, 8192)
}

func TestAddBatchBench(t *testing.T) {
	nLeafs := 5_000
	printTestContext("TestAddBatchBench: ", nLeafs, "Blake2b", "pebbledb")

	// prepare inputs
	var ks, vs [][]byte
	for i := 0; i < nLeafs; i++ {
		k := randomBytes(32)
		v := randomBytes(32)
		ks = append(ks, k)
		vs = append(vs, v)
	}

	benchAdd(t, ks, vs)

	benchAddBatch(t, ks, vs)
}

func benchAdd(t *testing.T, ks, vs [][]byte) {
	c := qt.New(t)

	database := metadb.NewTest(t)
	tree, err := NewTree(Config{database, 256, DefaultThresholdNLeafs, HashFunctionBlake2b})
	c.Assert(err, qt.IsNil)

	start := time.Now()
	for i := 0; i < len(ks); i++ {
		err = tree.Add(ks[i], vs[i])
		c.Assert(err, qt.IsNil)
	}
	if debug {
		printRes("	Add loop", time.Since(start))
		tree.dbg.print("		")
	}
}

func benchAddBatch(t *testing.T, ks, vs [][]byte) {
	c := qt.New(t)

	database := metadb.NewTest(t)
	tree, err := NewTree(Config{
		database, 256, DefaultThresholdNLeafs,
		HashFunctionBlake2b,
	})
	c.Assert(err, qt.IsNil)

	tree.dbgInit()

	start := time.Now()
	invalids, err := tree.AddBatch(ks, vs)
	if debug {
		printRes("	AddBatch", time.Since(start))
		tree.dbg.print("		")
	}
	c.Assert(err, qt.IsNil)
	c.Assert(len(invalids), qt.Equals, 0)
}

func TestDbgStats(t *testing.T) {
	c := qt.New(t)

	nLeafs := 10_000

	// prepare inputs
	var ks, vs [][]byte
	for i := 0; i < nLeafs; i++ {
		k := randomBytes(32)
		v := randomBytes(32)
		ks = append(ks, k)
		vs = append(vs, v)
	}

	// 1
	database1 := metadb.NewTest(t)
	tree1, err := NewTree(Config{
		database1, 256, DefaultThresholdNLeafs,
		HashFunctionBlake2b,
	})
	c.Assert(err, qt.IsNil)

	tree1.dbgInit()

	for i := 0; i < len(ks); i++ {
		err = tree1.Add(ks[i], vs[i])
		c.Assert(err, qt.IsNil)
	}

	// 2
	database2 := metadb.NewTest(t)
	tree2, err := NewTree(Config{
		database2, 256, DefaultThresholdNLeafs,
		HashFunctionBlake2b,
	})
	c.Assert(err, qt.IsNil)

	tree2.dbgInit()

	invalids, err := tree2.AddBatch(ks, vs)
	c.Assert(err, qt.IsNil)
	c.Assert(len(invalids), qt.Equals, 0)

	// 3
	database3 := metadb.NewTest(t)
	tree3, err := NewTree(Config{
		database3, 256, DefaultThresholdNLeafs,
		HashFunctionBlake2b,
	})
	c.Assert(err, qt.IsNil)

	tree3.dbgInit()

	// add few key-values
	// invalids, err = tree3.AddBatch(ks[:], vs[:])
	invalids, err = tree3.AddBatch(ks[:1000], vs[:1000])
	c.Assert(err, qt.IsNil)
	c.Assert(len(invalids), qt.Equals, 0)

	// add the rest of key-values
	invalids, err = tree3.AddBatch(ks[1000:], vs[1000:])
	c.Assert(err, qt.IsNil)
	c.Assert(len(invalids), qt.Equals, 0)

	checkRoots(c, tree1, tree2)
	checkRoots(c, tree1, tree3)

	if debug {
		fmt.Println("TestDbgStats")
		tree1.dbg.print("	add in loop in emptyTree ")
		tree2.dbg.print("	addbatch caseEmptyTree ")
		tree3.dbg.print("	addbatch caseNotEmptyTree ")
	}
}

func TestLoadVT(t *testing.T) {
	c := qt.New(t)

	nLeafs := 1024

	database := metadb.NewTest(t)
	tree, err := NewTree(Config{
		database, 256, DefaultThresholdNLeafs,
		HashFunctionPoseidon,
	})
	c.Assert(err, qt.IsNil)

	var keys, values [][]byte
	for i := 0; i < nLeafs; i++ {
		k := randomBytes(31)
		v := randomBytes(31)
		keys = append(keys, k)
		values = append(values, v)
	}
	invalids, err := tree.AddBatch(keys, values)
	c.Assert(err, qt.IsNil)
	c.Check(len(invalids), qt.Equals, 0)

	vt, err := tree.loadVT(tree.db)
	c.Assert(err, qt.IsNil)
	_, err = vt.computeHashes()
	c.Assert(err, qt.IsNil)

	// check that tree & vt roots are equal
	root, err := tree.Root()
	c.Assert(err, qt.IsNil)
	c.Check(root, qt.DeepEquals, vt.root.h)
}

// TestAddKeysWithEmptyValues calls AddBatch giving an array of empty values
func TestAddKeysWithEmptyValues(t *testing.T) {
	c := qt.New(t)

	nLeafs := 1024

	database := metadb.NewTest(t)
	tree, err := NewTree(Config{
		database, 256, DefaultThresholdNLeafs,
		HashFunctionPoseidon,
	})
	c.Assert(err, qt.IsNil)

	bLen := 32
	var keys, values [][]byte
	for i := 0; i < nLeafs; i++ {
		k := BigIntToBytesLE(bLen, big.NewInt(int64(i)))
		v := []byte{}
		keys = append(keys, k)
		values = append(values, v)
	}

	for i := 0; i < nLeafs; i++ {
		if err := tree.Add(keys[i], values[i]); err != nil {
			t.Fatal(err)
		}
	}

	database2 := metadb.NewTest(t)
	tree2, err := NewTree(Config{
		database2, 256, DefaultThresholdNLeafs,
		HashFunctionPoseidon,
	})
	c.Assert(err, qt.IsNil)
	tree2.dbgInit()

	invalids, err := tree2.AddBatch(keys, values)
	c.Assert(err, qt.IsNil)
	c.Check(len(invalids), qt.Equals, 0)
	// check that both trees roots are equal
	checkRoots(c, tree, tree2)

	// use tree3 to add nil value array
	database3 := metadb.NewTest(t)
	tree3, err := NewTree(Config{
		database3, 256, DefaultThresholdNLeafs,
		HashFunctionPoseidon,
	})
	c.Assert(err, qt.IsNil)

	invalids, err = tree3.AddBatch(keys, nil)
	c.Assert(err, qt.IsNil)
	c.Check(len(invalids), qt.Equals, 0)
	checkRoots(c, tree, tree3)

	kAux, proofV, siblings, existence, err := tree2.GenProof(keys[9])
	c.Assert(err, qt.IsNil)
	c.Assert(proofV, qt.DeepEquals, values[9])
	c.Assert(keys[9], qt.DeepEquals, kAux)
	c.Assert(existence, qt.IsTrue)

	// check with empty array
	root, err := tree.Root()
	c.Assert(err, qt.IsNil)
	err = CheckProof(tree.hashFunction, keys[9], []byte{}, root, siblings)
	c.Assert(err, qt.IsNil)

	// check with array with only 1 zero
	err = CheckProof(tree.hashFunction, keys[9], []byte{0}, root, siblings)
	c.Assert(err, qt.IsNil)

	// check with array with 32 zeroes
	e32 := []byte{
		0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0,
		0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0,
	}
	c.Assert(len(e32), qt.Equals, 32)
	err = CheckProof(tree.hashFunction, keys[9], e32, root, siblings)
	c.Assert(err, qt.IsNil)

	// check with array with value!=0 returns false at verification
	err = CheckProof(tree.hashFunction, keys[9], []byte{0, 1}, root, siblings)
	c.Assert(err, qt.ErrorMatches, "calculated vs expected root mismatch")
}

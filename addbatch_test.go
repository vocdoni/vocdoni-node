package arbo

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"math/big"
	"runtime"
	"testing"
	"time"

	qt "github.com/frankban/quicktest"
	"github.com/iden3/go-merkletree/db/leveldb"
	"github.com/iden3/go-merkletree/db/memory"
)

var debug = false

func printTestContext(prefix string, nLeafs int, hashName, dbName string) {
	if debug {
		fmt.Printf("%snCPU: %d, nLeafs: %d, hash: %s, db: %s\n",
			prefix, runtime.NumCPU(), nLeafs, hashName, dbName)
	}
}

func printRes(name string, duration time.Duration) {
	if debug {
		fmt.Printf("%s:	%s \n", name, duration)
	}
}

func debugTime(descr string, time1, time2 time.Duration) {
	if debug {
		fmt.Printf("%s was %f times faster than without AddBatch\n",
			descr, float64(time1)/float64(time2))
	}
}

func testInit(c *qt.C, n int) (*Tree, *Tree) {
	tree1, err := NewTree(memory.NewMemoryStorage(), 100, HashFunctionPoseidon)
	c.Assert(err, qt.IsNil)
	defer tree1.db.Close()

	tree2, err := NewTree(memory.NewMemoryStorage(), 100, HashFunctionPoseidon)
	c.Assert(err, qt.IsNil)
	defer tree2.db.Close()

	// add the initial leafs to fill a bit the trees before calling the
	// AddBatch method
	for i := 0; i < n; i++ {
		k := BigIntToBytes(big.NewInt(int64(i)))
		v := BigIntToBytes(big.NewInt(int64(i * 2)))
		if err := tree1.Add(k, v); err != nil {
			c.Fatal(err)
		}
		if err := tree2.Add(k, v); err != nil {
			c.Fatal(err)
		}
	}
	return tree1, tree2
}

func TestAddBatchCaseA(t *testing.T) {
	c := qt.New(t)

	nLeafs := 1024

	tree, err := NewTree(memory.NewMemoryStorage(), 100, HashFunctionPoseidon)
	c.Assert(err, qt.IsNil)
	defer tree.db.Close()

	start := time.Now()
	for i := 0; i < nLeafs; i++ {
		k := BigIntToBytes(big.NewInt(int64(i)))
		v := BigIntToBytes(big.NewInt(int64(i * 2)))
		if err := tree.Add(k, v); err != nil {
			t.Fatal(err)
		}
	}
	time1 := time.Since(start)

	tree2, err := NewTree(memory.NewMemoryStorage(), 100, HashFunctionPoseidon)
	c.Assert(err, qt.IsNil)
	defer tree2.db.Close()
	tree2.dbgInit()

	var keys, values [][]byte
	for i := 0; i < nLeafs; i++ {
		k := BigIntToBytes(big.NewInt(int64(i)))
		v := BigIntToBytes(big.NewInt(int64(i * 2)))
		keys = append(keys, k)
		values = append(values, v)
	}
	start = time.Now()
	indexes, err := tree2.AddBatch(keys, values)
	c.Assert(err, qt.IsNil)
	time2 := time.Since(start)
	if debug {
		debugTime("CASE A, AddBatch", time1, time2)
		printTestContext("	", nLeafs, "Poseidon", "memory")
		tree2.dbg.print("	")
	}
	c.Check(len(indexes), qt.Equals, 0)

	// check that both trees roots are equal
	c.Check(tree2.Root(), qt.DeepEquals, tree.Root())
}

func TestAddBatchCaseANotPowerOf2(t *testing.T) {
	c := qt.New(t)

	nLeafs := 1027

	tree, err := NewTree(memory.NewMemoryStorage(), 100, HashFunctionPoseidon)
	c.Assert(err, qt.IsNil)
	defer tree.db.Close()

	for i := 0; i < nLeafs; i++ {
		k := BigIntToBytes(big.NewInt(int64(i)))
		v := BigIntToBytes(big.NewInt(int64(i * 2)))
		if err := tree.Add(k, v); err != nil {
			t.Fatal(err)
		}
	}

	tree2, err := NewTree(memory.NewMemoryStorage(), 100, HashFunctionPoseidon)
	c.Assert(err, qt.IsNil)
	defer tree2.db.Close()

	var keys, values [][]byte
	for i := 0; i < nLeafs; i++ {
		k := BigIntToBytes(big.NewInt(int64(i)))
		v := BigIntToBytes(big.NewInt(int64(i * 2)))
		keys = append(keys, k)
		values = append(values, v)
	}
	indexes, err := tree2.AddBatch(keys, values)
	c.Assert(err, qt.IsNil)
	c.Check(len(indexes), qt.Equals, 0)

	// check that both trees roots are equal
	c.Check(tree2.Root(), qt.DeepEquals, tree.Root())
}

func randomBytes(n int) []byte {
	b := make([]byte, n)
	_, err := rand.Read(b)
	if err != nil {
		panic(err)
	}
	return b
}

func TestBuildTreeFromLeafsSingleThread(t *testing.T) {
	c := qt.New(t)
	tree1, err := NewTree(memory.NewMemoryStorage(), 100, HashFunctionBlake2b)
	c.Assert(err, qt.IsNil)
	defer tree1.db.Close()

	tree2, err := NewTree(memory.NewMemoryStorage(), 100, HashFunctionBlake2b)
	c.Assert(err, qt.IsNil)
	defer tree2.db.Close()

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

	kvs, err := tree2.keysValuesToKvs(keys, values)
	c.Assert(err, qt.IsNil)
	sortKvs(kvs)

	tree2.tx, err = tree2.db.NewTx()
	c.Assert(err, qt.IsNil)
	// indexes, err := tree2.buildTreeFromLeafsSingleThread(kvs)
	indexes, err := tree2.buildTreeFromLeafs(4, kvs)
	c.Assert(err, qt.IsNil)
	// tree1.PrintGraphviz(nil)
	// tree2.PrintGraphviz(nil)

	c.Check(len(indexes), qt.Equals, 0)

	// check that both trees roots are equal
	c.Check(tree2.Root(), qt.DeepEquals, tree1.Root())
	// 15b6a23945ae6c81342b7eb14e70fff50812dc8791cb15ec791eb08f91784139
}

func TestAddBatchCaseATestVector(t *testing.T) {
	c := qt.New(t)
	tree1, err := NewTree(memory.NewMemoryStorage(), 100, HashFunctionBlake2b)
	c.Assert(err, qt.IsNil)
	defer tree1.db.Close()

	tree2, err := NewTree(memory.NewMemoryStorage(), 100, HashFunctionBlake2b)
	c.Assert(err, qt.IsNil)
	defer tree2.db.Close()

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

	indexes, err := tree2.AddBatch(keys, values)
	c.Assert(err, qt.IsNil)
	// tree1.PrintGraphviz(nil)
	// tree2.PrintGraphviz(nil)

	c.Check(len(indexes), qt.Equals, 0)

	// check that both trees roots are equal
	// fmt.Println(hex.EncodeToString(tree1.Root()))
	c.Check(tree2.Root(), qt.DeepEquals, tree1.Root())
}

func TestAddBatchCaseARandomKeys(t *testing.T) {
	c := qt.New(t)

	nLeafs := 8

	tree1, err := NewTree(memory.NewMemoryStorage(), 100, HashFunctionBlake2b)
	c.Assert(err, qt.IsNil)
	defer tree1.db.Close()

	tree2, err := NewTree(memory.NewMemoryStorage(), 100, HashFunctionBlake2b)
	c.Assert(err, qt.IsNil)
	defer tree2.db.Close()

	var keys, values [][]byte
	for i := 0; i < nLeafs; i++ {
		keys = append(keys, randomBytes(32))
		// values = append(values, randomBytes(32))
		values = append(values, []byte{0})
		// fmt.Println("K", hex.EncodeToString(keys[i]))
	}

	// TMP:
	keys[0], _ = hex.DecodeString("1c7c2265e368314ca58ed2e1f33a326f1220e234a566d55c3605439dbe411642")
	keys[1], _ = hex.DecodeString("2c9f0a578afff5bfa4e0992a43066460faaab9e8e500db0b16647c701cdb16bf")
	keys[2], _ = hex.DecodeString("9cb87ec67e875c61390edcd1ab517f443591047709a4d4e45b0f9ed980857b8e")
	keys[3], _ = hex.DecodeString("9b4e9e92e974a589f426ceeb4cb291dc24893513fecf8e8460992dcf52621d4d")
	keys[4], _ = hex.DecodeString("1c45cb31f2fa39ec7b9ebf0fad40e0b8296016b5ce8844ae06ff77226379d9a5")
	keys[5], _ = hex.DecodeString("d8af98bbbb585129798ae54d5eabbc9d0561d583faf1663b3a3724d15bda4ec7")
	keys[6], _ = hex.DecodeString("3cd55dbfb8f975f20a0925dfbdabe79fa2d51dd0268afbb8ba6b01de9dfcdd3c")
	keys[7], _ = hex.DecodeString("5d0a9d6d9f197c091bf054fac9cb60e11ec723d6610ed8578e617b4d46cb43d5")

	for i := 0; i < len(keys); i++ {
		if err := tree1.Add(keys[i], values[i]); err != nil {
			t.Fatal(err)
		}
	}

	indexes, err := tree2.AddBatch(keys, values)
	c.Assert(err, qt.IsNil)
	// tree1.PrintGraphviz(nil)
	// tree2.PrintGraphviz(nil)

	c.Check(len(indexes), qt.Equals, 0)

	// check that both trees roots are equal
	c.Check(tree2.Root(), qt.DeepEquals, tree1.Root())
}

func TestAddBatchCaseB(t *testing.T) {
	c := qt.New(t)

	nLeafs := 1024
	initialNLeafs := 99 // TMP TODO use const minLeafsThreshold-1 once ready

	tree1, tree2 := testInit(c, initialNLeafs)
	tree2.dbgInit()

	start := time.Now()
	for i := initialNLeafs; i < nLeafs; i++ {
		k := BigIntToBytes(big.NewInt(int64(i)))
		v := BigIntToBytes(big.NewInt(int64(i * 2)))
		if err := tree1.Add(k, v); err != nil {
			t.Fatal(err)
		}
	}
	time1 := time.Since(start)

	// prepare the key-values to be added
	var keys, values [][]byte
	for i := initialNLeafs; i < nLeafs; i++ {
		k := BigIntToBytes(big.NewInt(int64(i)))
		v := BigIntToBytes(big.NewInt(int64(i * 2)))
		keys = append(keys, k)
		values = append(values, v)
	}
	start = time.Now()
	indexes, err := tree2.AddBatch(keys, values)
	c.Assert(err, qt.IsNil)
	time2 := time.Since(start)
	if debug {
		debugTime("CASE B, AddBatch", time1, time2)
		printTestContext("	", nLeafs, "Poseidon", "memory")
		tree2.dbg.print("	")
	}
	c.Check(len(indexes), qt.Equals, 0)

	// check that both trees roots are equal
	c.Check(tree2.Root(), qt.DeepEquals, tree1.Root())
}

func TestAddBatchCaseBRepeatedLeafs(t *testing.T) {
	c := qt.New(t)

	nLeafs := 1024
	initialNLeafs := 99 // TMP TODO use const minLeafsThreshold-1 once ready

	tree1, tree2 := testInit(c, initialNLeafs)

	for i := initialNLeafs; i < nLeafs; i++ {
		k := BigIntToBytes(big.NewInt(int64(i)))
		v := BigIntToBytes(big.NewInt(int64(i * 2)))
		if err := tree1.Add(k, v); err != nil {
			t.Fatal(err)
		}
	}

	// prepare the key-values to be added, including already existing keys
	var keys, values [][]byte
	for i := 0; i < nLeafs; i++ {
		k := BigIntToBytes(big.NewInt(int64(i)))
		v := BigIntToBytes(big.NewInt(int64(i * 2)))
		keys = append(keys, k)
		values = append(values, v)
	}
	indexes, err := tree2.AddBatch(keys, values)
	c.Assert(err, qt.IsNil)
	c.Check(len(indexes), qt.Equals, initialNLeafs)

	// check that both trees roots are equal
	c.Check(tree2.Root(), qt.DeepEquals, tree1.Root())
}

func TestCombineInKVSet(t *testing.T) {
	c := qt.New(t)

	var a, b, expected []kv
	for i := 0; i < 10; i++ {
		k := BigIntToBytes(big.NewInt(int64(i)))
		kv := kv{k: k}
		if i < 7 {
			a = append(a, kv)
		}
		if i >= 4 {
			b = append(b, kv)
		}
		expected = append(expected, kv)
	}

	r, invalids := combineInKVSet(a, b)
	for i := 0; i < len(r); i++ {
		c.Assert(r[i].k, qt.DeepEquals, expected[i].k)
	}
	c.Assert(len(invalids), qt.Equals, 7-4)
}

func TestGetKeysAtLevel(t *testing.T) {
	c := qt.New(t)

	tree, err := NewTree(memory.NewMemoryStorage(), 100, HashFunctionPoseidon)
	c.Assert(err, qt.IsNil)
	defer tree.db.Close()

	for i := 0; i < 32; i++ {
		k := BigIntToBytes(big.NewInt(int64(i)))
		v := BigIntToBytes(big.NewInt(int64(i * 2)))
		if err := tree.Add(k, v); err != nil {
			t.Fatal(err)
		}
	}

	keys, err := tree.getKeysAtLevel(2)
	c.Assert(err, qt.IsNil)
	expected := []string{
		"a5d5f14fce7348e40751496cf25d107d91b0bd043435b9577d778a01f8aa6111",
		"e9e8dd9b28a7f81d1ff34cb5cefc0146dd848b31031a427b79bdadb62e7f6910",
	}
	for i := 0; i < len(keys); i++ {
		c.Assert(hex.EncodeToString(keys[i]), qt.Equals, expected[i])
	}

	keys, err = tree.getKeysAtLevel(3)
	c.Assert(err, qt.IsNil)
	expected = []string{
		"9f12c13e52bca96ad4882a26558e48ab67ddd63e062b839207e893d961390f01",
		"16d246dd6826ec7346c7328f11c4261facf82d4689f33263ff6e207956a77f21",
		"4a22cc901c6337daa17a431fa20170684b710e5f551509511492ec24e81a8f2f",
		"470d61abcbd154977bffc9a9ec5a8daff0caabcf2a25e8441f604c79daa0f82d",
	}
	for i := 0; i < len(keys); i++ {
		c.Assert(hex.EncodeToString(keys[i]), qt.Equals, expected[i])
	}

	keys, err = tree.getKeysAtLevel(4)
	c.Assert(err, qt.IsNil)
	expected = []string{
		"7a5d1c81f7b96318012de3417e53d4f13df5b1337718651cd29d0cb0a66edd20",
		"3408213e4e844bdf3355eb8781c74e31626812898c2dbe141ed6d2c92256fc1c",
		"dfd8a4d0b6954a3e9f3892e655b58d456eeedf9367f27dfdd9bc2dd6a5577312",
		"9e99fbec06fb2a6725997c12c4995f62725eb4cce4808523a5a5e80cca64b007",
		"0befa1e070231dbf4e8ff841c05878cdec823e0c09594c24910a248b3ff5a628",
		"b7131b0a15c772a57005a4dc5d0d6dd4b3414f5d9ee7408ce5e86c5ab3520e04",
		"6d1abe0364077846a56bab1deb1a04883eb796b74fe531a7676a9a370f83ab21",
		"4270116394bede69cf9cd72069eca018238080380bef5de75be8dcbbe968e105",
	}
	for i := 0; i < len(keys); i++ {
		c.Assert(hex.EncodeToString(keys[i]), qt.Equals, expected[i])
	}
}

func TestSplitInBuckets(t *testing.T) {
	c := qt.New(t)

	nLeafs := 16
	kvs := make([]kv, nLeafs)
	for i := 0; i < nLeafs; i++ {
		k := BigIntToBytes(big.NewInt(int64(i)))
		v := BigIntToBytes(big.NewInt(int64(i * 2)))
		keyPath := make([]byte, 32)
		copy(keyPath[:], k)
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
			c.Check(hex.EncodeToString(buckets[i][j].k[:4]), qt.Equals, expected[i][j])
		}
	}
}

func TestAddBatchCaseC(t *testing.T) {
	c := qt.New(t)

	nLeafs := 1024
	initialNLeafs := 101 // TMP TODO use const minLeafsThreshold+1 once ready

	tree1, tree2 := testInit(c, initialNLeafs)
	tree2.dbgInit()

	start := time.Now()
	for i := initialNLeafs; i < nLeafs; i++ {
		k := BigIntToBytes(big.NewInt(int64(i)))
		v := BigIntToBytes(big.NewInt(int64(i * 2)))
		if err := tree1.Add(k, v); err != nil {
			t.Fatal(err)
		}
	}
	time1 := time.Since(start)

	// prepare the key-values to be added
	var keys, values [][]byte
	for i := initialNLeafs; i < nLeafs; i++ {
		k := BigIntToBytes(big.NewInt(int64(i)))
		v := BigIntToBytes(big.NewInt(int64(i * 2)))
		keys = append(keys, k)
		values = append(values, v)
	}
	start = time.Now()
	indexes, err := tree2.AddBatch(keys, values)
	c.Assert(err, qt.IsNil)
	time2 := time.Since(start)
	if debug {
		debugTime("CASE C, AddBatch", time1, time2)
		printTestContext("	", nLeafs, "Poseidon", "memory")
		tree2.dbg.print("	")
	}
	c.Check(len(indexes), qt.Equals, 0)

	// check that both trees roots are equal
	c.Check(tree2.Root(), qt.DeepEquals, tree1.Root())
}

func TestAddBatchCaseD(t *testing.T) {
	c := qt.New(t)

	nLeafs := 4096
	initialNLeafs := 900

	tree1, tree2 := testInit(c, initialNLeafs)
	tree2.dbgInit()

	start := time.Now()
	for i := initialNLeafs; i < nLeafs; i++ {
		k := BigIntToBytes(big.NewInt(int64(i)))
		v := BigIntToBytes(big.NewInt(int64(i * 2)))
		if err := tree1.Add(k, v); err != nil {
			t.Fatal(err)
		}
	}
	time1 := time.Since(start)

	// prepare the key-values to be added
	var keys, values [][]byte
	for i := initialNLeafs; i < nLeafs; i++ {
		k := BigIntToBytes(big.NewInt(int64(i)))
		v := BigIntToBytes(big.NewInt(int64(i * 2)))
		keys = append(keys, k)
		values = append(values, v)
	}
	start = time.Now()
	indexes, err := tree2.AddBatch(keys, values)
	c.Assert(err, qt.IsNil)
	time2 := time.Since(start)
	if debug {
		debugTime("CASE D, AddBatch", time1, time2)
		printTestContext("	", nLeafs, "Poseidon", "memory")
		tree2.dbg.print("	")
	}
	c.Check(len(indexes), qt.Equals, 0)

	// check that both trees roots are equal
	c.Check(tree2.Root(), qt.DeepEquals, tree1.Root())
}

func TestAddBatchCaseE(t *testing.T) {
	c := qt.New(t)

	nLeafs := 4096
	initialNLeafs := 900

	tree1, _ := testInit(c, initialNLeafs)

	start := time.Now()
	for i := initialNLeafs; i < nLeafs; i++ {
		k := BigIntToBytes(big.NewInt(int64(i)))
		v := BigIntToBytes(big.NewInt(int64(i * 2)))
		if err := tree1.Add(k, v); err != nil {
			t.Fatal(err)
		}
	}
	time1 := time.Since(start)

	tree2, err := NewTree(memory.NewMemoryStorage(), 100, HashFunctionPoseidon)
	c.Assert(err, qt.IsNil)
	defer tree2.db.Close()
	tree2.dbgInit()

	var keys, values [][]byte
	// add the initial leafs to fill a bit the tree before calling the
	// AddBatch method
	for i := 0; i < initialNLeafs; i++ {
		k := BigIntToBytes(big.NewInt(int64(i)))
		v := BigIntToBytes(big.NewInt(int64(i * 2)))
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
		k := BigIntToBytes(big.NewInt(int64(i)))
		v := BigIntToBytes(big.NewInt(int64(i * 2)))
		keys = append(keys, k)
		values = append(values, v)
	}
	start = time.Now()
	indexes, err := tree2.AddBatch(keys, values)
	c.Assert(err, qt.IsNil)
	time2 := time.Since(start)
	if debug {
		debugTime("CASE E, AddBatch", time1, time2)
		printTestContext("	", nLeafs, "Poseidon", "memory")
		tree2.dbg.print("	")
	}
	c.Check(len(indexes), qt.Equals, 0)

	// check that both trees roots are equal
	c.Check(tree2.Root(), qt.DeepEquals, tree1.Root())
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
	nLeafs := 50_000
	printTestContext("TestAddBatchBench: ", nLeafs, "Blake2b", "leveldb")

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

	dbDir := t.TempDir()
	storage, err := leveldb.NewLevelDbStorage(dbDir, false)
	c.Assert(err, qt.IsNil)
	tree, err := NewTree(storage, 140, HashFunctionBlake2b)
	c.Assert(err, qt.IsNil)

	if debug {
		tree.dbgInit()
	}
	start := time.Now()
	for i := 0; i < len(ks); i++ {
		err = tree.Add(ks[i], vs[i])
		c.Assert(err, qt.IsNil)
	}
	printRes("	Add loop", time.Since(start))
	tree.dbg.print("		")
}

func benchAddBatch(t *testing.T, ks, vs [][]byte) {
	c := qt.New(t)

	dbDir := t.TempDir()
	storage, err := leveldb.NewLevelDbStorage(dbDir, false)
	c.Assert(err, qt.IsNil)
	tree, err := NewTree(storage, 140, HashFunctionBlake2b)
	c.Assert(err, qt.IsNil)

	if debug {
		tree.dbgInit()
	}
	start := time.Now()
	invalids, err := tree.AddBatch(ks, vs)
	printRes("	AddBatch", time.Since(start))
	c.Assert(err, qt.IsNil)
	c.Assert(len(invalids), qt.Equals, 0)
	tree.dbg.print("		")
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
	tree1, err := NewTree(memory.NewMemoryStorage(), 100, HashFunctionBlake2b)
	c.Assert(err, qt.IsNil)
	defer tree1.db.Close()

	tree1.dbgInit()

	for i := 0; i < len(ks); i++ {
		err = tree1.Add(ks[i], vs[i])
		c.Assert(err, qt.IsNil)
	}

	// 2
	tree2, err := NewTree(memory.NewMemoryStorage(), 100, HashFunctionBlake2b)
	c.Assert(err, qt.IsNil)
	defer tree2.db.Close()

	tree2.dbgInit()

	invalids, err := tree2.AddBatch(ks, vs)
	c.Assert(err, qt.IsNil)
	c.Assert(len(invalids), qt.Equals, 0)

	// 3
	tree3, err := NewTree(memory.NewMemoryStorage(), 100, HashFunctionBlake2b)
	c.Assert(err, qt.IsNil)
	defer tree3.db.Close()

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

	c.Check(tree2.Root(), qt.DeepEquals, tree1.Root())
	c.Check(tree3.Root(), qt.DeepEquals, tree1.Root())

	if debug {
		fmt.Println("TestDbgStats")
		tree1.dbg.print("	add in loop    ")
		tree2.dbg.print("	addbatch caseA ")
		tree3.dbg.print("	addbatch caseD ")
	}
}

// func printLeafs(name string, t *Tree) {
//         w := bytes.NewBufferString("")
//
//         err := t.Iterate(func(k, v []byte) {
//                 if v[0] != PrefixValueLeaf {
//                         return
//                 }
//                 leafK, _ := readLeafValue(v)
//                 fmt.Fprintf(w, hex.EncodeToString(leafK[:4])+"\n")
//         })
//         if err != nil {
//                 panic(err)
//         }
//         err = ioutil.WriteFile(name, w.Bytes(), 0644)
//         if err != nil {
//                 panic(err)
//         }
//
// }

// func TestComputeCosts(t *testing.T) {
//         fmt.Println(computeSimpleAddCost(10))
//         fmt.Println(computeFromLeafsAddCost(10))
//
//         fmt.Println(computeSimpleAddCost(1024))
//         fmt.Println(computeFromLeafsAddCost(1024))
// }

// TODO test tree with nLeafs > minLeafsThreshold, but that at level L, there is
// less keys than nBuckets (so CASE C could be applied if first few leafs are
// added to balance the tree)

// TODO test adding batch with repeated keys in the batch
// TODO test adding batch with multiple invalid keys

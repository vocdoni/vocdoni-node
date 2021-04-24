package arbo

import (
	"encoding/hex"
	"fmt"
	"math/big"
	"testing"
	"time"

	qt "github.com/frankban/quicktest"
	"github.com/iden3/go-merkletree/db/memory"
)

func TestBatchAux(t *testing.T) { // TODO TMP this test will be delted
	c := qt.New(t)

	nLeafs := 16

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
	fmt.Println(time.Since(start))

	tree2, err := NewTree(memory.NewMemoryStorage(), 100, HashFunctionPoseidon)
	c.Assert(err, qt.IsNil)
	defer tree2.db.Close()

	for i := 0; i < 8; i++ {
		k := BigIntToBytes(big.NewInt(int64(i)))
		v := BigIntToBytes(big.NewInt(int64(i * 2)))
		if err := tree2.Add(k, v); err != nil {
			t.Fatal(err)
		}
	}
	// tree.PrintGraphviz(nil)
	// tree2.PrintGraphviz(nil)

	var keys, values [][]byte
	for i := 8; i < nLeafs; i++ {
		k := BigIntToBytes(big.NewInt(int64(i)))
		v := BigIntToBytes(big.NewInt(int64(i * 2)))
		keys = append(keys, k)
		values = append(values, v)
	}
	start = time.Now()
	indexes, err := tree2.AddBatchOpt(keys, values)
	c.Assert(err, qt.IsNil)
	fmt.Println(time.Since(start))
	c.Check(len(indexes), qt.Equals, 0)

	// check that both trees roots are equal
	c.Check(tree2.Root(), qt.DeepEquals, tree.Root())
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
	fmt.Println(time.Since(start))

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
	start = time.Now()
	indexes, err := tree2.AddBatchOpt(keys, values)
	c.Assert(err, qt.IsNil)
	fmt.Println(time.Since(start))
	c.Check(len(indexes), qt.Equals, 0)

	// check that both trees roots are equal
	c.Check(tree2.Root(), qt.DeepEquals, tree.Root())
}

func TestAddBatchCaseB(t *testing.T) {
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
	fmt.Println(time.Since(start))

	tree2, err := NewTree(memory.NewMemoryStorage(), 100, HashFunctionPoseidon)
	c.Assert(err, qt.IsNil)
	defer tree2.db.Close()

	// add the initial leafs to fill a bit the tree before calling the
	// AddBatch method
	for i := 0; i < 99; i++ { // TMP TODO use const minLeafsThreshold-1 once ready
		k := BigIntToBytes(big.NewInt(int64(i)))
		v := BigIntToBytes(big.NewInt(int64(i * 2)))
		if err := tree2.Add(k, v); err != nil {
			t.Fatal(err)
		}
	}

	var keys, values [][]byte
	for i := 99; i < nLeafs; i++ {
		k := BigIntToBytes(big.NewInt(int64(i)))
		v := BigIntToBytes(big.NewInt(int64(i * 2)))
		keys = append(keys, k)
		values = append(values, v)
	}
	start = time.Now()
	indexes, err := tree2.AddBatchOpt(keys, values)
	c.Assert(err, qt.IsNil)
	fmt.Println(time.Since(start))
	c.Check(len(indexes), qt.Equals, 0)

	// check that both trees roots are equal
	c.Check(tree2.Root(), qt.DeepEquals, tree.Root())
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
	fmt.Println(time.Since(start))

	tree2, err := NewTree(memory.NewMemoryStorage(), 100, HashFunctionPoseidon)
	c.Assert(err, qt.IsNil)
	defer tree2.db.Close()

	// add the initial leafs to fill a bit the tree before calling the
	// AddBatch method
	for i := 0; i < 101; i++ { // TMP TODO use const minLeafsThreshold-1 once ready
		k := BigIntToBytes(big.NewInt(int64(i)))
		v := BigIntToBytes(big.NewInt(int64(i * 2)))
		if err := tree2.Add(k, v); err != nil {
			t.Fatal(err)
		}
	}
	// tree2.PrintGraphviz(nil)

	var keys, values [][]byte
	for i := 101; i < nLeafs; i++ {
		k := BigIntToBytes(big.NewInt(int64(i)))
		v := BigIntToBytes(big.NewInt(int64(i * 2)))
		keys = append(keys, k)
		values = append(values, v)
	}
	start = time.Now()
	indexes, err := tree2.AddBatchOpt(keys, values)
	c.Assert(err, qt.IsNil)
	fmt.Println(time.Since(start))
	c.Check(len(indexes), qt.Equals, 0)

	// check that both trees roots are equal
	c.Check(tree2.Root(), qt.DeepEquals, tree.Root())

	// tree.PrintGraphviz(nil)
	// tree2.PrintGraphviz(nil)
	// // tree.PrintGraphvizFirstNLevels(nil, 4)
	// // tree2.PrintGraphvizFirstNLevels(nil, 4)
	// fmt.Println("TREE")
	// printLeafs("t1.txt", tree)
	// fmt.Println("TREE2")
	// printLeafs("t2.txt", tree2)
}

func TestAddBatchCaseD(t *testing.T) {
	c := qt.New(t)

	nLeafs := 4096

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
	fmt.Println(time.Since(start))

	tree2, err := NewTree(memory.NewMemoryStorage(), 100, HashFunctionPoseidon)
	c.Assert(err, qt.IsNil)
	defer tree2.db.Close()

	// add the initial leafs to fill a bit the tree before calling the
	// AddBatch method
	for i := 0; i < 900; i++ { // TMP TODO use const minLeafsThreshold+1 once ready
		k := BigIntToBytes(big.NewInt(int64(i)))
		v := BigIntToBytes(big.NewInt(int64(i * 2)))
		if err := tree2.Add(k, v); err != nil {
			t.Fatal(err)
		}
	}

	var keys, values [][]byte
	for i := 900; i < nLeafs; i++ {
		k := BigIntToBytes(big.NewInt(int64(i)))
		v := BigIntToBytes(big.NewInt(int64(i * 2)))
		keys = append(keys, k)
		values = append(values, v)
	}
	start = time.Now()
	indexes, err := tree2.AddBatchOpt(keys, values)
	c.Assert(err, qt.IsNil)
	fmt.Println(time.Since(start))
	c.Check(len(indexes), qt.Equals, 0)

	// check that both trees roots are equal
	c.Check(tree2.Root(), qt.DeepEquals, tree.Root())
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
//         fmt.Println(computeBottomUpAddCost(10))
//
//         fmt.Println(computeSimpleAddCost(1024))
//         fmt.Println(computeBottomUpAddCost(1024))
// }

// TODO test tree with nLeafs > minLeafsThreshold, but that at level L, there is
// less keys than nBuckets (so CASE C could be applied if first few leafs are
// added to balance the tree)

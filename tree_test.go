package arbo

import (
	"encoding/hex"
	"math/big"
	"testing"
	"time"

	qt "github.com/frankban/quicktest"
	"github.com/iden3/go-merkletree/db/memory"
)

func TestAddTestVectors(t *testing.T) {
	c := qt.New(t)

	// Poseidon test vectors generated using https://github.com/iden3/circomlib smt.js
	testVectorsPoseidon := []string{
		"0000000000000000000000000000000000000000000000000000000000000000",
		"13578938674299138072471463694055224830892726234048532520316387704878000008795",
		"5412393676474193513566895793055462193090331607895808993925969873307089394741",
		"14204494359367183802864593755198662203838502594566452929175967972147978322084",
	}
	testAdd(c, HashFunctionPoseidon, testVectorsPoseidon)

	testVectorsSha256 := []string{
		"0000000000000000000000000000000000000000000000000000000000000000",
		"46910109172468462938850740851377282682950237270676610513794735904325820156367",
		"59481735341404520835410489183267411392292882901306595567679529387376287440550",
		"20573794434149960984975763118181266662429997821552560184909083010514790081771",
	}
	testAdd(c, HashFunctionSha256, testVectorsSha256)
}

func testAdd(c *qt.C, hashFunc HashFunction, testVectors []string) {
	tree, err := NewTree(memory.NewMemoryStorage(), 10, hashFunc)
	c.Assert(err, qt.IsNil)
	defer tree.db.Close()

	c.Check(hex.EncodeToString(tree.Root()), qt.Equals, testVectors[0])

	err = tree.Add(
		BigIntToBytes(big.NewInt(1)),
		BigIntToBytes(big.NewInt(2)))
	c.Assert(err, qt.IsNil)
	rootBI := BytesToBigInt(tree.Root())
	c.Check(rootBI.String(), qt.Equals, testVectors[1])

	err = tree.Add(
		BigIntToBytes(big.NewInt(33)),
		BigIntToBytes(big.NewInt(44)))
	c.Assert(err, qt.IsNil)
	rootBI = BytesToBigInt(tree.Root())
	c.Check(rootBI.String(), qt.Equals, testVectors[2])

	err = tree.Add(
		BigIntToBytes(big.NewInt(1234)),
		BigIntToBytes(big.NewInt(9876)))
	c.Assert(err, qt.IsNil)
	rootBI = BytesToBigInt(tree.Root())
	c.Check(rootBI.String(), qt.Equals, testVectors[3])
}

func TestAddBatch(t *testing.T) {
	c := qt.New(t)
	tree, err := NewTree(memory.NewMemoryStorage(), 100, HashFunctionPoseidon)
	c.Assert(err, qt.IsNil)
	defer tree.db.Close()

	for i := 0; i < 1000; i++ {
		k := BigIntToBytes(big.NewInt(int64(i)))
		v := BigIntToBytes(big.NewInt(0))
		if err := tree.Add(k, v); err != nil {
			t.Fatal(err)
		}
	}

	rootBI := BytesToBigInt(tree.Root())
	c.Check(rootBI.String(), qt.Equals,
		"296519252211642170490407814696803112091039265640052570497930797516015811235")

	tree2, err := NewTree(memory.NewMemoryStorage(), 100, HashFunctionPoseidon)
	c.Assert(err, qt.IsNil)
	defer tree2.db.Close()

	var keys, values [][]byte
	for i := 0; i < 1000; i++ {
		k := BigIntToBytes(big.NewInt(int64(i)))
		v := BigIntToBytes(big.NewInt(0))
		keys = append(keys, k)
		values = append(values, v)
	}
	indexes, err := tree2.AddBatch(keys, values)
	c.Assert(err, qt.IsNil)
	c.Check(len(indexes), qt.Equals, 0)

	rootBI = BytesToBigInt(tree2.Root())
	c.Check(rootBI.String(), qt.Equals,
		"296519252211642170490407814696803112091039265640052570497930797516015811235")
}

func TestAddDifferentOrder(t *testing.T) {
	c := qt.New(t)
	tree1, err := NewTree(memory.NewMemoryStorage(), 100, HashFunctionPoseidon)
	c.Assert(err, qt.IsNil)
	defer tree1.db.Close()

	for i := 0; i < 16; i++ {
		k := BigIntToBytes(big.NewInt(int64(i)))
		v := BigIntToBytes(big.NewInt(0))
		if err := tree1.Add(k, v); err != nil {
			t.Fatal(err)
		}
	}

	tree2, err := NewTree(memory.NewMemoryStorage(), 100, HashFunctionPoseidon)
	c.Assert(err, qt.IsNil)
	defer tree2.db.Close()

	for i := 16 - 1; i >= 0; i-- {
		k := BigIntToBytes(big.NewInt(int64(i)))
		v := BigIntToBytes(big.NewInt(0))
		if err := tree2.Add(k, v); err != nil {
			t.Fatal(err)
		}
	}

	c.Check(hex.EncodeToString(tree2.Root()), qt.Equals, hex.EncodeToString(tree1.Root()))
	c.Check(hex.EncodeToString(tree1.Root()), qt.Equals,
		"3b89100bec24da9275c87bc188740389e1d5accfc7d88ba5688d7fa96a00d82f")
}

func TestAddRepeatedIndex(t *testing.T) {
	c := qt.New(t)
	tree, err := NewTree(memory.NewMemoryStorage(), 100, HashFunctionPoseidon)
	c.Assert(err, qt.IsNil)
	defer tree.db.Close()

	k := BigIntToBytes(big.NewInt(int64(3)))
	v := BigIntToBytes(big.NewInt(int64(12)))
	if err := tree.Add(k, v); err != nil {
		t.Fatal(err)
	}
	err = tree.Add(k, v)
	c.Assert(err, qt.Not(qt.IsNil))
	c.Check(err, qt.ErrorMatches, "max virtual level 100")
}

func TestUpdate(t *testing.T) {
	c := qt.New(t)
	tree, err := NewTree(memory.NewMemoryStorage(), 100, HashFunctionPoseidon)
	c.Assert(err, qt.IsNil)
	defer tree.db.Close()

	k := BigIntToBytes(big.NewInt(int64(20)))
	v := BigIntToBytes(big.NewInt(int64(12)))
	if err := tree.Add(k, v); err != nil {
		t.Fatal(err)
	}

	v = BigIntToBytes(big.NewInt(int64(11)))
	err = tree.Update(k, v)
	c.Assert(err, qt.IsNil)

	gettedKey, gettedValue, err := tree.Get(k)
	c.Assert(err, qt.IsNil)
	c.Check(gettedKey, qt.DeepEquals, k)
	c.Check(gettedValue, qt.DeepEquals, v)

	// add more leafs to the tree to do another test
	for i := 0; i < 16; i++ {
		k := BigIntToBytes(big.NewInt(int64(i)))
		v := BigIntToBytes(big.NewInt(int64(i * 2)))
		if err := tree.Add(k, v); err != nil {
			t.Fatal(err)
		}
	}

	k = BigIntToBytes(big.NewInt(int64(3)))
	v = BigIntToBytes(big.NewInt(int64(11)))
	// check that before the Update, value for 3 is !=11
	gettedKey, gettedValue, err = tree.Get(k)
	c.Assert(err, qt.IsNil)
	c.Check(gettedKey, qt.DeepEquals, k)
	c.Check(gettedValue, qt.Not(qt.DeepEquals), v)
	c.Check(gettedValue, qt.DeepEquals, BigIntToBytes(big.NewInt(6)))

	err = tree.Update(k, v)
	c.Assert(err, qt.IsNil)

	// check that after Update, the value for 3 is ==11
	gettedKey, gettedValue, err = tree.Get(k)
	c.Assert(err, qt.IsNil)
	c.Check(gettedKey, qt.DeepEquals, k)
	c.Check(gettedValue, qt.DeepEquals, v)
	c.Check(gettedValue, qt.DeepEquals, BigIntToBytes(big.NewInt(11)))
}

func TestAux(t *testing.T) { // TODO split in proper tests
	c := qt.New(t)
	tree, err := NewTree(memory.NewMemoryStorage(), 100, HashFunctionPoseidon)
	c.Assert(err, qt.IsNil)
	defer tree.db.Close()

	k := BigIntToBytes(big.NewInt(int64(1)))
	v := BigIntToBytes(big.NewInt(int64(0)))
	err = tree.Add(k, v)
	c.Assert(err, qt.IsNil)
	k = BigIntToBytes(big.NewInt(int64(256)))
	err = tree.Add(k, v)
	c.Assert(err, qt.IsNil)

	k = BigIntToBytes(big.NewInt(int64(257)))
	err = tree.Add(k, v)
	c.Assert(err, qt.IsNil)

	k = BigIntToBytes(big.NewInt(int64(515)))
	err = tree.Add(k, v)
	c.Assert(err, qt.IsNil)
	k = BigIntToBytes(big.NewInt(int64(770)))
	err = tree.Add(k, v)
	c.Assert(err, qt.IsNil)
	//
	// err = tree.PrintGraphviz(nil)
	// c.Assert(err, qt.IsNil)
}

func TestGet(t *testing.T) {
	c := qt.New(t)
	tree, err := NewTree(memory.NewMemoryStorage(), 100, HashFunctionPoseidon)
	c.Assert(err, qt.IsNil)
	defer tree.db.Close()

	for i := 0; i < 10; i++ {
		k := BigIntToBytes(big.NewInt(int64(i)))
		v := BigIntToBytes(big.NewInt(int64(i * 2)))
		if err := tree.Add(k, v); err != nil {
			t.Fatal(err)
		}
	}

	k := BigIntToBytes(big.NewInt(int64(7)))
	gettedKey, gettedValue, err := tree.Get(k)
	c.Assert(err, qt.IsNil)
	c.Check(gettedKey, qt.DeepEquals, k)
	c.Check(gettedValue, qt.DeepEquals, BigIntToBytes(big.NewInt(int64(7*2))))
}

func TestGenProofAndVerify(t *testing.T) {
	c := qt.New(t)
	tree, err := NewTree(memory.NewMemoryStorage(), 100, HashFunctionPoseidon)
	c.Assert(err, qt.IsNil)
	defer tree.db.Close()

	for i := 0; i < 10; i++ {
		k := BigIntToBytes(big.NewInt(int64(i)))
		v := BigIntToBytes(big.NewInt(int64(i * 2)))
		if err := tree.Add(k, v); err != nil {
			t.Fatal(err)
		}
	}

	k := BigIntToBytes(big.NewInt(int64(7)))
	siblings, err := tree.GenProof(k)
	c.Assert(err, qt.IsNil)

	k = BigIntToBytes(big.NewInt(int64(7)))
	v := BigIntToBytes(big.NewInt(int64(14)))
	verif, err := CheckProof(tree.hashFunction, k, v, tree.Root(), siblings)
	c.Assert(err, qt.IsNil)
	c.Check(verif, qt.IsTrue)
}

func TestDumpAndImportDump(t *testing.T) {
	c := qt.New(t)
	tree1, err := NewTree(memory.NewMemoryStorage(), 100, HashFunctionPoseidon)
	c.Assert(err, qt.IsNil)
	defer tree1.db.Close()

	for i := 0; i < 16; i++ {
		k := BigIntToBytes(big.NewInt(int64(i)))
		v := BigIntToBytes(big.NewInt(int64(i * 2)))
		if err := tree1.Add(k, v); err != nil {
			t.Fatal(err)
		}
	}

	e, err := tree1.Dump()
	c.Assert(err, qt.IsNil)

	tree2, err := NewTree(memory.NewMemoryStorage(), 100, HashFunctionPoseidon)
	c.Assert(err, qt.IsNil)
	defer tree2.db.Close()
	err = tree2.ImportDump(e)
	c.Assert(err, qt.IsNil)
	c.Check(tree2.Root(), qt.DeepEquals, tree1.Root())
	c.Check(hex.EncodeToString(tree2.Root()), qt.Equals,
		"0d93aaa3362b2f999f15e15728f123087c2eee716f01c01f56e23aae07f09f08")
}

func TestRWMutex(t *testing.T) {
	c := qt.New(t)
	tree, err := NewTree(memory.NewMemoryStorage(), 100, HashFunctionPoseidon)
	c.Assert(err, qt.IsNil)
	defer tree.db.Close()

	var keys, values [][]byte
	for i := 0; i < 1000; i++ {
		k := BigIntToBytes(big.NewInt(int64(i)))
		v := BigIntToBytes(big.NewInt(0))
		keys = append(keys, k)
		values = append(values, v)
	}
	go func() {
		_, err = tree.AddBatch(keys, values)
		if err != nil {
			panic(err)
		}
	}()

	time.Sleep(500 * time.Millisecond)
	k := BigIntToBytes(big.NewInt(int64(99999)))
	v := BigIntToBytes(big.NewInt(int64(99999)))
	if err := tree.Add(k, v); err != nil {
		t.Fatal(err)
	}
}

func BenchmarkAdd(b *testing.B) {
	// prepare inputs
	var ks, vs [][]byte
	for i := 0; i < 1000; i++ {
		k := BigIntToBytes(big.NewInt(int64(i)))
		v := BigIntToBytes(big.NewInt(int64(i)))
		ks = append(ks, k)
		vs = append(vs, v)
	}

	b.Run("Poseidon", func(b *testing.B) {
		benchmarkAdd(b, HashFunctionPoseidon, ks, vs)
	})
	b.Run("Sha256", func(b *testing.B) {
		benchmarkAdd(b, HashFunctionSha256, ks, vs)
	})
}

func benchmarkAdd(b *testing.B, hashFunc HashFunction, ks, vs [][]byte) {
	c := qt.New(b)
	tree, err := NewTree(memory.NewMemoryStorage(), 140, hashFunc)
	c.Assert(err, qt.IsNil)
	defer tree.db.Close()

	for i := 0; i < len(ks); i++ {
		if err := tree.Add(ks[i], vs[i]); err != nil {
			b.Fatal(err)
		}
	}
}

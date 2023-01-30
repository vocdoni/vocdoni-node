package arbo

import (
	"encoding/hex"
	"math"
	"math/big"
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	qt "github.com/frankban/quicktest"
	"go.vocdoni.io/dvote/db"
	"go.vocdoni.io/dvote/db/metadb"
	"go.vocdoni.io/dvote/db/pebbledb"
)

func checkRootBIString(c *qt.C, tree *Tree, expected string) {
	root, err := tree.Root()
	c.Assert(err, qt.IsNil)
	rootBI := BytesToBigInt(root)
	c.Check(rootBI.String(), qt.Equals, expected)
}

func TestDBTx(t *testing.T) {
	c := qt.New(t)

	dbBadger := metadb.NewTest(t)
	testDBTx(c, dbBadger)

	dbPebble, err := pebbledb.New(db.Options{Path: c.TempDir()})
	c.Assert(err, qt.IsNil)
	testDBTx(c, dbPebble)
}

func testDBTx(c *qt.C, database db.Database) {
	wTx := database.WriteTx()
	defer wTx.Discard()

	_, err := wTx.Get([]byte("a"))
	c.Assert(err, qt.Equals, db.ErrKeyNotFound)

	err = wTx.Set([]byte("a"), []byte("b"))
	c.Assert(err, qt.IsNil)

	v, err := wTx.Get([]byte("a"))
	c.Assert(err, qt.IsNil)
	c.Assert(v, qt.DeepEquals, []byte("b"))
}

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
	database := metadb.NewTest(c)
	tree, err := NewTree(Config{Database: database, MaxLevels: 256,
		HashFunction: hashFunc})
	c.Assert(err, qt.IsNil)

	root, err := tree.Root()
	c.Assert(err, qt.IsNil)
	c.Check(hex.EncodeToString(root), qt.Equals, testVectors[0])

	bLen := 32
	err = tree.Add(
		BigIntToBytes(bLen, big.NewInt(1)),
		BigIntToBytes(bLen, big.NewInt(2)))
	c.Assert(err, qt.IsNil)
	checkRootBIString(c, tree, testVectors[1])

	err = tree.Add(
		BigIntToBytes(bLen, big.NewInt(33)),
		BigIntToBytes(bLen, big.NewInt(44)))
	c.Assert(err, qt.IsNil)
	checkRootBIString(c, tree, testVectors[2])

	err = tree.Add(
		BigIntToBytes(bLen, big.NewInt(1234)),
		BigIntToBytes(bLen, big.NewInt(9876)))
	c.Assert(err, qt.IsNil)
	checkRootBIString(c, tree, testVectors[3])
}

func TestAddBatch(t *testing.T) {
	c := qt.New(t)
	database := metadb.NewTest(t)
	tree, err := NewTree(Config{Database: database, MaxLevels: 256,
		HashFunction: HashFunctionPoseidon})
	c.Assert(err, qt.IsNil)

	bLen := 32
	for i := 0; i < 1000; i++ {
		k := BigIntToBytes(bLen, big.NewInt(int64(i)))
		v := BigIntToBytes(bLen, big.NewInt(0))
		if err := tree.Add(k, v); err != nil {
			t.Fatal(err)
		}
	}

	checkRootBIString(c, tree,
		"296519252211642170490407814696803112091039265640052570497930797516015811235")

	database = metadb.NewTest(t)
	tree2, err := NewTree(Config{Database: database, MaxLevels: 256,
		HashFunction: HashFunctionPoseidon})
	c.Assert(err, qt.IsNil)

	var keys, values [][]byte
	for i := 0; i < 1000; i++ {
		k := BigIntToBytes(bLen, big.NewInt(int64(i)))
		v := BigIntToBytes(bLen, big.NewInt(0))
		keys = append(keys, k)
		values = append(values, v)
	}
	indexes, err := tree2.AddBatch(keys, values)
	c.Assert(err, qt.IsNil)
	c.Check(len(indexes), qt.Equals, 0)

	checkRootBIString(c, tree2,
		"296519252211642170490407814696803112091039265640052570497930797516015811235")
}

func TestAddDifferentOrder(t *testing.T) {
	c := qt.New(t)
	database1 := metadb.NewTest(t)
	tree1, err := NewTree(Config{Database: database1, MaxLevels: 256,
		HashFunction: HashFunctionPoseidon})
	c.Assert(err, qt.IsNil)

	bLen := 32
	for i := 0; i < 16; i++ {
		k := BigIntToBytes(bLen, big.NewInt(int64(i)))
		v := BigIntToBytes(bLen, big.NewInt(0))
		if err := tree1.Add(k, v); err != nil {
			t.Fatal(err)
		}
	}

	database2 := metadb.NewTest(t)
	tree2, err := NewTree(Config{Database: database2, MaxLevels: 256,
		HashFunction: HashFunctionPoseidon})
	c.Assert(err, qt.IsNil)

	for i := 16 - 1; i >= 0; i-- {
		k := BigIntToBytes(bLen, big.NewInt(int64(i)))
		v := BigIntToBytes(bLen, big.NewInt(0))
		if err := tree2.Add(k, v); err != nil {
			t.Fatal(err)
		}
	}

	root1, err := tree1.Root()
	c.Assert(err, qt.IsNil)
	root2, err := tree2.Root()
	c.Assert(err, qt.IsNil)
	c.Check(hex.EncodeToString(root2), qt.Equals, hex.EncodeToString(root1))
	c.Check(hex.EncodeToString(root1), qt.Equals,
		"3b89100bec24da9275c87bc188740389e1d5accfc7d88ba5688d7fa96a00d82f")
}

func TestAddRepeatedIndex(t *testing.T) {
	c := qt.New(t)
	database := metadb.NewTest(t)
	tree, err := NewTree(Config{Database: database, MaxLevels: 256,
		HashFunction: HashFunctionPoseidon})
	c.Assert(err, qt.IsNil)

	bLen := 32
	k := BigIntToBytes(bLen, big.NewInt(int64(3)))
	v := BigIntToBytes(bLen, big.NewInt(int64(12)))

	err = tree.Add(k, v)
	c.Assert(err, qt.IsNil)
	err = tree.Add(k, v) // repeating same key-value
	c.Check(err, qt.Equals, ErrKeyAlreadyExists)
}

func TestUpdate(t *testing.T) {
	c := qt.New(t)
	database := metadb.NewTest(t)
	tree, err := NewTree(Config{Database: database, MaxLevels: 256,
		HashFunction: HashFunctionPoseidon})
	c.Assert(err, qt.IsNil)

	bLen := 32
	k := BigIntToBytes(bLen, big.NewInt(int64(20)))
	v := BigIntToBytes(bLen, big.NewInt(int64(12)))
	if err := tree.Add(k, v); err != nil {
		t.Fatal(err)
	}

	v = BigIntToBytes(bLen, big.NewInt(int64(11)))
	err = tree.Update(k, v)
	c.Assert(err, qt.IsNil)

	gettedKey, gettedValue, err := tree.Get(k)
	c.Assert(err, qt.IsNil)
	c.Check(gettedKey, qt.DeepEquals, k)
	c.Check(gettedValue, qt.DeepEquals, v)

	// add more leafs to the tree to do another test
	for i := 0; i < 16; i++ {
		k := BigIntToBytes(bLen, big.NewInt(int64(i)))
		v := BigIntToBytes(bLen, big.NewInt(int64(i*2)))
		if err := tree.Add(k, v); err != nil {
			t.Fatal(err)
		}
	}

	k = BigIntToBytes(bLen, big.NewInt(int64(3)))
	v = BigIntToBytes(bLen, big.NewInt(int64(11)))
	// check that before the Update, value for 3 is !=11
	gettedKey, gettedValue, err = tree.Get(k)
	c.Assert(err, qt.IsNil)
	c.Check(gettedKey, qt.DeepEquals, k)
	c.Check(gettedValue, qt.Not(qt.DeepEquals), v)
	c.Check(gettedValue, qt.DeepEquals, BigIntToBytes(bLen, big.NewInt(6)))

	err = tree.Update(k, v)
	c.Assert(err, qt.IsNil)

	// check that after Update, the value for 3 is ==11
	gettedKey, gettedValue, err = tree.Get(k)
	c.Assert(err, qt.IsNil)
	c.Check(gettedKey, qt.DeepEquals, k)
	c.Check(gettedValue, qt.DeepEquals, v)
	c.Check(gettedValue, qt.DeepEquals, BigIntToBytes(bLen, big.NewInt(11)))
}

func TestAux(t *testing.T) { // TODO split in proper tests
	c := qt.New(t)
	database := metadb.NewTest(t)
	tree, err := NewTree(Config{Database: database, MaxLevels: 256,
		HashFunction: HashFunctionPoseidon})
	c.Assert(err, qt.IsNil)

	bLen := 32
	k := BigIntToBytes(bLen, big.NewInt(int64(1)))
	v := BigIntToBytes(bLen, big.NewInt(int64(0)))
	err = tree.Add(k, v)
	c.Assert(err, qt.IsNil)
	k = BigIntToBytes(bLen, big.NewInt(int64(256)))
	err = tree.Add(k, v)
	c.Assert(err, qt.IsNil)

	k = BigIntToBytes(bLen, big.NewInt(int64(257)))
	err = tree.Add(k, v)
	c.Assert(err, qt.IsNil)

	k = BigIntToBytes(bLen, big.NewInt(int64(515)))
	err = tree.Add(k, v)
	c.Assert(err, qt.IsNil)
	k = BigIntToBytes(bLen, big.NewInt(int64(770)))
	err = tree.Add(k, v)
	c.Assert(err, qt.IsNil)

	k = BigIntToBytes(bLen, big.NewInt(int64(388)))
	err = tree.Add(k, v)
	c.Assert(err, qt.IsNil)
	k = BigIntToBytes(bLen, big.NewInt(int64(900)))
	err = tree.Add(k, v)
	c.Assert(err, qt.IsNil)
	//
	// err = tree.PrintGraphviz(nil)
	// c.Assert(err, qt.IsNil)
}

func TestGet(t *testing.T) {
	c := qt.New(t)
	database := metadb.NewTest(t)
	tree, err := NewTree(Config{Database: database, MaxLevels: 256,
		HashFunction: HashFunctionPoseidon})
	c.Assert(err, qt.IsNil)

	bLen := 32
	for i := 0; i < 10; i++ {
		k := BigIntToBytes(bLen, big.NewInt(int64(i)))
		v := BigIntToBytes(bLen, big.NewInt(int64(i*2)))
		if err := tree.Add(k, v); err != nil {
			t.Fatal(err)
		}
	}

	k := BigIntToBytes(bLen, big.NewInt(int64(7)))
	gettedKey, gettedValue, err := tree.Get(k)
	c.Assert(err, qt.IsNil)
	c.Check(gettedKey, qt.DeepEquals, k)
	c.Check(gettedValue, qt.DeepEquals, BigIntToBytes(bLen, big.NewInt(int64(7*2))))
}

func TestBitmapBytes(t *testing.T) {
	c := qt.New(t)

	b := []byte{15}
	bits := bytesToBitmap(b)
	c.Assert(bits, qt.DeepEquals, []bool{true, true, true, true,
		false, false, false, false})
	b2 := bitmapToBytes(bits)
	c.Assert(b2, qt.DeepEquals, b)

	b = []byte{0, 15, 50}
	bits = bytesToBitmap(b)
	c.Assert(bits, qt.DeepEquals, []bool{false, false, false,
		false, false, false, false, false, true, true, true, true,
		false, false, false, false, false, true, false, false, true,
		true, false, false})
	b2 = bitmapToBytes(bits)
	c.Assert(b2, qt.DeepEquals, b)

	b = []byte{0, 1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11}
	bits = bytesToBitmap(b)
	b2 = bitmapToBytes(bits)
	c.Assert(b2, qt.DeepEquals, b)

	b = []byte("testbytes")
	bits = bytesToBitmap(b)
	b2 = bitmapToBytes(bits)
	c.Assert(b2, qt.DeepEquals, b)
}

func TestPackAndUnpackSiblings(t *testing.T) {
	c := qt.New(t)

	siblingsHex := []string{
		"0000000000000000000000000000000000000000000000000000000000000000",
		"0100000000000000000000000000000000000000000000000000000000000000",
		"0200000000000000000000000000000000000000000000000000000000000000",
		"0000000000000000000000000000000000000000000000000000000000000000",
		"0000000000000000000000000000000000000000000000000000000000000000",
		"0000000000000000000000000000000000000000000000000000000000000000",
		"0300000000000000000000000000000000000000000000000000000000000000",
		"0400000000000000000000000000000000000000000000000000000000000000",
		"0000000000000000000000000000000000000000000000000000000000000000",
		"0000000000000000000000000000000000000000000000000000000000000000",
		"0500000000000000000000000000000000000000000000000000000000000000",
	}
	siblings := make([][]byte, len(siblingsHex))
	var err error
	for i := 0; i < len(siblingsHex); i++ {
		siblings[i], err = hex.DecodeString(siblingsHex[i])
		c.Assert(err, qt.IsNil)
	}

	packed, err := PackSiblings(HashFunctionPoseidon, siblings)
	c.Assert(err, qt.IsNil)
	c.Assert(hex.EncodeToString(packed), qt.Equals, "a6000200c604"+
		"0100000000000000000000000000000000000000000000000000000000000000"+
		"0200000000000000000000000000000000000000000000000000000000000000"+
		"0300000000000000000000000000000000000000000000000000000000000000"+
		"0400000000000000000000000000000000000000000000000000000000000000"+
		"0500000000000000000000000000000000000000000000000000000000000000")

	unpacked, err := UnpackSiblings(HashFunctionPoseidon, packed)
	c.Assert(err, qt.IsNil)
	c.Assert(unpacked, qt.DeepEquals, siblings)

	// another test with other values
	siblingsHex = []string{
		"1ce165cb1124ed3a0a94b4e212aaf7e8079f49b2fbef916bc290c593fda9092a",
		"0000000000000000000000000000000000000000000000000000000000000000",
		"0000000000000000000000000000000000000000000000000000000000000000",
		"0000000000000000000000000000000000000000000000000000000000000000",
		"33018202c57d898b84338b16d1a4960e133c6a4d656cfec1bd62a9ea00611729",
		"bdbee2bd246ba0259a37be9a8740b550eed01c566aff0dca9a07306bcf731d13",
		"0000000000000000000000000000000000000000000000000000000000000000",
		"0000000000000000000000000000000000000000000000000000000000000000",
		"d43b04d7c2d0bba83b4291fea9ba0fec7830d17af54cbe9967fe90b8244d4e0d",
		"0000000000000000000000000000000000000000000000000000000000000000",
		"7def274dbb3a72dca44f01a8d9f2f21a5be84c171eecef8e2e4112e7277e262a",
	}
	siblings = make([][]byte, len(siblingsHex))
	for i := 0; i < len(siblingsHex); i++ {
		siblings[i], err = hex.DecodeString(siblingsHex[i])
		c.Assert(err, qt.IsNil)
	}

	packed, err = PackSiblings(HashFunctionPoseidon, siblings)
	c.Assert(err, qt.IsNil)
	c.Assert(hex.EncodeToString(packed), qt.Equals, "a60002003105"+
		"1ce165cb1124ed3a0a94b4e212aaf7e8079f49b2fbef916bc290c593fda9092a"+
		"33018202c57d898b84338b16d1a4960e133c6a4d656cfec1bd62a9ea00611729"+
		"bdbee2bd246ba0259a37be9a8740b550eed01c566aff0dca9a07306bcf731d13"+
		"d43b04d7c2d0bba83b4291fea9ba0fec7830d17af54cbe9967fe90b8244d4e0d"+
		"7def274dbb3a72dca44f01a8d9f2f21a5be84c171eecef8e2e4112e7277e262a")

	unpacked, err = UnpackSiblings(HashFunctionPoseidon, packed)
	c.Assert(err, qt.IsNil)
	c.Assert(unpacked, qt.DeepEquals, siblings)
}

func TestGenProofAndVerify(t *testing.T) {
	c := qt.New(t)
	database := metadb.NewTest(t)
	tree, err := NewTree(Config{Database: database, MaxLevels: 256,
		HashFunction: HashFunctionPoseidon})
	c.Assert(err, qt.IsNil)

	bLen := 32
	for i := 0; i < 10; i++ {
		k := BigIntToBytes(bLen, big.NewInt(int64(i)))
		v := BigIntToBytes(bLen, big.NewInt(int64(i*2)))
		if err := tree.Add(k, v); err != nil {
			t.Fatal(err)
		}
	}

	k := BigIntToBytes(bLen, big.NewInt(int64(7)))
	v := BigIntToBytes(bLen, big.NewInt(int64(14)))
	kAux, proofV, siblings, existence, err := tree.GenProof(k)
	c.Assert(err, qt.IsNil)
	c.Assert(proofV, qt.DeepEquals, v)
	c.Assert(k, qt.DeepEquals, kAux)
	c.Assert(existence, qt.IsTrue)

	root, err := tree.Root()
	c.Assert(err, qt.IsNil)
	verif, err := CheckProof(tree.hashFunction, k, v, root, siblings)
	c.Assert(err, qt.IsNil)
	c.Check(verif, qt.IsTrue)
}

func TestDumpAndImportDump(t *testing.T) {
	testDumpAndImportDump(t, false)
}

func TestDumpAndImportDumpInFile(t *testing.T) {
	testDumpAndImportDump(t, true)
}

func testDumpAndImportDump(t *testing.T, inFile bool) {
	c := qt.New(t)
	database1 := metadb.NewTest(t)
	tree1, err := NewTree(Config{Database: database1, MaxLevels: 256,
		HashFunction: HashFunctionPoseidon})
	c.Assert(err, qt.IsNil)

	bLen := 32
	for i := 0; i < 16; i++ {
		k := BigIntToBytes(bLen, big.NewInt(int64(i)))
		v := BigIntToBytes(bLen, big.NewInt(int64(i*2)))
		if err := tree1.Add(k, v); err != nil {
			t.Fatal(err)
		}
	}

	var e []byte
	filePath := c.TempDir()
	fileName := filepath.Join(filePath, "dump.bin")
	if inFile {
		f, err := os.Create(fileName)
		c.Assert(err, qt.IsNil)
		err = tree1.DumpWriter(nil, f)
		c.Assert(err, qt.IsNil)
	} else {
		e, err = tree1.Dump(nil)
		c.Assert(err, qt.IsNil)
	}

	database2 := metadb.NewTest(t)
	tree2, err := NewTree(Config{Database: database2, MaxLevels: 256,
		HashFunction: HashFunctionPoseidon})
	c.Assert(err, qt.IsNil)

	if inFile {
		f, err := os.Open(filepath.Clean(fileName))
		c.Assert(err, qt.IsNil)
		err = tree2.ImportDumpReader(f)
		c.Assert(err, qt.IsNil)
	} else {
		err = tree2.ImportDump(e)
		c.Assert(err, qt.IsNil)
	}

	root1, err := tree1.Root()
	c.Assert(err, qt.IsNil)
	root2, err := tree2.Root()
	c.Assert(err, qt.IsNil)
	c.Check(root2, qt.DeepEquals, root1)
	c.Check(hex.EncodeToString(root2), qt.Equals,
		"0d93aaa3362b2f999f15e15728f123087c2eee716f01c01f56e23aae07f09f08")
}

func TestRWMutex(t *testing.T) {
	c := qt.New(t)
	database := metadb.NewTest(t)
	tree, err := NewTree(Config{Database: database, MaxLevels: 256,
		HashFunction: HashFunctionPoseidon})
	c.Assert(err, qt.IsNil)

	bLen := 32
	var keys, values [][]byte
	for i := 0; i < 1000; i++ {
		k := BigIntToBytes(bLen, big.NewInt(int64(i)))
		v := BigIntToBytes(bLen, big.NewInt(0))
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
	k := BigIntToBytes(bLen, big.NewInt(int64(99999)))
	v := BigIntToBytes(bLen, big.NewInt(int64(99999)))
	if err := tree.Add(k, v); err != nil {
		t.Fatal(err)
	}
}

// TODO UPDATE
// func TestSetGetNLeafs(t *testing.T) {
//         c := qt.New(t)
//         database := metadb.NewTest(t).IsNil)
//         tree, err := NewTree(database, 100, HashFunctionPoseidon)
//         c.Assert(err, qt.IsNil)
//
//         // 0
//         tree.dbBatch = tree.db.NewBatch()
//
//         err = tree.setNLeafs(0)
//         c.Assert(err, qt.IsNil)
//
//         err = tree.dbBatch.Write()
//         c.Assert(err, qt.IsNil)
//
//         n, err := tree.GetNLeafs()
//         c.Assert(err, qt.IsNil)
//         c.Assert(n, qt.Equals, 0)
//
//         // 1024
//         tree.dbBatch = tree.db.NewBatch()
//
//         err = tree.setNLeafs(1024)
//         c.Assert(err, qt.IsNil)
//
//         err = tree.dbBatch.Write()
//         c.Assert(err, qt.IsNil)
//
//         n, err = tree.GetNLeafs()
//         c.Assert(err, qt.IsNil)
//         c.Assert(n, qt.Equals, 1024)
//
//         // 2**64 -1
//         tree.dbBatch = tree.db.NewBatch()
//
//         maxUint := ^uint(0)
//         maxInt := int(maxUint >> 1)
//
//         err = tree.setNLeafs(maxInt)
//         c.Assert(err, qt.IsNil)
//
//         err = tree.dbBatch.Write()
//         c.Assert(err, qt.IsNil)
//
//         n, err = tree.GetNLeafs()
//         c.Assert(err, qt.IsNil)
//         c.Assert(n, qt.Equals, maxInt)
// }

func TestAddBatchFullyUsed(t *testing.T) {
	c := qt.New(t)

	database1 := metadb.NewTest(t)
	tree1, err := NewTree(Config{Database: database1, MaxLevels: 4,
		HashFunction: HashFunctionPoseidon})
	c.Assert(err, qt.IsNil)

	database2 := metadb.NewTest(t)
	tree2, err := NewTree(Config{Database: database2, MaxLevels: 4,
		HashFunction: HashFunctionPoseidon})
	c.Assert(err, qt.IsNil)

	var keys, values [][]byte
	for i := 0; i < 16; i++ {
		k := BigIntToBytes(1, big.NewInt(int64(i)))
		v := k

		keys = append(keys, k)
		values = append(values, v)

		// add one by one expecting no error
		err := tree1.Add(k, v)
		c.Assert(err, qt.IsNil)
	}

	invalids, err := tree2.AddBatch(keys, values)
	c.Assert(err, qt.IsNil)
	c.Assert(0, qt.Equals, len(invalids))

	root1, err := tree1.Root()
	c.Assert(err, qt.IsNil)
	root2, err := tree2.Root()
	c.Assert(err, qt.IsNil)
	c.Assert(root1, qt.DeepEquals, root2)

	// get all key-values and check that are equal between both trees
	for i := 0; i < 16; i++ {
		auxK1, auxV1, err := tree1.Get(BigIntToBytes(1, big.NewInt(int64(i))))
		c.Assert(err, qt.IsNil)

		auxK2, auxV2, err := tree2.Get(BigIntToBytes(1, big.NewInt(int64(i))))
		c.Assert(err, qt.IsNil)

		c.Assert(auxK1, qt.DeepEquals, auxK2)
		c.Assert(auxV1, qt.DeepEquals, auxV2)
	}

	// try adding one more key to both trees (through Add & AddBatch) and
	// expect not being added due the tree is already full
	k := BigIntToBytes(1, big.NewInt(int64(16)))
	v := k
	err = tree1.Add(k, v)
	c.Assert(err, qt.Equals, ErrMaxVirtualLevel)

	invalids, err = tree2.AddBatch([][]byte{k}, [][]byte{v})
	c.Assert(err, qt.IsNil)
	c.Assert(1, qt.Equals, len(invalids))
}

func TestSetRoot(t *testing.T) {
	c := qt.New(t)
	database := metadb.NewTest(t)
	tree, err := NewTree(Config{Database: database, MaxLevels: 256,
		HashFunction: HashFunctionPoseidon})
	c.Assert(err, qt.IsNil)

	expectedRoot := "13742386369878513332697380582061714160370929283209286127733983161245560237407"

	// fill the tree
	bLen := 32
	var keys, values [][]byte
	for i := 0; i < 1000; i++ {
		k := BigIntToBytes(bLen, big.NewInt(int64(i)))
		v := BigIntToBytes(bLen, big.NewInt(int64(i)))
		keys = append(keys, k)
		values = append(values, v)
	}
	indexes, err := tree.AddBatch(keys, values)
	c.Assert(err, qt.IsNil)
	c.Check(len(indexes), qt.Equals, 0)
	checkRootBIString(c, tree,
		expectedRoot)

	// add one more k-v
	k := BigIntToBytes(bLen, big.NewInt(1000))
	v := BigIntToBytes(bLen, big.NewInt(1000))
	err = tree.Add(k, v)
	c.Assert(err, qt.IsNil)
	checkRootBIString(c, tree,
		"10747149055773881257049574592162159501044114324358186833013814735296193179713")

	// do a SetRoot, and expect the same root than the original tree
	pastRootBI, ok := new(big.Int).SetString(expectedRoot, 10)
	c.Assert(ok, qt.IsTrue)
	pastRoot := BigIntToBytes(32, pastRootBI)

	err = tree.SetRoot(pastRoot)
	c.Assert(err, qt.IsNil)
	checkRootBIString(c, tree, expectedRoot)

	// check that the tree can be updated
	err = tree.Add([]byte("test"), []byte("test"))
	c.Assert(err, qt.IsNil)
	err = tree.Update([]byte("test"), []byte("test"))
	c.Assert(err, qt.IsNil)

	// check that the k-v '1000' does not exist in the new tree
	_, _, err = tree.Get(k)
	c.Assert(err, qt.Equals, ErrKeyNotFound)

	// check that can be set an empty root
	err = tree.SetRoot(tree.emptyHash)
	c.Assert(err, qt.IsNil)
}

func TestSnapshot(t *testing.T) {
	c := qt.New(t)
	database := metadb.NewTest(t)
	tree, err := NewTree(Config{Database: database, MaxLevels: 256,
		HashFunction: HashFunctionPoseidon})
	c.Assert(err, qt.IsNil)

	// fill the tree
	bLen := 32
	var keys, values [][]byte
	for i := 0; i < 1000; i++ {
		k := BigIntToBytes(bLen, big.NewInt(int64(i)))
		v := BigIntToBytes(bLen, big.NewInt(int64(i)))
		keys = append(keys, k)
		values = append(values, v)
	}
	indexes, err := tree.AddBatch(keys, values)
	c.Assert(err, qt.IsNil)
	c.Check(len(indexes), qt.Equals, 0)
	checkRootBIString(c, tree,
		"13742386369878513332697380582061714160370929283209286127733983161245560237407")

	// do a snapshot, and expect the same root than the original tree
	snapshotTree, err := tree.Snapshot(nil)
	c.Assert(err, qt.IsNil)
	checkRootBIString(c, snapshotTree,
		"13742386369878513332697380582061714160370929283209286127733983161245560237407")

	// check that the snapshotTree can not be updated
	_, err = snapshotTree.AddBatch(keys, values)
	c.Assert(err, qt.Equals, ErrSnapshotNotEditable)
	err = snapshotTree.Add([]byte("test"), []byte("test"))
	c.Assert(err, qt.Equals, ErrSnapshotNotEditable)
	err = snapshotTree.Update([]byte("test"), []byte("test"))
	c.Assert(err, qt.Equals, ErrSnapshotNotEditable)
	err = snapshotTree.ImportDump(nil)
	c.Assert(err, qt.Equals, ErrSnapshotNotEditable)

	// update the original tree by adding a new key-value, and check that
	// snapshotTree still has the old root, but the original tree has a new
	// root
	err = tree.Add([]byte("test"), []byte("test"))
	c.Assert(err, qt.IsNil)
	checkRootBIString(c, snapshotTree,
		"13742386369878513332697380582061714160370929283209286127733983161245560237407")
	checkRootBIString(c, tree,
		"1025190963769001718196479367844646783678188389989148142691917685159698888868")
}

func TestGetFromSnapshotExpectArboErrKeyNotFound(t *testing.T) {
	c := qt.New(t)

	database := metadb.NewTest(t)
	tree, err := NewTree(Config{Database: database, MaxLevels: 256,
		HashFunction: HashFunctionPoseidon})
	c.Assert(err, qt.IsNil)

	bLen := 32
	k := BigIntToBytes(bLen, big.NewInt(int64(3)))

	root, err := tree.Root()
	c.Assert(err, qt.IsNil)
	tree, err = tree.Snapshot(root)
	c.Assert(err, qt.IsNil)

	_, _, err = tree.Get(k)
	c.Assert(err, qt.Equals, ErrKeyNotFound) // and not equal to db.ErrKeyNotFound
}

func TestKeyLen(t *testing.T) {
	c := qt.New(t)
	database := metadb.NewTest(t)
	// maxLevels is 100, keyPath length = ceil(maxLevels/8) = 13
	maxLevels := 100
	tree, err := NewTree(Config{Database: database, MaxLevels: maxLevels,
		HashFunction: HashFunctionBlake2b})
	c.Assert(err, qt.IsNil)

	// expect no errors when adding a key of only 4 bytes (when the
	// required length of keyPath for 100 levels would be 13 bytes)
	bLen := 4
	k := BigIntToBytes(bLen, big.NewInt(1))
	v := BigIntToBytes(bLen, big.NewInt(1))

	err = tree.Add(k, v)
	c.Assert(err, qt.IsNil)

	err = tree.Update(k, v)
	c.Assert(err, qt.IsNil)

	_, _, _, _, err = tree.GenProof(k)
	c.Assert(err, qt.IsNil)

	_, _, err = tree.Get(k)
	c.Assert(err, qt.IsNil)

	k = BigIntToBytes(bLen, big.NewInt(2))
	v = BigIntToBytes(bLen, big.NewInt(2))
	invalids, err := tree.AddBatch([][]byte{k}, [][]byte{v})
	c.Assert(err, qt.IsNil)
	c.Assert(len(invalids), qt.Equals, 0)

	// expect errors when adding a key bigger than maximum capacity of the
	// tree: ceil(maxLevels/8)
	maxLevels = 32
	database = metadb.NewTest(t)
	tree, err = NewTree(Config{Database: database, MaxLevels: maxLevels,
		HashFunction: HashFunctionBlake2b})
	c.Assert(err, qt.IsNil)

	maxKeyLen := int(math.Ceil(float64(maxLevels) / float64(8)))
	k = BigIntToBytes(maxKeyLen+1, big.NewInt(1))
	v = BigIntToBytes(maxKeyLen+1, big.NewInt(1))

	expectedErrMsg := "len(k) can not be bigger than ceil(maxLevels/8)," +
		" where len(k): 5, maxLevels: 32, max key len=ceil(maxLevels/8): 4." +
		" Might need a bigger tree depth (maxLevels>=40) in order to input" +
		" keys of length 5"

	err = tree.Add(k, v)
	c.Assert(err.Error(), qt.Equals, expectedErrMsg)

	err = tree.Update(k, v)
	c.Assert(err.Error(), qt.Equals, expectedErrMsg)

	_, _, _, _, err = tree.GenProof(k)
	c.Assert(err.Error(), qt.Equals, expectedErrMsg)

	_, _, err = tree.Get(k)
	c.Assert(err.Error(), qt.Equals, expectedErrMsg)

	// check AddBatch with few key-values
	invalids, err = tree.AddBatch([][]byte{k}, [][]byte{v})
	c.Assert(err, qt.IsNil)
	c.Assert(len(invalids), qt.Equals, 1)

	// check AddBatch with many key-values
	nCPU := flp2(runtime.NumCPU())
	nKVs := nCPU + 1
	var ks, vs [][]byte
	for i := 0; i < nKVs; i++ {
		ks = append(ks, BigIntToBytes(maxKeyLen+1, big.NewInt(1)))
		vs = append(vs, BigIntToBytes(maxKeyLen+1, big.NewInt(1)))
	}
	invalids, err = tree.AddBatch(ks, vs)
	c.Assert(err, qt.IsNil)
	c.Assert(len(invalids), qt.Equals, nKVs)

	// check that with maxKeyLen it can be added
	k = BigIntToBytes(maxKeyLen, big.NewInt(1))
	err = tree.Add(k, v)
	c.Assert(err, qt.IsNil)

	// check CheckProof check with key longer
	kAux, vAux, packedSiblings, existence, err := tree.GenProof(k)
	c.Assert(err, qt.IsNil)
	c.Assert(existence, qt.IsTrue)

	root, err := tree.Root()
	c.Assert(err, qt.IsNil)
	verif, err := CheckProof(tree.HashFunction(), kAux, vAux, root, packedSiblings)
	c.Assert(err, qt.IsNil)
	c.Assert(verif, qt.IsTrue)

	// use a similar key but with one zero, expect that CheckProof fails on
	// the verification
	kAux = append(kAux, 0)
	verif, err = CheckProof(tree.HashFunction(), kAux, vAux, root, packedSiblings)
	c.Assert(err, qt.IsNil)
	c.Assert(verif, qt.IsFalse)
}

func TestKeyLenBiggerThan32(t *testing.T) {
	c := qt.New(t)
	maxLevels := 264
	database := metadb.NewTest(t)
	tree, err := NewTree(Config{Database: database, MaxLevels: maxLevels,
		HashFunction: HashFunctionBlake2b})
	c.Assert(err, qt.IsNil)

	bLen := 33
	err = tree.Add(
		randomBytes(bLen),
		randomBytes(bLen))
	c.Assert(err, qt.IsNil)

	// 2nd key that we add, will find a node with len(key)==32 (due the
	// hash output size, expect that next Add does not give any error, as
	// internally it will use a keyPath of size corresponent to the
	// maxLevels size of the tree
	err = tree.Add(
		randomBytes(bLen),
		randomBytes(bLen))
	c.Assert(err, qt.IsNil)
}

func BenchmarkAdd(b *testing.B) {
	bLen := 32 // for both Poseidon & Sha256
	// prepare inputs
	var ks, vs [][]byte
	for i := 0; i < 1000; i++ {
		k := BigIntToBytes(bLen, big.NewInt(int64(i)))
		v := BigIntToBytes(bLen, big.NewInt(int64(i)))
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
	database := metadb.NewTest(c)
	tree, err := NewTree(Config{Database: database, MaxLevels: 140,
		HashFunction: hashFunc})
	c.Assert(err, qt.IsNil)

	for i := 0; i < len(ks); i++ {
		if err := tree.Add(ks[i], vs[i]); err != nil {
			b.Fatal(err)
		}
	}
}

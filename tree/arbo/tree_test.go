package arbo

import (
	"bytes"
	"encoding/hex"
	"fmt"
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
	rootBI := BytesLEToBigInt(root)
	c.Check(rootBI.String(), qt.Equals, expected)
}

func TestDBTx(t *testing.T) {
	c := qt.New(t)

	tdb := metadb.NewTest(t)
	testDBTx(c, tdb)

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
	tree, err := NewTree(Config{
		Database: database, MaxLevels: 256,
		HashFunction: hashFunc,
	})
	c.Assert(err, qt.IsNil)

	root, err := tree.Root()
	c.Assert(err, qt.IsNil)
	c.Check(hex.EncodeToString(root), qt.Equals, testVectors[0])

	bLen := 32
	err = tree.Add(
		BigIntToBytesLE(bLen, big.NewInt(1)),
		BigIntToBytesLE(bLen, big.NewInt(2)))
	c.Assert(err, qt.IsNil)
	checkRootBIString(c, tree, testVectors[1])

	err = tree.Add(
		BigIntToBytesLE(bLen, big.NewInt(33)),
		BigIntToBytesLE(bLen, big.NewInt(44)))
	c.Assert(err, qt.IsNil)
	checkRootBIString(c, tree, testVectors[2])

	err = tree.Add(
		BigIntToBytesLE(bLen, big.NewInt(1234)),
		BigIntToBytesLE(bLen, big.NewInt(9876)))
	c.Assert(err, qt.IsNil)
	checkRootBIString(c, tree, testVectors[3])
}

func TestAddBatch(t *testing.T) {
	c := qt.New(t)
	database := metadb.NewTest(t)
	tree, err := NewTree(Config{Database: database, MaxLevels: 256, HashFunction: HashFunctionPoseidon})
	c.Assert(err, qt.IsNil)

	bLen := 32
	for i := 0; i < 1000; i++ {
		k := BigIntToBytesLE(bLen, big.NewInt(int64(i)))
		v := BigIntToBytesLE(bLen, big.NewInt(0))
		if err := tree.Add(k, v); err != nil {
			t.Fatal(err)
		}
	}

	checkRootBIString(c, tree,
		"296519252211642170490407814696803112091039265640052570497930797516015811235")

	database = metadb.NewTest(t)
	tree2, err := NewTree(Config{Database: database, MaxLevels: 256, HashFunction: HashFunctionPoseidon})
	c.Assert(err, qt.IsNil)

	var keys, values [][]byte
	for i := 0; i < 1000; i++ {
		k := BigIntToBytesLE(bLen, big.NewInt(int64(i)))
		v := BigIntToBytesLE(bLen, big.NewInt(0))
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
	tree1, err := NewTree(Config{Database: database1, MaxLevels: 256, HashFunction: HashFunctionPoseidon})
	c.Assert(err, qt.IsNil)

	bLen := 32
	for i := 0; i < 16; i++ {
		k := BigIntToBytesLE(bLen, big.NewInt(int64(i)))
		v := BigIntToBytesLE(bLen, big.NewInt(0))
		if err := tree1.Add(k, v); err != nil {
			t.Fatal(err)
		}
	}

	database2 := metadb.NewTest(t)
	tree2, err := NewTree(Config{Database: database2, MaxLevels: 256, HashFunction: HashFunctionPoseidon})
	c.Assert(err, qt.IsNil)

	for i := 16 - 1; i >= 0; i-- {
		k := BigIntToBytesLE(bLen, big.NewInt(int64(i)))
		v := BigIntToBytesLE(bLen, big.NewInt(0))
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
	tree, err := NewTree(Config{Database: database, MaxLevels: 256, HashFunction: HashFunctionPoseidon})
	c.Assert(err, qt.IsNil)

	bLen := 32
	k := BigIntToBytesLE(bLen, big.NewInt(int64(3)))
	v := BigIntToBytesLE(bLen, big.NewInt(int64(12)))

	err = tree.Add(k, v)
	c.Assert(err, qt.IsNil)
	err = tree.Add(k, v) // repeating same key-value
	c.Check(err, qt.Equals, ErrKeyAlreadyExists)
}

func TestUpdate(t *testing.T) {
	c := qt.New(t)
	database := metadb.NewTest(t)
	tree, err := NewTree(Config{Database: database, MaxLevels: 256, HashFunction: HashFunctionPoseidon})
	c.Assert(err, qt.IsNil)

	bLen := 32
	k := BigIntToBytesLE(bLen, big.NewInt(int64(20)))
	v := BigIntToBytesLE(bLen, big.NewInt(int64(12)))
	if err := tree.Add(k, v); err != nil {
		t.Fatal(err)
	}

	v = BigIntToBytesLE(bLen, big.NewInt(int64(11)))
	err = tree.Update(k, v)
	c.Assert(err, qt.IsNil)

	gettedKey, gettedValue, err := tree.Get(k)
	c.Assert(err, qt.IsNil)
	c.Check(gettedKey, qt.DeepEquals, k)
	c.Check(gettedValue, qt.DeepEquals, v)

	// add more leafs to the tree to do another test
	for i := 0; i < 16; i++ {
		k := BigIntToBytesLE(bLen, big.NewInt(int64(i)))
		v := BigIntToBytesLE(bLen, big.NewInt(int64(i*2)))
		if err := tree.Add(k, v); err != nil {
			t.Fatal(err)
		}
	}

	k = BigIntToBytesLE(bLen, big.NewInt(int64(3)))
	v = BigIntToBytesLE(bLen, big.NewInt(int64(11)))
	// check that before the Update, value for 3 is !=11
	gettedKey, gettedValue, err = tree.Get(k)
	c.Assert(err, qt.IsNil)
	c.Check(gettedKey, qt.DeepEquals, k)
	c.Check(gettedValue, qt.Not(qt.DeepEquals), v)
	c.Check(gettedValue, qt.DeepEquals, BigIntToBytesLE(bLen, big.NewInt(6)))

	err = tree.Update(k, v)
	c.Assert(err, qt.IsNil)

	// check that after Update, the value for 3 is ==11
	gettedKey, gettedValue, err = tree.Get(k)
	c.Assert(err, qt.IsNil)
	c.Check(gettedKey, qt.DeepEquals, k)
	c.Check(gettedValue, qt.DeepEquals, v)
	c.Check(gettedValue, qt.DeepEquals, BigIntToBytesLE(bLen, big.NewInt(11)))
}

func TestRootOnTx(t *testing.T) {
	c := qt.New(t)
	database := metadb.NewTest(t)
	tree, err := NewTree(Config{Database: database, MaxLevels: 256, HashFunction: HashFunctionBlake2b})
	c.Assert(err, qt.IsNil)

	bLen := 32
	k := BigIntToBytesLE(bLen, big.NewInt(int64(20)))
	v := BigIntToBytesLE(bLen, big.NewInt(int64(12)))
	if err := tree.Add(k, v); err != nil {
		t.Fatal(err)
	}
	rootInitial, err := tree.Root()
	c.Assert(err, qt.IsNil)

	// Start a new transaction
	tx := database.WriteTx()

	// Update the value of the key
	v = BigIntToBytesLE(bLen, big.NewInt(int64(11)))
	err = tree.UpdateWithTx(tx, k, v)
	c.Assert(err, qt.IsNil)

	// Check that the root has not been updated yet
	rootBefore, err := tree.Root()
	c.Assert(err, qt.IsNil)
	c.Assert(rootBefore, qt.DeepEquals, rootInitial)

	newRoot, err := tree.RootWithTx(tx)
	c.Assert(err, qt.IsNil)
	c.Assert(newRoot, qt.Not(qt.DeepEquals), rootBefore)

	// Commit the tx
	c.Assert(tx.Commit(), qt.IsNil)
	tx = database.WriteTx()

	// Check the new root equals the one returned by Root()
	rootAfter, err := tree.Root()
	c.Assert(err, qt.IsNil)
	c.Assert(rootAfter, qt.DeepEquals, newRoot)

	// Add a new key-value pair
	k2 := BigIntToBytesLE(bLen, big.NewInt(int64(30)))
	v2 := BigIntToBytesLE(bLen, big.NewInt(int64(40)))
	if err := tree.AddWithTx(tx, k2, v2); err != nil {
		t.Fatal(err)
	}

	// Check that the tx root has been updated
	newRoot, err = tree.RootWithTx(tx)
	c.Assert(err, qt.IsNil)
	c.Assert(rootAfter, qt.Not(qt.DeepEquals), newRoot)

	c.Assert(tx.Commit(), qt.IsNil)
	tx = database.WriteTx()

	// Check that the root has been updated after commit
	rootInitial, err = tree.Root()
	c.Assert(err, qt.IsNil)
	c.Assert(rootInitial, qt.DeepEquals, newRoot)

	// Delete a key-value pair
	if err := tree.DeleteWithTx(tx, k); err != nil {
		t.Fatal(err)
	}

	// Check that the root has not been updated yet
	rootBefore, err = tree.Root()
	c.Assert(err, qt.IsNil)
	c.Assert(rootBefore, qt.DeepEquals, rootInitial)

	// Check that the tx root is different from the tree root
	newRoot, err = tree.RootWithTx(tx)
	c.Assert(err, qt.IsNil)
	c.Assert(newRoot, qt.Not(qt.DeepEquals), rootBefore)

	// Commit the transaction
	err = tx.Commit()
	c.Assert(err, qt.IsNil)

	// Check that the root has been updated
	rootAfter, err = tree.Root()
	c.Assert(err, qt.IsNil)
	c.Assert(rootAfter, qt.DeepEquals, newRoot)
}

func TestAux(t *testing.T) { // TODO split in proper tests
	c := qt.New(t)
	database := metadb.NewTest(t)
	tree, err := NewTree(Config{
		Database: database, MaxLevels: 256,
		HashFunction: HashFunctionPoseidon,
	})
	c.Assert(err, qt.IsNil)

	bLen := 32
	k := BigIntToBytesLE(bLen, big.NewInt(int64(1)))
	v := BigIntToBytesLE(bLen, big.NewInt(int64(0)))
	err = tree.Add(k, v)
	c.Assert(err, qt.IsNil)
	k = BigIntToBytesLE(bLen, big.NewInt(int64(256)))
	err = tree.Add(k, v)
	c.Assert(err, qt.IsNil)

	k = BigIntToBytesLE(bLen, big.NewInt(int64(257)))
	err = tree.Add(k, v)
	c.Assert(err, qt.IsNil)

	k = BigIntToBytesLE(bLen, big.NewInt(int64(515)))
	err = tree.Add(k, v)
	c.Assert(err, qt.IsNil)
	k = BigIntToBytesLE(bLen, big.NewInt(int64(770)))
	err = tree.Add(k, v)
	c.Assert(err, qt.IsNil)

	k = BigIntToBytesLE(bLen, big.NewInt(int64(388)))
	err = tree.Add(k, v)
	c.Assert(err, qt.IsNil)
	k = BigIntToBytesLE(bLen, big.NewInt(int64(900)))
	err = tree.Add(k, v)
	c.Assert(err, qt.IsNil)
	//
	// err = tree.PrintGraphviz(nil)
	// c.Assert(err, qt.IsNil)
}

func TestGet(t *testing.T) {
	c := qt.New(t)
	database := metadb.NewTest(t)
	tree, err := NewTree(Config{
		Database: database, MaxLevels: 256,
		HashFunction: HashFunctionPoseidon,
	})
	c.Assert(err, qt.IsNil)

	bLen := 32
	for i := 0; i < 10; i++ {
		k := BigIntToBytesLE(bLen, big.NewInt(int64(i)))
		v := BigIntToBytesLE(bLen, big.NewInt(int64(i*2)))
		if err := tree.Add(k, v); err != nil {
			t.Fatal(err)
		}
	}

	k := BigIntToBytesLE(bLen, big.NewInt(int64(7)))
	gettedKey, gettedValue, err := tree.Get(k)
	c.Assert(err, qt.IsNil)
	c.Check(gettedKey, qt.DeepEquals, k)
	c.Check(gettedValue, qt.DeepEquals, BigIntToBytesLE(bLen, big.NewInt(int64(7*2))))
}

func TestBitmapBytes(t *testing.T) {
	c := qt.New(t)

	b := []byte{15}
	bits := bytesToBitmap(b)
	c.Assert(bits, qt.DeepEquals, []bool{
		true, true, true, true,
		false, false, false, false,
	})
	b2 := bitmapToBytes(bits)
	c.Assert(b2, qt.DeepEquals, b)

	b = []byte{0, 15, 50}
	bits = bytesToBitmap(b)
	c.Assert(bits, qt.DeepEquals, []bool{
		false, false, false,
		false, false, false, false, false, true, true, true, true,
		false, false, false, false, false, true, false, false, true,
		true, false, false,
	})
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

	// test edge cases with invalid packed siblings
	_, err = UnpackSiblings(HashFunctionPoseidon, []byte{})
	c.Assert(err, qt.IsNotNil)
	_, err = UnpackSiblings(HashFunctionPoseidon, []byte{1, 0})
	c.Assert(err, qt.IsNotNil)
	_, err = UnpackSiblings(HashFunctionPoseidon, []byte{1, 0, 1, 0})
	c.Assert(err, qt.IsNotNil)
	_, err = UnpackSiblings(HashFunctionPoseidon, []byte{3, 0, 0})
	c.Assert(err, qt.IsNotNil)
	_, err = UnpackSiblings(HashFunctionPoseidon, []byte{4, 0, 1, 0})
	c.Assert(err, qt.IsNotNil)
}

func TestGenProofAndVerify(t *testing.T) {
	c := qt.New(t)
	database := metadb.NewTest(t)
	tree, err := NewTree(Config{
		Database: database, MaxLevels: 256,
		HashFunction: HashFunctionPoseidon,
	})
	c.Assert(err, qt.IsNil)

	bLen := 32
	for i := 0; i < 10; i++ {
		k := BigIntToBytesLE(bLen, big.NewInt(int64(i)))
		v := BigIntToBytesLE(bLen, big.NewInt(int64(i*2)))
		if err := tree.Add(k, v); err != nil {
			t.Fatal(err)
		}
	}

	k := BigIntToBytesLE(bLen, big.NewInt(int64(7)))
	v := BigIntToBytesLE(bLen, big.NewInt(int64(14)))
	kAux, proofV, siblings, existence, err := tree.GenProof(k)
	c.Assert(err, qt.IsNil)
	c.Assert(proofV, qt.DeepEquals, v)
	c.Assert(k, qt.DeepEquals, kAux)
	c.Assert(existence, qt.IsTrue)

	root, err := tree.Root()
	c.Assert(err, qt.IsNil)
	err = CheckProof(tree.hashFunction, k, v, root, siblings)
	c.Assert(err, qt.IsNil)
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
	tree1, err := NewTree(Config{
		Database: database1, MaxLevels: 256,
		HashFunction: HashFunctionPoseidon,
	})
	c.Assert(err, qt.IsNil)

	bLen := 32
	for i := 0; i < 16; i++ {
		k := BigIntToBytesLE(bLen, big.NewInt(int64(i)))
		v := BigIntToBytesLE(bLen, big.NewInt(int64(i*2)))
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
	tree2, err := NewTree(Config{
		Database: database2, MaxLevels: 256,
		HashFunction: HashFunctionPoseidon,
	})
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
	tree, err := NewTree(Config{
		Database: database, MaxLevels: 256,
		HashFunction: HashFunctionPoseidon,
	})
	c.Assert(err, qt.IsNil)

	bLen := 32
	var keys, values [][]byte
	for i := 0; i < 1000; i++ {
		k := BigIntToBytesLE(bLen, big.NewInt(int64(i)))
		v := BigIntToBytesLE(bLen, big.NewInt(0))
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
	k := BigIntToBytesLE(bLen, big.NewInt(int64(99999)))
	v := BigIntToBytesLE(bLen, big.NewInt(int64(99999)))
	if err := tree.Add(k, v); err != nil {
		t.Fatal(err)
	}
}

func TestAddBatchFullyUsed(t *testing.T) {
	c := qt.New(t)

	database1 := metadb.NewTest(t)
	tree1, err := NewTree(Config{
		Database: database1, MaxLevels: 4,
		HashFunction: HashFunctionPoseidon,
	})
	c.Assert(err, qt.IsNil)

	database2 := metadb.NewTest(t)
	tree2, err := NewTree(Config{
		Database: database2, MaxLevels: 4,
		HashFunction: HashFunctionPoseidon,
	})
	c.Assert(err, qt.IsNil)

	var keys, values [][]byte
	for i := 0; i < 16; i++ {
		k := BigIntToBytesLE(1, big.NewInt(int64(i)))
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
		auxK1, auxV1, err := tree1.Get(BigIntToBytesLE(1, big.NewInt(int64(i))))
		c.Assert(err, qt.IsNil)

		auxK2, auxV2, err := tree2.Get(BigIntToBytesLE(1, big.NewInt(int64(i))))
		c.Assert(err, qt.IsNil)

		c.Assert(auxK1, qt.DeepEquals, auxK2)
		c.Assert(auxV1, qt.DeepEquals, auxV2)
	}

	// try adding one more key to both trees (through Add & AddBatch) and
	// expect not being added due the tree is already full
	k := BigIntToBytesLE(1, big.NewInt(int64(16)))
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
	tree, err := NewTree(Config{
		Database: database, MaxLevels: 256,
		HashFunction: HashFunctionPoseidon,
	})
	c.Assert(err, qt.IsNil)

	expectedRoot := "13742386369878513332697380582061714160370929283209286127733983161245560237407"

	// fill the tree
	bLen := 32
	var keys, values [][]byte
	for i := 0; i < 1000; i++ {
		k := BigIntToBytesLE(bLen, big.NewInt(int64(i)))
		v := BigIntToBytesLE(bLen, big.NewInt(int64(i)))
		keys = append(keys, k)
		values = append(values, v)
	}
	for i, k := range keys {
		err := tree.Add(k, values[i])
		c.Assert(err, qt.IsNil)
	}
	checkRootBIString(c, tree,
		expectedRoot)

	// add one more k-v
	k := BigIntToBytesLE(bLen, big.NewInt(1000))
	v := BigIntToBytesLE(bLen, big.NewInt(1000))
	err = tree.Add(k, v)
	c.Assert(err, qt.IsNil)
	checkRootBIString(c, tree,
		"10747149055773881257049574592162159501044114324358186833013814735296193179713")

	// get the roots from level 2 and set one of them as the new root
	roots, err := tree.RootsFromLevel(2)
	c.Assert(err, qt.IsNil)
	c.Assert(tree.SetRoot(roots[0]), qt.IsNil)

	// check that the new root is the same as the one set
	root, err := tree.Root()
	c.Assert(err, qt.IsNil)
	c.Assert(string(root), qt.Equals, string(roots[0]))

	// check that the tree can be updated
	err = tree.Add(BigIntToBytesLE(bLen, big.NewInt(int64(1024))), []byte("test"))
	c.Assert(err, qt.IsNil)
	err = tree.Update(BigIntToBytesLE(bLen, big.NewInt(int64(1024))), []byte("test2"))
	c.Assert(err, qt.IsNil)

	// check that the k-v '1000' does not exist in the new tree
	_, _, err = tree.Get(k)
	c.Assert(err, qt.Equals, ErrKeyNotFound)

	// check that the root can't be set to an empty root
	err = tree.SetRoot(tree.emptyHash)
	c.Assert(err, qt.IsNotNil)
}

func TestSnapshot(t *testing.T) {
	c := qt.New(t)
	database := metadb.NewTest(t)
	tree, err := NewTree(Config{
		Database: database, MaxLevels: 256,
		HashFunction: HashFunctionPoseidon,
	})
	c.Assert(err, qt.IsNil)

	// fill the tree
	bLen := 32
	var keys, values [][]byte
	for i := 0; i < 1000; i++ {
		k := BigIntToBytesLE(bLen, big.NewInt(int64(i)))
		v := BigIntToBytesLE(bLen, big.NewInt(int64(i)))
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
	tree, err := NewTree(Config{
		Database: database, MaxLevels: 256,
		HashFunction: HashFunctionPoseidon,
	})
	c.Assert(err, qt.IsNil)

	bLen := 32
	k := BigIntToBytesLE(bLen, big.NewInt(int64(3)))

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
	tree, err := NewTree(Config{
		Database: database, MaxLevels: maxLevels,
		HashFunction: HashFunctionBlake2b,
	})
	c.Assert(err, qt.IsNil)

	// expect no errors when adding a key of only 4 bytes (when the
	// required length of keyPath for 100 levels would be 13 bytes)
	bLen := 4
	k := BigIntToBytesLE(bLen, big.NewInt(1))
	v := BigIntToBytesLE(bLen, big.NewInt(1))

	err = tree.Add(k, v)
	c.Assert(err, qt.IsNil)

	err = tree.Update(k, v)
	c.Assert(err, qt.IsNil)

	_, _, _, _, err = tree.GenProof(k)
	c.Assert(err, qt.IsNil)

	_, _, err = tree.Get(k)
	c.Assert(err, qt.IsNil)

	k = BigIntToBytesLE(bLen, big.NewInt(2))
	v = BigIntToBytesLE(bLen, big.NewInt(2))
	invalids, err := tree.AddBatch([][]byte{k}, [][]byte{v})
	c.Assert(err, qt.IsNil)
	c.Assert(len(invalids), qt.Equals, 0)

	// expect errors when adding a key bigger than maximum capacity of the
	// tree: ceil(maxLevels/8)
	maxLevels = 32
	database = metadb.NewTest(t)
	tree, err = NewTree(Config{
		Database: database, MaxLevels: maxLevels,
		HashFunction: HashFunctionBlake2b,
	})
	c.Assert(err, qt.IsNil)

	maxKeyLen := int(math.Ceil(float64(maxLevels) / float64(8)))
	k = BigIntToBytesLE(maxKeyLen+1, big.NewInt(1))
	v = BigIntToBytesLE(maxKeyLen+1, big.NewInt(1))

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
		ks = append(ks, BigIntToBytesLE(maxKeyLen+1, big.NewInt(1)))
		vs = append(vs, BigIntToBytesLE(maxKeyLen+1, big.NewInt(1)))
	}
	invalids, err = tree.AddBatch(ks, vs)
	c.Assert(err, qt.IsNil)
	c.Assert(len(invalids), qt.Equals, nKVs)

	// check that with maxKeyLen it can be added
	k = BigIntToBytesLE(maxKeyLen, big.NewInt(1))
	err = tree.Add(k, v)
	c.Assert(err, qt.IsNil)

	// check CheckProof check with key longer
	kAux, vAux, packedSiblings, existence, err := tree.GenProof(k)
	c.Assert(err, qt.IsNil)
	c.Assert(existence, qt.IsTrue)

	root, err := tree.Root()
	c.Assert(err, qt.IsNil)
	err = CheckProof(tree.HashFunction(), kAux, vAux, root, packedSiblings)
	c.Assert(err, qt.IsNil)

	// use a similar key but with one zero, expect that CheckProof fails on
	// the verification
	kAux = append(kAux, 0)
	err = CheckProof(tree.HashFunction(), kAux, vAux, root, packedSiblings)
	c.Assert(err, qt.ErrorMatches, "calculated vs expected root mismatch")
}

func TestKeyLenBiggerThan32(t *testing.T) {
	c := qt.New(t)
	maxLevels := 264
	database := metadb.NewTest(t)
	tree, err := NewTree(Config{
		Database: database, MaxLevels: maxLevels,
		HashFunction: HashFunctionBlake2b,
	})
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

func TestDelete(t *testing.T) {
	c := qt.New(t)
	database := metadb.NewTest(t)
	tree, err := NewTree(Config{
		Database: database, MaxLevels: 256,
		HashFunction: HashFunctionPoseidon,
	})
	c.Assert(err, qt.IsNil)

	bLen := 32
	// Add multiple key/value pairs to the tree
	keys := [][]byte{
		BigIntToBytesLE(bLen, big.NewInt(int64(1))),
		BigIntToBytesLE(bLen, big.NewInt(int64(2))),
		BigIntToBytesLE(bLen, big.NewInt(int64(3))),
		BigIntToBytesLE(bLen, big.NewInt(int64(4))),
	}
	values := [][]byte{
		BigIntToBytesLE(bLen, big.NewInt(int64(10))),
		BigIntToBytesLE(bLen, big.NewInt(int64(20))),
		BigIntToBytesLE(bLen, big.NewInt(int64(30))),
		BigIntToBytesLE(bLen, big.NewInt(int64(40))),
	}
	for i, key := range keys {
		err := tree.Add(key, values[i])
		c.Assert(err, qt.IsNil)
	}

	originalRoot, err := tree.Root()
	c.Assert(err, qt.IsNil)

	// Delete a key from the tree
	err = tree.Delete(keys[2])
	c.Assert(err, qt.IsNil)

	// Check if the key was deleted
	_, _, err = tree.Get(keys[2])
	c.Assert(err, qt.Equals, ErrKeyNotFound)

	// Check the root has changed
	afterDeleteRoot, err := tree.Root()
	c.Assert(err, qt.IsNil)
	c.Check(afterDeleteRoot, qt.Not(qt.DeepEquals), originalRoot)

	// Check if other keys are unaffected
	for i, key := range keys {
		if i == 2 {
			continue // skip the deleted key
		}
		_, value, err := tree.Get(key)
		c.Assert(err, qt.IsNil)
		c.Check(value, qt.DeepEquals, values[i])
	}

	// Check the root hash is the same after re-adding the deleted key
	err = tree.Add(keys[2], values[2]) // re-add the deleted key
	c.Assert(err, qt.IsNil)
	afterAddRoot, err := tree.Root()
	c.Assert(err, qt.IsNil)
	c.Check(afterAddRoot, qt.DeepEquals, originalRoot)
}

func BenchmarkAdd(b *testing.B) {
	bLen := 32 // for both Poseidon & Sha256
	// prepare inputs
	var ks, vs [][]byte
	for i := 0; i < 1000; i++ {
		k := BigIntToBytesLE(bLen, big.NewInt(int64(i)))
		v := BigIntToBytesLE(bLen, big.NewInt(int64(i)))
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
	tree, err := NewTree(Config{
		Database: database, MaxLevels: 140,
		HashFunction: hashFunc,
	})
	c.Assert(err, qt.IsNil)

	for i := 0; i < len(ks); i++ {
		if err := tree.Add(ks[i], vs[i]); err != nil {
			b.Fatal(err)
		}
	}
}

func TestDiskSizeBench(t *testing.T) {
	c := qt.New(t)

	nLeafs := 1000
	printTestContext("TestDiskSizeBench: ", nLeafs, "Blake2b", "pebble")

	// prepare inputs
	var ks, vs [][]byte
	for i := 0; i < nLeafs; i++ {
		k := randomBytes(32)
		v := randomBytes(32)
		ks = append(ks, k)
		vs = append(vs, v)
	}

	// create the database and tree
	dbDir := t.TempDir()
	database, err := metadb.New(db.TypePebble, dbDir)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = database.Close() })

	tree, err := NewTree(Config{database, 256, DefaultThresholdNLeafs, HashFunctionBlake2b})
	c.Assert(err, qt.IsNil)

	countDBitems := func() int {
		count := 0
		if err := tree.db.Iterate(nil, func(_, _ []byte) bool {
			// fmt.Printf("db item: %x\n", k)
			count++
			return true
		}); err != nil {
			t.Fatal(err)
		}
		return count
	}
	// add the leafs
	start := time.Now()
	tx := tree.db.WriteTx()
	for i := 0; i < len(ks); i++ {
		err = tree.AddWithTx(tx, ks[i], vs[i])
		c.Assert(err, qt.IsNil)
		//	fmt.Printf("add %x\n", ks[i])
	}
	c.Assert(tx.Commit(), qt.IsNil)
	printRes("	Add loop", time.Since(start))
	tree.dbg.print("		")
	size, err := dirSize(dbDir)
	c.Assert(err, qt.IsNil)
	dbItemsAdd := countDBitems()
	printRes("	Disk size (add)", fmt.Sprintf("%d MiB", size/(1024*1024)))
	printRes("	DB items", fmt.Sprintf("%d", dbItemsAdd))

	// delete the leafs
	start = time.Now()
	tx = tree.db.WriteTx()
	for i := 0; i < len(ks)/2; i++ {
		err = tree.DeleteWithTx(tx, ks[i])
		c.Assert(err, qt.IsNil)
		// fmt.Printf("deleted %x\n", ks[i])
	}
	c.Assert(tx.Commit(), qt.IsNil)
	printRes("	Delete loop", time.Since(start))
	tree.dbg.print("		")
	size, err = dirSize(dbDir)
	c.Assert(err, qt.IsNil)
	dbItemsDelete := countDBitems()
	printRes("	Disk size (delete)", fmt.Sprintf("%d MiB", size/(1024*1024)))
	printRes("	DB items", fmt.Sprintf("%d", dbItemsDelete))

	if dbItemsDelete+(dbItemsDelete/4) >= dbItemsAdd {
		t.Fatal("DB items after delete is too big")
	}

	// add the leafs deleted again
	start = time.Now()
	tx = tree.db.WriteTx()
	for i := 0; i < len(ks)/2; i++ {
		err = tree.AddWithTx(tx, ks[i], vs[i])
		c.Assert(err, qt.IsNil)
	}
	c.Assert(tx.Commit(), qt.IsNil)
	printRes("	Add2 loop", time.Since(start))
	tree.dbg.print("		")
	size, err = dirSize(dbDir)
	dbItemsAdd2 := countDBitems()
	c.Assert(err, qt.IsNil)
	printRes("	Disk size (add2)", fmt.Sprintf("%d MiB", size/(1024*1024)))
	printRes("	DB items", fmt.Sprintf("%d", dbItemsAdd2))
	if dbItemsAdd2 != dbItemsAdd {
		t.Fatal("DB items after add2 is not equal to DB items after add")
	}

	// update the leafs
	start = time.Now()
	tx = tree.db.WriteTx()
	for i := 0; i < len(ks); i++ {
		err = tree.UpdateWithTx(tx, ks[i], vs[len(ks)-i-1])
		c.Assert(err, qt.IsNil, qt.Commentf("k=%x", ks[i]))
		// fmt.Printf("updated %x\n", ks[i])
	}
	c.Assert(tx.Commit(), qt.IsNil)
	printRes("	Update loop", time.Since(start))
	tree.dbg.print("		")
	size, err = dirSize(dbDir)
	dbItemsUpdate := countDBitems()
	c.Assert(err, qt.IsNil)
	printRes("	Disk size (update)", fmt.Sprintf("%d MiB", size/(1024*1024)))
	printRes("	DB items", fmt.Sprintf("%d", dbItemsUpdate))
	if dbItemsUpdate != dbItemsAdd {
		t.Fatal("DB items after update is not equal to DB items after add")
	}

	start = time.Now()
	c.Assert(tree.db.Compact(), qt.IsNil)
	printRes("	Compact DB", time.Since(start))
	printRes("	Disk size (compact)", fmt.Sprintf("%d MiB", size/(1024*1024)))
	printRes("	DB items", fmt.Sprintf("%d", countDBitems()))
}

func dirSize(path string) (int64, error) {
	var size int64
	err := filepath.Walk(path, func(_ string, info os.FileInfo, err error) error {
		if err != nil {
			return nil
		}
		if !info.IsDir() {
			size += info.Size()
		}
		return nil
	})
	return size, err
}

func TestGetLeavesFromSubPath(t *testing.T) {
	c := qt.New(t)
	database := metadb.NewTest(t)
	tree, err := NewTree(Config{
		Database: database, MaxLevels: 256,
		HashFunction: HashFunctionPoseidon,
	})
	c.Assert(err, qt.IsNil)

	bLen := 32
	// Add keys with shared prefix and some without shared prefix
	keys := [][]byte{
		// Shared prefix keys
		append([]byte{0xFF, 0xFF}, BigIntToBytesLE(bLen-2, big.NewInt(int64(1)))...),
		append([]byte{0xFF, 0xFF}, BigIntToBytesLE(bLen-2, big.NewInt(int64(2)))...),
		append([]byte{0xFF, 0xFF}, BigIntToBytesLE(bLen-2, big.NewInt(int64(3)))...),
		// Non-shared prefix keys
		BigIntToBytesLE(bLen, big.NewInt(int64(4))),
		BigIntToBytesLE(bLen, big.NewInt(int64(5))),
	}
	sharedPrefixKeyNotAdded := append([]byte{0xFF, 0xFF}, BigIntToBytesLE(bLen-2, big.NewInt(int64(4)))...)
	values := [][]byte{
		BigIntToBytesLE(bLen, big.NewInt(int64(10))),
		BigIntToBytesLE(bLen, big.NewInt(int64(20))),
		BigIntToBytesLE(bLen, big.NewInt(int64(30))),
		BigIntToBytesLE(bLen, big.NewInt(int64(40))),
		BigIntToBytesLE(bLen, big.NewInt(int64(50))),
	}
	for i, key := range keys {
		err := tree.Add(key, values[i])
		c.Assert(err, qt.IsNil)
	}

	root, err := tree.Root()
	c.Assert(err, qt.IsNil)

	// Use the down function to get intermediate nodes on the path to a given leaf with shared prefix
	var intermediates [][]byte
	_, _, _, err = tree.down(
		database,
		sharedPrefixKeyNotAdded,
		root,
		[][]byte{},
		&intermediates,
		getPath(tree.maxLevels, sharedPrefixKeyNotAdded),
		0,
		false,
	)
	c.Assert(err, qt.IsNil)

	// For our test, we'll use the third intermediate node in the path
	//  (this assumes that it's the one shared by the leaves with the shared prefix)
	if len(intermediates) == 0 {
		t.Fatal("No intermediates found")
	}
	intermediateNodeKey := intermediates[2]

	subKeys, _, err := tree.getLeavesFromSubPath(database, intermediateNodeKey)
	c.Assert(err, qt.IsNil)

	// Check if all leaves with shared prefix are present in the result and leaves without the shared prefix are not
	for _, key := range keys {
		found := false
		for _, sk := range subKeys {
			if bytes.Equal(sk, key) {
				found = true
				break
			}
		}
		if bytes.HasPrefix(key, []byte{0xFF, 0xFF}) {
			c.Assert(found, qt.IsTrue, qt.Commentf("Expected leaf %x not found", key))
		} else {
			c.Assert(found, qt.IsFalse, qt.Commentf("Leaf %x should not have been found", key))
		}
	}
}

func TestTreeAfterDeleteAndReconstruct(t *testing.T) {
	nLeafs := 1000

	c := qt.New(t)
	database1 := metadb.NewTest(t)
	database2 := metadb.NewTest(t)

	// 1. A new tree (tree1) is created and filled with some data
	tree1, err := NewTree(Config{
		Database: database1, MaxLevels: 256,
		HashFunction: HashFunctionBlake2b,
	})
	c.Assert(err, qt.IsNil)

	// prepare inputs
	var keys, values [][]byte
	for i := 0; i < nLeafs; i++ {
		k := randomBytes(32)
		v := randomBytes(32)
		keys = append(keys, k)
		values = append(values, v)
	}

	for i, key := range keys {
		err := tree1.Add(key, values[i])
		c.Assert(err, qt.IsNil)
	}

	// 2. Delete some keys from the tree
	keysToDelete := [][]byte{keys[1], keys[3]}
	for _, key := range keysToDelete {
		err := tree1.Delete(key)
		c.Assert(err, qt.IsNil)
	}

	// 3. Create a second tree (tree2)
	tree2, err := NewTree(Config{
		Database: database2, MaxLevels: 256,
		HashFunction: HashFunctionBlake2b,
	})
	c.Assert(err, qt.IsNil)

	// 4. Add the non-deleted keys from tree1 to tree2
	for i, key := range keys {
		if !contains(keysToDelete, key) {
			err := tree2.Add(key, values[i])
			c.Assert(err, qt.IsNil)
		}
	}

	root1, err := tree1.Root()
	c.Assert(err, qt.IsNil)
	root2, err := tree2.Root()
	c.Assert(err, qt.IsNil)
	// 5. verify the root hash is the same for tree1 and tree2
	c.Assert(bytes.Equal(root1, root2), qt.IsTrue, qt.Commentf("Roots of tree1 and tree2 do not match"))
}

// Helper function to check if a byte slice array contains a byte slice
func contains(arr [][]byte, item []byte) bool {
	for _, a := range arr {
		if bytes.Equal(a, item) {
			return true
		}
	}
	return false
}

func TestTreeWithSingleLeaf(t *testing.T) {
	c := qt.New(t)

	database := metadb.NewTest(t)
	tree, err := NewTree(Config{
		Database: database, MaxLevels: 256,
		HashFunction: HashFunctionPoseidon,
	})
	c.Assert(err, qt.IsNil)

	// check empty root
	root, err := tree.Root()
	c.Assert(err, qt.IsNil)
	c.Assert(hex.EncodeToString(root), qt.DeepEquals, "0000000000000000000000000000000000000000000000000000000000000000")

	// add one entry
	err = tree.Add([]byte{0x01}, []byte{0x01})
	c.Assert(err, qt.IsNil)

	root, err = tree.Root()
	c.Assert(err, qt.IsNil)
	c.Assert(root, qt.HasLen, 32)

	leafKey, _, err := newLeafValue(HashFunctionPoseidon, []byte{0x01}, []byte{0x01})
	c.Assert(err, qt.IsNil)

	// check leaf key is actually the current root
	c.Assert(bytes.Equal(root, leafKey), qt.IsTrue)

	// delete the only entry and check the tree is empty again
	err = tree.Delete([]byte{0x01})
	c.Assert(err, qt.IsNil)
	root, err = tree.Root()
	c.Assert(err, qt.IsNil)
	c.Assert(hex.EncodeToString(root), qt.DeepEquals, "0000000000000000000000000000000000000000000000000000000000000000")
}

// BenchmarkAddVsAddBatch benchmarks the Add and AddBatch functions and compare they produce the same output.
// go test -run=NONE -bench=BenchmarkAddVsAddBatch -benchmem
func BenchmarkAddVsAddBatch(b *testing.B) {
	nLeaves := 200000
	bLen := 32
	// Prepare the initial set of keys and values
	ks1, vs1 := generateKV(bLen, nLeaves/2, 0)
	// Prepare the second set of keys and values
	ks2, vs2 := generateKV(bLen, nLeaves/2, nLeaves)

	var rootAdd, rootAddBatch []byte

	b.Run("Add", func(b *testing.B) {
		database := metadb.NewTest(b)
		tree, err := NewTree(Config{Database: database, MaxLevels: 256, HashFunction: HashFunctionBlake2b})
		if err != nil {
			b.Fatal(err)
		}

		// Add the first set of key/values
		for i := 0; i < len(ks1); i++ {
			if err := tree.Add(ks1[i], vs1[i]); err != nil {
				b.Fatal(err)
			}
		}

		// Execute some deletes
		for i := 0; i < len(ks1)/4; i++ {
			if err := tree.Delete(ks1[i]); err != nil {
				b.Fatal(err)
			}
		}

		// Execute some updates
		for i := len(ks1)/4 + 1; i < len(ks1)/2; i++ {
			if err := tree.Update(ks1[i], vs2[i]); err != nil {
				b.Fatal(err)
			}
		}

		// Add the second set of key/values
		for i := 0; i < len(ks2); i++ {
			if err := tree.Add(ks2[i], vs2[i]); err != nil {
				b.Fatal(err)
			}
		}

		rootAdd, err = tree.Root()
		if err != nil {
			b.Fatal(err)
		}
	})

	b.Run("AddBatch", func(b *testing.B) {
		database := metadb.NewTest(b)
		tree, err := NewTree(Config{Database: database, MaxLevels: 256, HashFunction: HashFunctionBlake2b})
		if err != nil {
			b.Fatal(err)
		}

		// Add the first set of key/values
		if _, err := tree.AddBatch(ks1, vs1); err != nil {
			b.Fatal(err)
		}

		// Execute some deletes
		for i := 0; i < len(ks1)/4; i++ {
			if err := tree.Delete(ks1[i]); err != nil {
				b.Fatal(err)
			}
		}

		// Execute some updates
		for i := len(ks1)/4 + 1; i < len(ks1)/2; i++ {
			if err := tree.Update(ks1[i], vs2[i]); err != nil {
				b.Fatal(err)
			}
		}
		// Add the second set of key/values
		if _, err := tree.AddBatch(ks2, vs2); err != nil {
			b.Fatal(err)
		}

		rootAddBatch, err = tree.Root()
		if err != nil {
			b.Fatal(err)
		}
	})

	// Verify that the roots are the same
	if !bytes.Equal(rootAdd, rootAddBatch) {
		b.Fatalf("Roots are different: Add root = %x, AddBatch root = %x", rootAdd, rootAddBatch)
	}
}

// generateKV generates a slice of keys and a slice of values.
// The keys are byte representations of sequential integers, and the values are random bytes.
func generateKV(bLen, count, offset int) ([][]byte, [][]byte) {
	var keys, values [][]byte

	for i := 0; i < count; i++ {
		// Convert integer i to a byte slice
		key := BigIntToBytesLE(bLen, big.NewInt(int64(offset+i)))
		value := randomBytes(bLen)

		keys = append(keys, key)
		values = append(values, value)
	}

	return keys, values
}

// TestAddVsAddBatch tests that the Add and AddBatch functions produce the same output.
func TestAddVsAddBatch(t *testing.T) {
	c := qt.New(t)

	// Create a new tree
	database := metadb.NewTest(t)
	tree, err := NewTree(Config{Database: database, MaxLevels: 256, HashFunction: HashFunctionBlake2b})
	c.Assert(err, qt.IsNil)

	// 1. Generate initial key-values and add them
	keys1, values1 := generateKV(32, 1000, 0)
	keys2, values2 := generateKV(32, 1000, 1000)

	for i, key := range keys1 {
		err := tree.Add(key, values1[i])
		c.Assert(err, qt.IsNil)
	}

	// 2. Execute some Deletes and Updates
	for i := 0; i < len(keys1)/2; i++ {
		err := tree.Delete(keys1[i])
		c.Assert(err, qt.IsNil)
	}

	for i := len(keys1) / 2; i < len(keys1); i++ {
		err := tree.Update(keys1[i], values2[i])
		c.Assert(err, qt.IsNil)
	}

	// 3. Add the second set of key-values
	for i, key := range keys2 {
		err := tree.Add(key, values2[i])
		c.Assert(err, qt.IsNil)
	}

	// Store the root hash after using Add method
	addRoot, err := tree.Root()
	c.Assert(err, qt.IsNil)

	// Repeat the operations using AddBatch
	database2 := metadb.NewTest(t)
	tree2, err := NewTree(Config{Database: database2, MaxLevels: 256, HashFunction: HashFunctionBlake2b})
	c.Assert(err, qt.IsNil)

	// 1. Add initial key-values
	inv, err := tree2.AddBatch(keys1, values1)
	c.Assert(err, qt.IsNil)
	c.Assert(inv, qt.HasLen, 0)

	// 2. Execute some Deletes and Updates
	for i := 0; i < len(keys1)/2; i++ {
		err := tree2.Delete(keys1[i])
		c.Assert(err, qt.IsNil)
	}

	for i := len(keys1) / 2; i < len(keys1); i++ {
		err := tree2.Update(keys1[i], values2[i])
		c.Assert(err, qt.IsNil)
	}

	// 3. Add more key-values
	inv, err = tree2.AddBatch(keys2, values2)
	c.Assert(err, qt.IsNil)
	c.Assert(inv, qt.HasLen, 0)

	// Get root after AddBatch method
	addBatchRoot, err := tree2.Root()
	c.Assert(err, qt.IsNil)

	// Compare the root hashes of the two trees
	c.Assert(addRoot, qt.DeepEquals, addBatchRoot)
}

package arbo

import (
	"encoding/hex"
	"math/big"
	"testing"

	qt "github.com/frankban/quicktest"
	"github.com/iden3/go-merkletree/db/memory"
)

func TestVirtualTreeTestVectors(t *testing.T) {
	c := qt.New(t)

	keys := [][]byte{
		BigIntToBytes(big.NewInt(1)),
		BigIntToBytes(big.NewInt(33)),
		BigIntToBytes(big.NewInt(1234)),
		BigIntToBytes(big.NewInt(123456789)),
	}
	values := [][]byte{
		BigIntToBytes(big.NewInt(2)),
		BigIntToBytes(big.NewInt(44)),
		BigIntToBytes(big.NewInt(9876)),
		BigIntToBytes(big.NewInt(987654321)),
	}

	// check the root for different batches of leafs
	testVirtualTree(c, 10, keys[:1], values[:1])
	testVirtualTree(c, 10, keys[:2], values[:2])
	testVirtualTree(c, 10, keys[:3], values[:3])
	testVirtualTree(c, 10, keys[:4], values[:4])
}

func TestVirtualTreeRandomKeys(t *testing.T) {
	c := qt.New(t)

	// test with hardcoded values
	keys := make([][]byte, 8)
	values := make([][]byte, 8)
	keys[0], _ = hex.DecodeString("1c7c2265e368314ca58ed2e1f33a326f1220e234a566d55c3605439dbe411642")
	keys[1], _ = hex.DecodeString("2c9f0a578afff5bfa4e0992a43066460faaab9e8e500db0b16647c701cdb16bf")
	keys[2], _ = hex.DecodeString("9cb87ec67e875c61390edcd1ab517f443591047709a4d4e45b0f9ed980857b8e")
	keys[3], _ = hex.DecodeString("9b4e9e92e974a589f426ceeb4cb291dc24893513fecf8e8460992dcf52621d4d")
	keys[4], _ = hex.DecodeString("1c45cb31f2fa39ec7b9ebf0fad40e0b8296016b5ce8844ae06ff77226379d9a5")
	keys[5], _ = hex.DecodeString("d8af98bbbb585129798ae54d5eabbc9d0561d583faf1663b3a3724d15bda4ec7")
	keys[6], _ = hex.DecodeString("3cd55dbfb8f975f20a0925dfbdabe79fa2d51dd0268afbb8ba6b01de9dfcdd3c")
	keys[7], _ = hex.DecodeString("5d0a9d6d9f197c091bf054fac9cb60e11ec723d6610ed8578e617b4d46cb43d5")

	// check the root for different batches of leafs
	testVirtualTree(c, 10, keys[:1], values[:1])
	testVirtualTree(c, 10, keys, values)

	// test with random values
	nLeafs := 1024

	keys = make([][]byte, nLeafs)
	values = make([][]byte, nLeafs)
	for i := 0; i < nLeafs; i++ {
		keys[i] = randomBytes(32)
		values[i] = []byte{0}
	}

	testVirtualTree(c, 100, keys, values)
}

func testVirtualTree(c *qt.C, maxLevels int, keys, values [][]byte) {
	c.Assert(len(keys), qt.Equals, len(values))

	// normal tree, to have an expected root value
	tree, err := NewTree(memory.NewMemoryStorage(), maxLevels, HashFunctionSha256)
	c.Assert(err, qt.IsNil)
	for i := 0; i < len(keys); i++ {
		err := tree.Add(keys[i], values[i])
		c.Assert(err, qt.IsNil)
	}

	// virtual tree
	vTree := newVT(maxLevels, HashFunctionSha256)

	c.Assert(vTree.root, qt.IsNil)

	for i := 0; i < len(keys); i++ {
		err := vTree.add(0, keys[i], values[i])
		c.Assert(err, qt.IsNil)
	}

	// compute hashes, and check Root
	_, err = vTree.computeHashes()
	c.Assert(err, qt.IsNil)
	c.Assert(vTree.root.h, qt.DeepEquals, tree.root)
}

func TestVirtualTreeAddBatch(t *testing.T) {
	c := qt.New(t)

	nLeafs := 2000
	maxLevels := 100

	keys := make([][]byte, nLeafs)
	values := make([][]byte, nLeafs)
	for i := 0; i < nLeafs; i++ {
		keys[i] = randomBytes(32)
		values[i] = randomBytes(32)
	}

	// normal tree, to have an expected root value
	tree, err := NewTree(memory.NewMemoryStorage(), maxLevels, HashFunctionBlake2b)
	c.Assert(err, qt.IsNil)
	for i := 0; i < len(keys); i++ {
		err := tree.Add(keys[i], values[i])
		c.Assert(err, qt.IsNil)
	}

	// virtual tree
	vTree := newVT(maxLevels, HashFunctionBlake2b)

	c.Assert(vTree.root, qt.IsNil)

	err = vTree.addBatch(keys, values)
	c.Assert(err, qt.IsNil)

	// compute hashes, and check Root
	_, err = vTree.computeHashes()
	c.Assert(err, qt.IsNil)
	c.Assert(vTree.root.h, qt.DeepEquals, tree.root)
}

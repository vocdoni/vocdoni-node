package arbo

import (
	"encoding/hex"
	"math"
	"math/big"
	"testing"

	qt "github.com/frankban/quicktest"
	"go.vocdoni.io/dvote/db/metadb"
)

// testVirtualTree adds the given key-values and tests the vt root against the
// Tree
func testVirtualTree(c *qt.C, maxLevels int, keys, values [][]byte) {
	c.Assert(len(keys), qt.Equals, len(values))

	// normal tree, to have an expected root value
	database := metadb.NewTest(c)
	tree, err := NewTree(Config{
		Database: database, MaxLevels: maxLevels,
		HashFunction: HashFunctionSha256,
	})
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
	root, err := tree.Root()
	c.Assert(err, qt.IsNil)
	c.Assert(vTree.root.h, qt.DeepEquals, root)
}

func TestVirtualTreeTestVectors(t *testing.T) {
	c := qt.New(t)

	maxLevels := 32
	keyLen := int(math.Ceil(float64(maxLevels) / float64(8)))
	keys := [][]byte{
		BigIntToBytesLE(keyLen, big.NewInt(1)),
		BigIntToBytesLE(keyLen, big.NewInt(33)),
		BigIntToBytesLE(keyLen, big.NewInt(1234)),
		BigIntToBytesLE(keyLen, big.NewInt(123456789)),
	}
	values := [][]byte{
		BigIntToBytesLE(keyLen, big.NewInt(2)),
		BigIntToBytesLE(keyLen, big.NewInt(44)),
		BigIntToBytesLE(keyLen, big.NewInt(9876)),
		BigIntToBytesLE(keyLen, big.NewInt(987654321)),
	}

	// check the root for different batches of leafs
	testVirtualTree(c, maxLevels, keys[:1], values[:1])
	testVirtualTree(c, maxLevels, keys[:2], values[:2])
	testVirtualTree(c, maxLevels, keys[:3], values[:3])
	testVirtualTree(c, maxLevels, keys[:4], values[:4])

	// test with hardcoded values
	testvectorKeys := []string{
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

	// check the root for different batches of leafs
	testVirtualTree(c, 256, keys[:1], values[:1])
	testVirtualTree(c, 256, keys, values)
}

func TestVirtualTreeRandomKeys(t *testing.T) {
	c := qt.New(t)

	// test with random values
	nLeafs := 1024
	keys := make([][]byte, nLeafs)
	values := make([][]byte, nLeafs)
	for i := 0; i < nLeafs; i++ {
		keys[i] = randomBytes(32)
		values[i] = randomBytes(32)
	}

	testVirtualTree(c, 256, keys, values)
}

func TestVirtualTreeAddBatch(t *testing.T) {
	c := qt.New(t)

	nLeafs := 2000
	maxLevels := 256

	keys := make([][]byte, nLeafs)
	values := make([][]byte, nLeafs)
	for i := 0; i < nLeafs; i++ {
		keys[i] = randomBytes(32)
		values[i] = randomBytes(32)
	}

	// normal tree, to have an expected root value
	database := metadb.NewTest(t)
	tree, err := NewTree(Config{
		Database: database, MaxLevels: maxLevels,
		HashFunction: HashFunctionBlake2b,
	})
	c.Assert(err, qt.IsNil)
	for i := 0; i < len(keys); i++ {
		err := tree.Add(keys[i], values[i])
		c.Assert(err, qt.IsNil)
	}

	// virtual tree
	vTree := newVT(maxLevels, HashFunctionBlake2b)

	c.Assert(vTree.root, qt.IsNil)

	invalids, err := vTree.addBatch(keys, values)
	c.Assert(err, qt.IsNil)
	c.Assert(len(invalids), qt.Equals, 0)

	// compute hashes, and check Root
	_, err = vTree.computeHashes()
	c.Assert(err, qt.IsNil)
	root, err := tree.Root()
	c.Assert(err, qt.IsNil)
	c.Assert(vTree.root.h, qt.DeepEquals, root)
}

func TestVirtualTreeAddBatchFullyUsed(t *testing.T) {
	c := qt.New(t)

	vTree1 := newVT(7, HashFunctionPoseidon) // used for add one by one
	vTree2 := newVT(7, HashFunctionPoseidon) // used for addBatch

	var keys, values [][]byte
	for i := 0; i < 128; i++ {
		k := BigIntToBytesLE(1, big.NewInt(int64(i)))
		v := k

		keys = append(keys, k)
		values = append(values, v)

		// add one by one expecting no error
		err := vTree1.add(0, k, v)
		c.Assert(err, qt.IsNil)
	}

	invalids, err := vTree2.addBatch(keys, values)
	c.Assert(err, qt.IsNil)
	c.Assert(0, qt.Equals, len(invalids))
}

func TestGetNodesAtLevel(t *testing.T) {
	c := qt.New(t)

	tree0 := vt{
		params: &params{
			maxLevels:    100,
			hashFunction: HashFunctionBlake2b,
			emptyHash:    make([]byte, HashFunctionBlake2b.Len()),
		},
		root: nil,
	}

	tree1 := vt{
		params: &params{
			maxLevels:    100,
			hashFunction: HashFunctionBlake2b,
			emptyHash:    make([]byte, HashFunctionBlake2b.Len()),
		},
		root: &node{
			l: &node{
				l: &node{
					k: []byte{0, 0, 0, 0},
					v: []byte{0, 0, 0, 0},
				},
				r: &node{
					k: []byte{0, 0, 0, 1},
					v: []byte{0, 0, 0, 1},
				},
			},
			r: &node{
				l: &node{
					k: []byte{0, 0, 0, 2},
					v: []byte{0, 0, 0, 2},
				},
				r: &node{
					k: []byte{0, 0, 0, 3},
					v: []byte{0, 0, 0, 3},
				},
			},
		},
	}
	// tree1.printGraphviz()

	tree2 := vt{
		params: &params{
			maxLevels:    100,
			hashFunction: HashFunctionBlake2b,
			emptyHash:    make([]byte, HashFunctionBlake2b.Len()),
		},
		root: &node{
			l: nil,
			r: &node{
				l: &node{
					l: &node{
						l: &node{
							k: []byte{0, 0, 0, 0},
							v: []byte{0, 0, 0, 0},
						},
						r: &node{
							k: []byte{0, 0, 0, 1},
							v: []byte{0, 0, 0, 1},
						},
					},
					r: &node{
						k: []byte{0, 0, 0, 2},
						v: []byte{0, 0, 0, 2},
					},
				},
				r: &node{
					k: []byte{0, 0, 0, 3},
					v: []byte{0, 0, 0, 3},
				},
			},
		},
	}
	// tree2.printGraphviz()

	tree3 := vt{
		params: &params{
			maxLevels:    100,
			hashFunction: HashFunctionBlake2b,
			emptyHash:    make([]byte, HashFunctionBlake2b.Len()),
		},
		root: &node{
			l: nil,
			r: &node{
				l: &node{
					l: &node{
						l: &node{
							k: []byte{0, 0, 0, 0},
							v: []byte{0, 0, 0, 0},
						},
						r: &node{
							k: []byte{0, 0, 0, 1},
							v: []byte{0, 0, 0, 1},
						},
					},
					r: &node{
						k: []byte{0, 0, 0, 2},
						v: []byte{0, 0, 0, 2},
					},
				},
				r: nil,
			},
		},
	}
	// tree3.printGraphviz()

	nodes0, err := tree0.getNodesAtLevel(2)
	c.Assert(err, qt.IsNil)
	c.Assert(len(nodes0), qt.DeepEquals, 4)
	c.Assert("0000", qt.DeepEquals, getNotNils(nodes0))

	nodes1, err := tree1.getNodesAtLevel(2)
	c.Assert(err, qt.IsNil)
	c.Assert(len(nodes1), qt.DeepEquals, 4)
	c.Assert("1111", qt.DeepEquals, getNotNils(nodes1))

	nodes1, err = tree1.getNodesAtLevel(3)
	c.Assert(err, qt.IsNil)
	c.Assert(len(nodes1), qt.DeepEquals, 8)
	c.Assert("00000000", qt.DeepEquals, getNotNils(nodes1))

	nodes2, err := tree2.getNodesAtLevel(2)
	c.Assert(err, qt.IsNil)
	c.Assert(len(nodes2), qt.DeepEquals, 4)
	c.Assert("0011", qt.DeepEquals, getNotNils(nodes2))

	nodes2, err = tree2.getNodesAtLevel(3)
	c.Assert(err, qt.IsNil)
	c.Assert(len(nodes2), qt.DeepEquals, 8)
	c.Assert("00001100", qt.DeepEquals, getNotNils(nodes2))

	nodes3, err := tree3.getNodesAtLevel(2)
	c.Assert(err, qt.IsNil)
	c.Assert(len(nodes3), qt.DeepEquals, 4)
	c.Assert("0010", qt.DeepEquals, getNotNils(nodes3))

	nodes3, err = tree3.getNodesAtLevel(3)
	c.Assert(err, qt.IsNil)
	c.Assert(len(nodes3), qt.DeepEquals, 8)
	c.Assert("00001100", qt.DeepEquals, getNotNils(nodes3))

	nodes3, err = tree3.getNodesAtLevel(4)
	c.Assert(err, qt.IsNil)
	c.Assert(len(nodes3), qt.DeepEquals, 16)
	c.Assert("0000000011000000", qt.DeepEquals, getNotNils(nodes3))
}

func getNotNils(nodes []*node) string {
	s := ""
	for i := 0; i < len(nodes); i++ {
		if nodes[i] == nil {
			s += "0"
		} else {
			s += "1"
		}
	}
	return s
}

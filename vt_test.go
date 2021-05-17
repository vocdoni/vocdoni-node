package arbo

import (
	"math/big"
	"testing"

	qt "github.com/frankban/quicktest"
)

func TestVirtualTree(t *testing.T) {
	c := qt.New(t)
	vTree := newVT(10, HashFunctionSha256)

	c.Assert(vTree.root, qt.IsNil)

	k := BigIntToBytes(big.NewInt(1))
	v := BigIntToBytes(big.NewInt(2))
	err := vTree.add(k, v)
	c.Assert(err, qt.IsNil)

	// check values
	c.Assert(vTree.root.k, qt.DeepEquals, k)
	c.Assert(vTree.root.v, qt.DeepEquals, v)

	// compute hashes
	pairs, err := vTree.computeHashes()
	c.Assert(err, qt.IsNil)
	c.Assert(len(pairs), qt.Equals, 1)

	rootBI := BytesToBigInt(vTree.root.h)
	c.Assert(rootBI.String(), qt.Equals,
		"46910109172468462938850740851377282682950237270676610513794735904325820156367")

	k = BigIntToBytes(big.NewInt(33))
	v = BigIntToBytes(big.NewInt(44))
	err = vTree.add(k, v)
	c.Assert(err, qt.IsNil)

	// compute hashes
	pairs, err = vTree.computeHashes()
	c.Assert(err, qt.IsNil)
	c.Assert(len(pairs), qt.Equals, 8)

	// err = vTree.printGraphviz()
	// c.Assert(err, qt.IsNil)

	rootBI = BytesToBigInt(vTree.root.h)
	c.Assert(rootBI.String(), qt.Equals,
		"59481735341404520835410489183267411392292882901306595567679529387376287440550")
	c.Assert(err, qt.IsNil)
}

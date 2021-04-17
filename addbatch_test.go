package arbo

import (
	"fmt"
	"math/big"
	"testing"
	"time"

	qt "github.com/frankban/quicktest"
	"github.com/iden3/go-merkletree/db/memory"
)

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

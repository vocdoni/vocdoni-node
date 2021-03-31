package arbo

import (
	"encoding/hex"
	"fmt"
	"math/big"
	"testing"

	"github.com/iden3/go-merkletree/db/memory"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAddTestVectors(t *testing.T) {
	// Poseidon test vectors generated using https://github.com/iden3/circomlib smt.js
	testVectorsPoseidon := []string{
		"0000000000000000000000000000000000000000000000000000000000000000",
		"13578938674299138072471463694055224830892726234048532520316387704878000008795",
		"5412393676474193513566895793055462193090331607895808993925969873307089394741",
		"14204494359367183802864593755198662203838502594566452929175967972147978322084",
	}
	testAdd(t, HashFunctionPoseidon, testVectorsPoseidon)

	testVectorsSha256 := []string{
		"0000000000000000000000000000000000000000000000000000000000000000",
		"46910109172468462938850740851377282682950237270676610513794735904325820156367",
		"59481735341404520835410489183267411392292882901306595567679529387376287440550",
		"20573794434149960984975763118181266662429997821552560184909083010514790081771",
	}
	testAdd(t, HashFunctionSha256, testVectorsSha256)
}

func testAdd(t *testing.T, hashFunc HashFunction, testVectors []string) {
	tree, err := NewTree(memory.NewMemoryStorage(), 10, hashFunc)
	assert.Nil(t, err)
	defer tree.db.Close()
	assert.Equal(t, testVectors[0], hex.EncodeToString(tree.Root()))

	err = tree.Add(
		BigIntToBytes(big.NewInt(1)),
		BigIntToBytes(big.NewInt(2)))
	assert.Nil(t, err)
	rootBI := BytesToBigInt(tree.Root())
	assert.Equal(t, testVectors[1], rootBI.String())

	err = tree.Add(
		BigIntToBytes(big.NewInt(33)),
		BigIntToBytes(big.NewInt(44)))
	assert.Nil(t, err)
	rootBI = BytesToBigInt(tree.Root())
	assert.Equal(t, testVectors[2], rootBI.String())

	err = tree.Add(
		BigIntToBytes(big.NewInt(1234)),
		BigIntToBytes(big.NewInt(9876)))
	assert.Nil(t, err)
	rootBI = BytesToBigInt(tree.Root())
	assert.Equal(t, testVectors[3], rootBI.String())
}

func TestAdd1000(t *testing.T) {
	tree, err := NewTree(memory.NewMemoryStorage(), 100, HashFunctionPoseidon)
	require.Nil(t, err)

	defer tree.db.Close()
	for i := 0; i < 1000; i++ {
		k := BigIntToBytes(big.NewInt(int64(i)))
		v := BigIntToBytes(big.NewInt(0))
		if err := tree.Add(k, v); err != nil {
			t.Fatal(err)
		}
	}

	rootBI := BytesToBigInt(tree.Root())
	assert.Equal(t,
		"296519252211642170490407814696803112091039265640052570497930797516015811235",
		rootBI.String())
}

func TestAddDifferentOrder(t *testing.T) {
	tree1, err := NewTree(memory.NewMemoryStorage(), 100, HashFunctionPoseidon)
	require.Nil(t, err)

	defer tree1.db.Close()
	for i := 0; i < 16; i++ {
		k := SwapEndianness(big.NewInt(int64(i)).Bytes())
		v := SwapEndianness(big.NewInt(0).Bytes())
		if err := tree1.Add(k, v); err != nil {
			t.Fatal(err)
		}
	}

	tree2, err := NewTree(memory.NewMemoryStorage(), 100, HashFunctionPoseidon)
	require.Nil(t, err)
	defer tree2.db.Close()
	for i := 16 - 1; i >= 0; i-- {
		k := big.NewInt(int64(i)).Bytes()
		v := big.NewInt(0).Bytes()
		if err := tree2.Add(k, v); err != nil {
			t.Fatal(err)
		}
	}

	assert.Equal(t, hex.EncodeToString(tree1.Root()), hex.EncodeToString(tree2.Root()))
	assert.Equal(t,
		"3b89100bec24da9275c87bc188740389e1d5accfc7d88ba5688d7fa96a00d82f",
		hex.EncodeToString(tree1.Root()))
}

func TestAddRepeatedIndex(t *testing.T) {
	tree, err := NewTree(memory.NewMemoryStorage(), 100, HashFunctionPoseidon)
	require.Nil(t, err)
	defer tree.db.Close()
	k := big.NewInt(int64(3)).Bytes()
	v := big.NewInt(int64(12)).Bytes()
	if err := tree.Add(k, v); err != nil {
		t.Fatal(err)
	}
	err = tree.Add(k, v)
	assert.NotNil(t, err)
	assert.Equal(t, fmt.Errorf("max virtual level 100"), err)
}

func TestAux(t *testing.T) {
	tree, err := NewTree(memory.NewMemoryStorage(), 100, HashFunctionPoseidon)
	require.Nil(t, err)
	defer tree.db.Close()
	k := BigIntToBytes(big.NewInt(int64(1)))
	v := BigIntToBytes(big.NewInt(int64(0)))
	err = tree.Add(k, v)
	assert.Nil(t, err)
	k = BigIntToBytes(big.NewInt(int64(256)))
	err = tree.Add(k, v)
	assert.Nil(t, err)

	k = BigIntToBytes(big.NewInt(int64(257)))
	err = tree.Add(k, v)
	assert.Nil(t, err)

	k = BigIntToBytes(big.NewInt(int64(515)))
	err = tree.Add(k, v)
	assert.Nil(t, err)
	k = BigIntToBytes(big.NewInt(int64(770)))
	err = tree.Add(k, v)
	assert.Nil(t, err)
}

func TestGenProofAndVerify(t *testing.T) {
	tree, err := NewTree(memory.NewMemoryStorage(), 100, HashFunctionPoseidon)
	require.Nil(t, err)

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
	assert.Nil(t, err)

	k = BigIntToBytes(big.NewInt(int64(7)))
	v := BigIntToBytes(big.NewInt(int64(14)))
	verif, err := CheckProof(tree.hashFunction, k, v, tree.Root(), siblings)
	require.Nil(t, err)
	assert.True(t, verif)
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
	tree, err := NewTree(memory.NewMemoryStorage(), 140, hashFunc)
	require.Nil(b, err)

	defer tree.db.Close()
	for i := 0; i < len(ks); i++ {
		if err := tree.Add(ks[i], vs[i]); err != nil {
			b.Fatal(err)
		}
	}
}

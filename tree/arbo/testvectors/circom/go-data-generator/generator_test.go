package main

import (
	"encoding/json"
	"math/big"
	"os"
	"testing"

	qt "github.com/frankban/quicktest"
	"go.vocdoni.io/dvote/db/metadb"
	"go.vocdoni.io/dvote/tree/arbo"
)

func TestGenerator(t *testing.T) {
	c := qt.New(t)
	database := metadb.NewTest(t)
	tree, err := arbo.NewTree(arbo.Config{
		Database: database, MaxLevels: 4,
		HashFunction: arbo.HashFunctionPoseidon,
	})
	c.Assert(err, qt.IsNil)

	testVector := [][]int64{
		{1, 11},
		{2, 22},
		{3, 33},
		{4, 44},
	}
	bLen := 1
	for i := 0; i < len(testVector); i++ {
		k := arbo.BigIntToBytes(bLen, big.NewInt(testVector[i][0]))
		v := arbo.BigIntToBytes(bLen, big.NewInt(testVector[i][1]))
		if err := tree.Add(k, v); err != nil {
			t.Fatal(err)
		}
	}

	// proof of existence
	k := arbo.BigIntToBytes(bLen, big.NewInt(int64(2)))
	cvp, err := tree.GenerateCircomVerifierProof(k)
	c.Assert(err, qt.IsNil)
	jCvp, err := json.Marshal(cvp)
	c.Assert(err, qt.IsNil)
	// store the data into a file that will be used at the circom test
	err = os.WriteFile("go-smt-verifier-inputs.json", jCvp, 0o600)
	c.Assert(err, qt.IsNil)

	// proof of non-existence
	k = arbo.BigIntToBytes(bLen, big.NewInt(int64(5)))
	cvp, err = tree.GenerateCircomVerifierProof(k)
	c.Assert(err, qt.IsNil)
	jCvp, err = json.Marshal(cvp)
	c.Assert(err, qt.IsNil)
	// store the data into a file that will be used at the circom test
	err = os.WriteFile("go-smt-verifier-non-existence-inputs.json", jCvp, 0o600)
	c.Assert(err, qt.IsNil)
}

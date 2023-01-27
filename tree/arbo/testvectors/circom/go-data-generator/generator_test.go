package main

import (
	"encoding/json"
	"io/ioutil"
	"math/big"
	"testing"

	qt "github.com/frankban/quicktest"
	"go.vocdoni.io/dvote/db"
	"go.vocdoni.io/dvote/db/badgerdb"
	"go.vocdoni.io/dvote/tree/arbo"
)

func TestGenerator(t *testing.T) {
	c := qt.New(t)
	database, err := badgerdb.New(db.Options{Path: c.TempDir()})
	c.Assert(err, qt.IsNil)
	tree, err := arbo.NewTree(arbo.Config{Database: database, MaxLevels: 4,
		HashFunction: arbo.HashFunctionPoseidon})
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
	err = ioutil.WriteFile("go-smt-verifier-inputs.json", jCvp, 0600)
	c.Assert(err, qt.IsNil)

	// proof of non-existence
	k = arbo.BigIntToBytes(bLen, big.NewInt(int64(5)))
	cvp, err = tree.GenerateCircomVerifierProof(k)
	c.Assert(err, qt.IsNil)
	jCvp, err = json.Marshal(cvp)
	c.Assert(err, qt.IsNil)
	// store the data into a file that will be used at the circom test
	err = ioutil.WriteFile("go-smt-verifier-non-existence-inputs.json", jCvp, 0600)
	c.Assert(err, qt.IsNil)
}

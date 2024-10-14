package arbo

import (
	"encoding/json"
	"math/big"
	"testing"

	qt "github.com/frankban/quicktest"
	"go.vocdoni.io/dvote/db/metadb"
)

func TestCircomVerifierProof(t *testing.T) {
	c := qt.New(t)
	database := metadb.NewTest(t)
	tree, err := NewTree(Config{
		Database: database, MaxLevels: 4,
		HashFunction: HashFunctionPoseidon,
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
		k := BigIntToBytesLE(bLen, big.NewInt(testVector[i][0]))
		v := BigIntToBytesLE(bLen, big.NewInt(testVector[i][1]))
		if err := tree.Add(k, v); err != nil {
			t.Fatal(err)
		}
	}

	// proof of existence
	k := BigIntToBytesLE(bLen, big.NewInt(int64(2)))
	cvp, err := tree.GenerateCircomVerifierProof(k)
	c.Assert(err, qt.IsNil)
	jCvp, err := json.Marshal(cvp)
	c.Assert(err, qt.IsNil)
	// test vector checked with a circom circuit (arbo/testvectors/circom)
	c.Assert(string(jCvp), qt.Equals, `{"fnc":0,"isOld0":"0","key":"2","oldK`+
		`ey":"0","oldValue":"0","root":"1355816845522055904274785395894906304622`+
		`6645447188878859760119761585093422436","siblings":["1162013050763544193`+
		`2056895853942898236773847390796721536119314875877874016518","5158240518`+
		`874928563648144881543092238925265313977134167935552944620041388700","0"`+
		`,"0"],"value":"22"}`)

	// proof of non-existence
	k = BigIntToBytesLE(bLen, big.NewInt(int64(5)))
	cvp, err = tree.GenerateCircomVerifierProof(k)
	c.Assert(err, qt.IsNil)
	jCvp, err = json.Marshal(cvp)
	c.Assert(err, qt.IsNil)
	// test vector checked with a circom circuit (arbo/testvectors/circom)
	c.Assert(string(jCvp), qt.Equals, `{"fnc":1,"isOld0":"0","key":"5","oldK`+
		`ey":"1","oldValue":"11","root":"135581684552205590427478539589490630462`+
		`26645447188878859760119761585093422436","siblings":["756056982086999933`+
		`1905412009838015295115276841209205575174464206730109811365","1276103081`+
		`3800436751877086580591648324911598798716611088294049841213649313596","0`+
		`","0"],"value":"11"}`)
}

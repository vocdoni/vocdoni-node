package circuit

import (
	"encoding/hex"
	"math/big"
	"testing"

	"go.vocdoni.io/dvote/crypto/ethereum"
	"go.vocdoni.io/dvote/tree/arbo"
	"go.vocdoni.io/dvote/util"

	qt "github.com/frankban/quicktest"
)

const (
	testElectionId = "c5d2460186f760d51371516148fd334b4199052f01538553aa9a020200000000"
	testRoot       = "21f20a61be6bb9415b777367989313a2640109990d187e397fa74256361f0e11"
)

var testSiblings = []string{"580248495380568564123114759404848148595751514101110571080218449954471889552", "633773650715998492339991508741183573324294111862264796224228057688802848801", "4989687640539066742397958643613126020089632025064585620245735039080884277747", "3863931244154987782138626341091989752721559524607603934673141845608721144566", "4688361268219000735189072482478199217683515606859266889918935760406393255791", "13548759886643907861771470850568634627625204202812868538547476494298706407763", "2714600331130374821301177119574534122901226309274343707259485383900427657102", "11263980701448890785172384726839875656945350242204041401041770089765910047533", "0", "0", "0", "0", "0", "0", "0", "0", "0", "0", "0", "0", "0", "0", "0", "0", "0", "0", "0", "0", "0", "0", "0", "0", "0", "0", "0", "0", "0", "0", "0", "0", "0", "0", "0", "0", "0", "0", "0", "0", "0", "0", "0", "0", "0", "0", "0", "0", "0", "0", "0", "0", "0", "0", "0", "0", "0", "0", "0", "0", "0", "0", "0", "0", "0", "0", "0", "0", "0", "0", "0", "0", "0", "0", "0", "0", "0", "0", "0", "0", "0", "0", "0", "0", "0", "0", "0", "0", "0", "0", "0", "0", "0", "0", "0", "0", "0", "0", "0", "0", "0", "0", "0", "0", "0", "0", "0", "0", "0", "0", "0", "0", "0", "0", "0", "0", "0", "0", "0", "0", "0", "0", "0", "0", "0", "0", "0", "0", "0", "0", "0", "0", "0", "0", "0", "0", "0", "0", "0", "0", "0", "0", "0", "0", "0", "0", "0", "0", "0", "0", "0", "0", "0"}

func TestGenerateCircuitInput(t *testing.T) {
	c := qt.New(t)

	// decode the election ID
	electionId, err := hex.DecodeString(testElectionId)
	c.Assert(err, qt.IsNil)
	// mock voter account and vote nullifier
	acc := ethereum.NewSignKeys()
	c.Assert(acc.Generate(), qt.IsNil)
	nullifier, err := acc.AccountSIKnullifier(electionId, nil)
	c.Assert(err, qt.IsNil)
	// mock the availableWeight
	availableWeight := new(big.Int).SetUint64(10)
	testVotePackage := util.RandomBytes(16)
	// calc vote hash
	voteHash := util.BytesToArboSplitStr(testVotePackage)
	// decode the test root
	hexTestRoot, err := hex.DecodeString(testRoot)
	c.Assert(err, qt.IsNil)
	// mock user signature
	signature, err := acc.SIKsignature()
	c.Assert(err, qt.IsNil)
	// mock expected circuit inputs
	expected := &CircuitInputs{
		ElectionId:      []string{"18517551409637235305922365793037451371", "135271561984151624501280044000043030166"},
		Nullifier:       new(big.Int).SetBytes(nullifier).String(),
		AvailableWeight: availableWeight.String(),
		VoteHash:        voteHash,
		SIKRoot:         "7714269703880573582519379213888374390024853519732158909852028066903886590497",
		CensusRoot:      "7714269703880573582519379213888374390024853519732158909852028066903886590497",

		Address:   arbo.BytesToBigInt(acc.Address().Bytes()).String(),
		Password:  "0",
		Signature: util.BigToFF(new(big.Int).SetBytes(signature)).String(),

		VoteWeight:     availableWeight.String(),
		CensusSiblings: testSiblings,
		SIKSiblings:    testSiblings,
	}
	// Generate correct inputs
	rawInputs, err := GenerateCircuitInput(CircuitInputsParameters{
		acc, nil,
		electionId, hexTestRoot, hexTestRoot, testVotePackage, testSiblings, testSiblings, nil,
		availableWeight,
	})
	c.Assert(err, qt.IsNil)
	c.Assert(rawInputs, qt.DeepEquals, expected)

	rawInputs, err = GenerateCircuitInput(CircuitInputsParameters{
		acc, nil,
		electionId, hexTestRoot, hexTestRoot, testVotePackage, testSiblings, testSiblings,
		big.NewInt(1), availableWeight,
	})
	c.Assert(err, qt.IsNil)
	c.Assert(rawInputs, qt.Not(qt.DeepEquals), expected)

	_, err = GenerateCircuitInput(CircuitInputsParameters{
		nil, nil, electionId,
		hexTestRoot, hexTestRoot, testVotePackage, testSiblings, testSiblings, big.NewInt(1),
		availableWeight,
	})
	c.Assert(err, qt.IsNotNil)
}

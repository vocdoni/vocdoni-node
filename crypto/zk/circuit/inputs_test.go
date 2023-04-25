package circuit

import (
	"encoding/hex"
	"math/big"
	"testing"

	qt "github.com/frankban/quicktest"
	"go.vocdoni.io/dvote/crypto/zk"
)

const (
	testAccountPrivateKey = "6430ab787ad5130942369901498a118fade013ebab5450efbfb6acac66d8fb88"
	testElectionId        = "c5d2460186f760d51371516148fd334b4199052f01538553aa9a020200000000"
	testCensusRoot        = "21f20a61be6bb9415b777367989313a2640109990d187e397fa74256361f0e11"
)

var testCensusSiblings = []string{"580248495380568564123114759404848148595751514101110571080218449954471889552", "633773650715998492339991508741183573324294111862264796224228057688802848801", "4989687640539066742397958643613126020089632025064585620245735039080884277747", "3863931244154987782138626341091989752721559524607603934673141845608721144566", "4688361268219000735189072482478199217683515606859266889918935760406393255791", "13548759886643907861771470850568634627625204202812868538547476494298706407763", "2714600331130374821301177119574534122901226309274343707259485383900427657102", "11263980701448890785172384726839875656945350242204041401041770089765910047533", "0", "0", "0", "0", "0", "0", "0", "0", "0", "0", "0", "0", "0", "0", "0", "0", "0", "0", "0", "0", "0", "0", "0", "0", "0", "0", "0", "0", "0", "0", "0", "0", "0", "0", "0", "0", "0", "0", "0", "0", "0", "0", "0", "0", "0", "0", "0", "0", "0", "0", "0", "0", "0", "0", "0", "0", "0", "0", "0", "0", "0", "0", "0", "0", "0", "0", "0", "0", "0", "0", "0", "0", "0", "0", "0", "0", "0", "0", "0", "0", "0", "0", "0", "0", "0", "0", "0", "0", "0", "0", "0", "0", "0", "0", "0", "0", "0", "0", "0", "0", "0", "0", "0", "0", "0", "0", "0", "0", "0", "0", "0", "0", "0", "0", "0", "0", "0", "0", "0", "0", "0", "0", "0", "0", "0", "0", "0", "0", "0", "0", "0", "0", "0", "0", "0", "0", "0", "0", "0", "0", "0", "0", "0", "0", "0", "0", "0", "0", "0", "0", "0", "0", "0"}

func TestGenerateCircuitInput(t *testing.T) {
	c := qt.New(t)
	// Decode correctly the census root
	censusRoot, err := hex.DecodeString(testCensusRoot)
	c.Assert(err, qt.IsNil)
	// Decode correctly the election ID
	electionId, err := hex.DecodeString(testElectionId)
	c.Assert(err, qt.IsNil)
	// Generate the ZkAddress from the seed
	zkAddr, err := zk.AddressFromString(testAccountPrivateKey)
	c.Assert(err, qt.IsNil)
	// Instance the correct vote weight
	factoryWeight := new(big.Int).SetUint64(10)
	// Instance expected inputs
	expected := &CircuitInputs{
		CensusRoot:     "7714269703880573582519379213888374390024853519732158909852028066903886590497",
		CensusSiblings: testCensusSiblings,
		VoteWeight:     "10",
		FactoryWeight:  "10",
		PrivateKey:     "6735248701457559886126785742277482466576784161746903995071090348762482970571",
		VoteHash:       []string{"242108076058607163538102198631955675649", "142667662805314151155817304537028292174"},
		ProcessId:      []string{"18517551409637235305922365793037451371", "135271561984151624501280044000043030166"},
		Nullifier:      "13830839320176376721270728875863016529251254252806875185281289627544884475042",
	}
	// Generate correct inputs
	rawInputs, err := GenerateCircuitInput(zkAddr, censusRoot, electionId, factoryWeight, factoryWeight, testCensusSiblings)
	c.Assert(err, qt.IsNil)
	c.Assert(rawInputs, qt.DeepEquals, expected)

	rawInputs, err = GenerateCircuitInput(zkAddr, censusRoot, electionId, new(big.Int).SetInt64(1), factoryWeight, testCensusSiblings)
	c.Assert(err, qt.IsNil)
	c.Assert(rawInputs, qt.Not(qt.DeepEquals), expected)

	_, err = GenerateCircuitInput(zkAddr, nil, electionId, new(big.Int).SetInt64(1), factoryWeight, testCensusSiblings)
	c.Assert(err, qt.IsNotNil)
}

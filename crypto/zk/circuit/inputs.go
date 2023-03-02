package circuit

import (
	"fmt"
	"math/big"

	"go.vocdoni.io/dvote/crypto/zk"
	"go.vocdoni.io/dvote/tree/arbo"
)

// CircuitInputs wraps all the necessary circuit parameters. They must all be
// strings or slices of strings, and the struct must be able to be encoded in
// json.
type CircuitInputs struct {
	CensusRoot     string   `json:"censusRoot"`
	CensusSiblings []string `json:"censusSiblings"`
	VotingWeight   string   `json:"votingWeight"`
	FactoryWeight  string   `json:"factoryWeight"`
	PrivateKey     string   `json:"privateKey"`
	VoteHash       []string `json:"voteHash"`
	ProcessId      []string `json:"processId"`
	Nullifier      string   `json:"nullifier"`
}

// GenerateCircuitInput receives the required parameters to encode them
// correctly to return the expected CircuitInputs. This function uses the
// ZkAddress to get the private key and generates the nullifier. Also encodes
// the census root, the election id and the weight provided, and includes the
// census siblings provided into the result.
func GenerateCircuitInput(zkAddr *zk.ZkAddress, censusRoot, electionId []byte,
	factoryWeight, votingWeight *big.Int, censusSiblings []string) (*CircuitInputs, error) {
	if zkAddr == nil || censusRoot == nil || electionId == nil ||
		factoryWeight == nil || votingWeight == nil || len(censusSiblings) == 0 {
		return nil, fmt.Errorf("bad arguments provided")
	}

	// get nullifier
	nullifier, err := zkAddr.Nullifier(electionId)
	if err != nil {
		return nil, fmt.Errorf("error generating the nullifier: %w", err)
	}

	return &CircuitInputs{
		// Encode census root
		CensusRoot:     arbo.BytesToBigInt(censusRoot).String(),
		CensusSiblings: censusSiblings,
		VotingWeight:   votingWeight.String(),
		FactoryWeight:  factoryWeight.String(),
		PrivateKey:     zkAddr.PrivKey.String(),
		// Encode weight into voteHash
		VoteHash: zk.BytesToArboStr(factoryWeight.Bytes()),
		// Encode electionId
		ProcessId: zk.BytesToArboStr(electionId),
		Nullifier: nullifier.String(),
	}, nil
}

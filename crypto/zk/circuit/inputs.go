package circuit

import (
	"encoding/json"
	"fmt"
	"math/big"

	"go.vocdoni.io/dvote/crypto/ethereum"
	"go.vocdoni.io/dvote/crypto/zk"
	"go.vocdoni.io/dvote/tree/arbo"
)

// CircuitInputs wraps all the necessary circuit parameters. They must all be
// strings or slices of strings, and the struct must be able to be encoded in
// json.
type CircuitInputs struct {
	// Public inputs
	ElectionId      []string `json:"electionId"`
	Nullifier       string   `json:"nullifier"`
	AvailableWeight string   `json:"availableWeight"`
	VoteHash        []string `json:"voteHash"`
	SikRoot         string   `json:"sikRoot"`
	CensusRoot      string   `json:"censusRoot"`

	// Private inputs
	Address   string `json:"address"`
	Password  string `json:"password"`
	Signature string `json:"signature"`

	VoteWeight     string   `json:"voteWeight"`
	CensusSiblings []string `json:"censusSiblings"`
	SikSiblings    []string `json:"sikSiblings"`
}

func (ci *CircuitInputs) String() string {
	bstr, _ := json.Marshal(ci)
	return string(bstr)
}

// GenerateCircuitInput receives the required parameters to encode them
// correctly to return the expected CircuitInputs. This function uses the
// ZkAddress to get the private key and generates the nullifier. Also encodes
// the census root, the election id and the weight provided, and includes the
// census siblings provided into the result.
func GenerateCircuitInput(account *ethereum.SignKeys, password, electionId,
	censusRoot, sikRoot []byte, censusSiblings, sikSiblings []string,
	voteWeight, availableWeight *big.Int) (*CircuitInputs, error) {
	if account == nil || electionId == nil || censusRoot == nil || sikRoot == nil ||
		availableWeight == nil || len(censusSiblings) == 0 || len(sikSiblings) == 0 {
		return nil, fmt.Errorf("bad arguments provided")
	}
	if voteWeight == nil {
		voteWeight = availableWeight
	}

	nullifier, err := account.Nullifier(electionId, password)
	if err != nil {
		return nil, fmt.Errorf("error generating nullifier: %w", err)
	}
	ffPassword := new(big.Int)
	if password != nil {
		ffPassword = zk.BigToFF(new(big.Int).SetBytes(password))
	}
	signature, err := account.SignEthereum([]byte(ethereum.DefaultSikPayload))
	if err != nil {
		return nil, err
	}

	return &CircuitInputs{
		ElectionId:      zk.BytesToArboStr(electionId),
		Nullifier:       new(big.Int).SetBytes(nullifier).String(),
		AvailableWeight: availableWeight.String(),
		VoteHash:        zk.BytesToArboStr(availableWeight.Bytes()),
		SikRoot:         arbo.BytesToBigInt(sikRoot).String(),
		CensusRoot:      arbo.BytesToBigInt(censusRoot).String(),

		Address:   arbo.BytesToBigInt(account.Address().Bytes()).String(),
		Password:  ffPassword.String(),
		Signature: zk.BigToFF(new(big.Int).SetBytes(signature)).String(),

		VoteWeight:     voteWeight.String(),
		CensusSiblings: censusSiblings,
		SikSiblings:    sikSiblings,
	}, nil
}

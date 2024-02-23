package circuit

import (
	"encoding/json"
	"fmt"
	"math/big"

	"go.vocdoni.io/dvote/crypto/ethereum"
	"go.vocdoni.io/dvote/tree/arbo"
	"go.vocdoni.io/dvote/util"
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
	SIKRoot         string   `json:"sikRoot"`
	CensusRoot      string   `json:"censusRoot"`

	// Private inputs
	Address   string `json:"address"`
	Password  string `json:"password"`
	Signature string `json:"signature"`

	VoteWeight     string   `json:"voteWeight"`
	CensusSiblings []string `json:"censusSiblings"`
	SIKSiblings    []string `json:"sikSiblings"`
}

func (ci *CircuitInputs) String() string {
	bstr, _ := json.Marshal(ci)
	return string(bstr)
}

// CircuitInputsParameters struct envolves all the parameters to generate the
// inputs for the current ZK circuit.
type CircuitInputsParameters struct {
	Account         *ethereum.SignKeys
	Password        []byte
	ElectionId      []byte
	CensusRoot      []byte
	SIKRoot         []byte
	VotePackage     []byte
	CensusSiblings  []string
	SIKSiblings     []string
	VoteWeight      *big.Int
	AvailableWeight *big.Int
}

// GenerateCircuitInput receives the required parameters to encode them
// correctly to return the expected CircuitInputs. This function uses the
// ZkAddress to get the private key and generates the nullifier. Also encodes
// the census root, the election id and the weight provided, and includes the
// census siblings provided into the result.
func GenerateCircuitInput(p CircuitInputsParameters) (*CircuitInputs, error) {
	if p.Account == nil || p.ElectionId == nil || p.CensusRoot == nil || p.SIKRoot == nil ||
		p.AvailableWeight == nil || len(p.VotePackage) == 0 || len(p.CensusSiblings) == 0 ||
		len(p.SIKSiblings) == 0 {
		return nil, fmt.Errorf("bad arguments provided")
	}
	if p.VoteWeight == nil {
		p.VoteWeight = p.AvailableWeight
	}

	nullifier, err := p.Account.AccountSIKnullifier(p.ElectionId, p.Password)
	if err != nil {
		return nil, fmt.Errorf("error generating nullifier: %w", err)
	}
	ffPassword := new(big.Int)
	if p.Password != nil {
		ffPassword = util.BigToFF(new(big.Int).SetBytes(p.Password))
	}
	signature, err := p.Account.SIKsignature()
	if err != nil {
		return nil, err
	}
	return &CircuitInputs{
		ElectionId:      util.BytesToArboSplitStr(p.ElectionId),
		Nullifier:       new(big.Int).SetBytes(nullifier).String(),
		AvailableWeight: p.AvailableWeight.String(),
		VoteHash:        util.BytesToArboSplitStr(p.VotePackage),
		SIKRoot:         arbo.BytesToBigInt(p.SIKRoot).String(),
		CensusRoot:      arbo.BytesToBigInt(p.CensusRoot).String(),

		Address:   arbo.BytesToBigInt(p.Account.Address().Bytes()).String(),
		Password:  ffPassword.String(),
		Signature: util.BigToFF(new(big.Int).SetBytes(signature)).String(),

		VoteWeight:     p.VoteWeight.String(),
		CensusSiblings: p.CensusSiblings,
		SIKSiblings:    p.SIKSiblings,
	}, nil
}

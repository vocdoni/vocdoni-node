package client

import (
	"go.vocdoni.io/dvote/crypto/zk/artifacts"
)

type SNARKProofCircom struct {
	A []string   `json:"pi_a"`
	B [][]string `json:"pi_b"`
	C []string   `json:"pi_c"`
	// PublicInputs []string // onl
}

type SNARKProof struct {
	A            []string
	B            []string
	C            []string
	PublicInputs []string // only nullifier
}

type SNARKProofInputs struct {
	CensusRoot     string   `json:"censusRoot"`
	CensusSiblings []string `json:"censusSiblings"`
	Index          string   `json:"index"`
	SecretKey      string   `json:"secretKey"`
	VoteHash       []string `json:"voteHash"`
	ProcessID      []string `json:"processId"`
	Nullifier      string   `json:"nullifier"`
}

type GenSNARKData struct {
	CircuitIndex  int                      `json:"circuitIndex"`
	CircuitConfig *artifacts.CircuitConfig `json:"circuitConfig"`
	Inputs        SNARKProofInputs         `json:"inputs"`
}

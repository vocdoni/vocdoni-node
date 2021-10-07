// Package zk provides utilities around the zkSNARK (Groth16) tooling.
package zk

import (
	"fmt"
	"math/big"
	"os"

	"github.com/vocdoni/go-snark/parsers"
	"github.com/vocdoni/go-snark/types"
	models "go.vocdoni.io/proto/build/go/models"
)

func LoadVkFromFile(path string) (*types.Vk, error) {
	vkJSON, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	vk, err := parsers.ParseVk(vkJSON)
	if err != nil {
		return nil, err
	}
	return vk, nil
}

func ProtobufZKProofToCircomProof(p *models.ProofZkSNARK) (*types.Proof, []*big.Int, error) {
	if len(p.A) != 3 && len(p.B) != 6 && len(p.C) != 3 {
		return nil, nil, fmt.Errorf("error on zkProof format")
	}
	proofString := parsers.ProofString{
		A: p.A,
		B: [][]string{
			{
				p.B[0],
				p.B[1],
			},
			{
				p.B[2],
				p.B[3],
			},
			{
				p.B[4],
				p.B[5],
			},
		},
		C: p.C,
	}
	publicInputsString := p.PublicInputs

	// parse zkProof & PublicInputs from tx.Proof
	proof, err := parsers.ProofStringToProof(proofString)
	if err != nil {
		return nil, nil, err
	}
	publicInputs, err := parsers.PublicSignalsStringToPublicSignals(publicInputsString)
	if err != nil {
		return nil, nil, err
	}

	return proof, publicInputs, nil
}

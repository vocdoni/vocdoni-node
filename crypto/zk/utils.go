// Package zk provides utilities around the zkSNARK (Groth16) tooling.
package zk

import (
	"crypto/sha256"
	"fmt"
	"math/big"
	"os"

	"github.com/vocdoni/arbo"
	"github.com/vocdoni/go-snark/parsers"
	"github.com/vocdoni/go-snark/types"
	"go.vocdoni.io/dvote/crypto/zk/prover"
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
	if len(p.A) != 3 || len(p.B) != 6 || len(p.C) != 3 {
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

func ProtobufZKProofToProverProof(p *models.ProofZkSNARK) (*prover.Proof, error) {
	if len(p.A) != 3 || len(p.B) != 6 || len(p.C) != 3 {
		return nil, fmt.Errorf("error on zkProof format")
	}

	return &prover.Proof{
		Data: prover.ProofData{
			A: p.A,
			B: [][]string{
				{p.B[0], p.B[1]},
				{p.B[2], p.B[3]},
				{p.B[4], p.B[5]},
			},
			C: p.C,
		},
		PubSignals: p.PublicInputs,
	}, nil
}

func GetZKProofPublicSignals(vote *models.VoteEnvelope, process *models.Process, weight *big.Int) []string {
	pubInputs := []string{}

	// 1. processId {2}
	pubInputs = append(pubInputs, arbo.BytesToBigInt(process.ProcessId[:16]).String())
	pubInputs = append(pubInputs, arbo.BytesToBigInt(process.ProcessId[16:]).String())

	// 2. censusRoot {1} -> Getting process.CensusRoot instead process.RollingCensusRoot
	pubInputs = append(pubInputs, arbo.BytesToBigInt(process.CensusRoot).String())

	// 3. nullifier {1}
	pubInputs = append(pubInputs, arbo.BytesToBigInt(vote.Nullifier).String())

	// 4. weight {1} -> Getting from models.VoteEnvelope
	pubInputs = append(pubInputs, weight.String())

	// 5. voteHash {2}
	voteHash := sha256.Sum256(vote.VotePackage)
	pubInputs = append(pubInputs, arbo.BytesToBigInt(voteHash[:16]).String())
	pubInputs = append(pubInputs, arbo.BytesToBigInt(voteHash[16:]).String())

	return pubInputs
}

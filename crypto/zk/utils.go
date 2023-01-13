// Package zk provides utilities around the zkSNARK (Groth16) tooling.
package zk

import (
	"crypto/sha256"
	"fmt"
	"math/big"

	"github.com/vocdoni/arbo"
	"go.vocdoni.io/dvote/crypto/zk/prover"
	"go.vocdoni.io/dvote/types"
	models "go.vocdoni.io/proto/build/go/models"
)

// func LoadVkFromFile(path string) (*types.Vk, error) {
// 	vkJSON, err := os.ReadFile(path)
// 	if err != nil {
// 		return nil, err
// 	}

// 	vk, err := parsers.ParseVk(vkJSON)
// 	if err != nil {
// 		return nil, err
// 	}
// 	return vk, nil
// }

// func ProtobufZKProofToCircomProof(p *models.ProofZkSNARK) (*types.Proof, []*big.Int, error) {
// 	if len(p.A) != 3 || len(p.B) != 6 || len(p.C) != 3 {
// 		return nil, nil, fmt.Errorf("error on zkProof format")
// 	}
// 	proofString := parsers.ProofString{
// 		A: p.A,
// 		B: [][]string{
// 			{
// 				p.B[0],
// 				p.B[1],
// 			},
// 			{
// 				p.B[2],
// 				p.B[3],
// 			},
// 			{
// 				p.B[4],
// 				p.B[5],
// 			},
// 		},
// 		C: p.C,
// 	}
// 	publicInputsString := p.PublicInputs

// 	// parse zkProof & PublicInputs from tx.Proof
// 	proof, err := parsers.ProofStringToProof(proofString)
// 	if err != nil {
// 		return nil, nil, err
// 	}
// 	publicInputs, err := parsers.PublicSignalsStringToPublicSignals(publicInputsString)
// 	if err != nil {
// 		return nil, nil, err
// 	}

// 	return proof, publicInputs, nil
// }

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

func ProverProofToProtobufZKProof(index int32, p *prover.Proof,
	electionId, censusRoot, nullifier types.HexBytes, weight *big.Int) (*models.ProofZkSNARK, error) {
	proof := &models.ProofZkSNARK{
		CircuitParametersIndex: index,
		A:                      p.Data.A,
		B: []string{
			p.Data.B[0][0], p.Data.B[0][1],
			p.Data.B[1][0], p.Data.B[1][1],
			p.Data.B[2][0], p.Data.B[2][1],
		},
		C: p.Data.C,
	}

	if electionId != nil && censusRoot != nil && nullifier != nil && weight != nil {
		proof.PublicInputs = GetZKProofPublicSignals(electionId, censusRoot, nullifier, weight)
	}

	return proof, nil
}

func GetZKProofPublicSignals(electionId, censusRoot, nullifier types.HexBytes, weight *big.Int) []string {
	pubInputs := []string{}

	// 1. processId {2}
	pubInputs = append(pubInputs, arbo.BytesToBigInt(electionId[:16]).String())
	pubInputs = append(pubInputs, arbo.BytesToBigInt(electionId[16:]).String())

	// 2. censusRoot {1} -> Getting process.CensusRoot instead process.RollingCensusRoot
	pubInputs = append(pubInputs, arbo.BytesToBigInt(censusRoot).String())

	// 3. nullifier {1}
	pubInputs = append(pubInputs, arbo.BytesToBigInt(nullifier).String())

	// 4. weight {1} -> Getting from models.VoteEnvelope
	pubInputs = append(pubInputs, weight.String())

	// 5. voteHash {2}
	voteHash := sha256.Sum256(weight.Bytes())
	pubInputs = append(pubInputs, arbo.BytesToBigInt(voteHash[:16]).String())
	pubInputs = append(pubInputs, arbo.BytesToBigInt(voteHash[16:]).String())

	return pubInputs
}

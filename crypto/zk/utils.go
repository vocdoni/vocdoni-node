// Package zk provides utilities around the zkSNARK (Groth16) tooling.
package zk

import (
	"crypto/sha256"
	"fmt"
	"math/big"

	"go.vocdoni.io/dvote/crypto/zk/prover"
	"go.vocdoni.io/dvote/tree/arbo"
	"go.vocdoni.io/dvote/types"
	models "go.vocdoni.io/proto/build/go/models"
)

// ProtobufZKProofToProverProof function parses the provided protobuf ready
// proof struct into a prover ready proof struct.
func ProtobufZKProofToProverProof(p *models.ProofZkSNARK) (*prover.Proof, error) {
	if len(p.A) != 3 || len(p.B) != 6 || len(p.C) != 3 {
		return nil, fmt.Errorf("wrong ZkSnark protobuf format")
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

// ProverProofToProtobufZKProof function encodes the proof provided into a
// protobuf ready struct using including the index of the circuit used. If the
// provided proof does not contains a defined public signals, the rest of the
// arguments are required to calculate that parameter. If the provided proof
// does not contains a defined public signals and any of the rest of the
// parameters is nil, the resulting struct will not contains any defined
// PublicInputs value.
func ProverProofToProtobufZKProof(p *prover.Proof,
	electionId, censusRoot, nullifier types.HexBytes, weight *big.Int) (*models.ProofZkSNARK, error) {
	if len(p.Data.A) != 3 || len(p.Data.B) != 3 || len(p.Data.C) != 3 {
		return nil, fmt.Errorf("wrong ZkSnark prover proof format")
	}

	proof := &models.ProofZkSNARK{
		A: p.Data.A,
		B: []string{
			p.Data.B[0][0], p.Data.B[0][1],
			p.Data.B[1][0], p.Data.B[1][1],
			p.Data.B[2][0], p.Data.B[2][1],
		},
		C: p.Data.C,
	}

	if p.PubSignals != nil && len(p.PubSignals) > 0 {
		if len(p.PubSignals) != 7 {
			return nil, fmt.Errorf("wrong ZkSnark prover public signals format")
		}

		proof.PublicInputs = p.PubSignals
	} else if electionId != nil && censusRoot != nil && nullifier != nil && weight != nil {
		proof.PublicInputs = zkProofPublicInputs(electionId, censusRoot, nullifier, weight)
	} else {
		return nil, fmt.Errorf("no enought arguments to generate the calc signals")
	}

	return proof, nil
}

// zkProofPublicInputs encodes the provided parameters in the correct order and
// codification into a slice of string arbo compatible.
func zkProofPublicInputs(electionId, censusRoot, nullifier types.HexBytes, weight *big.Int) []string {
	pubInputs := []string{}

	// 1. [2]processId
	pubInputs = append(pubInputs, arbo.BytesToBigInt(electionId[:16]).String())
	pubInputs = append(pubInputs, arbo.BytesToBigInt(electionId[16:]).String())

	// 2. censusRoot
	pubInputs = append(pubInputs, arbo.BytesToBigInt(censusRoot).String())

	// 3. nullifier
	pubInputs = append(pubInputs, arbo.BytesToBigInt(nullifier).String())

	// 4. weight
	pubInputs = append(pubInputs, weight.String())

	// 5. [2]voteHash
	voteHash := sha256.Sum256(weight.Bytes())
	pubInputs = append(pubInputs, arbo.BytesToBigInt(voteHash[:16]).String())
	pubInputs = append(pubInputs, arbo.BytesToBigInt(voteHash[16:]).String())

	return pubInputs
}

// LittleEndianToNBytes truncate the most significant n bytes of the provided
// little endian number provided and returns into a new big.Int.
func LittleEndianToNBytes(num *big.Int, n int) *big.Int {
	// To take the n most significant bytes of a little endian number its needed
	// to discard the first m bytes, where m = len(numBytes) - n
	numBytes := num.Bytes()
	m := len(numBytes) - n
	return new(big.Int).SetBytes(numBytes[m:])
}

// BytesToArboStr function calculates the sha256 hash (32 bytes) of the slice
// of bytes provided. Then, splits the hash into a two parts of 16 bytes, swap
// the endianess of that parts, encodes they into a two big.Ints and return both
// as strings into a []string.
func BytesToArboStr(input []byte) []string {
	hash := sha256.Sum256(input)
	return []string{
		new(big.Int).SetBytes(arbo.SwapEndianness(hash[:16])).String(),
		new(big.Int).SetBytes(arbo.SwapEndianness(hash[16:])).String(),
	}
}

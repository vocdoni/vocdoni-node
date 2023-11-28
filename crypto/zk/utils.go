// Package zk provides utilities around the zkSNARK (Groth16) tooling.
package zk

import (
	"crypto/sha256"
	"fmt"
	"math/big"

	"go.vocdoni.io/dvote/censustree"
	"go.vocdoni.io/dvote/crypto/zk/circuit"
	"go.vocdoni.io/dvote/crypto/zk/prover"
	"go.vocdoni.io/dvote/tree/arbo"
	"go.vocdoni.io/dvote/types"
	"go.vocdoni.io/proto/build/go/models"
)

// The proof structure is the following:
// 	{
// 		A: [3]bigint,
// 		B: [3][2]bigint,
// 		C: [3]bigint,
// 	}

// Default length of each proof parameters
const (
	proofALen    = 3
	proofBLen    = 6 // flatted
	proofBEncLen = 3 // matrix
	proofCLen    = 3
)

// ProtobufZKProofToProverProof parses the provided protobuf ready proof struct
// into a prover ready proof struct.
func ProtobufZKProofToProverProof(p *models.ProofZkSNARK) (*prover.Proof, error) {
	if len(p.A) != proofALen || len(p.B) != proofBLen || len(p.C) != proofCLen {
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

// ProverProofToProtobufZKProof encodes the proof provided into a protobuf ready
// struct using including the index of the circuit used. If the provided proof
// does not contains a defined public signals, the rest of the arguments are
// required to calculate that parameter. If the provided proof does not contains
// a defined public signals and any of the rest of the parameters is nil, the
// resulting struct will not contains any defined PublicInputs value.
func ProverProofToProtobufZKProof(p *prover.Proof, electionId, sikRoot,
	censusRoot, nullifier types.HexBytes, voteWeight *big.Int) (*models.ProofZkSNARK, error) {
	if len(p.Data.A) != proofALen || len(p.Data.B) != proofBEncLen || len(p.Data.C) != proofCLen {
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

	// if public signals are provided, check their format
	proof.PublicInputs = p.PubSignals
	if p.PubSignals != nil && len(p.PubSignals) != len(circuit.Global().Config.PublicSignals) {
		return nil, fmt.Errorf("wrong ZkSnark prover public signals format")
	}
	// if not, check if the rest of the arguments are provided and try to
	// generate the correct public signals
	if p.PubSignals == nil {
		if electionId == nil || sikRoot == nil || censusRoot == nil || nullifier == nil || voteWeight == nil {
			return nil, fmt.Errorf("not enough arguments to generate the public signals")
		}
		proof.PublicInputs = zkProofPublicInputs(electionId, sikRoot, censusRoot, nullifier, voteWeight)
	}
	return proof, nil
}

// zkProofPublicInputs encodes the provided parameters in the correct order and
// codification into a slice of string arbo compatible.
func zkProofPublicInputs(electionId, sikRoot, censusRoot, nullifier types.HexBytes, voteWeight *big.Int) []string {
	pubInputs := []string{}
	// 0. electionId[0]
	pubInputs = append(pubInputs, arbo.BytesToBigInt(electionId[:16]).String())
	// 1. electionId[1]
	pubInputs = append(pubInputs, arbo.BytesToBigInt(electionId[16:]).String())
	// 2. nullifier
	pubInputs = append(pubInputs, arbo.BytesToBigInt(nullifier).String())
	voteHash := sha256.Sum256(voteWeight.Bytes())
	// 3. voteHash[0]
	pubInputs = append(pubInputs, arbo.BytesToBigInt(voteHash[:16]).String())
	// 4. voteHash[1]
	pubInputs = append(pubInputs, arbo.BytesToBigInt(voteHash[16:]).String())
	// 5. sikRoot
	pubInputs = append(pubInputs, arbo.BytesToBigInt(sikRoot).String())
	// 6. censusRoot
	pubInputs = append(pubInputs, arbo.BytesToBigInt(censusRoot).String())
	// 7. availableWeight
	pubInputs = append(pubInputs, voteWeight.String())

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

// ProofToCircomSiblings function decodes the provided proof (or packaged
// siblings) and and encodes any contained siblings for use in a Circom circuit.
func ProofToCircomSiblings(proof []byte) ([]string, error) {
	rawSiblings, err := arbo.UnpackSiblings(arbo.HashFunctionPoseidon, proof)
	if err != nil {
		return nil, err
	}
	siblings := make([]string, censustree.DefaultMaxLevels+1)
	for i := 0; i < len(siblings); i++ {
		if i < len(rawSiblings) {
			siblings[i] = arbo.BytesToBigInt(rawSiblings[i]).String()
		} else {
			siblings[i] = "0"
		}
	}
	return siblings, nil
}

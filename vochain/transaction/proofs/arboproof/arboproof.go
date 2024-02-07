package arboproof

import (
	"fmt"
	"math/big"

	"go.vocdoni.io/dvote/censustree"
	"go.vocdoni.io/dvote/tree"
	"go.vocdoni.io/dvote/tree/arbo"
	"go.vocdoni.io/dvote/vochain/state"
	"go.vocdoni.io/proto/build/go/models"
)

var bigOne = big.NewInt(1)

// ProofVerifierArbo defines the interface for Arbo merkle-tree based proof verification systems.
type ProofVerifierArbo struct{}

// Verify verifies a proof with census origin OFF_CHAIN_TREE.
// Returns verification result and weight.
func (*ProofVerifierArbo) Verify(process *models.Process, envelope *models.VoteEnvelope, vID state.VoterID) (bool, *big.Int, error) {
	proof := envelope.Proof
	switch proof.Payload.(type) {
	case *models.Proof_Arbo:
		p := proof.GetArbo()
		if p == nil {
			return false, nil, fmt.Errorf("arbo proof is empty")
		}
		// get the merkle tree hashing function
		var hashFunc arbo.HashFunction = arbo.HashFunctionBlake2b
		switch p.Type {
		case models.ProofArbo_BLAKE2B:
			hashFunc = arbo.HashFunctionBlake2b
		case models.ProofArbo_POSEIDON:
			hashFunc = arbo.HashFunctionPoseidon
		default:
			return false, nil, fmt.Errorf("not recognized ProofArbo type: %s", p.Type)
		}
		if vID == nil {
			return false, nil, fmt.Errorf("voterID is nil")
		}
		// check if the proof key is for an address (default) or a pubKey
		var err error
		key := vID.Address()
		// TODO (lucasmenendez): Remove hashing of the address
		if p.Type != models.ProofArbo_POSEIDON {
			if key, err = hashFunc.Hash(key); err != nil {
				return false, nil, fmt.Errorf("cannot hash proof key: %w", err)
			}
			if len(key) > censustree.DefaultMaxKeyLen {
				key = key[:censustree.DefaultMaxKeyLen]
			}
		}
		valid, err := tree.VerifyProof(hashFunc, key, p.AvailableWeight, p.Siblings, process.CensusRoot)
		if !valid || err != nil {
			return false, nil, err
		}
		// Legacy: support p.LeafWeight == nil, assume then value=1
		if p.AvailableWeight == nil {
			return true, bigOne, nil
		}

		availableWeight := arbo.BytesToBigInt(p.AvailableWeight)
		if p.VoteWeight == nil {
			return true, availableWeight, nil
		}

		voteWeight := arbo.BytesToBigInt(p.VoteWeight)
		if voteWeight.Cmp(availableWeight) == 1 {
			return false, nil, fmt.Errorf("assigned weight exceeded")
		}
		return true, voteWeight, nil
	default:
		return false, nil, fmt.Errorf("unexpected proof.Payload type: %T",
			proof.Payload)
	}
}

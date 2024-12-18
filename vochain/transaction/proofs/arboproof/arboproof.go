package arboproof

import (
	"fmt"
	"math/big"

	"go.vocdoni.io/dvote/censustree"
	"go.vocdoni.io/dvote/crypto/ethereum"
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

		availableWeight := arbo.BytesLEToBigInt(p.AvailableWeight)
		if p.VoteWeight == nil {
			return true, availableWeight, nil
		}

		voteWeight := arbo.BytesLEToBigInt(p.VoteWeight)
		if voteWeight.Cmp(availableWeight) == 1 {
			return false, nil, fmt.Errorf("assigned weight exceeded")
		}
		return true, voteWeight, nil
	default:
		return false, nil, fmt.Errorf("unexpected proof.Payload type: %T",
			proof.Payload)
	}
}

// InitializeSignedVote initializes a signed vote. It does not check the proof nor includes the weight of the vote.
func InitializeSignedVote(voteEnvelope *models.VoteEnvelope, signedBody, signature []byte, height uint32) (*state.Vote, error) {
	// Create a new vote object with the provided parameters
	vote := &state.Vote{
		Height:               height,
		ProcessID:            voteEnvelope.ProcessId,
		VotePackage:          voteEnvelope.VotePackage,
		EncryptionKeyIndexes: voteEnvelope.EncryptionKeyIndexes,
	}

	// Check if the proof is nil or invalid
	if voteEnvelope.Proof == nil {
		return nil, fmt.Errorf("proof not found on transaction")
	}
	if voteEnvelope.Proof.Payload == nil {
		return nil, fmt.Errorf("invalid proof payload provided")
	}

	// Check if the signature or signed body is nil
	if signature == nil || signedBody == nil {
		return nil, fmt.Errorf("nil signature or body provided")
	}

	// Extract the public key from the signature
	pubKey, err := ethereum.PubKeyFromSignature(signedBody, signature)
	if err != nil {
		return nil, fmt.Errorf("cannot extract public key from signature: %w", err)
	}

	// Generate the voter ID and assign it to the vote
	vote.VoterID = append([]byte{state.VoterIDTypeECDSA}, pubKey...)

	// Extract the address from the public key and assign a nullifier to the vote
	addr, err := ethereum.AddrFromPublicKey(pubKey)
	if err != nil {
		return nil, fmt.Errorf("cannot extract address from public key: %w", err)
	}
	vote.Nullifier = state.GenerateNullifier(addr, vote.ProcessID)

	// Return the initialized vote object
	return vote, nil
}

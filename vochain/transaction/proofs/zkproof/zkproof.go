package zkproof

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"math/big"

	"go.vocdoni.io/dvote/crypto/zk"
	"go.vocdoni.io/dvote/crypto/zk/circuit"
	"go.vocdoni.io/dvote/crypto/zk/prover"
	"go.vocdoni.io/dvote/log"
	"go.vocdoni.io/dvote/vochain/state"
	"go.vocdoni.io/proto/build/go/models"
)

// ProofVerifierZk defines the interface for Zk proof verification systems.
type ProofVerifierZk struct{}

// Verify verifies a proof with census origin ZK. It returns the voting weight included in the proof.
// Note that SIK root is not verified here, the caller should verify it separately.
func (*ProofVerifierZk) Verify(process *models.Process, envelope *models.VoteEnvelope, _ state.VoterID) (bool, *big.Int, error) {
	if !circuit.IsLoaded() {
		return false, nil, fmt.Errorf("anonymous voting not supported, missing zk circuits data")
	}
	// get snark proof from vote envelope
	proof, err := proofFromEnvelope(envelope)
	if err != nil {
		return false, nil, err
	}
	// verify the process id
	proofProcessID, err := proof.ElectionID()
	if err != nil {
		return false, nil, fmt.Errorf("failed on parsing process id from public inputs provided: %w", err)
	}
	hashedPid := sha256.Sum256(process.ProcessId)
	if !bytes.Equal(hashedPid[:], proofProcessID) {
		log.Debugw("process id mismatch",
			"processID", hex.EncodeToString(process.ProcessId),
			"proofProcessID", hex.EncodeToString(proofProcessID))
		return false, nil, fmt.Errorf("process id mismatch %x != %x", process.ProcessId, proofProcessID)
	}
	// verify the census root
	proofCensusRoot, err := proof.CensusRoot()
	if err != nil {
		return false, nil, fmt.Errorf("failed on parsing census root from public inputs provided: %w", err)
	}
	if !bytes.Equal(process.CensusRoot, proofCensusRoot) {
		return false, nil, fmt.Errorf("census root mismatch")
	}
	// verify the votePackage hash
	proofVoteHash, err := proof.VoteHash()
	if err != nil {
		return false, nil, fmt.Errorf("failed on parsing vote hash from public inputs provided: %w", err)
	}
	hashedVotePackage := sha256.Sum256(envelope.VotePackage)
	if !bytes.Equal(hashedVotePackage[:], proofVoteHash) {
		log.Debugw("process id mismatch",
			"votPackage", hex.EncodeToString(envelope.VotePackage),
			"proofVotePackage", hex.EncodeToString(proofVoteHash))
		return false, nil, fmt.Errorf("vote hash mismatch")
	}
	// get vote weight from proof publicSignals
	weight, err := proof.VoteWeight()
	if err != nil {
		return false, nil, fmt.Errorf("failed on parsing vote weight from public inputs provided: %w", err)
	}

	// verify the proof with the circuit verification key
	if err := proof.Verify(circuit.Global().VerificationKey); err != nil {
		return false, nil, fmt.Errorf("zkSNARK proof verification failed: %w", err)
	}
	return true, weight, nil
}

// proofFromEnvelope returns the parsed ZkProof from the vote envelope.
func proofFromEnvelope(voteEnvelope *models.VoteEnvelope) (*prover.Proof, error) {
	proofZkSNARK := voteEnvelope.Proof.GetZkSnark()
	if proofZkSNARK == nil {
		return nil, fmt.Errorf("zkSNARK proof is empty")
	}
	// parse the ZkProof protobuf to prover.Proof
	proof, err := zk.ProtobufZKProofToProverProof(proofZkSNARK)
	if err != nil {
		return nil, fmt.Errorf("failed on zk.ProtobufZKProofToCircomProof: %w", err)
	}
	return proof, nil
}

// InitializeZkVote initializes a zkSNARK vote. It does not check the proof nor includes the weight of the vote.
func InitializeZkVote(voteEnvelope *models.VoteEnvelope, height uint32) (*state.Vote, []byte, error) {
	proof, err := proofFromEnvelope(voteEnvelope)
	if err != nil {
		return nil, nil, err
	}
	nullifier, err := proof.Nullifier()
	if err != nil {
		return nil, nil, fmt.Errorf("failed on parsing nullifier from public inputs: %w", err)
	}
	sikRoot, err := proof.SIKRoot()
	if err != nil {
		return nil, nil, fmt.Errorf("failed on getting sik root from the proof: %w", err)
	}
	return &state.Vote{
		Height:               height,
		ProcessID:            voteEnvelope.ProcessId,
		VotePackage:          voteEnvelope.VotePackage,
		Nullifier:            nullifier,
		EncryptionKeyIndexes: voteEnvelope.EncryptionKeyIndexes,
	}, sikRoot, nil
}

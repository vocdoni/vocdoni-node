package transaction

import (
	"bytes"
	"fmt"
	"math/big"

	"go.vocdoni.io/dvote/log"
	"go.vocdoni.io/dvote/vochain/state"
	"go.vocdoni.io/dvote/vochain/transaction/proofs/arboproof"
	"go.vocdoni.io/dvote/vochain/transaction/proofs/cspproof"
	"go.vocdoni.io/dvote/vochain/transaction/proofs/ethereumproof"
	"go.vocdoni.io/dvote/vochain/transaction/proofs/farcasterproof"
	"go.vocdoni.io/dvote/vochain/transaction/proofs/zkproof"

	"go.vocdoni.io/proto/build/go/models"
)

// ProofVerifier defines the interface for proof verification systems.
type ProofVerifier interface {
	Verify(process *models.Process, envelope *models.VoteEnvelope, vID state.VoterID) (bool, *big.Int, error)
}

// VerifyProof is a wrapper over all VerifyProofFunc(s) available which uses the process.CensusOrigin
// to execute the correct verification function.
func VerifyProof(process *models.Process, envelope *models.VoteEnvelope, vID state.VoterID) (bool, *big.Int, error) {
	// sanity checks
	if envelope == nil {
		return false, nil, fmt.Errorf("envelope is nil")
	}
	if envelope.Proof == nil {
		return false, nil, fmt.Errorf("proof is nil")
	}
	if process == nil || process.CensusRoot == nil {
		return false, nil, fmt.Errorf("process or census root are nil")
	}
	if !bytes.Equal(process.ProcessId, envelope.ProcessId) {
		return false, nil, fmt.Errorf("processID does not match")
	}
	if process.EnvelopeType == nil {
		return false, nil, fmt.Errorf("envelope type is nil")
	}

	log.Debugw("verify proof",
		"censusOrigin", process.CensusOrigin,
		"electionID", fmt.Sprintf("%x", process.ProcessId),
		"voterID", fmt.Sprintf("%x", vID),
		"address", fmt.Sprintf("%x", vID.Address()),
		"censusRoot", fmt.Sprintf("%x", process.CensusRoot),
	)

	// select the correct proof verifier
	var verifier ProofVerifier
	switch process.CensusOrigin {
	case models.CensusOrigin_OFF_CHAIN_TREE, models.CensusOrigin_OFF_CHAIN_TREE_WEIGHTED:
		if process.EnvelopeType.Anonymous {
			verifier = &zkproof.ProofVerifierZk{}
		} else {
			verifier = &arboproof.ProofVerifierArbo{}
		}
	case models.CensusOrigin_OFF_CHAIN_CA:
		verifier = &cspproof.ProofVerifierCSP{}
	case models.CensusOrigin_ERC20, models.CensusOrigin_MINI_ME:
		verifier = &ethereumproof.ProofVerifierEthereumStorage{}
	case models.CensusOrigin_FARCASTER_FRAME:
		verifier = &farcasterproof.FarcasterVerifier{}
	default:
		return false, nil, fmt.Errorf("census origin not compatible")
	}
	// execute the verification
	valid, weight, err := verifier.Verify(process, envelope, vID)
	if err != nil {
		return false, nil, fmt.Errorf("proof not valid: %w", err)
	}
	return valid, weight, nil
}

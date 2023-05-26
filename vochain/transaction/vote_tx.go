package transaction

import (
	"errors"
	"fmt"

	"go.vocdoni.io/dvote/crypto/ethereum"
	"go.vocdoni.io/dvote/crypto/zk"
	"go.vocdoni.io/dvote/log"
	vstate "go.vocdoni.io/dvote/vochain/state"
	"go.vocdoni.io/dvote/vochain/transaction/vochaintx"
	"go.vocdoni.io/proto/build/go/models"
)

// VoteTxCheck performs basic checks on a vote transaction.
func (t *TransactionHandler) VoteTxCheck(vtx *vochaintx.Tx, forCommit bool) (*vstate.Vote, error) {
	// Get the vote envelope from the transaction
	voteEnvelope := vtx.Tx.GetVote()
	if voteEnvelope == nil {
		return nil, fmt.Errorf("vote envelope is nil")
	}

	// Perform basic checks on the vote envelope
	if len(voteEnvelope.ProcessId) == 0 {
		return nil, fmt.Errorf("voteEnvelope.ProcessId is empty")
	}

	// Get the process associated with the vote
	process, err := t.state.Process(voteEnvelope.ProcessId, false)
	if err != nil {
		return nil, fmt.Errorf("cannot fetch processId: %w", err)
	}

	// Check that the process is not malformed
	if process == nil || process.EnvelopeType == nil || process.Mode == nil {
		return nil, fmt.Errorf("process %x malformed", voteEnvelope.ProcessId)
	}

	// Get the current height of the blockchain
	height := t.state.CurrentHeight()

	// Calculate the end block of the process
	endBlock := process.StartBlock + process.BlockCount

	// Check that the current height is within the bounds of the process
	if height < process.StartBlock {
		return nil, fmt.Errorf(
			"process %x starts at height %d, current height is %d",
			voteEnvelope.ProcessId, process.StartBlock, height)
	} else if height > endBlock {
		return nil, fmt.Errorf(
			"process %x finished at height %d, current height is %d",
			voteEnvelope.ProcessId, endBlock, height)
	}

	// Check that the process is in the READY state
	if process.Status != models.ProcessStatus_READY {
		return nil, fmt.Errorf(
			"process %x not in READY state - current state: %s",
			voteEnvelope.ProcessId, process.Status.String())
	}

	// Check if keys are required for encrypted votes and if they have been sent by a keykeeper
	if process.EnvelopeType.EncryptedVotes &&
		process.KeyIndex != nil &&
		*process.KeyIndex < 1 {
		return nil, fmt.Errorf("no keys available, voting is not possible")
	}

	// Check if the vote is already in the cache
	// In order to avoid double vote check (on checkTx and deliverTx), we use a memory vote cache.
	// An element can only be added to the vote cache during checkTx.
	// Every N seconds the old votes which are not yet in the blockchain will be removoteEnveloped from cache.
	// If the same vote (but different transaction) is send to the mempool, the cache will detect it
	// and vote will be discarted.
	//
	// We use CacheGetCopy because we will modify the vote to set
	// the Height.  If we don't work with a copy we are racing with
	// concurrent reads to the votes in the cache which happen in
	// in State.CachePurge run via a goroutine in
	// started in BaseApplication.BeginBlock.
	// Warning: vote cache might change during the execution of this function.
	vote := t.state.CacheGetCopy(vtx.TxID)
	fromCache := vote != nil

	if fromCache { // if the vote exists in cache
		// if not forCommit, it is a mempool check,
		// reject it since we already processed the transaction before.
		if !forCommit {
			return nil, ErrorAlreadyExistInCache
		}
		defer t.state.CacheDel(vtx.TxID)
		vote.Height = height // update vote height
	} else { // if vote not in cache, initialize it
		// Initialize the vote based on the envelope type
		if process.GetEnvelopeType().Anonymous {
			vote = initializeZkVote(voteEnvelope, height)
		} else {
			vote, err = initializeSignedVote(voteEnvelope, vtx.SignedBody, vtx.Signature, height)
			if err != nil {
				return nil, err
			}
		}

		// if process encrypted, check the vote is encrypted (includes at least one key index)
		if process.EnvelopeType.EncryptedVotes && len(vote.EncryptionKeyIndexes) == 0 {
			return nil, fmt.Errorf("no key indexes provided on vote package")
		}
	}

	// Check if the vote is valid for the current state
	isOverwrite, err := t.checkVoteCanBeCasted(vote.Nullifier, process)
	if err != nil {
		return nil, err
	}

	// Check if maxCensusSize is reached
	votesCount, err := t.state.CountVotes(process.ProcessId, false)
	if err != nil {
		return nil, fmt.Errorf("cannot count votes: %w", err)
	}
	// if maxCensusSize is reached, we should check if the vote is an overwrite
	if votesCount >= process.GetMaxCensusSize() && !isOverwrite {
		return nil, fmt.Errorf("maxCensusSize reached %d/%d", votesCount, process.GetMaxCensusSize())
	}

	// if vote was from cache, we already checked the proof, so we can return
	if fromCache {
		return vote, nil
	}

	// Verify the proof associated with the vote
	if process.EnvelopeType.Anonymous {
		if t.ZkCircuit == nil {
			return nil, fmt.Errorf("anonymous voting not supported, missing zk circuits data")
		}

		// Supports Groth16 proof generated from circom snark compatible
		// provoteEnveloper
		proofZkSNARK := voteEnvelope.Proof.GetZkSnark()
		if proofZkSNARK == nil {
			return nil, fmt.Errorf("zkSNARK proof is empty")
		}

		// Parse the ZkProof protobuf to prover.Proof
		proof, err := zk.ProtobufZKProofToProverProof(proofZkSNARK)
		if err != nil {
			return nil, fmt.Errorf("failed on zk.ProtobufZKProofToCircomProof: %w", err)
		}

		// Get vote weight from proof publicSignals
		vote.Weight, err = proof.Weight()
		if err != nil {
			return nil, fmt.Errorf("failed on parsing vote weight from public inputs provided: %w", err)
		}

		log.Infow("new vote",
			"type", "zkSNARK",
			"nullifier", fmt.Sprintf("%x", voteEnvelope.Nullifier),
			"electionID", fmt.Sprintf("%x", voteEnvelope.ProcessId),
		)

		// Get valid verification key and verify the proof parsed
		if err := proof.Verify(t.ZkCircuit.VerificationKey); err != nil {
			return nil, fmt.Errorf("zkSNARK proof verification failed")
		}

	} else { // Signature based voting
		// extract the ethereum address from the voterID
		addr := ethereum.AddrFromBytes(vote.VoterID.Address())

		// if not in cache, full check
		log.Infow("new vote",
			"type", "signature",
			"nullifier", fmt.Sprintf("%x", vote.Nullifier),
			"address", addr.Hex(),
			"electionID", fmt.Sprintf("%x", voteEnvelope.ProcessId),
			"height", height)

		// Verify the proof
		valid, weight, err := VerifyProof(process, voteEnvelope.Proof,
			process.CensusOrigin, process.CensusRoot, process.ProcessId,
			vote.VoterID)
		if err != nil {
			return nil, err
		}
		if !valid {
			return nil, fmt.Errorf("merkle proof verification failed")
		}
		vote.Weight = weight
	}

	// If not forCommit, add the vote to the cache
	if !forCommit {
		t.state.CacheAdd(vtx.TxID, vote)
	}
	return vote, nil
}

// initializeZkVote initializes a zkSNARK vote. It does not check the proof nor includes the weight of the vote.
func initializeZkVote(voteEnvelope *models.VoteEnvelope, height uint32) *vstate.Vote {
	return &vstate.Vote{
		Height:               height,
		ProcessID:            voteEnvelope.ProcessId,
		VotePackage:          voteEnvelope.VotePackage,
		Nullifier:            voteEnvelope.Nullifier,
		EncryptionKeyIndexes: voteEnvelope.EncryptionKeyIndexes,
	}
}

// initializeSignedVote initializes a signed vote. It does not check the proof nor includes the weight of the vote.
func initializeSignedVote(voteEnvelope *models.VoteEnvelope,
	signedBody, signature []byte, height uint32) (*vstate.Vote, error) {
	// Create a new vote object with the provided parameters
	vote := &vstate.Vote{
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
	vote.VoterID = append([]byte{vstate.VoterIDTypeECDSA}, pubKey...)

	// Extract the address from the public key and assign a nullifier to the vote
	addr, err := ethereum.AddrFromPublicKey(pubKey)
	if err != nil {
		return nil, fmt.Errorf("cannot extract address from public key: %w", err)
	}
	vote.Nullifier = vstate.GenerateNullifier(addr, vote.ProcessID)

	// Return the initialized vote object
	return vote, nil
}

// checkVoteCanBeCasted checks if a vote can be added to a process, either because it is new or
// because it is a valid overwrite.  Returns error if the vote cannot be casted. Returns true if
// the vote is an overwrite (however error must be also checked).
func (t *TransactionHandler) checkVoteCanBeCasted(nullifier []byte, process *models.Process) (bool, error) {
	// get the vote from the state to check if it exists
	stateVote, err := t.state.Vote(process.ProcessId, nullifier, false)
	if err != nil {
		// if vote does not exist, it is ok
		if errors.Is(err, vstate.ErrVoteNotFound) {
			return false, nil
		}
		return false, fmt.Errorf("error fetching vote %x: %w", nullifier, err)
	}
	// if vote exists, check if it has reached the max overwrite count
	if stateVote.OverwriteCount == nil {
		// if overwrite count is nil, it means it is the first overwrite, we set it to 0
		stateVote.OverwriteCount = new(uint32)
	}
	if *stateVote.OverwriteCount >= process.VoteOptions.MaxVoteOverwrites {
		return true, fmt.Errorf("vote %x overwrite count reached", nullifier)
	}
	return true, nil
}

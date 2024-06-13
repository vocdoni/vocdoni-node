package transaction

import (
	"encoding/hex"
	"errors"
	"fmt"

	"go.vocdoni.io/dvote/crypto/ethereum"
	"go.vocdoni.io/dvote/log"
	"go.vocdoni.io/dvote/util"
	vstate "go.vocdoni.io/dvote/vochain/state"
	"go.vocdoni.io/dvote/vochain/transaction/proofs/arboproof"
	"go.vocdoni.io/dvote/vochain/transaction/proofs/farcasterproof"
	"go.vocdoni.io/dvote/vochain/transaction/proofs/zkproof"
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

	// Get the current height and timestamp from the blockchain state
	height := t.state.CurrentHeight()
	currentTime, err := t.state.Timestamp(false)
	if err != nil {
		return nil, fmt.Errorf("cannot get current time: %w", err)
	}

	// Check the process accepts votes by height or timestamp window
	if process.Duration > 0 { // Timestamp based processes
		endTime := process.StartTime + process.Duration
		// Check that the current time is within the bounds of the process
		if currentTime < process.StartTime {
			return nil, fmt.Errorf(
				"process %x starts at time %s, current time is %s",
				voteEnvelope.ProcessId, util.TimestampToTime(process.StartTime).String(),
				util.TimestampToTime(currentTime).String())
		} else if currentTime > endTime {
			return nil, fmt.Errorf(
				"process %x finished at time %s, current time is %s",
				voteEnvelope.ProcessId, util.TimestampToTime(endTime).String(),
				util.TimestampToTime(currentTime).String())
		}
	} else { // Block count based processes. Remove when block count based processes are deprecated.
		log.Warnw("deprecated block count based vote detected", "process", hex.EncodeToString(process.ProcessId))
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
	// If the same vote (but different transaction) is sent to the mempool, the cache will detect it
	// and vote will be discarded.
	//
	// We use CacheGetCopy because we will modify the vote to set
	// the Height.  If we don't work with a copy we are racing with
	// concurrent reads to the votes in the cache which happen
	// in State.CachePurge run via a goroutine in
	// started in BaseApplication.BeginBlock.
	// Warning: vote cache might change during the execution of this function.
	vote := t.state.CacheGetCopy(vtx.TxID)
	sikRoot := []byte{}
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
		switch process.CensusOrigin {
		case models.CensusOrigin_OFF_CHAIN_TREE,
			models.CensusOrigin_OFF_CHAIN_TREE_WEIGHTED,
			models.CensusOrigin_OFF_CHAIN_CA,
			models.CensusOrigin_ERC20, models.CensusOrigin_MINI_ME:
			if process.GetEnvelopeType().Anonymous {
				vote, sikRoot, err = zkproof.InitializeZkVote(voteEnvelope, height)
			} else {
				vote, err = arboproof.InitializeSignedVote(voteEnvelope, vtx.SignedBody, vtx.Signature, height)
			}
		case models.CensusOrigin_FARCASTER_FRAME:
			vote, err = farcasterproof.InitializeFarcasterFrameVote(voteEnvelope, height)
		default:
			return nil, fmt.Errorf("could not initialize vote, census origin not compatible")
		}
		if err != nil {
			return nil, err
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

	// verify the proof associated with the vote
	switch {
	case process.EnvelopeType.Anonymous:
		// check if it is expired
		if t.state.ExpiredSIKRoot(sikRoot) {
			return nil, fmt.Errorf("expired sik root provided, generate the proof again")
		}
		// verify the proof
		valid := false
		valid, vote.Weight, err = VerifyProof(process, voteEnvelope, vote.VoterID)
		if err != nil {
			return nil, fmt.Errorf("proof not valid: %w", err)
		}
		if !valid {
			return nil, fmt.Errorf("proof not valid")
		}
		log.Debugw("new vote",
			"type", "zkSNARK",
			"weight", vote.Weight,
			"timestamp", currentTime,
			"height", height,
			"nullifier", fmt.Sprintf("%x", vote.Nullifier),
			"electionID", fmt.Sprintf("%x", voteEnvelope.ProcessId),
		)
	default:
		// Signature based voting
		// extract the ethereum address from the voterID
		addr := ethereum.AddrFromBytes(vote.VoterID.Address())

		// if not in cache, full check
		log.Debugw("new vote",
			"type", "signature",
			"nullifier", fmt.Sprintf("%x", vote.Nullifier),
			"address", addr.Hex(),
			"electionID", fmt.Sprintf("%x", voteEnvelope.ProcessId),
			"timestamp", currentTime,
			"height", height)

		// Verify the proof
		valid, weight, err := VerifyProof(process, voteEnvelope, vote.VoterID)
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

// Create a new vote object with the provided parameters
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

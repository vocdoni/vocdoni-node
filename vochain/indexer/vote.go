package indexer

import (
	"context"
	"database/sql"
	"encoding/hex"
	"errors"
	"fmt"
	"math/big"
	"time"

	"go.vocdoni.io/proto/build/go/models"

	"go.vocdoni.io/dvote/crypto/nacl"
	"go.vocdoni.io/dvote/log"
	"go.vocdoni.io/dvote/types"
	indexerdb "go.vocdoni.io/dvote/vochain/indexer/db"
	"go.vocdoni.io/dvote/vochain/indexer/indexertypes"
	"go.vocdoni.io/dvote/vochain/results"
	"go.vocdoni.io/dvote/vochain/state"
)

// ErrNoResultsYet is an error returned to indicate the process exist but
// it does not have yet reuslts.
var ErrNoResultsYet = fmt.Errorf("no results yet")

// ErrVoteNotFound is returned if the vote is not found in the indexer database.
var ErrVoteNotFound = fmt.Errorf("vote not found")

// GetEnvelopeReference gets the reference for an AddVote transaction.
// This reference can then be used to fetch the vote transaction directly from the BlockStore.
func (idx *Indexer) GetEnvelopeReference(nullifier []byte) (*indexertypes.VoteReference, error) {
	sqlStartTime := time.Now()

	queries, ctx, cancel := idx.timeoutQueries()
	defer cancel()
	sqlTxRefInner, err := queries.GetVoteReference(ctx, nullifier)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrVoteNotFound
		}
		return nil, err
	}
	log.Debugw(fmt.Sprintf("envelope reference on sqlite took %s", time.Since(sqlStartTime)),
		"nullifier", hex.EncodeToString(nullifier),
		"height", sqlTxRefInner.Height,
		"txIndex", sqlTxRefInner.TxIndex,
		"weight", sqlTxRefInner.Weight,
		"processId", hex.EncodeToString(sqlTxRefInner.ProcessID),
		"overwriteCount", sqlTxRefInner.OverwriteCount,
		"voterId", hex.EncodeToString(sqlTxRefInner.VoterID),
	)

	sqlTxRef := indexertypes.VoteReferenceFromDB(&sqlTxRefInner)
	return sqlTxRef, nil
}

// GetEnvelope retrieves an Envelope from the Blockchain block store identified by its nullifier.
// Returns the envelope and the signature (if any).
func (idx *Indexer) GetEnvelope(nullifier []byte) (*indexertypes.EnvelopePackage, error) {
	t := time.Now()
	voteRef, err := idx.GetEnvelopeReference(nullifier)
	if err != nil {
		return nil, err
	}
	vote, err := idx.App.State.Vote(voteRef.ProcessID, nullifier, true)
	if err != nil {
		return nil, ErrVoteNotFound
	}
	// TODO: get the txHash from another place, not the blockstore
	_, txHash, err := idx.App.GetTxHash(voteRef.Height, voteRef.TxIndex)
	if err != nil {
		return nil, err
	}

	log.Debugw("getEnvelope", "took", time.Since(t), "nullifier", hex.EncodeToString(nullifier))
	envelopePackage := &indexertypes.EnvelopePackage{
		VotePackage:          vote.VotePackage,
		EncryptionKeyIndexes: vote.EncryptionKeyIndexes,
		Weight:               voteRef.Weight.String(),
		OverwriteCount:       voteRef.OverwriteCount,
		Date:                 voteRef.CreationTime,
		Meta: indexertypes.EnvelopeMetadata{
			ProcessId: voteRef.ProcessID,
			Nullifier: nullifier,
			TxIndex:   voteRef.TxIndex,
			Height:    voteRef.Height,
			TxHash:    txHash,
		},
	}
	if len(envelopePackage.Meta.VoterID) > 0 {
		if envelopePackage.Meta.VoterID = voteRef.VoterID.Address(); envelopePackage.Meta.VoterID == nil {
			return nil, fmt.Errorf("cannot get voterID from public key: %w", err)
		}
	}
	return envelopePackage, nil
}

// GetEnvelopes retrieves all envelope metadata for a ProcessId.
// Returns ErrVoteNotFound if the envelope reference is not found.
func (idx *Indexer) GetEnvelopes(processId []byte, max, from int,
	searchTerm string) ([]*indexertypes.EnvelopeMetadata, error) {
	if from < 0 {
		return nil, fmt.Errorf("GetEnvelopes: invalid value: from is invalid value %d", from)
	}
	if max <= 0 {
		return nil, fmt.Errorf("GetEnvelopes: invalid value: max is invalid value %d", max)
	}
	envelopes := []*indexertypes.EnvelopeMetadata{}
	queries, ctx, cancel := idx.timeoutQueries()
	defer cancel()
	txRefs, err := queries.SearchVoteReferences(ctx, indexerdb.SearchVoteReferencesParams{
		ProcessID:       processId,
		NullifierSubstr: searchTerm,
		Limit:           int32(max),
		Offset:          int32(from),
	})
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrVoteNotFound
		}
		return nil, err
	}
	for _, txRef := range txRefs {
		_, txHash, err := idx.App.GetTxHash(uint32(txRef.Height), int32(txRef.TxIndex))
		if err != nil {
			return nil, err
		}
		envelopeMetadata := &indexertypes.EnvelopeMetadata{
			ProcessId: txRef.ProcessID,
			Nullifier: txRef.Nullifier,
			TxIndex:   int32(txRef.TxIndex),
			Height:    uint32(txRef.Height),
			TxHash:    txHash,
		}
		if len(txRef.VoterID) > 0 {
			envelopeMetadata.VoterID = txRef.VoterID.Address()
		}
		envelopes = append(envelopes, envelopeMetadata)
	}
	return envelopes, nil

}

// GetEnvelopeHeight returns the number of envelopes for a processId.
// If processId is empty, returns the total number of envelopes.
func (idx *Indexer) GetEnvelopeHeight(processID []byte) (uint64, error) {
	queries, ctx, cancel := idx.timeoutQueries()
	defer cancel()
	if len(processID) == 0 {
		height, err := queries.GetTotalProcessEnvelopeHeight(ctx)
		if err != nil {
			return 0, err
		}
		if height == nil {
			return 0, nil
		}
		return uint64(height.(int64)), nil
	}
	height, err := queries.GetProcessEnvelopeHeight(ctx, processID)
	return uint64(height), err
}

// finalizeResults process a finished voting, get the results from the state and saves it in the indexer Storage.
// Once this function is called, any future live vote event for the processId will be discarded.
func (idx *Indexer) finalizeResults(ctx context.Context, queries *indexerdb.Queries, process *models.Process) error {
	height := idx.App.Height()
	processID := process.ProcessId
	log.Debugw("finalize results", "processID", hex.EncodeToString(processID), "height", height)

	// Get the results
	r := results.ProtoToResults(process.Results)
	if _, err := queries.SetProcessResultsReady(ctx, indexerdb.SetProcessResultsReadyParams{
		ID:             processID,
		Votes:          encodeVotes(r.Votes),
		Weight:         r.Weight.String(),
		EnvelopeHeight: int64(r.EnvelopeHeight),
		BlockHeight:    int64(r.BlockHeight),
	}); err != nil {
		return err
	}

	// Remove the process from the live results
	idx.delProcessFromLiveResults(processID)

	// Set the results height
	if err := idx.setResultsHeight(processID, height); err != nil {
		return err
	}

	return nil
}

// GetResults returns the current result for a processId
func (idx *Indexer) GetResults(processID []byte) (*results.Results, error) {
	startTime := time.Now()
	defer func() { log.Debugf("GetResults sqlite took %s", time.Since(startTime)) }()

	// TODO(sqlite): getting the whole process is perhaps wasteful, but probably
	// does not matter much in the end
	queries, ctx, cancel := idx.timeoutQueries()
	defer cancel()
	sqlProcInner, err := queries.GetProcess(ctx, processID)
	if err != nil {
		return nil, err
	}
	sqlResults := indexertypes.ResultsFromDB(&sqlProcInner)

	return sqlResults, nil
}

// GetResultsWeight returns the current weight of cast votes for a processId.
func (idx *Indexer) GetResultsWeight(processID []byte) (*big.Int, error) {
	// TODO(mvdan): implement on sqlite if needed
	return nil, nil
}

// unmarshalVote decodes the base64 payload to a VotePackage struct type.
// If the state.VotePackage is encrypted the list of keys to decrypt it should be provided.
// The order of the Keys must be as it was encrypted.
// The function will reverse the order and use the decryption keys starting from the
// last one provided.
func unmarshalVote(VotePackage []byte, keys []string) (*state.VotePackage, error) {
	var vote state.VotePackage
	rawVote := make([]byte, len(VotePackage))
	copy(rawVote, VotePackage)
	// if encryption keys, decrypt the vote
	if len(keys) > 0 {
		for i := len(keys) - 1; i >= 0; i-- {
			priv, err := nacl.DecodePrivate(keys[i])
			if err != nil {
				return nil, fmt.Errorf("cannot create private key cipher: (%s)", err)
			}
			if rawVote, err = priv.Decrypt(rawVote); err != nil {
				return nil, fmt.Errorf("cannot decrypt vote with index key %d: %w", i, err)
			}
		}
	}
	if err := vote.Decode(rawVote); err != nil {
		return nil, fmt.Errorf("cannot unmarshal vote: %w", err)
	}
	return &vote, nil
}

// addLiveVote adds the envelope vote to the results. It does not commit to the database.
// This method is triggered by OnVote callback for each vote added to the blockchain.
// If encrypted vote, only weight will be updated.
func (idx *Indexer) addLiveVote(process *models.Process, VotePackage []byte, weight *big.Int, results *results.Results) error {
	// If live process, add vote to temporary results
	var vote *state.VotePackage
	if isOpenProcess(process) {
		var err error
		vote, err = unmarshalVote(VotePackage, nil)
		if err != nil {
			log.Warnf("cannot unmarshal vote: %v", err)
			vote = nil
		}
	}

	// Add the vote only if the election is unencrypted
	if vote != nil {
		if err := results.AddVote(vote.Votes, weight, nil); err != nil {
			return err
		}
	} else {
		// If encrypted, just add the weight
		results.Weight.Add(results.Weight, (*types.BigInt)(weight))
		results.EnvelopeHeight++
	}
	return nil
}

// addVoteIndex adds the nullifier reference to the kv for fetching vote Txs from BlockStore.
// This method is triggered by Commit callback for each vote added to the blockchain.
// If txn is provided the vote will be added on the transaction (without performing a commit).
func (idx *Indexer) addVoteIndex(ctx context.Context, queries *indexerdb.Queries, vote *state.Vote, txIndex int32) error {
	creationTime := idx.App.TimestampFromBlock(int64(vote.Height))
	if creationTime == nil {
		t := time.Now()
		creationTime = &t
	}
	weightStr := []byte("1")
	if vote.Weight != nil {
		var err error
		weightStr, err = vote.Weight.MarshalText()
		if err != nil {
			panic(err) // should never happen
		}
	}
	sqlStartTime := time.Now()

	if _, err := queries.CreateVoteReference(ctx, indexerdb.CreateVoteReferenceParams{
		Nullifier:      vote.Nullifier,
		ProcessID:      vote.ProcessID,
		Height:         int64(vote.Height),
		Weight:         string(weightStr),
		TxIndex:        int64(txIndex),
		OverwriteCount: int64(vote.Overwrites),
		// VoterID has a NOT NULL constraint, so we need to provide
		// a zero value for it since nil is not allowed
		VoterID:      nonNullBytes(vote.VoterID),
		CreationTime: *creationTime,
	}); err != nil {
		return err
	}

	log.Debugw("addVoteIndex sqlite",
		"nullifier", vote.Nullifier,
		"pid", vote.ProcessID,
		"blockHeight", vote.Height,
		"weight", weightStr,
		"voterID", hex.EncodeToString(vote.VoterID),
		"txIndex", txIndex,
		"overwrites", vote.Overwrites,
		"duration", time.Since(sqlStartTime).String(),
	)
	return nil
}

// addProcessToLiveResults adds the process id to the liveResultsProcs map
func (idx *Indexer) addProcessToLiveResults(pid []byte) {
	idx.liveResultsProcs.Store(string(pid), true)
}

// delProcessFromLiveResults removes the process id from the liveResultsProcs map
func (idx *Indexer) delProcessFromLiveResults(pid []byte) {
	idx.liveResultsProcs.Delete(string(pid))
}

// isProcessLiveResults returns true if the process id is in the liveResultsProcs map
func (idx *Indexer) isProcessLiveResults(pid []byte) bool {
	_, ok := idx.liveResultsProcs.Load(string(pid))
	return ok
}

// commitVotes adds the votes and weight from results to the local database.
// Important: it does not overwrite the already stored results but update them
// by adding the new content to the existing results.
func (idx *Indexer) commitVotes(pid []byte, partialResults, partialSubResults *results.Results, height uint32) error {
	// If the recovery bootstrap is running, wait
	idx.recoveryBootLock.RLock()
	defer idx.recoveryBootLock.RUnlock()
	return idx.commitVotesUnsafe(pid, partialResults, partialSubResults, height)
}

// commitVotesUnsafe does the same as commitVotes but it does not use locks.
func (idx *Indexer) commitVotesUnsafe(pid []byte, partialResults, partialSubResults *results.Results, height uint32) error {
	// TODO(sqlite): use a tx
	// TODO(sqlite): getting the whole process is perhaps wasteful, but probably
	// does not matter much in the end
	queries, ctx, cancel := idx.timeoutQueries()
	defer cancel()
	sqlProcInner, err := queries.GetProcess(ctx, pid)
	if err != nil {
		return err
	}
	results := indexertypes.ResultsFromDB(&sqlProcInner)
	if partialSubResults != nil {
		if err := results.Sub(partialSubResults); err != nil {
			return err
		}
	}
	if partialResults != nil {
		if err := results.Add(partialResults); err != nil {
			return err
		}
	}

	if _, err := queries.UpdateProcessResults(ctx, indexerdb.UpdateProcessResultsParams{
		ID:             pid,
		Votes:          encodeVotes(results.Votes),
		Weight:         results.Weight.String(),
		EnvelopeHeight: int64(results.EnvelopeHeight),
		BlockHeight:    int64(results.BlockHeight),
	}); err != nil {
		return err
	}
	return nil
}

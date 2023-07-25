package indexer

import (
	"context"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"math/big"

	"go.vocdoni.io/proto/build/go/models"

	"go.vocdoni.io/dvote/crypto/nacl"
	"go.vocdoni.io/dvote/log"
	"go.vocdoni.io/dvote/types"
	indexerdb "go.vocdoni.io/dvote/vochain/indexer/db"
	"go.vocdoni.io/dvote/vochain/indexer/indexertypes"
	"go.vocdoni.io/dvote/vochain/results"
	"go.vocdoni.io/dvote/vochain/state"
)

// ErrVoteNotFound is returned if the vote is not found in the indexer database.
var ErrVoteNotFound = fmt.Errorf("vote not found")

// GetEnvelope retrieves an Envelope from the Blockchain block store identified by its nullifier.
// Returns the envelope and the signature (if any).
func (idx *Indexer) GetEnvelope(nullifier []byte) (*indexertypes.EnvelopePackage, error) {
	voteRef, err := idx.readOnlyQuery.GetVote(context.TODO(), nullifier)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrVoteNotFound
		}
		return nil, err
	}

	envelopePackage := &indexertypes.EnvelopePackage{
		VotePackage:          []byte(voteRef.Package),
		EncryptionKeyIndexes: decodeArrayJSON[uint32](voteRef.EncryptionKeyIndexes),
		Weight:               voteRef.Weight,
		OverwriteCount:       uint32(voteRef.OverwriteCount),
		Date:                 voteRef.BlockTime,
		Meta: indexertypes.EnvelopeMetadata{
			VoterID:   voteRef.VoterID.Address(),
			ProcessId: voteRef.ProcessID,
			Nullifier: nullifier,
			TxIndex:   int32(voteRef.BlockIndex),
			Height:    uint32(voteRef.BlockHeight),
			TxHash:    voteRef.TxHash,
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
	txRefs, err := idx.readOnlyQuery.SearchVotes(context.TODO(), indexerdb.SearchVotesParams{
		ProcessID:       processId,
		NullifierSubstr: searchTerm,
		Limit:           int64(max),
		Offset:          int64(from),
	})
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrVoteNotFound
		}
		return nil, err
	}
	for _, txRef := range txRefs {
		envelopeMetadata := &indexertypes.EnvelopeMetadata{
			ProcessId: txRef.ProcessID,
			Nullifier: txRef.Nullifier,
			TxIndex:   int32(txRef.BlockIndex),
			Height:    uint32(txRef.BlockHeight),
			TxHash:    txRef.Hash,
		}
		if len(txRef.VoterID) > 0 {
			envelopeMetadata.VoterID = txRef.VoterID.Address()
		}
		envelopes = append(envelopes, envelopeMetadata)
	}
	return envelopes, nil

}

// CountVotes returns the total number of envelopes.
func (idx *Indexer) CountTotalVotes() (uint64, error) {
	height, err := idx.readOnlyQuery.CountVotes(context.TODO())
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
		ID:          processID,
		Votes:       encodeVotes(r.Votes),
		Weight:      encodeBigint(r.Weight),
		BlockHeight: int64(r.BlockHeight),
	}); err != nil {
		return err
	}

	// Remove the process from the live results
	idx.delProcessFromLiveResults(processID)

	return nil
}

// unmarshalVote decodes the base64 payload to a VotePackage struct type.
// If the state.VotePackage is encrypted the list of keys to decrypt it should be provided.
// The order of the Keys must be as it was encrypted.
// The function will reverse the order and use the decryption keys starting from the
// last one provided.
func unmarshalVote(VotePackage []byte, keys []string) (*state.VotePackage, error) {
	var rawVote []byte
	// if encryption keys, decrypt the vote
	if len(keys) > 0 {
		rawVote = make([]byte, len(VotePackage))
		copy(rawVote, VotePackage)
		for i := len(keys) - 1; i >= 0; i-- {
			priv, err := nacl.DecodePrivate(keys[i])
			if err != nil {
				return nil, fmt.Errorf("cannot create private key cipher: (%s)", err)
			}
			if rawVote, err = priv.Decrypt(rawVote); err != nil {
				return nil, fmt.Errorf("cannot decrypt vote with index key %d: %w", i, err)
			}
		}
	} else {
		rawVote = VotePackage
	}
	var vote state.VotePackage
	if err := vote.Decode(rawVote); err != nil {
		return nil, fmt.Errorf("cannot unmarshal vote: %w", err)
	}
	return &vote, nil
}

// addLiveVote adds the envelope vote to the results. It does not commit to the database.
// This method is triggered by OnVote callback for each vote added to the blockchain.
// If encrypted vote, only weight will be updated.
func (idx *Indexer) addLiveVote(process *indexertypes.Process, VotePackage []byte, weight *big.Int, results *results.Results) error {
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
	}
	return nil
}

// addVoteIndex adds the nullifier reference to the kv for fetching vote Txs from BlockStore.
// This method is triggered by Commit callback for each vote added to the blockchain.
// If txn is provided the vote will be added on the transaction (without performing a commit).
func (idx *Indexer) addVoteIndex(ctx context.Context, queries *indexerdb.Queries, vote *state.Vote, txIndex int32) error {
	weightStr := "1"
	if vote.Weight != nil {
		weightStr = encodeBigint((*types.BigInt)(vote.Weight))
	}
	if _, err := queries.CreateVote(ctx, indexerdb.CreateVoteParams{
		Nullifier:            vote.Nullifier,
		ProcessID:            vote.ProcessID,
		BlockHeight:          int64(vote.Height),
		BlockIndex:           int64(txIndex),
		Weight:               weightStr,
		OverwriteCount:       int64(vote.Overwrites),
		VoterID:              nonNullBytes(vote.VoterID),
		EncryptionKeyIndexes: encodeArrayJSON(vote.EncryptionKeyIndexes),
		Package:              string(vote.VotePackage),
	}); err != nil {
		return err
	}
	return nil
}

func encodeArrayJSON[T any](v []T) string {
	p, err := json.Marshal(v)
	if err != nil {
		panic(err) // should not happen
	}
	return string(p)
}

func decodeArrayJSON[T any](s string) []T {
	var v []T
	err := json.Unmarshal([]byte(s), &v)
	if err != nil {
		panic(err) // should not happen
	}
	return v
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
func (idx *Indexer) commitVotes(queries *indexerdb.Queries, pid []byte, partialResults, partialSubResults *results.Results, height uint32) error {
	// If the recovery bootstrap is running, wait
	idx.recoveryBootLock.RLock()
	defer idx.recoveryBootLock.RUnlock()
	return idx.commitVotesUnsafe(queries, pid, partialResults, partialSubResults, height)
}

// commitVotesUnsafe does the same as commitVotes but it does not use locks.
func (idx *Indexer) commitVotesUnsafe(queries *indexerdb.Queries, pid []byte, partialResults, partialSubResults *results.Results, _ uint32) error {
	// TODO(sqlite): getting the whole process is perhaps wasteful, but probably
	// does not matter much in the end
	procInner, err := queries.GetProcess(context.TODO(), pid)
	if err != nil {
		return err
	}
	results := indexertypes.ProcessFromDB(&procInner).Results()
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

	if _, err := queries.UpdateProcessResults(context.TODO(), indexerdb.UpdateProcessResultsParams{
		ID:          pid,
		Votes:       encodeVotes(results.Votes),
		Weight:      encodeBigint(results.Weight),
		BlockHeight: int64(results.BlockHeight),
	}); err != nil {
		return err
	}
	return nil
}

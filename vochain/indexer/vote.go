package indexer

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/hex"
	"errors"
	"fmt"
	"math/big"
	"strings"
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
		EncryptionKeyIndexes: indexertypes.DecodeJSON[[]uint32](voteRef.EncryptionKeyIndexes),
		Weight:               indexertypes.DecodeJSON[string](voteRef.Weight),
		OverwriteCount:       uint32(voteRef.OverwriteCount),
		Date:                 voteRef.BlockTime.Time,
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

// VoteList retrieves all envelope metadata for a processID and nullifier (both args do partial or full string match).
func (idx *Indexer) VoteList(limit, offset int, processID string, nullifier string,
) ([]*indexertypes.EnvelopeMetadata, uint64, error) {
	if offset < 0 {
		return nil, 0, fmt.Errorf("invalid value: offset cannot be %d", offset)
	}
	if limit <= 0 {
		return nil, 0, fmt.Errorf("invalid value: limit cannot be %d", limit)
	}
	results, err := idx.readOnlyQuery.SearchVotes(context.TODO(), indexerdb.SearchVotesParams{
		ProcessIDSubstr: processID,
		NullifierSubstr: strings.ToLower(nullifier), // we search in lowercase
		Limit:           int64(limit),
		Offset:          int64(offset),
	})
	if err != nil {
		return nil, 0, err
	}
	list := []*indexertypes.EnvelopeMetadata{}
	for _, txRef := range results {
		envelopeMetadata := &indexertypes.EnvelopeMetadata{
			ProcessId: txRef.ProcessID,
			Nullifier: txRef.Nullifier,
			TxIndex:   int32(txRef.BlockIndex),
			Height:    uint32(txRef.BlockHeight),
			TxHash:    txRef.Hash,
		}
		if len(txRef.VoterID) > 0 {
			envelopeMetadata.VoterID = state.VoterID(txRef.VoterID).Address()
		}
		list = append(list, envelopeMetadata)
	}
	count, err := idx.readOnlyQuery.CountVotes(context.TODO(), indexerdb.CountVotesParams{
		ProcessIDSubstr: processID,
		NullifierSubstr: strings.ToLower(nullifier), // we search in lowercase
	})
	if err != nil {
		return nil, 0, err
	}
	return list, uint64(count), nil
}

// CountTotalVotes returns the total number of envelopes.
func (idx *Indexer) CountTotalVotes() (uint64, error) {
	height, err := idx.readOnlyQuery.CountTotalVotes(context.TODO())
	return uint64(height), err
}

// finalizeResults process a finished voting, get the results from the state and saves it in the indexer Storage.
// Once this function is called, any future live vote event for the processId will be discarded.
func (idx *Indexer) finalizeResults(ctx context.Context, queries *indexerdb.Queries, process *models.Process) error {
	height := idx.App.Height()
	processID := process.ProcessId
	endDate := time.Unix(idx.App.Timestamp(), 0)
	log.Debugw("finalize results", "processID", hex.EncodeToString(processID), "height", height, "endDate", endDate.String())
	// Get the results
	r := results.ProtoToResults(process.Results)
	if _, err := queries.SetProcessResultsReady(ctx, indexerdb.SetProcessResultsReadyParams{
		ID:          processID,
		Votes:       indexertypes.EncodeJSON(r.Votes),
		Weight:      indexertypes.EncodeJSON(r.Weight),
		BlockHeight: int64(r.BlockHeight),
		EndDate:     endDate,
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
		rawVote = bytes.Clone(VotePackage)
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
func (*Indexer) addLiveVote(process *indexertypes.Process, VotePackage []byte, weight *big.Int, results *results.Results) error {
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

// commitVotesUnsafe adds the votes and weight from results to the local database.
// Important: it does not overwrite the already stored results but update them
// by adding the new content to the existing results.
func (*Indexer) commitVotesUnsafe(queries *indexerdb.Queries, pid []byte, results, partialResults, partialSubResults *results.Results, _ uint32) error {
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
		Votes:       indexertypes.EncodeJSON(results.Votes),
		Weight:      indexertypes.EncodeJSON(results.Weight),
		BlockHeight: int64(results.BlockHeight),
	}); err != nil {
		return err
	}
	return nil
}

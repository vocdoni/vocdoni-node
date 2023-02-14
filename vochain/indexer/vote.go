package indexer

import (
	"encoding/hex"
	"encoding/json"
	"fmt"
	"math/big"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"go.vocdoni.io/proto/build/go/models"

	"go.vocdoni.io/dvote/crypto/nacl"
	"go.vocdoni.io/dvote/log"
	"go.vocdoni.io/dvote/types"
	"go.vocdoni.io/dvote/vochain"
	indexerdb "go.vocdoni.io/dvote/vochain/indexer/db"
	"go.vocdoni.io/dvote/vochain/indexer/indexertypes"
	"go.vocdoni.io/dvote/vochain/results"
	"go.vocdoni.io/dvote/vochain/state"
)

// ErrNoResultsYet is an error returned to indicate the process exist but
// it does not have yet reuslts
var ErrNoResultsYet = fmt.Errorf("no results yet")

// Getindexertypes.VoteReference gets the reference for an AddVote transaction.
// This reference can then be used to fetch the vote transaction directly from the BlockStore.
func (s *Indexer) GetEnvelopeReference(nullifier []byte) (*indexertypes.VoteReference, error) {
	sqlStartTime := time.Now()

	queries, ctx, cancel := s.timeoutQueries()
	defer cancel()
	sqlTxRefInner, err := queries.GetVoteReference(ctx, nullifier)
	if err != nil {
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
func (s *Indexer) GetEnvelope(nullifier []byte) (*indexertypes.EnvelopePackage, error) {
	t := time.Now()
	voteRef, err := s.GetEnvelopeReference(nullifier)
	if err != nil {
		return nil, err
	}
	vote, err := s.App.State.Vote(voteRef.ProcessID, nullifier, true)
	if err != nil {
		return nil, err
	}
	// TODO: get the txHash from another place, not the blockstore
	_, txHash, err := s.App.GetTxHash(voteRef.Height, voteRef.TxIndex)
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
		envelopePackage.Meta.VoterID, err = voteRef.VoterID.Address()
		if err != nil {
			return nil, fmt.Errorf("cannot get voterID from public key: %w", err)
		}
	}
	return envelopePackage, nil
}

// WalkEnvelopes executes callback for each envelopes of the ProcessId.
// The callback function is executed async (in a goroutine) if async=true.
// The method will return once all goroutines have finished the work.
func (s *Indexer) WalkEnvelopes(processId []byte, async bool,
	callback func(*models.StateDBVote)) error {
	wg := sync.WaitGroup{}

	// There might be tens of thousands of votes.
	// When async==true, we don't want to process all of them at once,
	// as that could easily result in running out of memory.
	// Limit it to a relatively small amount of goroutines,
	// but also large enough to be faster than sequential processing.
	const limitConcurrentProcessing = 20
	semaphore := make(chan bool, limitConcurrentProcessing)

	queries, ctx, cancel := s.timeoutQueries()
	defer cancel()
	// TODO(sqlite): getting all votes as a single slice is not scalable.
	txRefs, err := queries.GetVoteReferencesByProcessID(ctx, processId)
	if err != nil {
		return err
	}
	for _, txRef := range txRefs {
		wg.Add(1)
		txRef := txRef // do not reuse the range var in case async==true
		processVote := func() {
			defer wg.Done()
			v, err := s.App.State.Vote(processId, txRef.Nullifier, true)
			if err != nil {
				log.Errorw(err, "cannot get vote from state")
				return
			}
			callback(v)
		}
		if async {
			go func() {
				semaphore <- true
				processVote()
				<-semaphore
			}()
		} else {
			processVote()
		}
	}

	wg.Wait()
	return nil
}

// GetEnvelopes retrieves all envelope metadata for a ProcessId.
func (s *Indexer) GetEnvelopes(processId []byte, max, from int,
	searchTerm string) ([]*indexertypes.EnvelopeMetadata, error) {
	if from < 0 {
		return nil, fmt.Errorf("GetEnvelopes: invalid value: from is invalid value %d", from)
	}
	if max <= 0 {
		return nil, fmt.Errorf("GetEnvelopes: invalid value: max is invalid value %d", max)
	}
	envelopes := []*indexertypes.EnvelopeMetadata{}
	queries, ctx, cancel := s.timeoutQueries()
	defer cancel()
	txRefs, err := queries.SearchVoteReferences(ctx, indexerdb.SearchVoteReferencesParams{
		ProcessID:       processId,
		NullifierSubstr: searchTerm,
		Limit:           int32(max),
		Offset:          int32(from),
	})
	if err != nil {
		return nil, err
	}
	for _, txRef := range txRefs {
		_, txHash, err := s.App.GetTxHash(uint32(txRef.Height), int32(txRef.TxIndex))
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
			envelopeMetadata.VoterID, err = txRef.VoterID.Address()
			if err != nil {
				return nil, fmt.Errorf("cannot get voterID from pubkey: %w", err)
			}
		}
		envelopes = append(envelopes, envelopeMetadata)
	}
	return envelopes, nil

}

// GetEnvelopeHeight returns the number of envelopes for a processId.
// If processId is empty, returns the total number of envelopes.
func (s *Indexer) GetEnvelopeHeight(processID []byte) (uint64, error) {
	queries, ctx, cancel := s.timeoutQueries()
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

// ComputeResult process a finished voting, compute the results and saves it in the Storage.
// Once this function is called, any future live vote event for the processId will be discarted.
func (s *Indexer) ComputeResult(processID []byte) error {
	height := s.App.Height()
	log.Debugf("computing results on height %d for %x", height, processID)

	// TODO(sqlite): use a single tx for everything here

	// Get process from database
	p, err := s.ProcessInfo(processID)
	if err != nil {
		return fmt.Errorf("computeResult: cannot load processID %x from database: %w", processID, err)
	}
	// Compute the results
	var results *results.Results
	if results, err = s.computeFinalResults(p); err != nil {
		return err
	}

	queries, ctx, cancel := s.timeoutQueries()
	defer cancel()
	if _, err := queries.SetProcessResultsReady(ctx, indexerdb.SetProcessResultsReadyParams{
		ID:             processID,
		Votes:          encodeVotes(results.Votes),
		Weight:         results.Weight.String(),
		EnvelopeHeight: int64(results.EnvelopeHeight),
		Signatures:     joinHexBytes(results.Signatures),
		BlockHeight:    int64(results.BlockHeight),
	}); err != nil {
		return err
	}
	// Execute callbacks
	for _, l := range s.eventOnResults {
		go l.OnComputeResults(results, p, height)
	}
	return nil
}

func joinHexBytes(list []types.HexBytes) string {
	strs := make([]string, len(list))
	for i, b := range list {
		strs[i] = hex.EncodeToString(b)
	}
	return strings.Join(strs, ",")
}

// GetResults returns the current result for a processId
func (s *Indexer) GetResults(processID []byte) (*results.Results, error) {
	startTime := time.Now()
	defer func() { log.Debugf("GetResults sqlite took %s", time.Since(startTime)) }()

	// TODO(sqlite): getting the whole process is perhaps wasteful, but probably
	// does not matter much in the end
	queries, ctx, cancel := s.timeoutQueries()
	defer cancel()
	sqlProcInner, err := queries.GetProcess(ctx, processID)
	if err != nil {
		return nil, err
	}
	sqlResults := indexertypes.ResultsFromDB(&sqlProcInner)

	return sqlResults, nil
}

// GetResultsWeight returns the current weight of cast votes for a processId.
func (s *Indexer) GetResultsWeight(processID []byte) (*big.Int, error) {
	// TODO(mvdan): implement on sqlite if needed
	return nil, nil
}

// unmarshalVote decodes the base64 payload to a VotePackage struct type.
// If the vochain.VotePackage is encrypted the list of keys to decrypt it should be provided.
// The order of the Keys must be as it was encrypted.
// The function will reverse the order and use the decryption keys starting from the
// last one provided.
func unmarshalVote(VotePackage []byte, keys []string) (*vochain.VotePackage, error) {
	var vote vochain.VotePackage
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
	if err := json.Unmarshal(rawVote, &vote); err != nil {
		return nil, fmt.Errorf("cannot unmarshal vote: %w", err)
	}
	return &vote, nil
}

// addLiveVote adds the envelope vote to the results. It does not commit to the database.
// This method is triggered by OnVote callback for each vote added to the blockchain.
// If encrypted vote, only weight will be updated.
func (s *Indexer) addLiveVote(pid []byte, VotePackage []byte, weight *big.Int, results *results.Results) error {
	// If live process, add vote to temporary results
	var vote *vochain.VotePackage
	if open, err := s.isOpenProcess(pid); open && err == nil {
		vote, err = unmarshalVote(VotePackage, []string{})
		if err != nil {
			log.Warnf("cannot unmarshal vote: %v", err)
			vote = nil
		}
	} else if err != nil {
		return fmt.Errorf("cannot check if process is open: %v", err)
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
func (s *Indexer) addVoteIndex(vote *state.Vote, txIndex int32) error {
	creationTime := s.App.TimestampFromBlock(int64(vote.Height))
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

	// TODO(mvdan): badgerhold shared a transaction via a parameter, consider
	// doing the same with sqlite
	queries, ctx, cancel := s.timeoutQueries()
	defer cancel()
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
func (s *Indexer) addProcessToLiveResults(pid []byte) {
	s.liveResultsProcs.Store(string(pid), true)
}

// delProcessFromLiveResults removes the process id from the liveResultsProcs map
func (s *Indexer) delProcessFromLiveResults(pid []byte) {
	s.liveResultsProcs.Delete(string(pid))
}

// isProcessLiveResults returns true if the process id is in the liveResultsProcs map
func (s *Indexer) isProcessLiveResults(pid []byte) bool {
	_, ok := s.liveResultsProcs.Load(string(pid))
	return ok
}

// commitVotes adds the votes and weight from results to the local database.
// Important: it does not overwrite the already stored results but update them
// by adding the new content to the existing results.
func (s *Indexer) commitVotes(pid []byte, partialResults, partialSubResults *results.Results, height uint32) error {
	// If the recovery bootstrap is running, wait
	s.recoveryBootLock.RLock()
	defer s.recoveryBootLock.RUnlock()
	return s.commitVotesUnsafe(pid, partialResults, partialSubResults, height)
}

// commitVotesUnsafe does the same as commitVotes but it does not use locks.
func (s *Indexer) commitVotesUnsafe(pid []byte, partialResults, partialSubResults *results.Results, height uint32) error {
	// TODO(sqlite): use a tx
	// TODO(sqlite): getting the whole process is perhaps wasteful, but probably
	// does not matter much in the end
	queries, ctx, cancel := s.timeoutQueries()
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

// computeFinalResults walks through the envelopes of a process and computes the results.
func (s *Indexer) computeFinalResults(p *indexertypes.Process) (*results.Results, error) {
	if p == nil {
		return nil, fmt.Errorf("process is nil")
	}
	if p.VoteOpts.MaxCount == 0 || p.VoteOpts.MaxValue == 0 {
		return nil, fmt.Errorf("computeNonLiveResults: maxCount and/or maxValue is zero")
	}
	if p.VoteOpts.MaxCount > results.MaxQuestions || p.VoteOpts.MaxValue > results.MaxOptions {
		return nil, fmt.Errorf("maxCount and/or maxValue overflows hardcoded maximum")
	}
	results := &results.Results{
		Votes:        results.NewEmptyVotes(int(p.VoteOpts.MaxCount), int(p.VoteOpts.MaxValue)+1),
		ProcessID:    p.ID,
		Weight:       new(types.BigInt).SetUint64(0),
		Final:        true,
		VoteOpts:     p.VoteOpts,
		EnvelopeType: p.Envelope,
		BlockHeight:  s.App.Height(),
	}

	var nvotes atomic.Uint64
	var err error
	lock := sync.Mutex{}

	if err = s.WalkEnvelopes(p.ID, true, func(vote *models.StateDBVote) {
		var vp *vochain.VotePackage
		var err error
		if p.Envelope.EncryptedVotes {
			if len(p.PrivateKeys) < len(vote.EncryptionKeyIndexes) {
				log.Error("encryptionKeyIndexes has too many fields")
				return
			}
			keys := []string{}
			for _, k := range vote.EncryptionKeyIndexes {
				if k >= types.KeyKeeperMaxKeyIndex {
					log.Warn("key index overflow")
					return
				}
				keys = append(keys, p.PrivateKeys[k])
			}
			if len(keys) == 0 {
				log.Warn("no keys provided or wrong index")
				return
			}
			vp, err = unmarshalVote(vote.VotePackage, keys)
		} else {
			vp, err = unmarshalVote(vote.VotePackage, []string{})
		}
		if err != nil {
			log.Debugf("vote invalid: %v", err)
			return
		}

		if err = results.AddVote(vp.Votes, new(big.Int).SetBytes(vote.Weight), &lock); err != nil {
			log.Warnf("addVote failed: %v", err)
			return
		}
		nvotes.Add(1)
	}); err == nil {
		log.Infow("computed results",
			"process", p.ID.String(),
			"votes", nvotes.Load(),
			"results", results.String(),
		)
	}
	results.EnvelopeHeight = nvotes.Load()
	return results, err
}

// BuildProcessResult takes the indexer Results type and builds the protobuf type ProcessResult.
// EntityId should be provided as addition field to include in ProcessResult.
func BuildProcessResult(results *results.Results, entityID []byte) *models.ProcessResult {
	// build the protobuf type for Results
	qr := []*models.QuestionResult{}
	for i := range results.Votes {
		qr = append(qr, &models.QuestionResult{})
		for j := range results.Votes[i] {
			qr[i].Question = append(qr[i].Question, results.Votes[i][j].Bytes())
		}
	}
	return &models.ProcessResult{
		ProcessId: results.ProcessID,
		EntityId:  entityID,
		Votes:     qr,
	}
}

package scrutinizer

import (
	"encoding/json"
	"fmt"
	"math/big"
	"sync"
	"sync/atomic"
	"time"

	badger "github.com/dgraph-io/badger/v3"
	"github.com/timshannon/badgerhold/v3"
	"go.vocdoni.io/proto/build/go/models"
	"google.golang.org/protobuf/proto"

	"go.vocdoni.io/dvote/crypto/nacl"
	"go.vocdoni.io/dvote/log"
	"go.vocdoni.io/dvote/types"
	"go.vocdoni.io/dvote/vochain"
	"go.vocdoni.io/dvote/vochain/scrutinizer/indexertypes"
)

// ErrNoResultsYet is an error returned to indicate the process exist but
// it does not have yet reuslts
var ErrNoResultsYet = fmt.Errorf("no results yet")

// ErrNotFoundIndatabase is raised if a database query returns no results
var ErrNotFoundInDatabase = badgerhold.ErrNotFound

// Getindexertypes.VoteReference gets the reference for an AddVote transaction.
// This reference can then be used to fetch the vote transaction directly from the BlockStore.
func (s *Scrutinizer) GetEnvelopeReference(nullifier []byte) (*indexertypes.VoteReference, error) {
	startTime := time.Now()
	defer func() { log.Debugf("GetEnvelopeReference took %s", time.Since(startTime)) }()
	txRef := &indexertypes.VoteReference{}
	return txRef, s.db.FindOne(txRef, badgerhold.Where(badgerhold.Key).Eq(nullifier))
}

// GetEnvelope retreives an Envelope from the Blockchain block store identified by its nullifier.
// Returns the envelope and the signature (if any).
func (s *Scrutinizer) GetEnvelope(nullifier []byte) (*indexertypes.EnvelopePackage, error) {
	startTime := time.Now()
	defer func() { log.Debugf("GetEnvelope took %s", time.Since(startTime)) }()
	t := time.Now()
	voteRef, err := s.GetEnvelopeReference(nullifier)
	if err != nil {
		return nil, err
	}
	stx, txHash, err := s.App.GetTxHash(voteRef.Height, voteRef.TxIndex)
	if err != nil {
		return nil, err
	}
	tx := &models.Tx{}
	if err := proto.Unmarshal(stx.Tx, tx); err != nil {
		return nil, err
	}
	envelope := tx.GetVote()
	if envelope == nil {
		return nil, fmt.Errorf("transaction is not an Envelope")
	}
	log.Debugf("getEnvelope took %s", time.Since(t))
	return &indexertypes.EnvelopePackage{
		Nonce:                envelope.Nonce,
		VotePackage:          envelope.VotePackage,
		EncryptionKeyIndexes: envelope.EncryptionKeyIndexes,
		Weight:               voteRef.Weight.String(),
		Signature:            stx.Signature,
		Meta: indexertypes.EnvelopeMetadata{
			ProcessId: envelope.ProcessId,
			Nullifier: nullifier,
			TxIndex:   voteRef.TxIndex,
			Height:    voteRef.Height,
			TxHash:    txHash,
		},
	}, nil
}

// WalkEnvelopes executes callback for each envelopes of the ProcessId.
// The callback function is executed async (in a goroutine) if async=true.
// The method will return once all goroutines have finished the work.
func (s *Scrutinizer) WalkEnvelopes(processId []byte, async bool,
	callback func(*models.VoteEnvelope, *big.Int)) error {
	wg := sync.WaitGroup{}

	// There might be tens of thousands of votes.
	// When async==true, we don't want to process all of them at once,
	// as that could easily result in running out of memory.
	// Limit it to a relatively small amount of goroutines,
	// but also large enough to be faster than sequential processing.
	const limitConcurrentProcessing = 20
	semaphore := make(chan bool, limitConcurrentProcessing)

	err := s.db.ForEach(
		badgerhold.Where("ProcessID").Eq(processId).Index("ProcessID"),
		func(txRef *indexertypes.VoteReference) error {
			wg.Add(1)
			processVote := func() {
				defer wg.Done()
				stx, err := s.App.GetTx(txRef.Height, txRef.TxIndex)
				if err != nil {
					log.Errorf("could not get tx: %v", err)
					return
				}
				tx := &models.Tx{}
				if err := proto.Unmarshal(stx.Tx, tx); err != nil {
					log.Warnf("could not unmarshal tx: %v", err)
					return
				}
				envelope := tx.GetVote()
				if envelope == nil {
					log.Errorf("transaction is not an Envelope")
					return
				}
				callback(envelope, txRef.Weight)
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
			return nil
		})
	wg.Wait()
	return err
}

// GetEnvelopes retreives all Envelopes of a ProcessId from the Blockchain block store
func (s *Scrutinizer) GetEnvelopes(processId []byte, max, from int,
	searchTerm string) ([]*indexertypes.EnvelopeMetadata, error) {
	if from < 0 {
		return nil, fmt.Errorf("envelopeList: invalid value: from is invalid value %d", from)
	}
	envelopes := []*indexertypes.EnvelopeMetadata{}
	var err error
	// check pid
	if len(processId) == types.ProcessIDsize {
		err = s.db.ForEach(
			badgerhold.Where("ProcessID").Eq(processId).Index("ProcessID").
				And("Nullifier").MatchFunc(searchMatchFunc(searchTerm)).
				SortBy("Height").
				Skip(from).
				Limit(max),
			func(txRef *indexertypes.VoteReference) error {
				stx, txHash, err := s.App.GetTxHash(txRef.Height, txRef.TxIndex)
				if err != nil {
					return err
				}
				tx := &models.Tx{}
				if err := proto.Unmarshal(stx.Tx, tx); err != nil {
					return err
				}
				envelope := tx.GetVote()
				if envelope == nil {
					return fmt.Errorf("transaction is not an Envelope")
				}
				envelope.Nullifier = txRef.Nullifier
				envelopes = append(envelopes, &indexertypes.EnvelopeMetadata{
					ProcessId: processId,
					Nullifier: txRef.Nullifier,
					TxIndex:   txRef.TxIndex,
					Height:    txRef.Height,
					TxHash:    txHash,
				})
				return nil
			})
	} else if len(searchTerm) > 0 { // Search nullifiers without process id
		err = s.db.ForEach(
			badgerhold.
				Where("Nullifier").MatchFunc(searchMatchFunc(searchTerm)).
				SortBy("Height").
				Skip(from).
				Limit(max),
			func(txRef *indexertypes.VoteReference) error {
				stx, txHash, err := s.App.GetTxHash(txRef.Height, txRef.TxIndex)
				if err != nil {
					return err
				}
				tx := &models.Tx{}
				if err := proto.Unmarshal(stx.Tx, tx); err != nil {
					return err
				}
				envelope := tx.GetVote()
				if envelope == nil {
					return fmt.Errorf("transaction is not an Envelope")
				}
				envelope.Nullifier = txRef.Nullifier
				envelopes = append(envelopes, &indexertypes.EnvelopeMetadata{
					ProcessId: processId,
					Nullifier: txRef.Nullifier,
					TxIndex:   txRef.TxIndex,
					Height:    txRef.Height,
					TxHash:    txHash,
				})
				return nil
			})
	} else {
		return nil, fmt.Errorf("cannot get envelope status: (malformed processId)")
	}
	return envelopes, err
}

// GetEnvelopeHeight returns the number of envelopes for a processId.
// If processId is empty, returns the total number of envelopes.
func (s *Scrutinizer) GetEnvelopeHeight(processID []byte) (uint64, error) {
	startTime := time.Now()
	defer func() { log.Debugf("GetEnvelopeHeight took %s", time.Since(startTime)) }()
	if len(processID) == 0 {
		// If no processID is provided, count all envelopes
		envelopeCountStore := &indexertypes.CountStore{}
		if err := s.db.Get(indexertypes.CountStoreEnvelopes, envelopeCountStore); err != nil {
			return 0, err
		}
		return envelopeCountStore.Count, nil
	}
	// Check if the envelope height is cached
	val := s.envelopeHeightCache.Get(string(processID))
	if val != nil {
		return val.(uint64), nil
	}
	// If not cached, make the expensive query
	results := &indexertypes.Results{}
	if err := s.db.FindOne(results, badgerhold.Where(badgerhold.Key).Eq(processID)); err != nil {
		return 0, err
	}
	// If final, store them in cache (won't change anymore)
	if results.Final {
		s.envelopeHeightCache.Add(string(processID), results.EnvelopeHeight)
	}
	return results.EnvelopeHeight, nil
}

// ComputeResult process a finished voting, compute the results and saves it in the Storage.
// Once this function is called, any future live vote event for the processId will be discarted.
func (s *Scrutinizer) ComputeResult(processID []byte) error {
	height := s.App.Height()
	log.Debugf("computing results on height %d for %x", height, processID)

	// Get process from database
	p, err := s.ProcessInfo(processID)
	if err != nil {
		return fmt.Errorf("computeResult: cannot load processID %x from database: %w", processID, err)
	}
	// Compute the results
	var results *indexertypes.Results
	if results, err = s.computeFinalResults(p); err != nil {
		return err
	}

	// We need to use UpdateMatching here because of a race condition between ComputeResult and
	// updateProcess. If we fetch the process, compute the results, then update the process,
	// ComputeResult can override the process status set by updateProcess. UpdateMatching
	// gets rid of the time between fetching and updating the process, eliminating this race.
	if err := s.queryWithRetries(func() error {
		return s.db.UpdateMatching(&indexertypes.Process{},
			badgerhold.Where(badgerhold.Key).Eq(processID),
			func(record interface{}) error {
				update, ok := record.(*indexertypes.Process)
				if !ok {
					return fmt.Errorf("record isn't the correct type! Wanted Result, got %T", record)
				}
				update.HaveResults = true
				update.FinalResults = true
				return nil
			},
		)
	}); err != nil {
		return fmt.Errorf("computeResult: cannot update processID %x: %v", processID, err)
	}
	s.addVoteLock.Lock()
	defer s.addVoteLock.Unlock()
	if err := s.queryWithRetries(func() error { return s.db.Upsert(processID, results) }); err != nil {
		return err
	}

	// Execute callbacks
	for _, l := range s.eventOnResults {
		go l.OnComputeResults(results, p, height)
	}
	return nil
}

// GetResults returns the current result for a processId
func (s *Scrutinizer) GetResults(processID []byte) (*indexertypes.Results, error) {
	startTime := time.Now()
	defer func() { log.Debugf("GetResults took %s", time.Since(startTime)) }()
	// Check if results are in cache and return them.
	// Note that we don't use an atomic cache,
	// since that would mean always inserting into the cache.
	// TODO: remove this cache once we replace badgerhold
	val := s.resultsCache.Get(string(processID))
	if val != nil {
		return val.(*indexertypes.Results), nil
	}
	// If not in cache, execute the expensive query
	s.addVoteLock.RLock()
	defer s.addVoteLock.RUnlock()
	results := &indexertypes.Results{}
	if err := s.db.FindOne(results, badgerhold.Where(badgerhold.Key).
		Eq(processID)); err != nil {
		if err == badgerhold.ErrNotFound {
			return nil, ErrNoResultsYet
		}
		return nil, err
	}
	// If results are final, store in cache for next queries
	if results.Final {
		s.resultsCache.Add(string(processID), results)
	}
	return results, nil
}

// GetResultsWeight returns the current weight of cast votes for a processId.
func (s *Scrutinizer) GetResultsWeight(processID []byte) (*big.Int, error) {
	startTime := time.Now()
	defer func() { log.Debugf("GetResultsWeight took %s", time.Since(startTime)) }()
	s.addVoteLock.RLock()
	defer s.addVoteLock.RUnlock()
	results := &indexertypes.Results{}
	if err := s.db.FindOne(results, badgerhold.Where(badgerhold.Key).Eq(processID)); err != nil {
		return nil, err
	}
	return results.Weight, nil
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
func (s *Scrutinizer) addLiveVote(pid []byte, VotePackage []byte, weight *big.Int,
	results *indexertypes.Results) error {
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
		results.Weight.Add(results.Weight, weight)
		results.EnvelopeHeight++
	}
	return nil
}

// addVoteIndex adds the nullifier reference to the kv for fetching vote Txs from BlockStore.
// This method is triggered by Commit callback for each vote added to the blockchain.
// If txn is provided the vote will be added on the transaction (without performing a commit).
func (s *Scrutinizer) addVoteIndex(nullifier, pid []byte, blockHeight uint32,
	weight []byte, txIndex int32, txn *badger.Txn) error {
	if txn != nil {
		return s.db.TxInsert(txn, nullifier, &indexertypes.VoteReference{
			Nullifier:    nullifier,
			ProcessID:    pid,
			Height:       blockHeight,
			Weight:       new(big.Int).SetBytes(weight),
			TxIndex:      txIndex,
			CreationTime: time.Now(),
		})
	}
	return s.queryWithRetries(func() error {
		return s.db.Insert(nullifier, &indexertypes.VoteReference{
			Nullifier:    nullifier,
			ProcessID:    pid,
			Height:       blockHeight,
			Weight:       new(big.Int).SetBytes(weight),
			TxIndex:      txIndex,
			CreationTime: time.Now(),
		})
	})
}

// addProcessToLiveResults adds the process id to the liveResultsProcs map
func (s *Scrutinizer) addProcessToLiveResults(pid []byte) {
	s.liveResultsProcs.Store(string(pid), true)
}

// delProcessFromLiveResults removes the process id from the liveResultsProcs map
func (s *Scrutinizer) delProcessFromLiveResults(pid []byte) {
	s.liveResultsProcs.Delete(string(pid))
}

// isProcessLiveResults returns true if the process id is in the liveResultsProcs map
func (s *Scrutinizer) isProcessLiveResults(pid []byte) bool {
	_, ok := s.liveResultsProcs.Load(string(pid))
	return ok
}

// commitVotes adds the votes and weight from results to the local database.
// Important: it does not overwrite the already stored results but update them
// by adding the new content to the existing results.
func (s *Scrutinizer) commitVotes(pid []byte,
	partialResults *indexertypes.Results, height uint32) error {
	// If the recovery bootstrap is running, wait
	s.recoveryBootLock.RLock()
	defer s.recoveryBootLock.RUnlock()
	// The next lock avoid Transaction Conflicts
	s.addVoteLock.Lock()
	defer s.addVoteLock.Unlock()
	return s.commitVotesUnsafe(pid, partialResults, height)
}

// commitVotesUnsafe does the same as commitVotes but it does not use locks.
func (s *Scrutinizer) commitVotesUnsafe(pid []byte,
	partialResults *indexertypes.Results, height uint32) error {
	update := func(record interface{}) error {
		stored, ok := record.(*indexertypes.Results)
		if !ok {
			return fmt.Errorf("record isn't the correct type! Wanted Result, got %T", record)
		}
		// If already final, don't update.
		if stored.Final {
			return nil
		}
		return stored.Add(partialResults)
	}

	if err := s.queryWithRetries(func() error {
		return s.db.UpdateMatching(&indexertypes.Results{},
			badgerhold.Where(badgerhold.Key).Eq(pid), update)
	}); err != nil {
		log.Debugf("saved %d votes with total weight of %s on process %x", len(partialResults.Votes),
			partialResults.Weight, pid)
		return err
	}
	return nil
}

// computeFinalResults walks through the envelopes of a process and computes the results.
func (s *Scrutinizer) computeFinalResults(p *indexertypes.Process) (*indexertypes.Results, error) {
	if p == nil {
		return nil, fmt.Errorf("process is nil")
	}
	if p.VoteOpts.MaxCount == 0 || p.VoteOpts.MaxValue == 0 {
		return nil, fmt.Errorf("computeNonLiveResults: maxCount and/or maxValue is zero")
	}
	if p.VoteOpts.MaxCount > MaxQuestions || p.VoteOpts.MaxValue > MaxOptions {
		return nil, fmt.Errorf("maxCount and/or maxValue overflows hardcoded maximum")
	}
	results := &indexertypes.Results{
		Votes:        indexertypes.NewEmptyVotes(int(p.VoteOpts.MaxCount), int(p.VoteOpts.MaxValue)+1),
		ProcessID:    p.ID,
		Weight:       new(big.Int).SetUint64(0),
		Final:        true,
		VoteOpts:     p.VoteOpts,
		EnvelopeType: p.Envelope,
		BlockHeight:  s.App.Height(),
	}

	var nvotes uint64
	var err error
	lock := sync.Mutex{}

	if err = s.WalkEnvelopes(p.ID, true, func(vote *models.VoteEnvelope,
		weight *big.Int) {
		var vp *vochain.VotePackage
		var err error
		if p.Envelope.GetEncryptedVotes() {
			if len(p.PrivateKeys) < len(vote.GetEncryptionKeyIndexes()) {
				log.Errorf("encryptionKeyIndexes has too many fields")
				return
			} else {
				keys := []string{}
				for _, k := range vote.GetEncryptionKeyIndexes() {
					if k >= types.KeyKeeperMaxKeyIndex {
						log.Warn("key index overflow")
						return
					}
					keys = append(keys, p.PrivateKeys[k])
				}
				if len(keys) == 0 || err != nil {
					log.Warn("no keys provided or wrong index")
					return
				} else {
					vp, err = unmarshalVote(vote.GetVotePackage(), keys)
				}
			}
		} else {
			vp, err = unmarshalVote(vote.GetVotePackage(), []string{})
		}
		if err != nil {
			log.Debugf("vote invalid: %v", err)
			return
		}

		if err = results.AddVote(vp.Votes, weight, &lock); err != nil {
			log.Warnf("addVote failed: %v", err)
			return
		}
		atomic.AddUint64(&nvotes, 1)
	}); err == nil {
		log.Infof("computed results for process %x with %d votes", p.ID, nvotes)
		log.Debugf("results: %s", results)
	}
	results.EnvelopeHeight = nvotes
	return results, err
}

// BuildProcessResult takes the indexer Results type and builds the protobuf type ProcessResult.
// EntityId should be provided as addition field to include in ProcessResult.
func BuildProcessResult(results *indexertypes.Results, entityID []byte) *models.ProcessResult {
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

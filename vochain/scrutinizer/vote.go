package scrutinizer

import (
	"encoding/json"
	"fmt"
	"math"
	"math/big"
	"strings"
	"time"

	"github.com/timshannon/badgerhold/v3"
	"go.vocdoni.io/proto/build/go/models"

	"go.vocdoni.io/dvote/crypto/nacl"
	"go.vocdoni.io/dvote/log"
	"go.vocdoni.io/dvote/types"
)

// ErrNoResultsYet is an error returned to indicate the process exist but
// it does not have yet reuslts
var ErrNoResultsYet = fmt.Errorf("no results yet")

// The string to search on the KV database error to identify a transaction conflict.
// If the KV (currently badger) returns this error, it is considered non fatal and the
// transaction will be retried until it works.
// This check is made comparing string in order to avoid importing a specific KV
// implementation, thus let badgerhold abstract it.
const kvErrorStringForRetry = "Transaction Conflict"

// ComputeResult process a finished voting, compute the results and saves it in the Storage.
// Once this function is called, any future live vote event for the processId will be discarted.
func (s *Scrutinizer) ComputeResult(processID []byte) error {
	log.Debugf("computing results for %x", processID)

	// Get process from database
	p, err := s.ProcessInfo(processID)
	if err != nil {
		return fmt.Errorf("computeResult: cannot load processID %x from database: %w", processID, err)
	}

	// Compute the results
	var results *Results
	if results, err = s.computeFinalResults(p); err != nil {
		return err
	}
	p.HaveResults = true
	p.FinalResults = true
	if err := s.db.Update(processID, p); err != nil {
		return fmt.Errorf("computeResults: cannot update processID %x: %w, ", processID, err)
	}
	s.addVoteLock.Lock()
	defer s.addVoteLock.Unlock()
	if err := s.db.Upsert(processID, results); err != nil {
		return err
	}

	// Execute callbacks
	for _, l := range s.eventListeners {
		go l.OnComputeResults(results)
	}
	return nil
}

// GetResults returns the current result for a processId aggregated in a two dimension int slice
func (s *Scrutinizer) GetResults(processID []byte) (*Results, error) {
	if n, err := s.db.Count(&Process{},
		badgerhold.Where("ID").Eq(processID).
			And("HaveResults").Eq(true).
			And("Status").Ne(int32(models.ProcessStatus_CANCELED))); err != nil {
		return nil, err
	} else if n == 0 {
		return nil, ErrNoResultsYet
	}

	s.addVoteLock.RLock()
	defer s.addVoteLock.RUnlock()
	results := &Results{}
	if err := s.db.FindOne(results, badgerhold.Where("ProcessID").Eq(processID)); err != nil {
		if err == badgerhold.ErrNotFound {
			return nil, ErrNoResultsYet
		}
		return nil, err
	}
	return results, nil
}

// GetResultsWeight returns the current weight of cast votes for a processId.
func (s *Scrutinizer) GetResultsWeight(processID []byte) (*big.Int, error) {
	s.addVoteLock.RLock()
	defer s.addVoteLock.RUnlock()
	results := &Results{}
	if err := s.db.FindOne(results, badgerhold.Where("ProcessID").Eq(processID)); err != nil {
		return nil, err
	}
	return results.Weight, nil
}

// unmarshalVote decodes the base64 payload to a VotePackage struct type.
// If the votePackage is encrypted the list of keys to decrypt it should be provided.
// The order of the Keys must be as it was encrypted.
// The function will reverse the order and use the decryption keys starting from the
// last one provided.
func unmarshalVote(votePackage []byte, keys []string) (*types.VotePackage, error) {
	var vote types.VotePackage
	rawVote := make([]byte, len(votePackage))
	copy(rawVote, votePackage)
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
func (s *Scrutinizer) addLiveVote(envelope *models.Vote, results *Results) error {
	pid := envelope.GetProcessId()
	if pid == nil {
		return fmt.Errorf("cannot find process for envelope")
	}
	p, err := s.VochainState.Process(pid, false)
	if err != nil {
		return fmt.Errorf("cannot get process %x: %w", pid, err)
	}

	if p.EnvelopeType == nil {
		return fmt.Errorf("envelope type is nil")
	}

	// If live process, add vote to temporary results
	var vote *types.VotePackage
	if s.isProcessLiveResults(pid) {
		vote, err = unmarshalVote(envelope.GetVotePackage(), []string{})
		if err != nil {
			log.Warnf("cannot unmarshal vote: %v", err)
			vote = nil
		}
	}

	// Add weight to the process Results (if empty, consider weight=1)
	weight := new(big.Int).SetUint64(1)
	if w := envelope.GetWeight(); w != nil {
		weight = new(big.Int).SetBytes(w)
	}
	if results.Weight == nil {
		results.Weight = new(big.Int).SetUint64(0)
	}
	results.Weight.Add(results.Weight, weight)

	// Add the vote only if the election is unencrypted
	if vote != nil {
		if results.Votes, err = addVote(results.Votes,
			vote.Votes,
			weight,
			p.GetVoteOptions(),
			p.GetEnvelopeType()); err != nil {
			return err
		}
	}
	return nil
}

func (s *Scrutinizer) addProcessToLiveResults(pid []byte) {
	s.liveResultsProcs.Store(string(pid), true)
}

func (s *Scrutinizer) delProcessFromLiveResults(pid []byte) {
	s.liveResultsProcs.Delete(string(pid))
}

func (s *Scrutinizer) isProcessLiveResults(pid []byte) bool {
	_, ok := s.liveResultsProcs.Load(string(pid))
	return ok
}

// commitVotes adds the votes and weight from results to the local database.
// It does not overwrite the stored results but update them by adding the new content.
func (s *Scrutinizer) commitVotes(pid []byte, results *Results) error {
	s.addVoteLock.Lock()
	defer s.addVoteLock.Unlock()
	update := func(record interface{}) error {
		update, ok := record.(*Results)
		if !ok {
			return fmt.Errorf("record isn't the correct type! Wanted Result, got %T", record)
		}
		update.Weight.Add(update.Weight, results.Weight)

		if len(results.Votes) == 0 {
			return nil
		}

		if len(results.Votes) != len(update.Votes) {
			return fmt.Errorf("commitVotes: different length for results.Vote and update.Votes")
		}
		for i := range update.Votes {
			if len(update.Votes[i]) != len(results.Votes[i]) {
				return fmt.Errorf("commitVotes: different number of options for question %d", i)
			}
			for j := range update.Votes[i] {
				update.Votes[i][j].Add(update.Votes[i][j], results.Votes[i][j])
			}
		}

		return nil
	}

	// TODO (PAU):
	// Since the Commit() callback is executed on a goroutine, if scrutinizer have pending votes to add
	// and the program is stoped. The missing votes won't be added on next run.
	// Either a recovery mechanism or a persistent cache should be used in order to avoid this problem.

	var err error
	// If badgerhold returns a transaction conflict error message, we try again
	// until it works (while maxTries < 1000)
	maxTries := 1000
	for maxTries > 0 {
		err = s.db.UpdateMatching(&Results{},
			badgerhold.Where("ProcessID").Eq(pid).And("Final").Eq(false), update)
		if err == nil {
			break
		}
		if strings.Contains(err.Error(), kvErrorStringForRetry) {
			maxTries--
			time.Sleep(5 * time.Millisecond)
			continue
		}
		return err
	}
	if maxTries == 0 {
		err = fmt.Errorf("got too much transaction conflicts")
	}
	if err != nil {
		log.Debugf("saved %d votes with total weight of %s on process %x", len(results.Votes),
			results.Weight, pid)
	}
	return err
}

func (s *Scrutinizer) computeFinalResults(p *Process) (*Results, error) {
	if p == nil {
		return nil, fmt.Errorf("process is nil")
	}
	if p.VoteOpts.MaxCount == 0 || p.VoteOpts.MaxValue == 0 {
		return nil, fmt.Errorf("computeNonLiveResults: maxCount or maxValue are zero")
	}
	if p.VoteOpts.MaxCount > MaxQuestions || p.VoteOpts.MaxValue > MaxOptions {
		return nil, fmt.Errorf("maxCount or maxValue overflows hardcoded maximums")
	}
	results := &Results{
		Votes:     newEmptyVotes(int(p.VoteOpts.MaxCount), int(p.VoteOpts.MaxValue)+1),
		ProcessID: p.ID,
		Weight:    new(big.Int).SetUint64(0),
		Final:     true,
	}

	var nvotes int
	// 8.3M seems enough for now
	for _, e := range s.VochainState.EnvelopeList(p.ID, 0, 32<<18, false) {
		vote, err := s.VochainState.Envelope(p.ID, e, false)
		if err != nil {
			log.Warn(err)
			continue
		}
		var vp *types.VotePackage
		err = nil
		if p.Envelope.GetEncryptedVotes() {
			if len(p.PrivateKeys) < len(vote.GetEncryptionKeyIndexes()) {
				err = fmt.Errorf("encryptionKeyIndexes has too many fields")
			} else {
				keys := []string{}
				for _, k := range vote.GetEncryptionKeyIndexes() {
					if k >= types.KeyKeeperMaxKeyIndex {
						err = fmt.Errorf("key index overflow")
						break
					}
					keys = append(keys, p.PrivateKeys[k])
				}
				if len(keys) == 0 || err != nil {
					err = fmt.Errorf("no keys provided or wrong index")
				} else {
					vp, err = unmarshalVote(vote.GetVotePackage(), keys)
				}
			}
		} else {
			vp, err = unmarshalVote(vote.GetVotePackage(), []string{})
		}
		if err != nil {
			log.Debugf("vote invalid: %v", err)
			continue
		}
		weight := new(big.Int).SetUint64(1)
		if vw := vote.GetWeight(); len(vw) > 0 {
			weight = new(big.Int).SetBytes(vw)
		}
		results.Weight.Add(results.Weight, weight)
		if results.Votes, err = addVote(results.Votes,
			vp.Votes,
			weight,
			p.VoteOpts,
			p.Envelope); err != nil {
			log.Debugf("vote invalid: %v", err)
			continue
		}
		nvotes++
	}
	log.Infof("computed results for process %x with %d votes", p.ID, nvotes)
	log.Debugf("results: %s", results.String())
	return results, nil
}

func addVote(currentResults [][]*big.Int, voteValues []int, weight *big.Int,
	options *models.ProcessVoteOptions, envelopeType *models.EnvelopeType) ([][]*big.Int, error) {
	if len(currentResults) > 0 && len(currentResults) != int(options.MaxCount) {
		return currentResults, fmt.Errorf("addVote: currentResults size mismatch %d != %d",
			len(currentResults), options.MaxCount)
	}
	if options == nil {
		return currentResults, fmt.Errorf("addVote: processVoteOptions is nil")
	}
	if envelopeType == nil {
		return currentResults, fmt.Errorf("addVote: envelopeType is nil")
	}
	// MaxCount
	if len(voteValues) > int(options.MaxCount) || len(voteValues) > MaxOptions {
		return currentResults, fmt.Errorf("max count overflow %d",
			len(voteValues))
	}

	// UniqueValues
	if envelopeType.UniqueValues {
		votes := make(map[int]bool, len(voteValues))
		for _, v := range voteValues {
			if votes[v] {
				return currentResults, fmt.Errorf("values are not unique")
			}
			votes[v] = true
		}
	}

	// Max Value
	if options.MaxValue > 0 {
		for _, v := range voteValues {
			if uint32(v) > options.MaxValue {
				return currentResults, fmt.Errorf("max value overflow %d", v)
			}
		}
	}

	// Total cost
	if options.MaxTotalCost > 0 {
		cost := float64(0)
		for _, v := range voteValues {
			cost += math.Pow(float64(v), float64(options.CostExponent))
		}
		if cost > float64(options.MaxTotalCost) {
			return currentResults, fmt.Errorf("max total cost overflow: %f", cost)
		}
	}

	// If weight not provided, assume weight = 1
	if weight == nil {
		weight = new(big.Int).SetUint64(1)
	}
	if len(currentResults) == 0 {
		currentResults = newEmptyVotes(int(options.MaxCount), int(options.MaxValue)+1)
	}
	for q, opt := range voteValues {
		currentResults[q][opt].Add(currentResults[q][opt], weight)
	}

	return currentResults, nil
}

// BuildProcessResult takes the indexer Results type and builds the protobuf type ProcessResult.
// EntityId should be provided as addition field to include in ProcessResult.
func BuildProcessResult(results *Results, entityID []byte) *models.ProcessResult {
	// build the protobuf type for Results
	qr := []*models.QuestionResult{}
	for i := range results.Votes {
		qr := append(qr, &models.QuestionResult{})
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

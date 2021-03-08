package scrutinizer

import (
	"encoding/json"
	"fmt"
	"math"
	"math/big"

	"github.com/timshannon/badgerhold/v3"
	"go.vocdoni.io/proto/build/go/models"

	"go.vocdoni.io/dvote/crypto/nacl"
	"go.vocdoni.io/dvote/log"
	"go.vocdoni.io/dvote/types"
)

// ErrNoResultsYet is an error returned to indicate the process exist but
// it does not have yet reuslts
var ErrNoResultsYet = fmt.Errorf("no results yet")

// ComputeResult process a finished voting, compute the results and saves it in the Storage
func (s *Scrutinizer) ComputeResult(processID []byte) error {
	log.Debugf("computing results for %x", processID)
	// Get process from database
	p, err := s.ProcessInfo(processID)
	if err != nil {
		return fmt.Errorf("computeResult: cannot load processID %x from database: %w", processID, err)
	}
	// Compute the results
	// If poll-vote, results have been computed during their arrival
	isLive, err := s.isLiveResultsProcess(processID)
	if err != nil {
		return err
	}
	var pv *models.ProcessResult
	if isLive {
		if pv, err = s.computeLiveResults(processID); err != nil {
			return err
		}
	} else {
		if pv, err = s.computeNonLiveResults(p); err != nil {
			return err
		}
	}

	// add results if process is not live or isLive and status is ended
	if !isLive || (isLive && p.Status == int32(models.ProcessStatus_ENDED)) {
		for _, l := range s.eventListeners {
			pv.EntityId = p.EntityID
			pv.ProcessId = p.ID
			l.OnComputeResults(pv)
		}
	}
	p.HaveResults = true
	p.FinalResults = true
	if err := s.db.Update(processID, p); err != nil {
		return fmt.Errorf("computeResults: cannot update processID %x: %w, ", processID, err)
	}
	return s.db.Update(processID, &Results{
		Votes:     pv.Votes,
		ProcessID: processID,
	})
}

// GetResults returns the current result for a processId aggregated in a two dimension int slice
// TODO (pau): improve this approach, instead of 2 queries make just 1
func (s *Scrutinizer) GetResults(processID []byte) (*Results, error) {
	log.Debugf("finding results for %x", processID)
	if n, err := s.db.Count(&Process{},
		badgerhold.Where("ID").Eq(processID).
			And("HaveResults").Eq(true).
			And("Status").Ne(int32(models.ProcessStatus_CANCELED))); err != nil {
		return nil, err
	} else if n == 0 {
		return nil, ErrNoResultsYet
	}

	results := &Results{}
	if err := s.db.FindOne(results, badgerhold.Where("ProcessID").Eq(processID)); err != nil {
		if err == badgerhold.ErrNotFound {
			return nil, ErrNoResultsYet
		}
		return nil, err
	}
	return results, nil
}

// PrintResults returns a human friendly interpretation of the results.
func PrintResults(r *models.ProcessResult) (results string) {
	value := new(big.Int)
	for _, q := range r.Votes {
		results += " ["
		for j := range q.Question {
			value.SetBytes(q.Question[j])
			results += value.String()
			if j < len(q.Question)-1 {
				results += ","
			}
		}
		results += "]"
	}
	return results
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

func (s *Scrutinizer) addLiveResultsVote(envelope *models.Vote) error {
	pid := envelope.GetProcessId()
	if pid == nil {
		return fmt.Errorf("cannot find process for envelope")
	}
	p, err := s.VochainState.Process(pid, false)
	if err != nil {
		return fmt.Errorf("cannot get process %x: %w", pid, err)
	}
	vote, err := unmarshalVote(envelope.GetVotePackage(), []string{})
	if err != nil {
		return err
	}
	if len(vote.Votes) > MaxQuestions {
		return fmt.Errorf("too many elements on addVote")
	}

	return s.db.UpdateMatching(&Results{},
		badgerhold.Where("ProcessID").Eq(pid),
		func(record interface{}) error {
			update, ok := record.(*Results)
			if !ok {
				return fmt.Errorf("record isn't the correct type!  Wanted Result, got %T", record)
			}
			if err := addVote(update.Votes,
				vote.Votes,
				envelope.GetWeight(),
				p.GetVoteOptions(),
				p.GetEnvelopeType()); err != nil {
				return err
			}
			log.Debugf("addVote %v on process %x", vote.Votes, pid)
			return nil
		})
}

func (s *Scrutinizer) computeLiveResults(processID []byte) (*models.ProcessResult, error) {
	results := &Results{}
	if err := s.db.FindOne(results, badgerhold.Where("ProcessID").Eq(processID)); err != nil {
		return nil, err
	}
	return &models.ProcessResult{
		Votes:     results.Votes,
		ProcessId: processID,
	}, nil
}

func (s *Scrutinizer) computeNonLiveResults(p *Process) (*models.ProcessResult, error) {
	if p == nil {
		return nil, fmt.Errorf("process is nil")
	}
	if p.VoteOpts.MaxCount == 0 || p.VoteOpts.MaxValue == 0 {
		return nil, fmt.Errorf("computeNonLiveResults: maxCount or maxValue are zero")
	}

	if p.VoteOpts.MaxCount > MaxQuestions || p.VoteOpts.MaxValue > MaxOptions {
		return nil, fmt.Errorf("maxCount or maxValue overflows hardcoded maximums")
	}
	pv := newEmptyResults(int(p.VoteOpts.MaxCount), int(p.VoteOpts.MaxValue)+1)

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
		if err := addVote(pv.Votes,
			vp.Votes,
			vote.GetWeight(),
			p.VoteOpts,
			p.Envelope); err != nil {
			log.Debugf("vote invalid: %v", err)
			continue
		}
		nvotes++
	}
	log.Infof("computed results for process %x with %d votes", p.ID, nvotes)
	log.Debugf("results: %s", PrintResults(pv))
	return pv, nil
}

func addVote(currentResults []*models.QuestionResult,
	voteValues []int,
	weight []byte,
	options *models.ProcessVoteOptions,
	envelopeType *models.EnvelopeType) error {
	if options == nil {
		return fmt.Errorf("addVote: processVoteOptions is nil")
	}
	if envelopeType == nil {
		return fmt.Errorf("addVote: envelopeType is nil")
	}
	// MaxCount
	if len(voteValues) > int(options.MaxCount) || len(voteValues) > MaxOptions {
		return fmt.Errorf("max count overflow %d",
			len(voteValues))
	}

	// UniqueValues
	if envelopeType.UniqueValues {
		votes := make(map[int]bool, len(voteValues))
		for _, v := range voteValues {
			if votes[v] {
				return fmt.Errorf("values are not unique")
			}
			votes[v] = true
		}
	}

	// Max Value
	if options.MaxValue > 0 {
		for _, v := range voteValues {
			if uint32(v) > options.MaxValue {
				return fmt.Errorf("max value overflow %d", v)
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
			return fmt.Errorf("max total cost overflow: %f", cost)
		}
	}

	// If weight not provided, assume weight = 1
	if weight == nil {
		weight = new(big.Int).SetUint64(1).Bytes()
	}
	value := new(big.Int)
	iweight := new(big.Int)
	for q, opt := range voteValues {
		value.SetBytes(currentResults[q].Question[opt])
		value.Add(value, iweight.SetBytes(weight))
		currentResults[q].Question[opt] = value.Bytes()
	}

	return nil
}

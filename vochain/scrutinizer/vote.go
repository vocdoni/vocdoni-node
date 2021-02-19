package scrutinizer

import (
	"encoding/json"
	"fmt"
	"math"
	"math/big"

	"github.com/dgraph-io/badger/v2"
	"go.vocdoni.io/proto/build/go/models"
	"google.golang.org/protobuf/proto"

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
	// Check if process exist
	p, err := s.VochainState.Process(processID, false)
	if err != nil {
		return err
	}

	// If result already exist, skipping
	_, err = s.Storage.Get(s.Encode("results", processID))
	if err == nil {
		return fmt.Errorf("process %x already computed", processID)
	}
	if err != badger.ErrKeyNotFound {
		return err
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
		// Delete liveResults temporary storage
		if err = s.Storage.Del(s.Encode("liveProcess", processID)); err != nil {
			return err
		}
	} else {
		if pv, err = s.computeNonLiveResults(p); err != nil {
			return err
		}
	}

	// add results if process is not live or isLive and status is ended
	if !isLive || (isLive && p.Status == models.ProcessStatus_ENDED) {
		for _, l := range s.eventListeners {
			pv.EntityId = p.EntityId
			pv.ProcessId = p.ProcessId
			l.OnComputeResults(pv)
		}
	}

	result, err := proto.Marshal(pv)
	if err != nil {
		return err
	}

	return s.Storage.Put(s.Encode("results", processID), result)
}

// VoteResult returns the current result for a processId summarized in a two dimension int slice
func (s *Scrutinizer) VoteResult(processID []byte) (*models.ProcessResult, error) {
	// Check if process exist
	_, err := s.VochainState.Process(processID, false)
	if err != nil {
		return nil, err
	}
	log.Debugf("finding results for %x", processID)
	// If exist a summary of the voting process, just return it
	var pv models.ProcessResult
	processBytes, err := s.Storage.Get(s.Encode("results", processID))
	if err != nil && err != badger.ErrKeyNotFound {
		return nil, err
	}
	if err == nil {
		if err := proto.Unmarshal(processBytes, &pv); err != nil {
			return nil, err
		}
		return &pv, nil
	}

	// If results are not available, check if the process is PollVote (live)
	isLive, err := s.isLiveResultsProcess(processID)
	if err != nil {
		return nil, err
	}
	if !isLive {
		return nil, ErrNoResultsYet
	}

	// Return live results
	return s.computeLiveResults(processID)
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
		return fmt.Errorf("too many questions on addVote")
	}
	processBytes, err := s.Storage.Get(s.Encode("liveProcess", pid))
	if err != nil {
		return fmt.Errorf("error adding vote to process %x, skipping addVote: (%s)",
			pid, err)
	}

	var pv models.ProcessResult
	if err := proto.Unmarshal(processBytes, &pv); err != nil {
		return fmt.Errorf("cannot unmarshal vote (%s)", err)
	}
	if err := addVote(pv.Votes,
		vote.Votes,
		envelope.GetWeight(),
		p.GetVoteOptions(),
		p.GetEnvelopeType()); err != nil {
		return err
	}

	processBytes, err = proto.Marshal(&pv)
	if err != nil {
		return err
	}

	if err := s.Storage.Put(s.Encode("liveProcess", pid), processBytes); err != nil {
		return err
	}

	log.Debugf("addVote %v on process %x", vote.Votes, pid)
	return nil
}

func (s *Scrutinizer) computeLiveResults(processID []byte) (*models.ProcessResult, error) {
	pv := new(models.ProcessResult)
	pb, err := s.Storage.Get(s.Encode("liveProcess", processID))
	if err != nil {
		return nil, err
	}
	if err = proto.Unmarshal(pb, pv); err != nil {
		return nil, err
	}
	return pv, nil
}

func (s *Scrutinizer) computeNonLiveResults(p *models.Process) (*models.ProcessResult, error) {
	options := p.GetVoteOptions()
	if options == nil {
		return nil, fmt.Errorf("computeNonLiveResults: vote options is nil")
	}
	if options.MaxCount == 0 || options.MaxValue == 0 {
		return nil, fmt.Errorf("computeNonLiveResults: maxCount or maxValue are zero")
	}

	if options.MaxCount > MaxQuestions || options.MaxValue > MaxOptions {
		return nil, fmt.Errorf("maxCount or maxValue overflows hardcoded maximums")
	}
	pv := emptyProcess(int(options.MaxCount), int(options.MaxValue)+1)

	var nvotes int
	pid := p.GetProcessId()
	// 8.3M seems enough for now
	for _, e := range s.VochainState.EnvelopeList(pid, 0, 32<<18, false) {
		vote, err := s.VochainState.Envelope(pid, e, false)
		if err != nil {
			log.Warn(err)
			continue
		}
		var vp *types.VotePackage
		err = nil
		if p.EnvelopeType.GetEncryptedVotes() {
			if len(p.GetEncryptionPrivateKeys()) < len(vote.GetEncryptionKeyIndexes()) {
				err = fmt.Errorf("encryptionKeyIndexes has too many fields")
			} else {
				keys := []string{}
				for _, k := range vote.GetEncryptionKeyIndexes() {
					if k >= types.KeyKeeperMaxKeyIndex {
						err = fmt.Errorf("key index overflow")
						break
					}
					keys = append(keys, p.EncryptionPrivateKeys[k])
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
			p.GetVoteOptions(),
			p.GetEnvelopeType()); err != nil {

			log.Debugf("vote invalid: %v", err)
			continue
		}
		nvotes++
	}
	log.Infof("computed results for process %x with %d votes", pid, nvotes)
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

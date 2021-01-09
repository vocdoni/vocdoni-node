package scrutinizer

import (
	"encoding/json"
	"fmt"
	"math/big"

	"github.com/dgraph-io/badger/v2"
	"go.vocdoni.io/proto/build/go/models"
	"google.golang.org/protobuf/proto"

	"go.vocdoni.io/dvote/crypto/nacl"
	"go.vocdoni.io/dvote/events"
	"go.vocdoni.io/dvote/log"
	"go.vocdoni.io/dvote/types"
)

// ErrNoResultsYet is an error returned to indicate the process exist but it does not have yet reuslts
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
	if (!isLive || (isLive && p.Status == models.ProcessStatus_ENDED)) && s.EventDispatcher != nil {
		go s.EventDispatcher.Collect(pv, events.ScrutinizerOnprocessresult)
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
// The function will reverse the order and use the decryption keys starting from the last one provided.
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
	if envelope.ProcessId == nil {
		return fmt.Errorf("cannot find process for envelope")
	}
	vote, err := unmarshalVote(envelope.VotePackage, []string{})
	if err != nil {
		return err
	}
	if len(vote.Votes) > MaxQuestions {
		return fmt.Errorf("too many questions on addVote")
	}
	processBytes, err := s.Storage.Get(s.Encode("liveProcess", envelope.ProcessId))
	if err != nil {
		return fmt.Errorf("error adding vote to process %x, skipping addVote: (%s)", envelope.ProcessId, err)
	}

	var pv models.ProcessResult
	if err := proto.Unmarshal(processBytes, &pv); err != nil {
		return fmt.Errorf("cannot unmarshal vote (%s)", err)
	}
	addVote(pv.Votes, vote.Votes, envelope.GetWeight())

	processBytes, err = proto.Marshal(&pv)
	if err != nil {
		return err
	}

	if err := s.Storage.Put(s.Encode("liveProcess", envelope.ProcessId), processBytes); err != nil {
		return err
	}

	log.Debugf("addVote on process %x", envelope.ProcessId)
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
	return pruneVoteResult(pv), nil
}

func (s *Scrutinizer) computeNonLiveResults(p *models.Process) (*models.ProcessResult, error) {
	pv := emptyProcess(0, 0)
	var nvotes int
	for _, e := range s.VochainState.EnvelopeList(p.ProcessId, 0, 32<<18, false) { // 8.3M seems enough for now
		vote, err := s.VochainState.Envelope(p.ProcessId, e, false)
		if err != nil {
			log.Warn(err)
			continue
		}
		var vp *types.VotePackage
		err = nil
		if p.EnvelopeType.EncryptedVotes {
			if len(p.EncryptionPrivateKeys) < len(vote.EncryptionKeyIndexes) {
				err = fmt.Errorf("encryptionKeyIndexes has too many fields")
			} else {
				keys := []string{}
				for _, k := range vote.EncryptionKeyIndexes {
					if k >= types.KeyKeeperMaxKeyIndex {
						err = fmt.Errorf("key index overflow")
						break
					}
					keys = append(keys, p.EncryptionPrivateKeys[k])
				}
				if len(keys) == 0 || err != nil {
					err = fmt.Errorf("no keys provided or wrong index")
				} else {
					vp, err = unmarshalVote(vote.VotePackage, keys)
				}
			}
		} else {
			vp, err = unmarshalVote(vote.VotePackage, []string{})
		}
		if err != nil {
			log.Warn(err)
			continue
		}
		addVote(pv.Votes, vp.Votes, vote.GetWeight())
		nvotes++
	}
	log.Infof("computed results for process %x with %d votes", p.ProcessId, nvotes)
	return pruneVoteResult(pv), nil
}

func addVote(currentResults []*models.QuestionResult, voteValues []int, weight []byte) {
	value := new(big.Int)
	iweight := new(big.Int)
	for q, opt := range voteValues {
		if opt > MaxOptions {
			log.Warn("option overflow on computeResult, skipping vote...")
			continue
		}
		value.SetBytes(currentResults[q].Question[opt])
		value.Add(value, iweight.SetBytes(weight))
		currentResults[q].Question[opt] = value.Bytes()
	}
}

// To-be-improved
func pruneVoteResult(pv *models.ProcessResult) *models.ProcessResult {
	value := new(big.Int)
	zero := big.NewInt(0)
	pvv := proto.Clone(pv).(*models.ProcessResult)
	var pvc models.ProcessResult
	min := MaxQuestions - 1
	for ; min >= 0; min-- { // find the real size of first dimension (questions with some answer)
		j := 0
		for ; j < MaxOptions; j++ {
			value.SetBytes(pvv.Votes[min].Question[j])
			if value.Cmp(zero) != 0 {
				//if pvv.Votes[min].Question[j] != 0 {
				break
			}
		}
		if j < MaxOptions {
			break
		} // we found a non-empty question, this is the min. Stop iteration.
	}

	for i := 0; i <= min; i++ { // copy the options for each question but pruning options too
		pvc.Votes = make([]*models.QuestionResult, i+1)
		for i2 := 0; i2 <= i; i2++ { // copy only the first non-zero values
			j2 := MaxOptions - 1
			for ; j2 >= 0; j2-- {
				value.SetBytes(pvv.Votes[i2].Question[j2])
				if value.Cmp(zero) != 0 {
					break
				}
			}
			pvc.Votes[i2] = new(models.QuestionResult)
			pvc.Votes[i2].Question = make([][]byte, j2+1)
			for qi := range pvv.Votes[i2].Question {
				if qi > j2 {
					break
				}
				pvc.Votes[i2].Question[qi] = make([]byte, len(pvv.Votes[i2].Question[qi]))
				copy(pvc.Votes[i2].Question[qi], pvv.Votes[i2].Question[qi])
			}
		}
	}
	log.Debugf("computed pruned results: %s", PrintResults(&pvc))
	return &pvc
}

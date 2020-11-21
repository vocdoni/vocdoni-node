package scrutinizer

import (
	"encoding/json"
	"fmt"

	"github.com/dgraph-io/badger/v2"
	"github.com/vocdoni/dvote-protobuf/build/go/models"
	"google.golang.org/protobuf/proto"

	"gitlab.com/vocdoni/go-dvote/crypto/nacl"
	"gitlab.com/vocdoni/go-dvote/log"
	"gitlab.com/vocdoni/go-dvote/types"
)

// ErrNoResultsYet is an error returned to indicate the process exist but it does not have yet reuslts
var ErrNoResultsYet = fmt.Errorf("no results yet")

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
	pid := envelope.ProcessId
	if pid == nil {
		return fmt.Errorf("cannot find process for envelope")
	}
	vote, err := unmarshalVote(envelope.VotePackage, []string{})
	if err != nil {
		return err
	}
	if len(vote.Votes) > MaxQuestions {
		return fmt.Errorf("too many questions on addVote")
	}
	processBytes, err := s.Storage.Get(s.encode("liveProcess", pid))
	if err != nil {
		return fmt.Errorf("error adding vote to process %x, skipping addVote: (%s)", pid, err)
	}

	var pv models.ProcessResult
	if err := proto.Unmarshal(processBytes, &pv); err != nil {
		return fmt.Errorf("cannot unmarshal vote (%s)", err)
	}

	for q, opt := range vote.Votes {
		if opt > MaxOptions {
			log.Warn("option overflow on addVote")
			continue
		}
		pv.Votes[q].Question[opt]++
	}

	processBytes, err = proto.Marshal(&pv)
	if err != nil {
		return err
	}

	if err := s.Storage.Put(s.encode("liveProcess", pid), processBytes); err != nil {
		return err
	}

	log.Debugf("addVote on process %x", pid)
	return nil
}

// ComputeResult process a finished voting, compute the results and saves it in the Storage
func (s *Scrutinizer) ComputeResult(processID []byte) error {
	log.Debugf("computing results for %x", processID)
	// Check if process exist
	p, err := s.VochainState.Process(processID, false)
	if err != nil {
		return err
	}

	// If result already exist, skipping
	_, err = s.Storage.Get(s.encode("results", processID))
	if err == nil {
		return fmt.Errorf("process %x already computed", processID)
	}
	if err != nil && err != badger.ErrKeyNotFound {
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
		if err = s.Storage.Del(s.encode("liveProcess", processID)); err != nil {
			return err
		}
	} else {
		if pv, err = s.computeNonLiveResults(processID, p); err != nil {
			return err
		}
	}

	result, err := proto.Marshal(pv)
	if err != nil {
		return err
	}

	return s.Storage.Put(s.encode("results", processID), result)
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
	processBytes, err := s.Storage.Get(s.encode("results", processID))
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

func (s *Scrutinizer) computeLiveResults(processID []byte) (*models.ProcessResult, error) {
	var pb []byte
	var pv models.ProcessResult
	pb, err := s.Storage.Get(s.encode("liveProcess", processID))
	if err != nil {
		return nil, err
	}
	if err = proto.Unmarshal(pb, &pv); err != nil {
		return nil, err
	}
	pruneVoteResult(&pv)
	log.Debugf("computed live results for %x", processID)
	return &pv, nil
}

func (s *Scrutinizer) computeNonLiveResults(processID []byte, p *models.Process) (*models.ProcessResult, error) {
	pv := emptyProcess(0, 0)
	var nvotes int
	for _, e := range s.VochainState.EnvelopeList(processID, 0, 32<<18, false) { // 8.3M seems enough for now
		v, err := s.VochainState.Envelope(processID, e, false)
		if err != nil {
			log.Warn(err)
			continue
		}
		var vp *types.VotePackage
		err = nil
		if p.EnvelopeType.EncryptedVotes {
			if len(p.EncryptionPrivateKeys) < len(v.EncryptionKeyIndexes) {
				err = fmt.Errorf("encryptionKeyIndexes has too many fields")
			} else {
				keys := []string{}
				for _, k := range v.EncryptionKeyIndexes {
					if k >= types.MaxKeyIndex {
						err = fmt.Errorf("key index overflow")
						break
					}
					keys = append(keys, p.EncryptionPrivateKeys[k])
				}
				if len(keys) == 0 || err != nil {
					err = fmt.Errorf("no keys provided or wrong index")
				} else {
					vp, err = unmarshalVote(v.VotePackage, keys)
				}
			}
		} else {
			vp, err = unmarshalVote(v.VotePackage, []string{})
		}
		if err != nil {
			log.Warn(err)
			continue
		}
		for question, opt := range vp.Votes {
			if opt > MaxOptions {
				log.Warn("option overflow on computeResult, skipping vote...")
				continue
			}
			pv.Votes[question].Question[opt]++
		}
		nvotes++
	}
	pruneVoteResult(pv)
	log.Infof("computed results for process %x with %d votes", processID, nvotes)
	return pv, nil
}

// To-be-improved
func pruneVoteResult(pv *models.ProcessResult) {
	pvv := proto.Clone(pv).(*models.ProcessResult)
	var pvc models.ProcessResult
	min := MaxQuestions - 1
	for ; min >= 0; min-- { // find the real size of first dimension (questions with some answer)
		j := 0
		for ; j < MaxOptions; j++ {
			if pvv.Votes[min].Question[j] != 0 {
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
				if pvv.Votes[i2].Question[j2] != 0 {
					break
				}
			}
			pvc.Votes[i2].Question = make([]uint32, j2+1)
			copy(pvc.Votes[i2].Question, pvv.Votes[i2].Question)

		}
	}
	pv = proto.Clone(&pvc).(*models.ProcessResult)
}

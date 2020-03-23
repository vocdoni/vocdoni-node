package scrutinizer

import (
	"encoding/base64"
	"encoding/json"
	"strings"

	"gitlab.com/vocdoni/go-dvote/db"
	"gitlab.com/vocdoni/go-dvote/log"
	"gitlab.com/vocdoni/go-dvote/types"
	"gitlab.com/vocdoni/go-dvote/util"
	"gitlab.com/vocdoni/go-dvote/vochain"
)

const (
	// MaxQuestions is the maximum number of questions allowed in a VotePackage
	MaxQuestions = 64
	// MaxOptions is the maximum number of options allowed in a VotePackage question
	MaxOptions = 64
)

// Scrutinizer is the component which makes the accounting of the voting processes and keeps it indexed in a local database
type Scrutinizer struct {
	VochainState *vochain.State
	Storage      *db.LevelDbStorage
}

// ProcessVotes represents the results of a voting process using a two dimensions slice [ question1:[option1,option2], question2:[option1,option2], ...]
type ProcessVotes [][]uint32

// NewScrutinizer returns an instance of the Scrutinizer
// using the local storage database of dbPath and integrated into the state vochain instance
func NewScrutinizer(dbPath string, state *vochain.State) (*Scrutinizer, error) {
	var s Scrutinizer
	var err error
	s.VochainState = state
	s.Storage, err = db.NewLevelDbStorage(dbPath, false)
	s.VochainState.AddCallback("addProcess", s.onProcess)
	s.VochainState.AddCallback("addVote", s.onVote)
	return &s, err
}

func (s *Scrutinizer) onProcess(v interface{}) {
	d := v.(*types.ScrutinizerOnProcessData)
	if util.IsHexWithLength(d.ProcessID, 64) && util.IsHexWithLength(d.EntityID, 64) {
		s.addProcess(d.ProcessID)
		s.addEntity(d.EntityID)
	}
}

func (s *Scrutinizer) onVote(v interface{}) {
	d := v.(*types.Vote)
	if util.IsHexWithLength(d.ProcessID, 64) &&
		util.IsHexWithLength(d.Nullifier, 64) &&
		util.IsHexWithLength(d.Nonce, 64) &&
		util.IsHexWithLength(d.Signature, 130) {
		s.addVote(d)
	}
}

func (s *Scrutinizer) addEntity(eid string) {
	log.Debugf("add new entity %s to scrutinizer local database", eid)
	entity, err := s.Storage.Get([]byte(types.ScrutinizerEntityPrefix + eid))
	if err == nil || len(entity) > 0 {
		log.Debugf("entity %s already exists", eid)
		return
	}

	entityBytes, err := s.VochainState.Codec.MarshalBinaryBare([]byte{})
	if err != nil {
		log.Error(err)
		return
	}

	if err := s.Storage.Put([]byte(types.ScrutinizerEntityPrefix+eid), entityBytes); err != nil {
		log.Error(err)
		return
	}
	log.Infof("entity %s added", eid)
}

func (s *Scrutinizer) addProcess(pid string) {
	log.Debugf("add new process %s to scrutinizer local database", pid)
	process, err := s.Storage.Get([]byte(types.ScrutinizerProcessPrefix + pid))
	if err == nil || len(process) > 0 {
		log.Errorf("process %s already exist!", pid)
		return
	}
	pv := make([][]uint32, MaxQuestions)
	for i := range pv {
		pv[i] = make([]uint32, MaxOptions)
	}

	process, err = s.VochainState.Codec.MarshalBinaryBare(pv)
	if err != nil {
		log.Error(err)
		return
	}

	if err := s.Storage.Put([]byte(types.ScrutinizerProcessPrefix+pid), process); err != nil {
		log.Error(err)
		return
	}
	log.Infof("process %s added", pid)
}

func (s *Scrutinizer) addVote(envelope *types.Vote) {
	rawVote, err := base64.StdEncoding.DecodeString(envelope.VotePackage)
	if err != nil {
		log.Error(err)
		return
	}

	var vote types.VotePackage
	if err := json.Unmarshal(rawVote, &vote); err != nil {
		log.Error(err)
		return
	}
	if len(vote.Votes) > MaxQuestions {
		log.Error("too many questions on addVote")
		return
	}

	process, err := s.Storage.Get([]byte(types.ScrutinizerProcessPrefix + envelope.ProcessID))
	if err != nil {
		log.Warnf("process %s does not exist, skipping addVote", envelope.ProcessID)
		return
	}
	var pv ProcessVotes

	if err := s.VochainState.Codec.UnmarshalBinaryBare(process, &pv); err != nil {
		log.Error("cannot unmarshal vote (%s)", err.Error())
		return
	}

	for question, opt := range vote.Votes {
		if opt > MaxOptions {
			log.Warn("option overflow on addVote")
			continue
		}
		pv[question][opt]++
	}

	process, err = s.VochainState.Codec.MarshalBinaryBare(pv)
	if err != nil {
		log.Error(err)
		return
	}

	log.Debugf("addVote on process %s", envelope.ProcessID)
	if err := s.Storage.Put([]byte(types.ScrutinizerProcessPrefix+envelope.ProcessID), process); err != nil {
		log.Error(err)
	}
}

// ProcessInfo returns the available information regarding an election process id
func (s *Scrutinizer) ProcessInfo(processID string) (*types.Process, error) {
	return s.VochainState.Process(processID)
}

// VoteResult returns the current result for a processId summarized in a two dimension int slice
func (s *Scrutinizer) VoteResult(processID string) ([][]uint32, error) {
	processBytes, err := s.Storage.Get([]byte(types.ScrutinizerProcessPrefix + processID))
	if err != nil {
		return nil, err
	}
	var pv ProcessVotes
	if err := s.VochainState.Codec.UnmarshalBinaryBare(processBytes, &pv); err != nil {
		return nil, err
	}
	return pruneVoteResult(pv), nil
}

// List returns a list of keys matching a given prefix
func (s *Scrutinizer) List(max int, from, prefix string) (list []string) {
	iter := s.Storage.LevelDB().NewIterator(nil, nil)
	if len(from) > 0 {
		iter.Seek([]byte(from))
	}
	for iter.Next() {
		if max < 1 {
			break
		}
		if strings.HasPrefix(string(iter.Key()), prefix) {
			list = append(list, string(iter.Key()[2:]))
			max--
		}
	}
	iter.Release()
	return
}

// To-be-improved
func pruneVoteResult(pv ProcessVotes) ProcessVotes {
	var pvc [][]uint32
	min := MaxQuestions - 1
	for ; min >= 0; min-- { // find the real size of first dimension (questions with some answer)
		j := 0
		for ; j < MaxOptions; j++ {
			if pv[min][j] != 0 {
				break
			}
		}
		if j < MaxOptions {
			break
		} // we found a non-empty question, this is the min. Stop iteration.
	}

	for i := 0; i <= min; i++ { // copy the options for each question but pruning options too
		pvc = make([][]uint32, i+1)
		for i2 := 0; i2 <= i; i2++ { // copy only the first non-zero values
			j2 := MaxOptions - 1
			for ; j2 >= 0; j2-- {
				if pv[i2][j2] != 0 {
					break
				}
			}
			pvc[i2] = make([]uint32, j2+1)
			copy(pvc[i2], pv[i2])
		}
	}
	return pvc
}

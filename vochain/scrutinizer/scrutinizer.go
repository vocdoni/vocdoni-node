package scrutinizer

import (
	"encoding/base64"
	"encoding/json"
	amino "github.com/tendermint/go-amino"
	"gitlab.com/vocdoni/go-dvote/db"
	"gitlab.com/vocdoni/go-dvote/log"
	"gitlab.com/vocdoni/go-dvote/types"
	"gitlab.com/vocdoni/go-dvote/vochain"
)

const (
	MaxQuestions = 64
	MaxOptions   = 64
)

type Scrutinizer struct {
	VochainState *vochain.State
	Storage      *db.LevelDbStorage
	Codec        *amino.Codec
}

type VotePackage struct {
	Votes []int `json:"votes"`
}

type ProcessVotes [][]uint32

func NewScrutinizer(dbPath string, state *vochain.State) (*Scrutinizer, error) {
	var s Scrutinizer
	var err error
	s.VochainState = state
	s.Codec = s.VochainState.Codec
	s.Storage, err = db.NewLevelDbStorage(dbPath, false)
	s.VochainState.AddCallback("addVote", s.addVote)
	return &s, err
}

func (s *Scrutinizer) addVote(v interface{}) {
	envelope := v.(*types.Vote)
	rawVote, err := base64.StdEncoding.DecodeString(envelope.VotePackage)
	if err != nil {
		log.Error(err)
		return
	}

	var vote VotePackage
	if err := json.Unmarshal(rawVote, &vote); err != nil {
		log.Error(err)
		return
	}
	if len(vote.Votes) > MaxQuestions {
		log.Error("too many questions on addVote")
		return
	}

	process, err := s.Storage.Get([]byte(envelope.ProcessID))
	var pv ProcessVotes

	if err != nil {
		log.Debugf("add new process %s to scrutinizer local database", envelope.ProcessID)
		pv = make([][]uint32, MaxQuestions)
		for i := range pv {
			pv[i] = make([]uint32, MaxOptions)
		}
		for question, opt := range vote.Votes {
			if opt > MaxOptions {
				log.Warn("option overflow on addVote")
				continue
			}
			pv[question][opt] = 1
		}
	} else {
		err = s.Codec.UnmarshalBinaryBare(process, &pv)
		if err != nil {
			log.Error("cannot unmarshal process votes (%s)", err.Error())
			return
		}
		for question, opt := range vote.Votes {
			if opt > MaxOptions {
				log.Warn("option overflow on addVote")
				continue
			}
			pv[question][opt]++
		}
	}

	process, err = s.Codec.MarshalBinaryBare(pv)
	if err != nil {
		log.Error(err)
		return
	}

	log.Debugf("addVote on process %s", envelope.ProcessID)
	err = s.Storage.Put([]byte(envelope.ProcessID), process)
	if err != nil {
		log.Error(err)
	}
}

func (s *Scrutinizer) VoteResult(processId string) ([][]uint32, error) {
	processBytes, err := s.Storage.Get([]byte(processId))
	if err != nil {
		return nil, err
	}
	var pv ProcessVotes
	err = s.Codec.UnmarshalBinaryBare(processBytes, &pv)
	if err != nil {
		return nil, err
	}
	return pv, nil
}

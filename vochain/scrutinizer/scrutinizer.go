package scrutinizer

/*
	Scrutinizer keeps 4 diferent database entries (splited by key prefix)

	+ ProcessEnding: key is block number. Used for schedule results computing
	+ LiveProcess: key is processId. Temporary storage for live results (poll-vote)
	+ Entity: key is entityId: List of known entities
	+ Results: key is processId: Final results for a process
*/

import (
	"strings"

	"gitlab.com/vocdoni/go-dvote/db"
	"gitlab.com/vocdoni/go-dvote/log"
	"gitlab.com/vocdoni/go-dvote/types"
	"gitlab.com/vocdoni/go-dvote/vochain"
)

const (
	// MaxQuestions is the maximum number of questions allowed in a VotePackage
	MaxQuestions = 64
	// MaxOptions is the maximum number of options allowed in a VotePackage question
	MaxOptions = 64
	//NoKeyStorageError is the database error returned if a key is not found
	NoKeyStorageError = "Key not found"
)

// Scrutinizer is the component which makes the accounting of the voting processes and keeps it indexed in a local database
type Scrutinizer struct {
	VochainState *vochain.State
	Storage      db.Database
	votePool     []*types.Vote
	processPool  []*types.ScrutinizerOnProcessData
	resultsPool  []*types.ScrutinizerOnProcessData
}

// ProcessVotes represents the results of a voting process using a two dimensions slice [ question1:[option1,option2], question2:[option1,option2], ...]
type ProcessVotes [][]uint32

// ProcessEndingList represents a list of ending voting processes
type ProcessEndingList []string

// NewScrutinizer returns an instance of the Scrutinizer
// using the local storage database of dbPath and integrated into the state vochain instance
func NewScrutinizer(dbPath string, state *vochain.State) (*Scrutinizer, error) {
	s := &Scrutinizer{VochainState: state}
	var err error
	s.Storage, err = db.NewBadgerDB(dbPath)
	if err != nil {
		return nil, err
	}
	s.VochainState.AddEventListener(s)
	return s, nil
}

// Commit is called by the APP when a block is confirmed and included into the chain
func (s *Scrutinizer) Commit(height int64) {
	// Check if there are processes that need results computing
	// this can be run async
	go s.checkFinishedProcesses(height)

	// Add Entity and register new active process
	var isLive bool
	var err error
	var nvotes int64
	for _, p := range s.processPool {
		s.addEntity(p.EntityID, p.ProcessID)
		if isLive, err = s.isLiveResultsProcess(p.ProcessID); err != nil {
			log.Errorf("cannot check if process is live results: (%s)", err)
			continue
		}
		if isLive {
			s.addLiveResultsProcess(p.ProcessID)
		}
	}

	for i, p := range s.resultsPool {
		s.registerPendingProcess(p.ProcessID, height+int64(i+1))
	}

	// Add votes collected by onVote (live results)
	for _, v := range s.votePool {
		if err = s.addLiveResultsVote(v); err != nil {
			log.Errorf("cannot add live vote: (%s)", err)
			continue
		}
		nvotes++
	}
	if nvotes > 0 {
		log.Infof("added %d live votes from block %d", nvotes, height)
	}
}

//Rollback removes the non commited pending operations
func (s *Scrutinizer) Rollback() {
	s.votePool = []*types.Vote{}
	s.processPool = []*types.ScrutinizerOnProcessData{}
	s.resultsPool = []*types.ScrutinizerOnProcessData{}
}

// OnProcess scrutinizer stores the processID and entityID
func (s *Scrutinizer) OnProcess(pid, eid string) {
	var data = types.ScrutinizerOnProcessData{EntityID: eid, ProcessID: pid}
	s.processPool = append(s.processPool, &data)
}

// OnVote scrutinizer stores the votes if liveResults enabled
func (s *Scrutinizer) OnVote(v *types.Vote) {
	isLive, err := s.isLiveResultsProcess(v.ProcessID)
	if err != nil {
		log.Errorf("cannot check if process is live results: (%s)", err)
		return
	}
	if isLive {
		s.votePool = append(s.votePool, v)
	}
}

// OnCancel scrutinizer stores the processID and entityID
func (s *Scrutinizer) OnCancel(pid string) {
	// TBD: compute final live results?
}

// OnProcessKeys does nothing
func (s *Scrutinizer) OnProcessKeys(pid, pub, com string) {
	// do nothing
}

// OnRevealKeys checks if all keys have been revealed and in such case add the process to the results queue
func (s *Scrutinizer) OnRevealKeys(pid, pub, com string) {
	p, err := s.VochainState.Process(pid, false)
	if err != nil {
		log.Errorf("cannot fetch process %s from state: (%s)", pid, err)
		return
	}
	// if all keys have been revealed, compute the results
	if p.KeyIndex < 1 {
		data := types.ScrutinizerOnProcessData{EntityID: p.EntityID, ProcessID: pid}
		s.resultsPool = append(s.resultsPool, &data)
	}
}

// List returns a list of keys matching a given prefix
func (s *Scrutinizer) List(max int, from, prefix string) (list []string) {
	iter := s.Storage.NewIterator().(*db.BadgerIterator) // TODO(mvdan): don't type assert
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

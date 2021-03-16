package scrutinizer

import (
	"math/big"

	"github.com/timshannon/badgerhold/v3"
	"go.vocdoni.io/dvote/log"
	"go.vocdoni.io/dvote/types"
	"go.vocdoni.io/dvote/vochain"
	"go.vocdoni.io/proto/build/go/models"
)

const (
	// MaxQuestions is the maximum number of questions allowed in a VotePackage
	MaxQuestions = 64
	// MaxOptions is the maximum number of options allowed in a VotePackage question
	MaxOptions = 64
)

// EventListener is an interface used for executing custom functions during the
// events of the tally of a process.
type EventListener interface {
	OnComputeResults(results *models.ProcessResult)
}

// AddEventListener adds a new event listener, to receive method calls on block
// events as documented in EventListener.
func (s *Scrutinizer) AddEventListener(l EventListener) {
	s.eventListeners = append(s.eventListeners, l)
}

// Scrutinizer is the component which makes the accounting of the voting processes
// and keeps it indexed in a local database.
type Scrutinizer struct {
	VochainState *vochain.State
	// votePool is the list of votes that will be added on the current block
	votePool []*models.Vote
	// newProcessPool is the list of new process IDs on the current block
	newProcessPool []*types.ScrutinizerOnProcessData
	// updateProcessPool is the list of process IDs that require sync with the state database
	updateProcessPool [][]byte
	// resultsPool is the list of processes that finish on the current block
	resultsPool []*types.ScrutinizerOnProcessData
	// eventListeners is the list of external callbacks that will be executed by the scrutinizer
	eventListeners []EventListener
	db             *badgerhold.Store
}

// NewScrutinizer returns an instance of the Scrutinizer
// using the local storage database of dbPath and integrated into the state vochain instance
func NewScrutinizer(dbPath string, state *vochain.State) (*Scrutinizer, error) {
	s := &Scrutinizer{VochainState: state}
	var err error
	s.db, err = InitDB(dbPath)
	if err != nil {
		return nil, err
	}
	s.VochainState.AddEventListener(s)
	return s, nil
}

// Commit is called by the APP when a block is confirmed and included into the chain
func (s *Scrutinizer) Commit(height int64) {
	// Add Entity and register new active process
	var nvotes int64
	for _, p := range s.newProcessPool {
		if err := s.newEmptyProcess(p.ProcessID); err != nil {
			log.Warnf("commit: cannot create new empty process: %v", err)
		}
	}

	// Add votes collected by onVote (live results)
	for _, v := range s.votePool {
		if err := s.addLiveVote(v); err != nil {
			log.Errorf("cannot add live vote: (%s)", err)
			continue
		}
		nvotes++
	}
	if nvotes > 0 {
		log.Infof("added %d live votes from block %d", nvotes, height)
	}

	// Update existing processes
	for _, p := range s.updateProcessPool {
		if err := s.updateProcess(p); err != nil {
			log.Warnf("commit: cannot update process %x: %v", p, err)
		}
	}
	for _, p := range s.resultsPool {
		if err := s.setResultsHeight(p.ProcessID, uint32(height+1)); err != nil {
			log.Warnf("commit: cannot update process %x: %v", p.ProcessID, err)
		}
		log.Infof("scheduled results computation on next block for %x", p.ProcessID)
	}

	// Check if there are processes that need results computing
	// this can be run async
	go s.computePendingProcesses(uint32(height))
}

// Rollback removes the non committed pending operations
func (s *Scrutinizer) Rollback() {
	s.votePool = []*models.Vote{}
	s.newProcessPool = []*types.ScrutinizerOnProcessData{}
	s.resultsPool = []*types.ScrutinizerOnProcessData{}
	s.updateProcessPool = [][]byte{}
}

// OnProcess scrutinizer stores the processID and entityID
func (s *Scrutinizer) OnProcess(pid, eid []byte, censusRoot, censusURI string) {
	data := &types.ScrutinizerOnProcessData{EntityID: eid, ProcessID: pid}
	s.newProcessPool = append(s.newProcessPool, data)
}

// OnVote scrutinizer stores the votes if liveResults enabled
func (s *Scrutinizer) OnVote(v *models.Vote) {
	s.votePool = append(s.votePool, v)
}

// OnCancel scrutinizer stores the processID and entityID
func (s *Scrutinizer) OnCancel(pid []byte) {
	s.updateProcessPool = append(s.updateProcessPool, pid)
	// TODO: if live, should set ResultsHeight to 0 and HaveResults to false
}

// OnProcessKeys does nothing
func (s *Scrutinizer) OnProcessKeys(pid []byte, pub, commit string) {
	s.updateProcessPool = append(s.updateProcessPool, pid)
}

func (s *Scrutinizer) OnProcessStatusChange(pid []byte, status models.ProcessStatus) {
	if status == models.ProcessStatus_ENDED {
		if live, err := s.isLiveResultsProcess(pid); err != nil {
			log.Warn(err)
		} else if live {
			s.resultsPool = append(s.resultsPool, &types.ScrutinizerOnProcessData{ProcessID: pid})
		}
	}
	s.updateProcessPool = append(s.updateProcessPool, pid)
}

// OnRevealKeys checks if all keys have been revealed and in such case add the
// process to the results queue
func (s *Scrutinizer) OnRevealKeys(pid []byte, priv, reveal string) {
	p, err := s.VochainState.Process(pid, false)
	if err != nil {
		log.Errorf("cannot fetch process %s from state: (%s)", pid, err)
		return
	}
	if p.KeyIndex == nil {
		log.Errorf("keyindex is nil")
		return
	}
	// if all keys have been revealed, compute the results
	if *p.KeyIndex < 1 {
		data := types.ScrutinizerOnProcessData{EntityID: p.EntityId, ProcessID: pid}
		s.resultsPool = append(s.resultsPool, &data)
	}
	s.updateProcessPool = append(s.updateProcessPool, pid)
}

// Temporary until we use Protobuf for the API
func (s *Scrutinizer) GetFriendlyResults(votes []*models.QuestionResult) [][]string {
	r := [][]string{}
	value := new(big.Int)
	for i, v := range votes {
		r = append(r, []string{})
		for j := range v.Question {
			value.SetBytes(v.Question[j])
			r[i] = append(r[i], value.String())
		}
	}
	return r
}

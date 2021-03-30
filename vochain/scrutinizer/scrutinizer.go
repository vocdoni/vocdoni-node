package scrutinizer

import (
	"bytes"
	"fmt"
	"math/big"
	"sync"
	"time"

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
	OnComputeResults(results *Results)
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
	votePool map[string][]*models.Vote
	// newProcessPool is the list of new process IDs on the current block
	newProcessPool []*types.ScrutinizerOnProcessData
	// updateProcessPool is the list of process IDs that require sync with the state database
	updateProcessPool [][]byte
	// resultsPool is the list of processes that finish on the current block
	resultsPool []*types.ScrutinizerOnProcessData
	// list of live processes (those on which the votes will be computed on arrival)
	liveResultsProcs sync.Map
	// eventListeners is the list of external callbacks that will be executed by the scrutinizer
	eventListeners []EventListener
	db             *badgerhold.Store
	addVoteLock    sync.RWMutex
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
	go s.AfterSyncBootstrap()
	return s, nil
}

func (s *Scrutinizer) AfterSyncBootstrap() {
	// Some grace time to avoid false positive on IsSynchronizing()
	time.Sleep(time.Second * 10)
	for s.VochainState.IsSynchronizing() {
		time.Sleep(time.Second * 2)
	}
	log.Infof("running scrutinizer after-sync bootstrap")
	prcs := bytes.Buffer{}
	err := s.db.ForEach(
		badgerhold.Where("FinalResults").Eq(false),
		func(p *Process) error {
			if !p.Envelope.EncryptedVotes {
				s.addProcessToLiveResults(p.ID)
				prcs.WriteString(fmt.Sprintf("%x ", p.ID))
			}
			return nil
		})
	if err != nil {
		log.Error(err)
	}
	log.Infof("current live results processes: %s", prcs.String())
	// TODO: COUNT ALL PENDING LIVE VOTES
}

// Commit is called by the APP when a block is confirmed and included into the chain
func (s *Scrutinizer) Commit(height int64) (error, bool) {
	// Add Entity and register new active process
	for _, p := range s.newProcessPool {
		if err := s.newEmptyProcess(p.ProcessID); err != nil {
			log.Errorf("commit: cannot create new empty process: %v", err)
			continue
		}
		if live, err := s.isOpenProcess(p.ProcessID); err != nil {
			log.Errorf("cannot check if process is live results: %v", err)

		} else if live && !s.VochainState.IsSynchronizing() {
			// Only add live processes if the vochain is not synchronizing
			s.addProcessToLiveResults(p.ProcessID)

		}
	}

	// Update existing processes
	for _, p := range s.updateProcessPool {
		if err := s.updateProcess(p); err != nil {
			log.Errorf("commit: cannot update process %x: %v", p, err)
			continue
		}
	}

	// Schedule results computation
	for _, p := range s.resultsPool {
		if err := s.setResultsHeight(p.ProcessID, uint32(height+1)); err != nil {
			log.Errorf("commit: cannot update process %x: %v", p.ProcessID, err)
			continue
		}
		s.delProcessFromLiveResults(p.ProcessID)
		log.Infof("scheduled results computation on next block for %x", p.ProcessID)
	}

	// Check if there are processes that need results computing
	// this can be run async
	go s.computePendingProcesses(uint32(height))

	// Add votes collected by onVote (live results)
	// this can be run async
	go func() {
		nvotes := 0
		timer := time.Now()
		for pid, votes := range s.votePool {
			results := &Results{Weight: new(big.Int).SetUint64(0)}
			for _, v := range votes {
				if err := s.addLiveVote(v, results); err != nil {
					log.Warnf("vote cannot be added: %v", err)
				} else {
					nvotes++
				}
			}
			if err := s.commitVotes([]byte(pid), results); err != nil {
				log.Errorf("cannot commit live votes: (%v)", err)
			}
		}
		if nvotes > 0 {
			log.Infof("added %d live votes on block %d, took %d ms",
				nvotes, height, time.Since(timer).Milliseconds())
		}
	}()
	return nil, false
}

// Rollback removes the non committed pending operations
func (s *Scrutinizer) Rollback() {
	s.votePool = make(map[string][]*models.Vote)
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
	if _, ok := s.votePool[string(v.ProcessId)]; ok {
		s.votePool[string(v.ProcessId)] = append(s.votePool[string(v.ProcessId)], v)
	} else {
		s.votePool[string(v.ProcessId)] = []*models.Vote{v}
	}
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
		if live, err := s.isOpenProcess(pid); err != nil {
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

func (s *Scrutinizer) OnProcessResults(pid []byte, results []*models.QuestionResult) error {
	// TODO: check results are valid and return an error if not.
	// This is very dangerous since an Oracle would be able to create a consensus failure,
	// the validaros (that do not check the results) and the full-nodes (with the scrutinizer enabled)
	// would compute different state hash.
	// As a temporary solution, lets compare results but do not return error.
	myResults, err := s.GetResults(pid)
	if err != nil || myResults == nil {
		log.Errorf("cannot validate results: %v", err)
		return nil
	}
	myVotes := BuildProcessResult(myResults, nil).GetVotes()
	if len(myVotes) != len(results) {
		log.Errorf("results validation failed: wrong result questions size")
		return nil
	}
	for i, q := range results {
		if len(q.Question) != len(myVotes[i].Question) {
			log.Errorf("results validation failed: wrong question size")
			return nil
		}
		for j, v := range q.Question {
			if !bytes.Equal(v, myVotes[i].Question[j]) {
				log.Errorf("results validation failed: wrong question result")
				return nil
			}
		}
	}
	log.Infof("blockchain results for process %x are correct!", pid)
	return nil
}

func GetFriendlyResults(votes [][]*big.Int) [][]string {
	r := [][]string{}
	for i := range votes {
		r = append(r, []string{})
		for j := range votes[i] {
			r[i] = append(r[i], votes[i][j].String())
		}
	}
	return r
}

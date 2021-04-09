package scrutinizer

import (
	"bytes"
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
	// MaxEnvelopeListSize is the maximum number of envelopes a process can store.
	// 8.3M seems enough for now
	MaxEnvelopeListSize = 32 << 18
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
	App *vochain.BaseApplication
	// voteIndexPool is the list of votes that will be indexed in the database
	voteIndexPool []*VoteWithIndex
	// votePool is the list of votes that should be live counted, grouped by processId
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

	// addVoteLock is used to avoid Transaction Conflicts on the KV database.
	// It is not critical and the code should be able to recover from a Conflict, but we
	// try to minimize this situations in order to improve performance on the KV.
	// TODO (pau): remove this mutex and relay on the KV layer
	addVoteLock sync.RWMutex
	// recoveryBootLock prevents Commit() to add new votes while the recovery bootstratp is
	// being executed.
	recoveryBootLock sync.RWMutex
}

// VoteWithIndex holds a Vote and a txIndex. Model for the VotePool.
type VoteWithIndex struct {
	vote    *models.Vote
	txIndex int32
}

// NewScrutinizer returns an instance of the Scrutinizer
// using the local storage database of dbPath and integrated into the state vochain instance
func NewScrutinizer(dbPath string, app *vochain.BaseApplication) (*Scrutinizer, error) {
	s := &Scrutinizer{App: app}
	var err error
	s.db, err = InitDB(dbPath)
	if err != nil {
		return nil, err
	}
	s.App.State.AddEventListener(s)
	return s, nil
}

// AfterSyncBootstrap is a blocking function that waits until the Vochain is synchronized
// and then execute a set of recovery actions. It mainly checks for those processes which are
// still open (live) and updates all temporary data (current voting weight and live results
// if unecrypted). This method might be called on a goroutine after initializing the Scrutinizer.
func (s *Scrutinizer) AfterSyncBootstrap() {
	// During the first seconds/milliseconds of the Vochain startup, Tendermint might report that
	// the chain is not synchronizing since it still does not have any peer and do not know the
	// actual size of the blockchain. If afterSyncBootStrap is executed on this specific moment,
	// the Wait loop would pass.
	syncSignals := 5
	for {
		// Add some grace time to avoid false positive on IsSynchronizing()
		if !s.App.IsSynchronizing() {
			syncSignals--
		} else {
			syncSignals = 5
		}
		if syncSignals == 0 {
			break
		}
		time.Sleep(time.Second * 1)
	}
	log.Infof("running scrutinizer after-sync bootstrap")
	// Block the new votes addition until the recovery finishes.
	s.recoveryBootLock.Lock()
	defer s.recoveryBootLock.Unlock()
	// Find those processes which do not have yet final results,
	// they are considered live so we need to compute the temporary
	// results (or only its weight in case of Encrypted)
	prcs := [][]byte{}
	err := s.db.ForEach(
		badgerhold.Where("FinalResults").Eq(false),
		func(p *Process) error {
			prcs = append(prcs, p.ID)
			return nil
		})
	if err != nil {
		log.Error(err)
	}
	log.Infof("recovered %d live results processes", len(prcs))
	log.Infof("starting live results recovery computation")
	startTime := time.Now()
	for _, p := range prcs {
		// In order to recover the full list of live results, we need
		// to reset the existing Results and count them again from scratch.
		// Since we cannot be sure if there are votes missing, we need to
		// perform the full computation.
		process, err := s.App.State.Process(p, false)
		if err != nil {
			log.Errorf("cannot fetch process: %v", err)
			continue
		}
		options := process.GetVoteOptions()
		if err := s.db.Upsert(p, &Results{
			ProcessID: p,
			// MaxValue requires +1 since 0 is also an option
			Votes:      newEmptyVotes(int(options.MaxCount), int(options.MaxValue)+1),
			Weight:     new(big.Int).SetUint64(0),
			Signatures: []types.HexBytes{},
		}); err != nil {
			log.Errorf("cannot upsert results to db: %v", err)
			continue
		}

		// Count the votes, add them to results (in memory, without any db transaction)
		results := &Results{Weight: new(big.Int).SetUint64(0)}
		for _, e := range s.App.State.EnvelopeList(p, 0, MaxEnvelopeListSize, true) {
			vote, err := s.App.State.Envelope(p, e, true)
			if err != nil {
				log.Warn(err)
				continue
			}
			if err := s.addLiveVote(vote, results); err != nil {
				log.Warn(err)
			}
		}

		// Store the results on the persisten database
		if err := s.commitVotes(p, results); err != nil {
			log.Errorf("cannot commit live votes: (%v)", err)
		}

		// Add process to live results so new votes will be added
		s.addProcessToLiveResults(p)
	}
	log.Infof("live resuts recovery computation finished, took %s", time.Since(startTime))
}

// Commit is called by the APP when a block is confirmed and included into the chain
func (s *Scrutinizer) Commit(height uint32) error {
	// Add Entity and register new active process
	for _, p := range s.newProcessPool {
		if err := s.newEmptyProcess(p.ProcessID); err != nil {
			log.Errorf("commit: cannot create new empty process: %v", err)
			continue
		}
		if live, err := s.isOpenProcess(p.ProcessID); err != nil {
			log.Errorf("cannot check if process is live results: %v", err)
		} else if live && !s.App.IsSynchronizing() {
			// Only add live processes if the vochain is not synchronizing
			s.addProcessToLiveResults(p.ProcessID)
		}
	}

	// Update existing processes
	for _, p := range s.updateProcessPool {
		if err := s.updateProcess(p); err != nil {
			log.Errorf("commit: cannot update process %x: %v", p, err)
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

	startTime := time.Now()
	for _, v := range s.voteIndexPool {
		// TODO: This should perform a single db Tx
		if err := s.addVoteIndex(v.vote.Nullifier,
			v.vote.ProcessId,
			height,
			v.txIndex); err != nil {
			log.Warn(err)
		}
	}
	if len(s.voteIndexPool) > 0 {
		log.Infof("indexed %d new envelopes, took %s",
			len(s.voteIndexPool), time.Since(startTime))
	}
	// Check if there are processes that need results computing
	// this can be run async
	go s.computePendingProcesses(uint32(height))

	// Add votes collected by onVote (live results), can be run async
	go func() {
		// If the recovery bootstrap is running, wait.
		s.recoveryBootLock.RLock()
		defer s.recoveryBootLock.RUnlock()
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
			log.Infof("added %d live votes on block %d, took %s",
				nvotes, height, time.Since(timer))
		}
	}()

	return nil
}

// Rollback removes the non committed pending operations
func (s *Scrutinizer) Rollback() {
	s.votePool = make(map[string][]*models.Vote)
	s.voteIndexPool = []*VoteWithIndex{}
	s.newProcessPool = []*types.ScrutinizerOnProcessData{}
	s.resultsPool = []*types.ScrutinizerOnProcessData{}
	s.updateProcessPool = [][]byte{}
}

// OnProcess scrutinizer stores the processID and entityID
func (s *Scrutinizer) OnProcess(pid, eid []byte, censusRoot, censusURI string, txIndex int32) {
	data := &types.ScrutinizerOnProcessData{EntityID: eid, ProcessID: pid}
	s.newProcessPool = append(s.newProcessPool, data)
}

// OnVote scrutinizer stores the votes if the processId is live results (on going)
// and the blockchain is not synchronizing.
func (s *Scrutinizer) OnVote(v *models.Vote, txIndex int32) {
	if s.isProcessLiveResults(v.ProcessId) {
		s.votePool[string(v.ProcessId)] = append(s.votePool[string(v.ProcessId)], v)
	}
	s.voteIndexPool = append(s.voteIndexPool, &VoteWithIndex{vote: v, txIndex: txIndex})
}

// OnCancel scrutinizer stores the processID and entityID
func (s *Scrutinizer) OnCancel(pid []byte, txIndex int32) {
	s.updateProcessPool = append(s.updateProcessPool, pid)
}

// OnProcessKeys does nothing
func (s *Scrutinizer) OnProcessKeys(pid []byte, pub, commit string, txIndex int32) {
	s.updateProcessPool = append(s.updateProcessPool, pid)
}

func (s *Scrutinizer) OnProcessStatusChange(pid []byte, status models.ProcessStatus, txIndex int32) {
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
func (s *Scrutinizer) OnRevealKeys(pid []byte, priv, reveal string, txIndex int32) {
	p, err := s.App.State.Process(pid, false)
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

func (s *Scrutinizer) OnProcessResults(pid []byte, results []*models.QuestionResult, txIndex int32) error {
	// TODO: check results are valid and return an error if not.
	// This is very dangerous since an Oracle would be able to create a consensus failure,
	// the validaros (that do not check the results) and the full-nodes (with the scrutinizer enabled)
	// would compute different state hash.
	// As a temporary solution, lets compare results but just print the error.

	// This code must be run async in order to not delay the consensus. The results retreival
	// could require some time.
	go func() {
		var myResults *Results
		var err error
		retries := 20
		for {
			if retries == 0 {
				log.Errorf("could not fetch results after max retries")
				return
			}
			myResults, err = s.GetResults(pid)
			if err == nil {
				break
			}
			if err == ErrNoResultsYet {
				time.Sleep(2 * time.Second)
				retries--
				continue
			}
			log.Errorf("cannot validate results: %v", err)
			return
		}

		myVotes := BuildProcessResult(myResults, nil).GetVotes()
		if len(myVotes) != len(results) {
			log.Errorf("results validation failed: wrong result questions size")
			return
		}
		for i, q := range results {
			if len(q.Question) != len(myVotes[i].Question) {
				log.Errorf("results validation failed: wrong question size")
				return
			}
			for j, v := range q.Question {
				if !bytes.Equal(v, myVotes[i].Question[j]) {
					log.Errorf("results validation failed: wrong question result")
					return
				}
			}
		}
		log.Infof("published results for process %x are correct!", pid)
	}()
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

package scrutinizer

/*
	Scrutinizer keeps 4 different database entries (splited by key prefix)

	+ ProcessEnding: key is block number. Used for schedule results computing
	+ LiveProcess: key is processId. Temporary storage for live results (poll-vote)
	+ Entity: key is entityId: List of known entities
	+ Results: key is processId: Final results for a process
*/

import (
	"bytes"
	"fmt"
	"math/big"

	"go.vocdoni.io/dvote/db"
	"go.vocdoni.io/dvote/events"
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

// Scrutinizer is the component which makes the accounting of the voting processes and keeps it indexed in a local database
type Scrutinizer struct {
	VochainState    *vochain.State
	Storage         db.Database
	votePool        []*models.Vote
	processPool     []*types.ScrutinizerOnProcessData
	resultsPool     []*types.ScrutinizerOnProcessData
	entityCount     int64
	EventDispatcher *events.Dispatcher
}

// NewScrutinizer returns an instance of the Scrutinizer
// using the local storage database of dbPath and integrated into the state vochain instance
func NewScrutinizer(dbPath string, state *vochain.State) (*Scrutinizer, error) {
	s := &Scrutinizer{VochainState: state}
	var err error
	s.Storage, err = db.NewBadgerDB(dbPath)
	if err != nil {
		return nil, err
	}
	s.entityCount = int64(len(s.List(int64(^uint(0)>>1), []byte{}, []byte{types.ScrutinizerEntityPrefix})))
	s.VochainState.AddEventListener(s)
	return s, nil
}

// AttachEventCollector attaches the event collector to the scrutinizer
func (s *Scrutinizer) AttachEventCollector(d *events.Dispatcher) {
	s.EventDispatcher = d
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

//Rollback removes the non committed pending operations
func (s *Scrutinizer) Rollback() {
	s.votePool = []*models.Vote{}
	s.processPool = []*types.ScrutinizerOnProcessData{}
	s.resultsPool = []*types.ScrutinizerOnProcessData{}
}

// OnProcess scrutinizer stores the processID and entityID
func (s *Scrutinizer) OnProcess(pid, eid []byte, censusRoot, censusURI string) {
	data := &types.ScrutinizerOnProcessData{EntityID: eid, ProcessID: pid}
	s.processPool = append(s.processPool, data)
}

// OnVote scrutinizer stores the votes if liveResults enabled
func (s *Scrutinizer) OnVote(v *models.Vote) {
	isLive, err := s.isLiveResultsProcess(v.ProcessId)
	if err != nil {
		log.Errorf("cannot check if process is live results: (%s)", err)
		return
	}
	if isLive {
		s.votePool = append(s.votePool, v)
	}
}

// OnCancel scrutinizer stores the processID and entityID
func (s *Scrutinizer) OnCancel(pid []byte) {
	// TBD: compute final live results?
}

// OnProcessKeys does nothing
func (s *Scrutinizer) OnProcessKeys(pid []byte, pub, com string) {
	// do nothing
}

func (s *Scrutinizer) OnProcessStatusChange(pid []byte, status models.ProcessStatus) {
	// do nothing
}

// OnRevealKeys checks if all keys have been revealed and in such case add the process to the results queue
func (s *Scrutinizer) OnRevealKeys(pid []byte, pub, com string) {
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
}

// List returns a list of keys matching a given prefix. If from is specified, it will seek to the prefix+form key (if found).
func (s *Scrutinizer) List(max int64, from, prefix []byte) [][]byte {
	iter := s.Storage.NewIterator().(*db.BadgerIterator) // TODO(mvdan): don't type assert
	list := [][]byte{}
	for iter.Iter.Seek([]byte(fmt.Sprintf("%s%s", prefix, from))); iter.Iter.ValidForPrefix(prefix); iter.Iter.Next() {
		key := iter.Key()[len(prefix):]
		if len(from) > 0 && bytes.Equal(key, from) {
			// We don't include "from" in the result.
			continue
		}
		keyCopy := make([]byte, len(key))
		copy(keyCopy, key)
		list = append(list, keyCopy)
		if max--; max < 1 {
			break
		}
	}
	iter.Release()
	return list
}

// Temporary until we use Protobuf for the API
func (s *Scrutinizer) GetFriendlyResults(result *models.ProcessResult) [][]string {
	r := [][]string{}
	value := new(big.Int)
	for i, v := range result.Votes {
		r = append(r, []string{})
		for j := range v.Question {
			value.SetBytes(v.Question[j])
			r[i] = append(r[i], value.String())
		}
	}
	return r
}

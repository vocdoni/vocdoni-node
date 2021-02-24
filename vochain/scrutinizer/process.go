package scrutinizer

import (
	"bytes"
	"fmt"
	"math/big"
	"sync/atomic"

	"github.com/dgraph-io/badger/v2"
	"go.vocdoni.io/proto/build/go/models"
	"google.golang.org/protobuf/proto"

	"go.vocdoni.io/dvote/log"
	"go.vocdoni.io/dvote/types"
	"go.vocdoni.io/dvote/util"
)

// ProcessInfo returns the available information regarding an election process id
func (s *Scrutinizer) ProcessInfo(pid []byte) (*models.Process, error) {
	return s.VochainState.Process(pid, false)
}

// ProcessList returns the list of processes (finished or not) for a specific entity.
func (s *Scrutinizer) ProcessList(entityID []byte, fromID []byte, max int64, namespace *uint32) ([][]byte, error) {
	plistkey := []byte{types.ScrutinizerEntityPrefix}
	plistkey = append(plistkey, entityID...)
	processList, err := s.Storage.Get(plistkey)
	if err != nil {
		return nil, err
	}
	processListResult := [][]byte{}
	fromLock := len(fromID) > 0
	for _, process := range util.SplitBytes(processList, types.ProcessIDsize) {
		if max < 1 || len(process) < 1 {
			break
		}
		if !fromLock {
			keyCopy := make([]byte, len(process))
			copy(keyCopy, process)
			if p, err := s.VochainState.Process(keyCopy, true); err != nil {
				log.Debugf("error fetching process %x: %s", p.ProcessId, err)
				continue
			} else if namespace == nil {
				processListResult = append(processListResult, keyCopy)
				max--
			} else {
				if p.Namespace == *namespace {
					processListResult = append(processListResult, keyCopy)
					max--
				}
			}
		}
		if fromLock && bytes.Equal(fromID, process) {
			fromLock = false
		}
	}
	return processListResult, nil
}

// ProcessListWithResults returns the list of process ID with already computed results
func (s *Scrutinizer) ProcessListWithResults(max int64, fromID []byte) []string {
	process := []string{}
	for _, p := range s.List(max, fromID, []byte{types.ScrutinizerResultsPrefix}) {
		process = append(process, fmt.Sprintf("%x", p))
	}
	return process
}

// ProcessListWithLiveResults returns the list of process ID which have live results (not encrypted)
func (s *Scrutinizer) ProcessListWithLiveResults(max int64, fromID []byte) []string {
	process := []string{}
	for _, p := range s.List(max, fromID, []byte{types.ScrutinizerLiveProcessPrefix}) {
		process = append(process, fmt.Sprintf("%x", p))
	}
	return process
}

// EntityList returns the list of entities indexed by the scrutinizer
func (s *Scrutinizer) EntityList(max int64, fromID []byte) []string {
	entities := []string{}
	for _, e := range s.List(max, fromID, []byte{types.ScrutinizerEntityPrefix}) {
		entities = append(entities, fmt.Sprintf("%x", e))
	}
	return entities
}

// EntityCount return the number of entities indexed by the scrutinizer
func (s *Scrutinizer) EntityCount() int64 {
	return atomic.LoadInt64(&s.entityCount)
}

// Return whether a process must have live results or not
func (s *Scrutinizer) isLiveResultsProcess(processID []byte) (bool, error) {
	p, err := s.ProcessInfo(processID)
	if err != nil {
		return false, err
	}
	return !p.EnvelopeType.EncryptedVotes, nil
}

// checks if the current heigh has scheduled ending processes, if so compute and store results
func (s *Scrutinizer) checkFinishedProcesses(height int64) {
	pidListBytes, err := s.Storage.Get(s.Encode("processEnding", []byte(fmt.Sprintf("%d", height))))
	if err != nil || pidListBytes == nil {
		return
	}
	var pidList models.ProcessEndingList
	if err := proto.Unmarshal(pidListBytes, &pidList); err != nil {
		log.Error(err)
		return
	}
	for _, p := range pidList.GetProcessList() {
		if err := s.ComputeResult(p); err != nil {
			log.Errorf("cannot compute results for %x: (%s)", p, err)
		}
	}
	// Remove entry from storage
	if err := s.Storage.Del(s.Encode("processEnding", []byte(fmt.Sprintf("%d", height)))); err != nil {
		log.Error(err)
	}
}

// creates a new empty process and stores it into the database
func (s *Scrutinizer) newEmptyLiveProcess(pid []byte) (*models.ProcessResult, error) {
	p, err := s.VochainState.Process(pid, false)
	if err != nil {
		return nil, fmt.Errorf("cannot create new empty live process: %w", err)
	}
	options := p.GetVoteOptions()
	if options == nil {
		return nil, fmt.Errorf("newEmptyLiveProcess: vote options is nil")
	}
	if options.MaxCount == 0 || options.MaxValue == 0 {
		return nil, fmt.Errorf("newEmptyLiveProcess: maxCount or maxValue are zero")
	}

	// Check for overflows
	if options.MaxCount > MaxQuestions || options.MaxValue > MaxOptions {
		return nil, fmt.Errorf("maxCount or maxValue overflows hardcoded maximums")
	}
	// MaxValue requires +1 since 0 is also an option
	pv := emptyProcess(int(options.MaxCount), int(options.MaxValue)+1)
	process, err := proto.Marshal(pv)
	if err != nil {
		return nil, err
	}
	if err := s.Storage.Put(s.Encode("liveProcess", pid), process); err != nil {
		return nil, err
	}
	return pv, nil
}

// Pending processes are those processes which are scheduled for being computed.
// On the database we are storing: height=>{proceess1, process2, process3}
func (s *Scrutinizer) registerPendingProcess(pid []byte, height int64) {
	scheduledBlock := []byte(fmt.Sprintf("%d", height))

	pidListBytes, err := s.Storage.Get(s.Encode("processEnding", scheduledBlock))
	// TODO(mvdan): use a generic "key not found" database error instead
	if err != nil && err != badger.ErrKeyNotFound {
		log.Error(err)
		return
	}
	var pidList models.ProcessEndingList
	if len(pidListBytes) > 0 {
		if err := proto.Unmarshal(pidListBytes, &pidList); err != nil {
			log.Error(err)
			return
		}
	}
	pidList.ProcessList = append(pidList.ProcessList, pid)

	pidListBytes, err = proto.Marshal(&pidList)
	if err != nil {
		log.Error(err)
		return
	}
	if err = s.Storage.Put(s.Encode("processEnding", scheduledBlock), pidListBytes); err != nil {
		log.Error(err)
		return
	}
	log.Infof("process %x results computation scheduled for block %s", pid, scheduledBlock)
}

func (s *Scrutinizer) addLiveResultsProcess(pid []byte) {
	log.Infof("add new process %x to live results", pid)
	process, err := s.Storage.Get(s.Encode("liveProcess", pid))
	if err != nil && err != badger.ErrKeyNotFound {
		log.Error(err)
		return
	}
	if len(process) > 0 {
		return
	}
	// Create empty process and store it
	if _, err := s.newEmptyLiveProcess(pid); err != nil {
		log.Error(err)
	}
}

// Encode encodes scrutinizer specific data adding a prefix for its inner database
func (s *Scrutinizer) Encode(t string, data []byte) []byte {
	switch t {
	case "entity":
		return append([]byte{types.ScrutinizerEntityPrefix}, data...)
	case "liveProcess":
		return append([]byte{types.ScrutinizerLiveProcessPrefix}, data...)
	case "results":
		return append([]byte{types.ScrutinizerResultsPrefix}, data...)
	case "processEnding":
		return append([]byte{types.ScrutinizerProcessEndingPrefix}, data...)
	}
	panic("scrutinizer encode type not known")
}

func (s *Scrutinizer) addEntity(eid, pid []byte) {
	// TODO(mvdan): use a prefixed database
	storagekey := s.Encode("entity", eid)
	processList, err := s.Storage.Get(storagekey)
	if err != nil && err != badger.ErrKeyNotFound {
		log.Errorf("addEntity: %s", err)
		return
	}
	if err == badger.ErrKeyNotFound {
		log.Infof("added new entity %x to scrutinizer", eid)
	}
	if len(pid) != types.ProcessIDsize {
		log.Errorf("addEntity: pid size is not correct, got %d", len(pid))
		return
	}
	processList = append(processList, pid...)
	if err := s.Storage.Put(storagekey, processList); err != nil {
		log.Error(err)
		return
	}
	log.Infof("added new process %x to scrutinizer", pid)
	atomic.AddInt64(&s.entityCount, 1)
}

func emptyProcess(questions, options int) *models.ProcessResult {
	if questions == 0 || options == 0 {
		return nil
	}
	pv := new(models.ProcessResult)
	pv.Votes = make([]*models.QuestionResult, questions)
	zero := big.NewInt(0)
	for i := range pv.Votes {
		pv.Votes[i] = new(models.QuestionResult)
		pv.Votes[i].Question = make([][]byte, options)
		for j := range pv.Votes[i].Question {
			pv.Votes[i].Question[j] = zero.Bytes()
		}
	}
	return pv
}

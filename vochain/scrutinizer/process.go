package scrutinizer

import (
	"bytes"
	"encoding/hex"
	"fmt"
	"sync/atomic"

	"github.com/dgraph-io/badger/v2"
	"github.com/vocdoni/dvote-protobuf/build/go/models"
	"google.golang.org/protobuf/proto"

	"gitlab.com/vocdoni/go-dvote/log"
	"gitlab.com/vocdoni/go-dvote/types"
	"gitlab.com/vocdoni/go-dvote/util"
)

// ProcessInfo returns the available information regarding an election process id
func (s *Scrutinizer) ProcessInfo(pid []byte) (*models.Process, error) {
	return s.VochainState.Process(pid, false)
}

// ProcessList returns the list of processes (finished or not) for a specific entity.
func (s *Scrutinizer) ProcessList(entityID []byte, fromID []byte, max int64) ([][]byte, error) {
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
			processListResult = append(processListResult, keyCopy)
			max--
		}
		if fromLock && bytes.Equal(fromID, process) {
			fromLock = false
		}
	}
	return processListResult, nil
}

// ProcessListWithResults returns the list of process ID with already computed results
func (s *Scrutinizer) ProcessListWithResults(max int64, fromID string) ([]string, error) {
	from, err := hex.DecodeString(fromID)
	if err != nil {
		return nil, err
	}
	process := []string{}
	for _, p := range s.List(max, from, []byte{types.ScrutinizerResultsPrefix}) {
		process = append(process, fmt.Sprintf("%x", p))
	}
	return process, nil
}

// ProcessListWithLiveResults returns the list of process ID which have live results (not encrypted)
func (s *Scrutinizer) ProcessListWithLiveResults(max int64, fromID string) ([]string, error) {
	from, err := hex.DecodeString(fromID)
	if err != nil {
		return nil, err
	}
	process := []string{}
	for _, p := range s.List(max, from, []byte{types.ScrutinizerLiveProcessPrefix}) {
		process = append(process, fmt.Sprintf("%x", p))
	}
	return process, nil
}

// EntityList returns the list of entities indexed by the scrutinizer
func (s *Scrutinizer) EntityList(max int64, fromID string) ([]string, error) {
	var err error
	var from []byte
	if len(fromID) > 0 {
		from, err = hex.DecodeString(util.TrimHex(fromID))
		if err != nil {
			return nil, err
		}
	}
	entities := []string{}
	for _, e := range s.List(max, from, []byte{types.ScrutinizerEntityPrefix}) {
		entities = append(entities, fmt.Sprintf("%x", e))
	}
	return entities, nil
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
	pidListBytes, err := s.Storage.Get(s.encode("processEnding", []byte(fmt.Sprintf("%d", height))))
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
	if err := s.Storage.Del(s.encode("processEnding", []byte(fmt.Sprintf("%d", height)))); err != nil {
		log.Error(err)
	}
}

// creates a new empty process and stores it into the database
func (s *Scrutinizer) newEmptyLiveProcess(pid []byte) (*models.ProcessResult, error) {
	pv := emptyProcess(0, 0)
	process, err := proto.Marshal(pv)
	if err != nil {
		return nil, err
	}
	if err := s.Storage.Put(s.encode("liveProcess", pid), process); err != nil {
		return nil, err
	}
	return pv, nil
}

// Pending processes are those processes which are scheduled for being computed.
// On the database we are storing: height=>{proceess1, process2, process3}
func (s *Scrutinizer) registerPendingProcess(pid []byte, height int64) {
	scheduledBlock := []byte(fmt.Sprintf("%d", height))

	pidListBytes, err := s.Storage.Get(s.encode("processEnding", scheduledBlock))
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
	if err = s.Storage.Put(s.encode("processEnding", scheduledBlock), pidListBytes); err != nil {
		log.Error(err)
		return
	}
	log.Infof("process %x results computation scheduled for block %s", pid, scheduledBlock)
}

func (s *Scrutinizer) addLiveResultsProcess(pid []byte) {
	log.Infof("add new process %x to live results", pid)
	process, err := s.Storage.Get(s.encode("liveProcess", pid))
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

func (s *Scrutinizer) encode(t string, data []byte) []byte {
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
	storagekey := s.encode("entity", eid)
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
	if questions == 0 {
		questions = MaxQuestions
	}
	if options == 0 {
		options = MaxOptions
	}
	pv := new(models.ProcessResult)
	pv.Votes = make([]*models.QuestionResult, questions)
	for i := range pv.Votes {
		pv.Votes[i] = new(models.QuestionResult)
		pv.Votes[i].Question = make([]uint32, options)
	}
	return pv
}

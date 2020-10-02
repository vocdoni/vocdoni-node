package scrutinizer

import (
	"bytes"
	"encoding/hex"
	"fmt"
	"sync/atomic"

	"github.com/dgraph-io/badger/v2"

	"gitlab.com/vocdoni/go-dvote/log"
	"gitlab.com/vocdoni/go-dvote/types"
)

// ProcessInfo returns the available information regarding an election process id
func (s *Scrutinizer) ProcessInfo(pid []byte) (*types.Process, error) {
	return s.VochainState.Process(pid, false)
}

func (s *Scrutinizer) ProcessList(entityID []byte, fromID []byte, max int64) ([]string, error) {
	plistkey := []byte{types.ScrutinizerEntityPrefix}
	plistkey = append(plistkey, entityID...)
	processList, err := s.Storage.Get(plistkey)
	if err != nil {
		return nil, err
	}

	processListString := []string{}
	fromLock := len(fromID) > 0
	for _, process := range bytes.Split(processList, []byte{types.ScrutinizerEntityProcessSeparator}) {
		if max < 1 || len(process) < 1 {
			break
		}
		if !fromLock {
			processListString = append(processListString, fmt.Sprintf("%x", process))
			max--
		}
		if fromLock && string(fromID) == string(process) {
			fromLock = false
		}
	}

	return processListString, nil
}

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

func (s *Scrutinizer) EntityList(max int64, fromID string) ([]string, error) {
	from, err := hex.DecodeString(fromID)
	if err != nil {
		return nil, err
	}
	entities := []string{}
	for _, e := range s.List(max, from, []byte{types.ScrutinizerEntityPrefix}) {
		entities = append(entities, fmt.Sprintf("%x", e))
	}
	return entities, nil
}

func (s *Scrutinizer) EntityCount() int64 {
	return atomic.LoadInt64(&s.entityCount)
}

// Return whether a process must have live results or not
func (s *Scrutinizer) isLiveResultsProcess(processID []byte) (bool, error) {
	p, err := s.ProcessInfo(processID)
	if err != nil {
		return false, err
	}
	return !p.IsEncrypted(), nil
}

// checks if the current heigh has scheduled ending processes, if so compute and store results
func (s *Scrutinizer) checkFinishedProcesses(height int64) {
	pidListBytes, err := s.Storage.Get(s.encode("processEnding", []byte(fmt.Sprintf("%d", height))))
	if err != nil || pidListBytes == nil {
		return
	}
	var pidList ProcessEndingList
	if err := s.VochainState.Codec.UnmarshalBinaryBare(pidListBytes, &pidList); err != nil {
		log.Error(err)
		return
	}
	for _, p := range pidList {
		if err := s.ComputeResult(p); err != nil {
			log.Errorf("cannot compute results for %s: (%s)", p, err)
		}
	}
	// Remove entry from storage
	if err := s.Storage.Del(s.encode("processEnding", []byte(fmt.Sprintf("%d", height)))); err != nil {
		log.Error(err)
	}
}

// creates a new empty process and stores it into the database
func (s *Scrutinizer) newEmptyLiveProcess(pid []byte) (ProcessVotes, error) {
	pv := emptyProcess()
	process, err := s.VochainState.Codec.MarshalBinaryBare(&pv)
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
	var pidList ProcessEndingList
	if len(pidListBytes) > 0 {
		if err := s.VochainState.Codec.UnmarshalBinaryBare(pidListBytes, &pidList); err != nil {
			log.Error(err)
			return
		}
	}
	pidList = append(pidList, pid)

	pidListBytes, err = s.VochainState.Codec.MarshalBinaryBare(pidList)
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
	case "processSeparator":
		return append(data, types.ScrutinizerEntityProcessSeparator)
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
		log.Error(err)
		return
	}
	processList = append(processList, s.encode("processSeparator", pid)...)
	if err := s.Storage.Put(storagekey, processList); err != nil {
		log.Error(err)
		return
	}
	log.Debugf("added new entity %x to scrutinizer", eid)
	atomic.AddInt64(&s.entityCount, 1)
}

func emptyProcess() ProcessVotes {
	pv := make(ProcessVotes, MaxQuestions)
	for i := range pv {
		pv[i] = make([]uint32, MaxOptions)
	}
	return pv
}

package scrutinizer

import (
	"fmt"

	"github.com/dgraph-io/badger/v2"

	"gitlab.com/vocdoni/go-dvote/log"
	"gitlab.com/vocdoni/go-dvote/types"
	"gitlab.com/vocdoni/go-dvote/util"
)

// ProcessInfo returns the available information regarding an election process id
func (s *Scrutinizer) ProcessInfo(processID string) (*types.Process, error) {
	return s.VochainState.Process(processID, false)
}

// Return whether a process must have live results or not
func (s *Scrutinizer) isLiveResultsProcess(processID string) (bool, error) {
	p, err := s.ProcessInfo(util.TrimHex(processID))
	if err != nil {
		return false, err
	}
	return !p.IsEncrypted(), nil
}

// checks if the current heigh has scheduled ending processes, if so compute and store results
func (s *Scrutinizer) checkFinishedProcesses(height int64) {
	// TODO(mvdan): replace Has+get with just Get
	exists, err := s.Storage.Has([]byte(types.ScrutinizerProcessEndingPrefix + fmt.Sprintf("%d", height)))
	if err != nil {
		log.Error(err)
		return
	}
	if !exists {
		return
	}

	var pidList ProcessEndingList
	pidListBytes, err := s.Storage.Get([]byte(types.ScrutinizerProcessEndingPrefix + fmt.Sprintf("%d", height)))
	if err != nil {
		log.Error(err)
		return
	}
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
	if err = s.Storage.Del([]byte(types.ScrutinizerProcessEndingPrefix + fmt.Sprintf("%d", height))); err != nil {
		log.Error(err)
	}

}

// creates a new empty process and stores it into the database
func (s *Scrutinizer) newEmptyLiveProcess(pid string) (ProcessVotes, error) {
	pv := emptyProcess()
	process, err := s.VochainState.Codec.MarshalBinaryBare(&pv)
	if err != nil {
		return nil, err
	}
	if err := s.Storage.Put([]byte(types.ScrutinizerLiveProcessPrefix+pid), process); err != nil {
		return nil, err
	}
	return pv, nil
}

// Pending processes are those processes which are scheduled for being computed.
// On the database we are storing: height=>{proceess1, process2, process3}
func (s *Scrutinizer) registerPendingProcess(pid string, height int64) {
	scheduledBlock := fmt.Sprintf("%d", height)

	pidListBytes, err := s.Storage.Get([]byte(types.ScrutinizerProcessEndingPrefix + scheduledBlock))
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
	if err = s.Storage.Put([]byte(types.ScrutinizerProcessEndingPrefix+scheduledBlock), pidListBytes); err != nil {
		log.Error(err)
		return
	}
	log.Infof("process %s results computation scheduled for block %s", pid, scheduledBlock)
}

func (s *Scrutinizer) addLiveResultsProcess(pid string) {
	log.Infof("add new process %s to live results", pid)
	process, err := s.Storage.Get([]byte(types.ScrutinizerLiveProcessPrefix + pid))
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

func (s *Scrutinizer) addEntity(eid string, pid string) {
	// TODO(mvdan): use a prefixed database
	storagekey := []byte(types.ScrutinizerEntityPrefix + eid)
	processList, err := s.Storage.Get(storagekey)
	if err != nil && err != badger.ErrKeyNotFound {
		log.Error(err)
		return
	}
	if err := s.Storage.Put(storagekey, append(processList, []byte(pid+types.ScrutinizerEntityProcessSeparator)...)); err != nil {
		log.Error(err)
		return
	}
	log.Debugf("add new entity %s to scrutinizer", eid)
}

func emptyProcess() ProcessVotes {
	pv := make(ProcessVotes, MaxQuestions)
	for i := range pv {
		pv[i] = make([]uint32, MaxOptions)
	}
	return pv
}

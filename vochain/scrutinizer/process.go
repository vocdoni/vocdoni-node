package scrutinizer

import (
	"fmt"

	"gitlab.com/vocdoni/go-dvote/log"
	"gitlab.com/vocdoni/go-dvote/types"
)

// ProcessInfo returns the available information regarding an election process id
func (s *Scrutinizer) ProcessInfo(processID string) (*types.Process, error) {
	return s.VochainState.Process(processID)
}

// Return whether a process must have live results or not
func (s *Scrutinizer) isLiveResultsProcess(processID string) (bool, error) {
	p, err := s.ProcessInfo(processID)
	if err != nil {
		return false, err
	}
	if p.Type == types.PollVote || p.Type == types.PetitionSign {
		return true, nil
	}
	return false, nil
}

// checks if the current heigh has scheduled ending processes, if so compute and store results
func (s *Scrutinizer) checkFinishedProcesses(height int64) {
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
	err = s.VochainState.Codec.UnmarshalBinaryBare(pidListBytes, &pidList)
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

// active processes are those processes which are not yet finished
// on the database we are storing: height=>{proceess1, process2, process3}
func (s *Scrutinizer) registerActiveProcess(pid string) {
	process, err := s.VochainState.Process(pid)
	if err != nil {
		log.Error(err)
		return
	}
	endBlock := fmt.Sprintf("%d", process.StartBlock+process.NumberOfBlocks)

	pidListBytes, err := s.Storage.Get([]byte(types.ScrutinizerProcessEndingPrefix + endBlock))
	if err != nil && err.Error() != NoKeyStorageError {
		log.Error(err)
		return
	}
	var pidList ProcessEndingList
	if len(pidListBytes) > 0 {
		err = s.VochainState.Codec.UnmarshalBinaryBare(pidListBytes, &pidList)
	}
	pidList = append(pidList, pid)

	pidListBytes, err = s.VochainState.Codec.MarshalBinaryBare(pidList)
	if err != nil {
		log.Error(err)
		return
	}
	if err = s.Storage.Put([]byte(types.ScrutinizerProcessEndingPrefix+endBlock), pidListBytes); err != nil {
		log.Error(err)
		return
	}
	log.Infof("process %s scheduled for block %s", pid, endBlock)
}

func (s *Scrutinizer) addLiveResultsProcess(pid string) {
	log.Infof("add new process %s to live results", pid)
	process, err := s.Storage.Get([]byte(types.ScrutinizerLiveProcessPrefix + pid))
	if err != nil && err.Error() != NoKeyStorageError {
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

func (s *Scrutinizer) addEntity(eid string) {
	// TODO(mvdan): use a prefixed database
	entity, err := s.Storage.Has([]byte(types.ScrutinizerEntityPrefix + eid))
	if err != nil && err.Error() != NoKeyStorageError {
		log.Error(err)
		return
	}
	if entity {
		return
	}
	if err := s.Storage.Put([]byte(types.ScrutinizerEntityPrefix+eid), []byte{}); err != nil {
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

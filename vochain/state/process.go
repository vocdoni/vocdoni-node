package state

import (
	"bytes"
	"errors"
	"fmt"
	"strconv"

	"go.vocdoni.io/dvote/log"
	"go.vocdoni.io/dvote/statedb"
	"go.vocdoni.io/dvote/tree/arbo"
	"go.vocdoni.io/dvote/types"
	"go.vocdoni.io/dvote/util"
	"go.vocdoni.io/proto/build/go/models"
	"google.golang.org/protobuf/proto"
)

var (
	emptyVotesRoot = make([]byte, StateChildTreeCfg(ChildTreeVotes).HashFunc().Len())
)

// AddProcess adds a new process to the vochain.  Adding a process with a
// ProcessId that already exists will return an error.
func (v *State) AddProcess(p *models.Process) error {
	newProcessBytes, err := proto.Marshal(
		&models.StateDBProcess{Process: p, VotesRoot: emptyVotesRoot})
	if err != nil {
		return fmt.Errorf("cannot marshal process bytes: %w", err)
	}
	v.tx.Lock()
	err = v.tx.DeepAdd(p.ProcessId, newProcessBytes, StateTreeCfg(TreeProcess))
	v.tx.Unlock()
	if err != nil {
		return err
	}
	censusURI := ""
	if p.CensusURI != nil {
		censusURI = *p.CensusURI
	}
	log.Infow("new election",
		"processId", fmt.Sprintf("%x", p.ProcessId),
		"entityId", fmt.Sprintf("%x", p.EntityId),
		"startBlock", p.StartBlock,
		"endBlock", p.BlockCount+p.StartBlock,
		"startTime", util.TimestampToTime(p.StartTime).String(),
		"endTime", util.TimestampToTime(p.StartTime+p.Duration).String(),
		"mode", p.Mode,
		"envelopeType", p.EnvelopeType,
		"voteOptions", p.VoteOptions,
		"censusRoot", fmt.Sprintf("%x", p.CensusRoot),
		"censusOrigin", models.CensusOrigin_name[int32(p.CensusOrigin)],
		"maxCensusSize", p.MaxCensusSize,
		"status", p.Status,
		"height", v.CurrentHeight(),
		"censusURI", censusURI)
	for _, l := range v.eventListeners {
		l.OnProcess(p.ProcessId, p.EntityId, fmt.Sprintf("%x", p.CensusRoot), censusURI, v.txCounter.Load())
	}
	return nil
}

// CancelProcess sets the process canceled attribute to true
func (v *State) CancelProcess(pid []byte) error { // LEGACY
	process, err := v.Process(pid, false)
	if err != nil {
		return err
	}
	if process.Status == models.ProcessStatus_CANCELED {
		return nil
	}
	process.Status = models.ProcessStatus_CANCELED
	updatedProcessBytes, err := proto.Marshal(process)
	if err != nil {
		return fmt.Errorf("cannot marshal updated process bytes: %w", err)
	}
	v.tx.Lock()
	err = v.tx.DeepSet(pid, updatedProcessBytes, StateTreeCfg(TreeProcess))
	v.tx.Unlock()
	if err != nil {
		return err
	}
	for _, l := range v.eventListeners {
		l.OnCancel(pid, v.txCounter.Load())
	}
	return nil
}

func getProcess(mainTreeView statedb.TreeViewer, pid []byte) (*models.Process, error) {
	processBytes, err := mainTreeView.DeepGet(pid, StateTreeCfg(TreeProcess))
	if errors.Is(err, arbo.ErrKeyNotFound) {
		return nil, ErrProcessNotFound
	} else if err != nil {
		return nil, err
	}
	var process models.StateDBProcess
	err = proto.Unmarshal(processBytes, &process)
	if err != nil {
		return nil, fmt.Errorf("cannot unmarshal process (%s): %w", pid, err)
	}
	return process.Process, nil
}

// Process returns a process info given a processId if exists
func (v *State) Process(pid []byte, committed bool) (*models.Process, error) {
	if !committed {
		v.tx.RLock()
		defer v.tx.RUnlock()
	}
	return getProcess(v.mainTreeViewer(committed), pid)
}

// CountProcesses returns the overall number of processes the vochain has
func (v *State) CountProcesses(committed bool) (uint64, error) {
	// TODO: Once statedb.TreeView.Size() works, replace this by that.
	if !committed {
		v.tx.RLock()
		defer v.tx.RUnlock()
	}
	processesTree, err := v.mainTreeViewer(committed).SubTree(StateTreeCfg(TreeProcess))
	if err != nil {
		return 0, err
	}
	return processesTree.Size()
}

// ListProcessIDs returns the full list of process identifiers (pid).
func (v *State) ListProcessIDs(committed bool) ([][]byte, error) {
	if !committed {
		v.tx.RLock()
		defer v.tx.RUnlock()
	}
	processesTree, err := v.mainTreeViewer(committed).SubTree(StateTreeCfg(TreeProcess))
	if err != nil {
		return nil, err
	}
	var pids [][]byte
	if err := processesTree.Iterate(func(key []byte, value []byte) bool {
		p := make([]byte, len(key))
		copy(p, key)
		pids = append(pids, p)
		return false
	}); err != nil {
		return nil, err
	}
	return pids, nil
}

func updateProcess(tx *treeTxWithMutex, p *models.Process, pid []byte) error {
	processesTree, err := tx.SubTree(StateTreeCfg(TreeProcess))
	if err != nil {
		return err
	}
	processBytes, err := processesTree.Get(pid)
	if errors.Is(err, arbo.ErrKeyNotFound) {
		return ErrProcessNotFound
	} else if err != nil {
		return err
	}
	var process models.StateDBProcess
	if err := proto.Unmarshal(processBytes, &process); err != nil {
		return err
	}
	updatedProcessBytes, err := proto.Marshal(
		&models.StateDBProcess{Process: p, VotesRoot: process.VotesRoot})
	if err != nil {
		return fmt.Errorf("cannot marshal updated process bytes: %w", err)
	}
	return processesTree.Set(pid, updatedProcessBytes)
}

// UpdateProcess updates an existing process
func (v *State) UpdateProcess(p *models.Process, pid []byte) error {
	if p == nil || len(p.ProcessId) != types.ProcessIDsize {
		return ErrProcessNotFound
	}
	// update the process
	// try to update startBlocks database
	switch p.Status {
	case models.ProcessStatus_READY:
		if err := v.ProcessBlockRegistry.SetStartBlock(p.ProcessId, p.StartBlock); err != nil {
			return err
		}
	case models.ProcessStatus_PAUSED:
		if err := v.ProcessBlockRegistry.SetStartBlock(p.ProcessId, v.CurrentHeight()); err != nil {
			return err
		}
	case models.ProcessStatus_ENDED:
		if err := v.ProcessBlockRegistry.DeleteStartBlock(p.ProcessId); err != nil {
			return err
		}
	}
	v.tx.Lock()
	defer v.tx.Unlock()
	return updateProcess(&v.tx, p, pid)
}

// SetProcessStatus changes the process status to the one provided.
// One of ready, ended, canceled, paused, results.
// If commit is true, the change is committed to the state.
// Transition checks from the current status to the new one are performed.
func (v *State) SetProcessStatus(pid []byte, newstatus models.ProcessStatus, commit bool) error {
	process, err := v.Process(pid, false)
	if err != nil {
		return err
	}
	currentTime, err := v.Timestamp(false)
	if err != nil {
		return fmt.Errorf("setProcessStatus: cannot get current timestamp: %w", err)
	}
	currentStatus := process.Status

	// Check for state duplication
	if currentStatus == newstatus {
		return fmt.Errorf("process %x already in %s state", pid, newstatus.String())
	}

	// Check if the state transition is valid
	switch newstatus {
	case models.ProcessStatus_READY:
		// Transition to READY is only allowed from PAUSED
		if currentStatus != models.ProcessStatus_PAUSED {
			return fmt.Errorf("cannot set process status from %s to ready", currentStatus.String())
		}

	case models.ProcessStatus_PAUSED:
		// Transition to PAUSED is only allowed from READY
		if currentStatus != models.ProcessStatus_READY {
			return fmt.Errorf("cannot set process status from %s to paused", currentStatus.String())
		}
		// Check if the process is interruptible
		if !process.Mode.Interruptible {
			return fmt.Errorf("cannot pause process %x, it is not interruptible", pid)
		}

	case models.ProcessStatus_ENDED:
		// Transition to ENDED is only allowed from READY or PAUSED
		if currentStatus != models.ProcessStatus_READY && currentStatus != models.ProcessStatus_PAUSED {
			return fmt.Errorf("process %x can only be ended from ready or paused status", pid)
		}
		// Additional condition for interruptible and timing
		if !process.Mode.Interruptible && currentTime < process.StartTime+process.Duration {
			return fmt.Errorf("process %x is not interruptible and cannot be ended prematurely", pid)
		}

	case models.ProcessStatus_CANCELED:
		// Transition to CANCELED is not allowed from ENDED or RESULTS
		if currentStatus == models.ProcessStatus_ENDED || currentStatus == models.ProcessStatus_RESULTS {
			return fmt.Errorf("cannot set state to canceled from ended or results")
		}
		// Additional condition for non-interruptible process
		if currentStatus != models.ProcessStatus_PAUSED && !process.Mode.Interruptible {
			return fmt.Errorf("process %x is not interruptible, cannot change state to canceled", pid)
		}

	case models.ProcessStatus_RESULTS:
		// Transition to RESULTS is only allowed from ENDED
		if currentStatus != models.ProcessStatus_ENDED {
			return fmt.Errorf("cannot set state to results from %s", currentStatus.String())
		}

	default:
		// Handle unknown status
		return fmt.Errorf("process status %s unknown", newstatus.String())
	}

	// If all checks pass, the transition is valid
	if commit {
		process.Status = newstatus
		if err := v.UpdateProcess(process, process.ProcessId); err != nil {
			return err
		}
		for _, l := range v.eventListeners {
			l.OnProcessStatusChange(process.ProcessId, newstatus, v.txCounter.Load())
		}
	}
	return nil
}

// SetProcessResults sets the results for a given process and calls the event listeners.
func (v *State) SetProcessResults(pid []byte, result *models.ProcessResult) error {
	process, err := v.Process(pid, false)
	if err != nil {
		return err
	}
	// Check if the state transition is valid
	// process must be ended, ready or results for setting the results
	if process.Status != models.ProcessStatus_ENDED &&
		process.Status != models.ProcessStatus_READY {
		return fmt.Errorf("cannot set results, invalid status: %s", process.Status)
	}
	if process.Status == models.ProcessStatus_READY &&
		process.StartBlock+process.BlockCount > v.CurrentHeight() {
		return fmt.Errorf("cannot set state to results, process is still alive")
	}

	process.Results = result
	process.Status = models.ProcessStatus_RESULTS
	if err := v.UpdateProcess(process, process.ProcessId); err != nil {
		return fmt.Errorf("cannot set results: %w", err)
	}
	// Call event listeners
	for _, l := range v.eventListeners {
		l.OnProcessResults(process.ProcessId, result, v.txCounter.Load())
	}
	return nil
}

// GetProcessResults returns a friendly representation of the results stored in the State (if any).
func (v *State) GetProcessResults(pid []byte) ([][]*types.BigInt, error) {
	// TO-DO (pau): use a LRU cache for results
	process, err := v.Process(pid, true)
	if err != nil {
		return nil, err
	}
	if process.Results == nil {
		return nil, fmt.Errorf("no results for process %x", pid)
	}
	return GetFriendlyResults(process.Results.GetVotes()), nil
}

// SetProcessCensus sets the census for a given process, only if that process enables dynamic census
func (v *State) SetProcessCensus(pid, censusRoot []byte, censusURI string, commit bool) error {
	process, err := v.Process(pid, false)
	if err != nil {
		return err
	}
	// check valid state transition
	// dynamic census
	if !process.Mode.DynamicCensus {
		return fmt.Errorf(
			"cannot update census, only processes with dynamic census can update their census")
	}
	// census origin
	if !CensusOrigins[process.CensusOrigin].AllowCensusUpdate {
		return fmt.Errorf(
			"cannot update census, invalid census origin: %s", process.CensusOrigin)
	}
	// status
	if !(process.Status == models.ProcessStatus_READY) &&
		!(process.Status == models.ProcessStatus_PAUSED) {
		return fmt.Errorf(
			"cannot update census, process status must be READY or PAUSED and is: %s",
			process.Status.String())
	}
	// check not same censusRoot
	if bytes.Equal(censusRoot, process.CensusRoot) {
		return fmt.Errorf("cannot update census, same censusRoot")
	}

	if CensusOrigins[process.CensusOrigin].NeedsURI && censusURI == "" {
		return fmt.Errorf("process requires URI but an empty one was provided")
	}

	if commit {
		process.CensusRoot = censusRoot
		process.CensusURI = &censusURI
		if err := v.UpdateProcess(process, process.ProcessId); err != nil {
			return err
		}
		for _, l := range v.eventListeners {
			l.OnCensusUpdate(process.ProcessId, process.CensusRoot, censusURI)
		}
	}

	return nil
}

// SetMaxProcessSize sets the global maximum number voters allowed in an election.
func (v *State) SetMaxProcessSize(size uint64) error {
	v.tx.Lock()
	defer v.tx.Unlock()
	return v.tx.DeepSet([]byte("maxProcessSize"), []byte(strconv.FormatUint(size, 10)), StateTreeCfg(TreeExtra))
}

// MaxProcessSize returns the global maximum number voters allowed in an election.
func (v *State) MaxProcessSize() (uint64, error) {
	v.tx.RLock()
	defer v.tx.RUnlock()
	size, err := v.tx.DeepGet([]byte("maxProcessSize"), StateTreeCfg(TreeExtra))
	if err != nil {
		return 0, err
	}
	return strconv.ParseUint(string(size), 10, 64)
}

// SetNetworkCapacity sets the total capacity (in votes per block) of the network.
func (v *State) SetNetworkCapacity(capacity uint64) error {
	v.tx.Lock()
	defer v.tx.Unlock()
	return v.tx.DeepSet([]byte("networkCapacity"), []byte(strconv.FormatUint(capacity, 10)), StateTreeCfg(TreeExtra))
}

// NetworkCapacity returns the total capacity (in votes per block) of the network.
func (v *State) NetworkCapacity() (uint64, error) {
	v.tx.RLock()
	defer v.tx.RUnlock()
	size, err := v.tx.DeepGet([]byte("networkCapacity"), StateTreeCfg(TreeExtra))
	if err != nil {
		if errors.Is(err, arbo.ErrKeyNotFound) {
			return 0, nil
		}
		return 0, err
	}
	return strconv.ParseUint(string(size), 10, 64)
}

package state

import (
	"bytes"
	"errors"
	"fmt"

	"github.com/vocdoni/arbo"
	"go.vocdoni.io/dvote/log"
	"go.vocdoni.io/dvote/statedb"
	"go.vocdoni.io/dvote/types"
	"go.vocdoni.io/proto/build/go/models"
	"google.golang.org/protobuf/proto"
)

var (
	emptyVotesRoot                 = make([]byte, StateChildTreeCfg(ChildTreeVotes).HashFunc().Len())
	emptyCensusRoot                = make([]byte, StateChildTreeCfg(ChildTreeCensus).HashFunc().Len())
	emptyPreRegisterNullifiersRoot = make([]byte, StateChildTreeCfg(ChildTreePreRegisterNullifiers).HashFunc().Len())
)

// AddProcess adds a new process to the vochain.  Adding a process with a
// ProcessId that already exists will return an error.
func (v *State) AddProcess(p *models.Process) error {
	preRegister := p.Mode != nil && p.Mode.PreRegister
	anonymous := p.EnvelopeType != nil && p.EnvelopeType.Anonymous
	if preRegister {
		p.RollingCensusRoot = emptyCensusRoot
		p.NullifiersRoot = emptyPreRegisterNullifiersRoot
	}

	newProcessBytes, err := proto.Marshal(
		&models.StateDBProcess{Process: p, VotesRoot: emptyVotesRoot})
	if err != nil {
		return fmt.Errorf("cannot marshal process bytes: %w", err)
	}
	v.Tx.Lock()
	err = func() error {
		if err := v.Tx.DeepAdd(p.ProcessId, newProcessBytes, StateTreeCfg(TreeProcess)); err != nil {
			return err
		}
		// If Mode.PreRegister && EnvelopeType.Anonymous we create (by
		// opening) a new empty poseidon census tree and nullifier tree
		// at p.ProcessId.
		if preRegister && anonymous {
			census, err := v.Tx.DeepSubTree(
				StateTreeCfg(TreeProcess),
				StateChildTreeCfg(ChildTreeCensusPoseidon).WithKey(p.ProcessId),
			)
			if err != nil {
				return err
			}
			// We store census size as little endian 64 bits.  Set it to 0.
			if err := statedb.SetUint64(census.NoState(), keyCensusLen, 0); err != nil {
				return err
			}
			if _, err = v.Tx.DeepSubTree(StateTreeCfg(TreeProcess),
				StateChildTreeCfg(ChildTreePreRegisterNullifiers).WithKey(p.ProcessId)); err != nil {
				return err
			}
		}
		return v.setProcessIDByStartBlock(p.ProcessId, p.StartBlock)
	}()
	v.Tx.Unlock()
	if err != nil {
		return err
	}
	censusURI := ""
	if p.CensusURI != nil {
		censusURI = *p.CensusURI
	}
	log.Infow("new election", map[string]interface{}{
		"processId":     fmt.Sprintf("%x", p.ProcessId),
		"entityId":      fmt.Sprintf("%x", p.EntityId),
		"startBlock":    p.StartBlock,
		"endBlock":      p.BlockCount + p.StartBlock,
		"mode":          p.Mode,
		"envelopeType":  p.EnvelopeType,
		"voteOptions":   p.VoteOptions,
		"censusRoot":    fmt.Sprintf("%x", p.CensusRoot),
		"censusOrigin":  models.CensusOrigin_name[int32(p.CensusOrigin)],
		"maxCensusSize": p.MaxCensusSize,
		"status":        p.Status,
		"height":        v.CurrentHeight(),
		"censusURI":     censusURI,
	})
	for _, l := range v.eventListeners {
		l.OnProcess(p.ProcessId, p.EntityId, fmt.Sprintf("%x", p.CensusRoot), censusURI, v.TxCounter())
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
	v.Tx.Lock()
	err = v.Tx.DeepSet(pid, updatedProcessBytes, StateTreeCfg(TreeProcess))
	v.Tx.Unlock()
	if err != nil {
		return err
	}
	for _, l := range v.eventListeners {
		l.OnCancel(pid, v.TxCounter())
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
		v.Tx.RLock()
		defer v.Tx.RUnlock()
	}
	return getProcess(v.mainTreeViewer(committed), pid)
}

// CountProcesses returns the overall number of processes the vochain has
func (v *State) CountProcesses(committed bool) (uint64, error) {
	// TODO: Once statedb.TreeView.Size() works, replace this by that.
	if !committed {
		v.Tx.RLock()
		defer v.Tx.RUnlock()
	}
	processesTree, err := v.mainTreeViewer(committed).SubTree(StateTreeCfg(TreeProcess))
	if err != nil {
		return 0, err
	}
	var count uint64
	if err := processesTree.Iterate(func(key []byte, value []byte) bool {
		count++
		return false
	}); err != nil {
		return 0, err
	}
	return count, nil
}

// ListProcessIDs returns the full list of process identifiers (pid).
func (v *State) ListProcessIDs(committed bool) ([][]byte, error) {
	if !committed {
		v.Tx.RLock()
		defer v.Tx.RUnlock()
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
	v.Tx.Lock()
	defer v.Tx.Unlock()
	return updateProcess(&v.Tx, p, pid)
}

// SetProcessStatus changes the process status to the one provided.
// One of ready, ended, canceled, paused, results.
// Transition checks are handled inside this function, so the caller
// does not need to worry about it.
func (v *State) SetProcessStatus(pid []byte, newstatus models.ProcessStatus, commit bool) error {
	process, err := v.Process(pid, false)
	if err != nil {
		return err
	}
	currentStatus := process.Status

	// Check if the state transition is valid
	switch newstatus {
	case models.ProcessStatus_READY:
		if currentStatus != models.ProcessStatus_PAUSED {
			return fmt.Errorf("cannot set process status from %s to ready", currentStatus.String())
		}
		if currentStatus == models.ProcessStatus_READY {
			return fmt.Errorf("process %x already in ready state", pid)
		}
	case models.ProcessStatus_ENDED:
		if currentStatus != models.ProcessStatus_READY {
			return fmt.Errorf("process %x can only be ended from ready status", pid)
		}
		if !process.Mode.Interruptible {
			if v.CurrentHeight() <= process.BlockCount+process.StartBlock {
				return fmt.Errorf("process %x is not interruptible, cannot change state to %s",
					pid, newstatus.String())
			}
		}
	case models.ProcessStatus_CANCELED:
		if currentStatus == models.ProcessStatus_CANCELED {
			return fmt.Errorf("process %x already in canceled state", pid)
		}
		if currentStatus == models.ProcessStatus_ENDED || currentStatus == models.ProcessStatus_RESULTS {
			return fmt.Errorf("cannot set state to canceled from ended or results")
		}
		if currentStatus != models.ProcessStatus_PAUSED && !process.Mode.Interruptible {
			return fmt.Errorf("process %x is not interruptible, cannot change state to %s",
				pid, newstatus.String())
		}
	case models.ProcessStatus_PAUSED:
		if currentStatus != models.ProcessStatus_READY {
			return fmt.Errorf("cannot set process status from %s to paused", currentStatus.String())
		}
		if currentStatus == models.ProcessStatus_PAUSED {
			return fmt.Errorf("process %x already in paused state", pid)
		}
		if !process.Mode.Interruptible {
			return fmt.Errorf("cannot pause process %x, it is not interruptible ", pid)
		}
	case models.ProcessStatus_RESULTS:
		if currentStatus == models.ProcessStatus_RESULTS {
			return fmt.Errorf("process %x already in results state", pid)
		}
		if currentStatus != models.ProcessStatus_ENDED && currentStatus != models.ProcessStatus_READY {
			return fmt.Errorf("cannot set state to results from %s", currentStatus.String())
		}
		if currentStatus == models.ProcessStatus_READY &&
			process.StartBlock+process.BlockCount <= v.CurrentHeight() {
			return fmt.Errorf("cannot set state to results from %s, process is still alive",
				currentStatus.String())
		}
	default:
		return fmt.Errorf("process status %s unknown", newstatus.String())
	}

	if commit {
		process.Status = newstatus
		if err := v.UpdateProcess(process, process.ProcessId); err != nil {
			return err
		}
		for _, l := range v.eventListeners {
			l.OnProcessStatusChange(process.ProcessId, process.Status, v.TxCounter())
		}
	}
	return nil
}

// SetProcessResults sets the results for a given process
func (v *State) SetProcessResults(pid []byte, result *models.ProcessResult, commit bool) error {
	process, err := v.Process(pid, false)
	if err != nil {
		return err
	}
	// Check if the state transition is valid
	// process must be ended, ready or results for setting the results
	if process.Status != models.ProcessStatus_ENDED &&
		process.Status != models.ProcessStatus_READY &&
		process.Status != models.ProcessStatus_RESULTS {
		return fmt.Errorf("cannot set results, invalid status: %s", process.Status)
	}
	if process.Status == models.ProcessStatus_READY &&
		process.StartBlock+process.BlockCount > v.CurrentHeight() {
		return fmt.Errorf("cannot set state to results, process is still alive")
	}
	if !bytes.Equal(result.ProcessId, process.ProcessId) {
		return fmt.Errorf(
			"invalid process id on result provided, expected: %x got: %x",
			process.ProcessId, result.ProcessId)
	}
	if !bytes.Equal(result.EntityId, process.EntityId) {
		return fmt.Errorf(
			"invalid entity id on result provided, expected: %x got: %x",
			process.EntityId, result.EntityId)
	}
	if result.OracleAddress == nil {
		return fmt.Errorf("cannot set results, oracle address is nil")
	}

	if commit {
		// Warning: if we don't set a maximum block number on which results can be
		// set by oracles, once oracles become third party contributed (by staking an
		// amount of tokens), there will be a potential risk of overflow on the
		// process.Results slice
		if process.Results == nil {
			n, err := v.Oracles(false)
			if err != nil {
				return fmt.Errorf("cannot set results: %w", err)
			}
			// use len*2 for security, a new oracle could be registered during
			// the time window from the first results tx to the last one
			process.Results = make([]*models.ProcessResult, len(n)*2)
		}
		// check the oracle has not already sent a results
		for _, pr := range process.Results {
			if bytes.Equal(pr.GetOracleAddress(), result.OracleAddress) {
				return fmt.Errorf("results already set for this oracle address")
			}
		}
		process.Results = append(process.Results, result)
		process.Status = models.ProcessStatus_RESULTS
		if err := v.UpdateProcess(process, process.ProcessId); err != nil {
			return fmt.Errorf("cannot set results: %w", err)
		}
		// Call event listeners
		for _, l := range v.eventListeners {
			l.OnProcessResults(process.ProcessId, result, v.TxCounter())
		}
	}
	return nil
}

// GetProcessResults returns a friendly representation of the results stored in the State (if any).
func (v *State) GetProcessResults(pid []byte) ([][]string, error) {
	// TO-DO (pau): use a LRU cache for results
	process, err := v.Process(pid, true)
	if err != nil {
		return nil, err
	}
	if len(process.Results) > 0 {
		// TO-DO (pau): return the whole list of oracle results
		return GetFriendlyResults(process.Results[0].GetVotes()), nil
	}
	return nil, fmt.Errorf("no results for process %x", pid)
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
			"cannot update census, invalid census origin: %s", process.CensusOrigin.String())
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
	}

	return nil
}

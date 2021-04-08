package vochain

import (
	"bytes"
	"fmt"

	"go.vocdoni.io/dvote/types"
	"go.vocdoni.io/proto/build/go/models"
	"google.golang.org/protobuf/proto"
)

// AddProcess adds or overides a new process to vochain
func (v *State) AddProcess(p *models.Process) error {
	newProcessBytes, err := proto.Marshal(p)
	if err != nil {
		return fmt.Errorf("cannot marshal process bytes: %w", err)
	}
	v.Lock()
	err = v.Store.Tree(ProcessTree).Add(p.ProcessId, newProcessBytes)
	v.Unlock()
	if err != nil {
		return err
	}
	censusURI := ""
	if p.CensusURI != nil {
		censusURI = *p.CensusURI
	}
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
	v.Lock()
	err = v.Store.Tree(ProcessTree).Add(pid, updatedProcessBytes)
	v.Unlock()
	if err != nil {
		return err
	}
	for _, l := range v.eventListeners {
		l.OnCancel(pid, v.TxCounter())
	}
	return nil
}

// Process returns a process info given a processId if exists
func (v *State) Process(pid []byte, isQuery bool) (*models.Process, error) {
	var processBytes []byte
	var err error
	v.RLock()
	if isQuery {
		processBytes = v.Store.ImmutableTree(ProcessTree).Get(pid)
	} else {
		processBytes = v.Store.Tree(ProcessTree).Get(pid)
	}
	v.RUnlock()
	if processBytes == nil {
		return nil, ErrProcessNotFound
	}
	process := new(models.Process)
	err = proto.Unmarshal(processBytes, process)
	if err != nil {
		return nil, fmt.Errorf("cannot unmarshal process (%s): %w", pid, err)
	}
	return process, nil
}

// CountProcesses returns the overall number of processes the vochain has
func (v *State) CountProcesses(isQuery bool) int64 {
	v.RLock()
	defer v.RUnlock()
	if isQuery {
		return int64(v.Store.ImmutableTree(ProcessTree).Count())
	}
	return int64(v.Store.Tree(ProcessTree).Count())

}

// set process stores in the database the process
func (v *State) setProcess(process *models.Process, pid []byte) error {
	if process == nil || len(process.ProcessId) != types.ProcessIDsize {
		return ErrProcessNotFound
	}
	updatedProcessBytes, err := proto.Marshal(process)
	if err != nil {
		return fmt.Errorf("cannot marshal updated process bytes: %w", err)
	}
	v.Lock()
	defer v.Unlock()
	if err := v.Store.Tree(ProcessTree).Add(pid, updatedProcessBytes); err != nil {
		return err
	}
	return nil
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
			if v.Header(false).Height <= int64(process.BlockCount+process.StartBlock) {
				return fmt.Errorf("process %x is not interruptible, cannot change state to %s",
					pid, newstatus.String())
			}
		}
	case models.ProcessStatus_CANCELED:
		if currentStatus == models.ProcessStatus_CANCELED {
			return fmt.Errorf("process %x already in canceled state", pid)
		}
		if currentStatus == models.ProcessStatus_ENDED || currentStatus == models.ProcessStatus_RESULTS {
			return fmt.Errorf("cannot set state canceled from ended or results")
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
		if currentStatus != models.ProcessStatus_ENDED {
			return fmt.Errorf("cannot set state results from %s", currentStatus.String())
		}
	default:
		return fmt.Errorf("process status %s unknown", newstatus.String())
	}

	if commit {
		process.Status = newstatus
		if err := v.setProcess(process, process.ProcessId); err != nil {
			return err
		}
		for _, l := range v.eventListeners {
			l.OnProcessStatusChange(process.ProcessId, process.Status, v.TxCounter())
		}
	}
	return nil
}

// SetProcessResults sets the results for a given process. Only if the process status
// allows and the format of the results allows to do so
// TODO: allow result confirm with another Tx (in order to have N/M oracle signatures)
func (v *State) SetProcessResults(pid []byte, result *models.ProcessResult, commit bool) error {
	process, err := v.Process(pid, false)
	if err != nil {
		return err
	}
	// Check if the state transition is valid
	// process must be ended for setting the results
	if process.Status != models.ProcessStatus_ENDED {
		return fmt.Errorf("cannot set results, invalid status: %s", process.Status)
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

	if commit {
		// If some of the event listeners return an error, do not include the transaction
		for _, l := range v.eventListeners {
			if err := l.OnProcessResults(process.ProcessId, result.Votes, v.TxCounter()); err != nil {
				return err
			}
		}
		process.Results = result
		process.Status = models.ProcessStatus_RESULTS
		if err := v.setProcess(process, process.ProcessId); err != nil {
			return err
		}
	}
	return nil
}

func (v *State) SetProcessCensus(pid, censusRoot []byte, censusURI string, commit bool) error {
	process, err := v.Process(pid, false)
	if err != nil {
		return err
	}
	// check valid state transition
	// dynamic census
	if !process.Mode.DynamicCensus {
		return fmt.Errorf(
			"cannot update census, only processes with dynamic census can update its census")
	}
	// census origin
	if !types.CensusOrigins[process.CensusOrigin].AllowCensusUpdate {
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

	if types.CensusOrigins[process.CensusOrigin].NeedsURI && censusURI == "" {
		return fmt.Errorf("process requires URI but an empty one was provided")
	}

	if commit {
		process.CensusRoot = censusRoot
		process.CensusURI = &censusURI
		if err := v.setProcess(process, process.ProcessId); err != nil {
			return err
		}
	}

	return nil
}

// NewProcessTxCheck is an abstraction of ABCI checkTx for creating a new process
func NewProcessTxCheck(vtx *models.Tx, txBytes, signature []byte, state *State) (*models.Process, error) {
	tx := vtx.GetNewProcess()
	if tx.Process == nil {
		return nil, fmt.Errorf("process data is empty")
	}
	// basic required fields check
	if tx.Process.VoteOptions == nil || tx.Process.EnvelopeType == nil || tx.Process.Mode == nil {
		return nil, fmt.Errorf("missing required fields (voteOptions, envelopeType or processMode)")
	}
	if tx.Process.VoteOptions.MaxCount == 0 || tx.Process.VoteOptions.MaxValue == 0 {
		return nil, fmt.Errorf("missing vote options parameters (maxCount or maxValue)")
	}
	// check signature available
	if signature == nil || tx == nil || txBytes == nil {
		return nil, fmt.Errorf("missing signature or new process transaction")
	}
	// get oracles
	oracles, err := state.Oracles(false)
	if err != nil || len(oracles) == 0 {
		return nil, fmt.Errorf("cannot check authorization against a nil or empty oracle list")
	}

	header := state.Header(false)
	if header == nil {
		return nil, fmt.Errorf("cannot fetch state header")
	}
	// start and endblock sanity check
	if int64(tx.Process.StartBlock) < header.Height {
		return nil, fmt.Errorf(
			"cannot add process with start block lower or equal than the current height")
	}
	if tx.Process.BlockCount <= 0 {
		return nil, fmt.Errorf(
			"cannot add process with duration lower or equal than the current height")
	}
	authorized, addr, err := verifySignatureAgainstOracles(oracles, txBytes, signature)
	if err != nil {
		return nil, err
	}
	if !authorized {
		return nil, fmt.Errorf("unauthorized to create a process, recovered addr is %s", addr.Hex())
	}
	// get process
	_, err = state.Process(tx.Process.ProcessId, false)
	if err == nil {
		return nil, fmt.Errorf("process with id (%x) already exists", tx.Process.ProcessId)
	}

	// check valid/implemented process types
	switch {
	case tx.Process.EnvelopeType.Anonymous:
		return nil, fmt.Errorf("anonymous process not yet implemented")
	case tx.Process.EnvelopeType.Serial:
		return nil, fmt.Errorf("serial process not yet implemented")
	}

	if tx.Process.EnvelopeType.EncryptedVotes || tx.Process.EnvelopeType.Anonymous {
		// We consider the zero value as nil for security
		tx.Process.EncryptionPublicKeys = make([]string, types.KeyKeeperMaxKeyIndex)
		tx.Process.EncryptionPrivateKeys = make([]string, types.KeyKeeperMaxKeyIndex)
		tx.Process.CommitmentKeys = make([]string, types.KeyKeeperMaxKeyIndex)
		tx.Process.RevealKeys = make([]string, types.KeyKeeperMaxKeyIndex)
	}
	return tx.Process, nil
}

// SetProcessTxCheck is an abstraction of ABCI checkTx for canceling an existing process
func SetProcessTxCheck(vtx *models.Tx, txBytes, signature []byte, state *State) error {
	tx := vtx.GetSetProcess()
	// check signature available
	if signature == nil || tx == nil || txBytes == nil {
		return fmt.Errorf("missing signature on set process transaction")
	}
	// get oracles
	oracles, err := state.Oracles(false)
	if err != nil || len(oracles) == 0 {
		return fmt.Errorf("cannot check authorization against a nil or empty oracle list")
	}
	// check signature
	authorized, addr, err := verifySignatureAgainstOracles(oracles, txBytes, signature)
	if err != nil {
		return err
	}
	if !authorized {
		return fmt.Errorf("unauthorized to set process status, recovered addr is %s", addr.Hex())
	}
	// get process
	process, err := state.Process(tx.ProcessId, false)
	if err != nil {
		return fmt.Errorf("cannot get process %x: %w", tx.ProcessId, err)
	}

	switch tx.Txtype {
	case models.TxType_SET_PROCESS_RESULTS:
		return state.SetProcessResults(process.ProcessId, tx.GetResults(), false)
	case models.TxType_SET_PROCESS_STATUS:
		return state.SetProcessStatus(process.ProcessId, tx.GetStatus(), false)
	case models.TxType_SET_PROCESS_CENSUS:
		return state.SetProcessCensus(process.ProcessId, tx.GetCensusRoot(), tx.GetCensusURI(), false)
	default:
		return fmt.Errorf("unknown set process tx type: %s", tx.Txtype)
	}
}

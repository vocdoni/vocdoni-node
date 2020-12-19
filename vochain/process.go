package vochain

import (
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
	mkuri := ""
	if p.CensusMkURI != nil {
		mkuri = *p.CensusMkURI
	}
	for _, l := range v.eventListeners {
		l.OnProcess(p.ProcessId, p.EntityId, fmt.Sprintf("%x", p.CensusMkRoot), mkuri)
	}
	return nil
}

// CancelProcess sets the process canceled atribute to true
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
	err = v.Store.Tree(ProcessTree).Add([]byte(pid), updatedProcessBytes)
	v.Unlock()
	if err != nil {
		return err
	}
	for _, l := range v.eventListeners {
		l.OnCancel(pid)
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

// SetProcessStatus changes the process status to the one provided. One of ready, ended, canceled, paused, results.
// Transition checks are handled inside this function, so the caller does not need to worry about it.
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
				return fmt.Errorf("process %x is not interruptible, cannot change state to %s", pid, newstatus.String())
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
			return fmt.Errorf("process %x is not interruptible, cannot change state to %s", pid, newstatus.String())
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
			l.OnProcessStatusChange(process.ProcessId, process.Status)
		}
	}
	return nil
}

// NewProcessTxCheck is an abstraction of ABCI checkTx for creating a new process
func NewProcessTxCheck(vtx *models.Tx, state *State) (*models.Process, error) {
	tx := vtx.GetNewProcess()
	if tx.Process == nil {
		return nil, fmt.Errorf("process data is empty")
	}
	// check signature available
	if vtx.Signature == nil || tx == nil {
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
		return nil, fmt.Errorf("cannot add process with start block lower or equal than the current tendermint height")
	}
	if tx.Process.BlockCount <= 0 {
		return nil, fmt.Errorf("cannot add process with duration lower or equal than the current tendermint height")
	}
	signedBytes, err := proto.Marshal(tx)
	if err != nil {
		return nil, fmt.Errorf("cannot marshal new process transaction")
	}
	authorized, addr, err := verifySignatureAgainstOracles(oracles, signedBytes, fmt.Sprintf("%x", vtx.Signature))
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

// CancelProcessTxCheck is an abstraction of ABCI checkTx for canceling an existing process
func CancelProcessTxCheck(vtx *models.Tx, state *State) error { // LEGACY
	tx := vtx.GetCancelProcess()
	// check signature available
	if vtx.Signature == nil || tx == nil {
		return fmt.Errorf("missing signature or setProcess transaction")
	}
	// get oracles
	oracles, err := state.Oracles(false)
	if err != nil || len(oracles) == 0 {
		return fmt.Errorf("cannot check authorization against a nil or empty oracle list")
	}
	// check signature
	signedBytes, err := proto.Marshal(tx)
	if err != nil {
		return fmt.Errorf("cannot marshal new process transaction")
	}
	authorized, addr, err := verifySignatureAgainstOracles(oracles, signedBytes, fmt.Sprintf("%x", vtx.Signature))
	if err != nil {
		return err
	}
	if !authorized {
		return fmt.Errorf("unauthorized to set a process status, recovered addr is %s", addr.Hex())
	}
	// get process
	process, err := state.Process(tx.ProcessId, false)
	if err != nil {
		return fmt.Errorf("cannot set process status %x: %s", tx.ProcessId, err)
	}
	// check process not already canceled or finalized
	if process.Status != models.ProcessStatus_READY {
		return fmt.Errorf("cannot cancel a not ready process")
	}
	endBlock := process.StartBlock + process.BlockCount
	var height int64
	if h := state.Header(false); h != nil {
		height = h.Height
	}
	if int64(endBlock) < height {
		return fmt.Errorf("cannot cancel a finalized process")
	}
	return nil
}

// SetProcessTxCheck is an abstraction of ABCI checkTx for canceling an existing process
func SetProcessTxCheck(vtx *models.Tx, state *State) error {
	tx := vtx.GetSetProcess()
	// check signature available
	if vtx.Signature == nil || tx == nil {
		return fmt.Errorf("missing signature on set process transaction")
	}
	// get oracles
	oracles, err := state.Oracles(false)
	if err != nil || len(oracles) == 0 {
		return fmt.Errorf("cannot check authorization against a nil or empty oracle list")
	}
	// check signature
	signedBytes, err := proto.Marshal(tx)
	if err != nil {
		return fmt.Errorf("cannot marshal new process transaction")
	}
	authorized, addr, err := verifySignatureAgainstOracles(oracles, signedBytes, fmt.Sprintf("%x", vtx.Signature))
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

	return state.SetProcessStatus(process.ProcessId, tx.GetStatus(), false)
}

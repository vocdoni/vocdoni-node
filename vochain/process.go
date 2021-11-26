package vochain

import (
	"bytes"
	"errors"
	"fmt"

	"github.com/vocdoni/arbo"
	"go.vocdoni.io/dvote/crypto/zk/artifacts"
	"go.vocdoni.io/dvote/log"
	"go.vocdoni.io/dvote/statedb"
	"go.vocdoni.io/dvote/types"
	"go.vocdoni.io/proto/build/go/models"
	"google.golang.org/protobuf/proto"
)

var emptyVotesRoot = make([]byte, VotesCfg.HashFunc().Len())
var emptyCensusRoot = make([]byte, CensusCfg.HashFunc().Len())
var emptyPreRegisterNullifiersRoot = make([]byte, PreRegisterNullifiersCfg.HashFunc().Len())

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
		if err := v.Tx.DeepAdd(p.ProcessId, newProcessBytes, ProcessesCfg); err != nil {
			return err
		}
		// If Mode.PreRegister && EnvelopeType.Anonymous we create (by
		// opening) a new empty poseidon census tree and nullifier tree
		// at p.ProcessId.
		if preRegister && anonymous {
			census, err := v.Tx.DeepSubTree(ProcessesCfg, CensusPoseidonCfg.WithKey(p.ProcessId))
			if err != nil {
				return err
			}
			// We store census size as little endian 64 bits.  Set it to 0.
			if err := statedb.SetUint64(census.NoState(), keyCensusLen, 0); err != nil {
				return err
			}
			if _, err = v.Tx.DeepSubTree(ProcessesCfg,
				PreRegisterNullifiersCfg.WithKey(p.ProcessId)); err != nil {
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
	err = v.Tx.DeepSet(pid, updatedProcessBytes, ProcessesCfg)
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
	processBytes, err := mainTreeView.DeepGet(pid, ProcessesCfg)
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
func (v *State) Process(pid []byte, isQuery bool) (*models.Process, error) {
	if !isQuery {
		v.Tx.RLock()
		defer v.Tx.RUnlock()
	}
	return getProcess(v.mainTreeViewer(isQuery), pid)
}

// CountProcesses returns the overall number of processes the vochain has
func (v *State) CountProcesses(isQuery bool) (uint64, error) {
	// TODO: Once statedb.TreeView.Size() works, replace this by that.
	if !isQuery {
		v.Tx.RLock()
		defer v.Tx.RUnlock()
	}
	processesTree, err := v.mainTreeViewer(isQuery).SubTree(ProcessesCfg)
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

func updateProcess(tx *treeTxWithMutex, p *models.Process, pid []byte) error {
	processesTree, err := tx.SubTree(ProcessesCfg)
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

// updateProcess updates an existing process
func (v *State) updateProcess(p *models.Process, pid []byte) error {
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
		if err := v.updateProcess(process, process.ProcessId); err != nil {
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
	// process must be ended or ready for setting the results
	if process.Status != models.ProcessStatus_ENDED && process.Status != models.ProcessStatus_READY {
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

	if commit {
		process.Results = result
		process.Status = models.ProcessStatus_RESULTS
		if err := v.updateProcess(process, process.ProcessId); err != nil {
			return err
		}
		// Call event listeners
		for _, l := range v.eventListeners {
			if err := l.OnProcessResults(process.ProcessId, result, v.TxCounter()); err != nil {
				log.Warnf("onProcessResults callback error: %v", err)
			}
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
		if err := v.updateProcess(process, process.ProcessId); err != nil {
			return err
		}
	}

	return nil
}

// NewProcessTxCheck is an abstraction of ABCI checkTx for creating a new process
func (app *BaseApplication) NewProcessTxCheck(vtx *models.Tx, txBytes,
	signature []byte, state *State) (*models.Process, error) {
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
	// start and endblock sanity check
	if tx.Process.StartBlock < state.CurrentHeight() {
		return nil, fmt.Errorf(
			"cannot add process with start block lower than or equal to the current height")
	}
	if tx.Process.BlockCount <= 0 {
		return nil, fmt.Errorf(
			"cannot add process with duration lower than or equal to the current height")
	}
	// Check if transaction owner has enough funds to create a process
	authorized, addr, err := state.VerifyAccountBalance(txBytes, signature, NewProcessCost)
	if err != nil {
		return nil, err
	}
	if authorized {
		// If owner authorized, check entityId matches with owner address
		if !bytes.Equal(tx.Process.EntityId, addr.Bytes()) {
			return nil, fmt.Errorf("process entityID and transaction owner do not match")
		}
	} else {
		// Check if the transaction comes from an oracle
		// Oracles can create processes with any entityID
		if authorized, err = state.IsOracle(addr); err != nil {
			return nil, err
		}
	}
	// If owner without balance and not an oracle, fail
	if !authorized {
		return nil, fmt.Errorf("unauthorized to create a process, recovered addr is %s", addr.Hex())
	}
	// get process
	_, err = state.Process(tx.Process.ProcessId, false)
	if err == nil {
		return nil, fmt.Errorf("process with id (%x) already exists", tx.Process.ProcessId)
	}

	// check valid/implemented process types
	// pre-regiser and anonymous must be either both enabled or disabled, as
	// we only support a single scenario of pre-register + anonymous.
	if tx.Process.Mode.PreRegister != tx.Process.EnvelopeType.Anonymous {
		return nil, fmt.Errorf("pre-register mode only supported " +
			"with anonymous envelope type and viceversa")
	}
	if tx.Process.Mode.PreRegister &&
		(tx.Process.MaxCensusSize == nil || *tx.Process.MaxCensusSize <= 0) {
		return nil, fmt.Errorf("pre-register mode requires setting " +
			"maxCensusSize to be > 0")
	}
	if tx.Process.Mode.PreRegister && tx.Process.EnvelopeType.Anonymous {
		var circuits []artifacts.CircuitConfig
		if genesis, ok := Genesis[app.chainId]; ok {
			circuits = genesis.CircuitsConfig
		} else {
			log.Warn("Using dev network genesis CircuitsConfig")
			circuits = Genesis["dev"].CircuitsConfig
		}
		if len(circuits) == 0 {
			return nil, fmt.Errorf("no circuit configs in the %v genesis", app.chainId)
		}
		if *tx.Process.MaxCensusSize > uint64(circuits[len(circuits)-1].Parameters[0]) {
			return nil, fmt.Errorf("maxCensusSize for anonymous envelope "+
				"cannot be bigger than the parameter for the biggest circuit (%v)",
				circuits[len(circuits)-1].Parameters[0])
		}
	}

	// TODO: Enable support for PreRegiser without Anonymous.  Figure out
	// all the required changes to support a process with a rolling census
	// that is not Anonymous.
	if tx.Process.EnvelopeType.Serial {
		return nil, fmt.Errorf("serial process not yet implemented")
	}

	if tx.Process.EnvelopeType.EncryptedVotes || tx.Process.EnvelopeType.Anonymous {
		// We consider the zero value as nil for security
		tx.Process.EncryptionPublicKeys = make([]string, types.KeyKeeperMaxKeyIndex)
		tx.Process.EncryptionPrivateKeys = make([]string, types.KeyKeeperMaxKeyIndex)
	}
	return tx.Process, nil
}

// SetProcessTxCheck is an abstraction of ABCI checkTx for canceling an existing process
func SetProcessTxCheck(vtx *models.Tx, txBytes, signature []byte, state *State) error {
	tx := vtx.GetSetProcess()
	// check signature available
	if signature == nil || tx == nil || txBytes == nil {
		return fmt.Errorf("missing signature on setProcess transaction")
	}

	// Check if transaction owner has enough funds to create a process
	authorized, addr, err := state.VerifyAccountBalance(txBytes, signature, SetProcessCost)
	if err != nil {
		return err
	}
	// get process
	process, err := state.Process(tx.ProcessId, false)
	if err != nil {
		return fmt.Errorf("cannot get process %x: %w", tx.ProcessId, err)
	}
	isOracle := false
	if authorized {
		// If owner authorized, check entityId matches with owner address
		if !bytes.Equal(process.EntityId, addr.Bytes()) {
			return fmt.Errorf("process entityID and transaction owner do not match")
		}
	} else {
		// Check if the transaction comes from an oracle
		// Oracles can create processes with any entityID
		if isOracle, err = state.IsOracle(addr); err != nil {
			return err
		}
	}

	// If owner without balance and not an oracle, fail
	if !authorized && !isOracle {
		return fmt.Errorf("unauthorized to set process status, recovered addr is %s", addr.Hex())
	}
	switch tx.Txtype {
	case models.TxType_SET_PROCESS_RESULTS:
		if !isOracle {
			return fmt.Errorf("only oracles can execute set process results transaction")
		}
		return state.SetProcessResults(process.ProcessId, tx.GetResults(), false)
	case models.TxType_SET_PROCESS_STATUS:
		return state.SetProcessStatus(process.ProcessId, tx.GetStatus(), false)
	case models.TxType_SET_PROCESS_CENSUS:
		return state.SetProcessCensus(process.ProcessId, tx.GetCensusRoot(), tx.GetCensusURI(), false)
	default:
		return fmt.Errorf("unknown setProcess tx type: %s", tx.Txtype)
	}
}

package vochain

import (
	"bytes"
	"errors"
	"fmt"

	"github.com/ethereum/go-ethereum/common"
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
	processesTree, err := v.mainTreeViewer(committed).SubTree(ProcessesCfg)
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
		if err := v.updateProcess(process, process.ProcessId); err != nil {
			return fmt.Errorf("cannot set results: %w", err)
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
		if err := v.updateProcess(process, process.ProcessId); err != nil {
			return err
		}
	}

	return nil
}

// NewProcessTxCheck is an abstraction of ABCI checkTx for creating a new process
func (app *BaseApplication) NewProcessTxCheck(vtx *models.Tx, txBytes,
	signature []byte, state *State) (*models.Process, common.Address, error) {
	tx := vtx.GetNewProcess()
	if tx.Process == nil {
		return nil, common.Address{}, fmt.Errorf("process data is empty")
	}
	// basic required fields check
	if tx.Process.VoteOptions == nil || tx.Process.EnvelopeType == nil || tx.Process.Mode == nil {
		return nil, common.Address{}, fmt.Errorf("missing required fields (voteOptions, envelopeType or processMode)")
	}
	if tx.Process.VoteOptions.MaxCount == 0 {
		return nil, common.Address{}, fmt.Errorf("missing vote maxCount parameter")
	}
	// check signature available
	if signature == nil || tx == nil || txBytes == nil {
		return nil, common.Address{}, fmt.Errorf("missing signature or new process transaction")
	}
	// start and block count sanity check
	// if startBlock is zero or one, the process will be enabled on the next block
	if tx.Process.StartBlock == 0 || tx.Process.StartBlock == 1 {
		tx.Process.StartBlock = state.CurrentHeight() + 1
	} else if tx.Process.StartBlock < state.CurrentHeight() {
		return nil, common.Address{}, fmt.Errorf(
			"cannot add process with start block lower than or equal to the current height")
	}
	if tx.Process.BlockCount <= 0 {
		return nil, common.Address{}, fmt.Errorf(
			"cannot add process with duration lower than or equal to the current height")
	}
	// get tx cost
	cost, err := state.TxCost(models.TxType_NEW_PROCESS, false)
	if err != nil {
		return nil, common.Address{}, fmt.Errorf("cannot get NewProcessTx transaction cost: %w", err)
	}
	addr, acc, err := state.AccountFromSignature(txBytes, signature)
	if err != nil {
		return nil, common.Address{}, err
	}
	// check balance and nonce
	if acc.Balance < cost {
		return nil, common.Address{}, ErrNotEnoughBalance
	}
	if acc.Nonce != tx.Nonce {
		return nil, common.Address{}, ErrAccountNonceInvalid
	}
	// check if process entityID matches tx sender
	if !bytes.Equal(tx.Process.EntityId, addr.Bytes()) {
		// Check if the transaction comes from an oracle
		// Oracles can create processes with any entityID
		isOracle, err := state.IsOracle(*addr)
		if err != nil {
			return nil, common.Address{}, err
		}
		if !isOracle {
			return nil, common.Address{}, fmt.Errorf("unauthorized to set process status, recovered addr is %s", addr.Hex())
		}
	}
	// get process
	_, err = state.Process(tx.Process.ProcessId, false)
	if err == nil {
		return nil, common.Address{}, fmt.Errorf("process with id (%x) already exists", tx.Process.ProcessId)
	}

	// check valid/implemented process types
	// pre-regiser and anonymous must be either both enabled or disabled, as
	// we only support a single scenario of pre-register + anonymous.
	if tx.Process.Mode.PreRegister != tx.Process.EnvelopeType.Anonymous {
		return nil, common.Address{}, fmt.Errorf("pre-register mode only supported " +
			"with anonymous envelope type and viceversa")
	}
	if tx.Process.Mode.PreRegister &&
		(tx.Process.MaxCensusSize == nil || *tx.Process.MaxCensusSize <= 0) {
		return nil, common.Address{}, fmt.Errorf("pre-register mode requires setting " +
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
			return nil, common.Address{}, fmt.Errorf("no circuit configs in the %v genesis", app.chainId)
		}
		if tx.Process.MaxCensusSize == nil {
			return nil, common.Address{}, fmt.Errorf("maxCensusSize is not provided")
		}
		if *tx.Process.MaxCensusSize > uint64(circuits[len(circuits)-1].Parameters[0]) {
			return nil, common.Address{}, fmt.Errorf("maxCensusSize for anonymous envelope "+
				"cannot be bigger than the parameter for the biggest circuit (%v)",
				circuits[len(circuits)-1].Parameters[0])
		}
	}

	// TODO: Enable support for PreRegiser without Anonymous.  Figure out
	// all the required changes to support a process with a rolling census
	// that is not Anonymous.
	if tx.Process.EnvelopeType.Serial {
		return nil, common.Address{}, fmt.Errorf("serial process not yet implemented")
	}

	if tx.Process.EnvelopeType.EncryptedVotes || tx.Process.EnvelopeType.Anonymous {
		// We consider the zero value as nil for security
		tx.Process.EncryptionPublicKeys = make([]string, types.KeyKeeperMaxKeyIndex)
		tx.Process.EncryptionPrivateKeys = make([]string, types.KeyKeeperMaxKeyIndex)
	}
	return tx.Process, *addr, nil
}

// SetProcessTxCheck is an abstraction of ABCI checkTx for canceling an existing process
func SetProcessTxCheck(vtx *models.Tx, txBytes, signature []byte, state *State) (common.Address, error) {
	tx := vtx.GetSetProcess()
	// check signature available
	if signature == nil || tx == nil || txBytes == nil {
		return common.Address{}, fmt.Errorf("missing signature on setProcess transaction")
	}
	// get tx cost
	cost, err := state.TxCost(tx.Txtype, false)
	if err != nil {
		return common.Address{}, fmt.Errorf("cannot get %s transaction cost: %w", tx.Txtype.String(), err)
	}
	addr, acc, err := state.AccountFromSignature(txBytes, signature)
	if err != nil {
		return common.Address{}, err
	}
	// check balance and nonce
	if acc.Balance < cost {
		return common.Address{}, ErrNotEnoughBalance
	}
	if acc.Nonce != tx.Nonce {
		return common.Address{}, ErrAccountNonceInvalid
	}
	// get process
	process, err := state.Process(tx.ProcessId, false)
	if err != nil {
		return common.Address{}, fmt.Errorf("cannot get process %x: %w", tx.ProcessId, err)
	}
	// check process entityID matches tx sender
	isOracle := false
	if !bytes.Equal(process.EntityId, addr.Bytes()) {
		// Check if the transaction comes from an oracle
		// Oracles can create processes with any entityID
		isOracle, err = state.IsOracle(*addr)
		if err != nil {
			return common.Address{}, err
		}
		if !isOracle {
			return common.Address{}, fmt.Errorf("unauthorized to set process status, recovered addr is %s", addr.Hex())
		}
	}
	switch tx.Txtype {
	case models.TxType_SET_PROCESS_RESULTS:
		if !isOracle {
			return common.Address{}, fmt.Errorf("only oracles can execute set process results transaction")
		}
		if acc.Balance < cost {
			return common.Address{}, ErrNotEnoughBalance
		}
		if acc.Nonce != tx.Nonce {
			return common.Address{}, ErrAccountNonceInvalid
		}
		results := tx.GetResults()
		if !bytes.Equal(results.OracleAddress, addr.Bytes()) {
			return common.Address{}, fmt.Errorf("cannot set results, oracle address provided in results does not match")
		}
		return *addr, state.SetProcessResults(process.ProcessId, results, false)
	case models.TxType_SET_PROCESS_STATUS:
		return *addr, state.SetProcessStatus(process.ProcessId, tx.GetStatus(), false)
	case models.TxType_SET_PROCESS_CENSUS:
		return *addr, state.SetProcessCensus(process.ProcessId, tx.GetCensusRoot(), tx.GetCensusURI(), false)
	default:
		return common.Address{}, fmt.Errorf("unknown setProcess tx type: %s", tx.Txtype)
	}
}

package vochain

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"sync/atomic"
	"time"

	"github.com/ethereum/go-ethereum/common"
	lru "github.com/hashicorp/golang-lru"
	"github.com/vocdoni/arbo"
	"go.vocdoni.io/dvote/db"
	"go.vocdoni.io/dvote/db/metadb"
	"go.vocdoni.io/dvote/log"
	"go.vocdoni.io/dvote/statedb"

	"go.vocdoni.io/proto/build/go/models"
	"google.golang.org/protobuf/proto"
)

const (
	// defaultHashLen is the most common default hash length for Arbo hashes.  This
	// is the value of arbo.HashFunctionSha256.Len(), arbo.HashFunctionPoseidon.Len() and
	// arbo.HashFunctionBlake2b.Len()
	defaultHashLen = 32
	// storageDirectory is the final directory name where all files are stored.
	storageDirectory = "vcstate"
	// snapshotsDirectory is the final directory name where the snapshots are stored.
	snapshotsDirectory = "snapshots"
)

type ErrHaltVochain struct {
	reason error
}

func (e ErrHaltVochain) Error() string { return fmt.Sprintf("halting vochain: %v", e.reason) }
func (e ErrHaltVochain) Unwrap() error { return e.reason }

// State represents the state of the vochain application
type State struct {
	// data directory for storing files
	dataDir string
	// db is the underlying key-value database used by the StateDB
	db db.Database
	// Store contains the StateDB.  We match every StateDB commit version
	// with the block height.
	Store *statedb.StateDB
	// Tx must always be accessed via mutex because there will be
	// concurrent operations on it.  In particular, while the Tx is being
	// written serially via Tendermint DeliverTx (which updates the StateDB
	// by processing Vochain transactions), parallel calls of Tendermint
	// CheckTx happen (which read the temporary state kept in the Tx to
	// validate Vochain transactions).
	Tx                treeTxWithMutex
	mainTreeViewValue atomic.Value
	DisableVoteCache  atomic.Value
	voteCache         *lru.Cache
	txCounter         int32
	eventListeners    []EventListener
	// currentHeight is the height of the current started block
	currentHeight uint32
	// chainID identifies the blockchain
	chainID string
}

// NewState creates a new State
func NewState(dbType, dataDir string) (*State, error) {
	database, err := metadb.New(dbType, filepath.Join(dataDir, storageDirectory))
	if err != nil {
		return nil, err
	}
	sdb, err := initStateDB(database)
	if err != nil {
		return nil, fmt.Errorf("cannot init StateDB: %s", err)
	}
	voteCache, err := lru.New(voteCacheSize)
	if err != nil {
		return nil, err
	}
	version, err := sdb.Version()
	if err != nil {
		return nil, err
	}
	root, err := sdb.Hash()
	if err != nil {
		return nil, err
	}
	log.Infof("state database is ready at version %d with hash %x",
		version, root)
	tx, err := sdb.BeginTx()
	if err != nil {
		return nil, err
	}
	mainTreeView, err := sdb.TreeView(nil)
	if err != nil {
		return nil, err
	}
	s := &State{
		dataDir:   dataDir,
		db:        database,
		Store:     sdb,
		Tx:        treeTxWithMutex{TreeTx: tx},
		voteCache: voteCache,
	}
	s.DisableVoteCache.Store(false)
	s.setMainTreeView(mainTreeView)
	return s, os.MkdirAll(filepath.Join(dataDir, storageDirectory, snapshotsDirectory), 0775)
}

// initStateDB initializes the StateDB with the default subTrees
func initStateDB(database db.Database) (*statedb.StateDB, error) {
	log.Infof("initializing StateDB")
	sdb := statedb.NewStateDB(database)
	startTime := time.Now()
	defer log.Infof("StateDB load took %s", time.Since(startTime))
	root, err := sdb.Hash()
	if err != nil {
		return nil, err
	}
	if !bytes.Equal(root, make([]byte, len(root))) {
		// StateDB already initialized if StateDB.Root != emptyHash
		return sdb, nil
	}
	update, err := sdb.BeginTx()
	if err != nil {
		return nil, err
	}
	defer update.Discard()
	// Create the Extra, Oracles, Validators and Processes subtrees (from
	// mainTree) by adding leaves in the mainTree that contain the
	// corresponding tree roots, and opening the subTrees for the first
	// time.
	treeCfg := StateTreeCfg(TreeExtra)
	if err := update.Add(treeCfg.Key(),
		make([]byte, treeCfg.HashFunc().Len())); err != nil {
		return nil, err
	}
	if _, err := update.SubTree(treeCfg); err != nil {
		return nil, err
	}
	treeCfg = StateTreeCfg(TreeOracles)
	if err := update.Add(treeCfg.Key(),
		make([]byte, treeCfg.HashFunc().Len())); err != nil {
		return nil, err
	}
	if _, err := update.SubTree(treeCfg); err != nil {
		return nil, err
	}
	treeCfg = StateTreeCfg(TreeValidators)
	if err := update.Add(treeCfg.Key(),
		make([]byte, treeCfg.HashFunc().Len())); err != nil {
		return nil, err
	}
	if _, err := update.SubTree(treeCfg); err != nil {
		return nil, err
	}
	treeCfg = StateTreeCfg(TreeProcess)
	if err := update.Add(treeCfg.Key(),
		make([]byte, treeCfg.HashFunc().Len())); err != nil {
		return nil, err
	}
	if _, err := update.SubTree(treeCfg); err != nil {
		return nil, err
	}
	treeCfg = StateTreeCfg(TreeAccounts)
	if err := update.Add(treeCfg.Key(),
		make([]byte, treeCfg.HashFunc().Len())); err != nil {
		return nil, err
	}
	if _, err := update.SubTree(treeCfg); err != nil {
		return nil, err
	}
	treeCfg = StateTreeCfg(TreeFaucet)
	if err := update.Add(treeCfg.Key(),
		make([]byte, treeCfg.HashFunc().Len())); err != nil {
		return nil, err
	}
	if _, err := update.SubTree(treeCfg); err != nil {
		return nil, err
	}

	return sdb, update.Commit(0)
}

// SetChainID sets the blockchain identifier.
func (v *State) SetChainID(chID string) {
	v.chainID = chID
}

var exist = []byte{1}

// AddOracle adds a trusted oracle given its address if not exists
func (v *State) AddOracle(address common.Address) error {
	v.Tx.Lock()
	defer v.Tx.Unlock()
	return v.Tx.DeepSet(address.Bytes(), exist, StateTreeCfg(TreeOracles))
}

// RemoveOracle removes a trusted oracle given its address if exists
func (v *State) RemoveOracle(address common.Address) error {
	v.Tx.Lock()
	defer v.Tx.Unlock()
	oracles, err := v.Tx.SubTree(StateTreeCfg(TreeOracles))
	if err != nil {
		return err
	}
	if _, err := oracles.Get(address.Bytes()); errors.Is(err, arbo.ErrKeyNotFound) {
		return fmt.Errorf("oracle not found: %w", err)
	} else if err != nil {
		return err
	}
	return oracles.Set(address.Bytes(), nil)
}

// Oracles returns the current oracles list
// When committed is false, the operation is executed also on not yet commited
// data from the currently open StateDB transaction.
// When committed is true, the operation is executed on the last commited version.
func (v *State) Oracles(committed bool) ([]common.Address, error) {
	if !committed {
		v.Tx.RLock()
		defer v.Tx.RUnlock()
	}

	oraclesTree, err := v.mainTreeViewer(committed).SubTree(StateTreeCfg(TreeOracles))
	if err != nil {
		return nil, err
	}

	var oracles []common.Address
	if err := oraclesTree.Iterate(func(key, value []byte) bool {
		// removed oracles are still in the tree but with value set to nil
		if len(value) == 0 {
			return true
		}
		oracles = append(oracles, common.BytesToAddress(key))
		return true
	}); err != nil {
		return nil, err
	}
	return oracles, nil
}

// IsOracle returns true if the address is a valid oracle
func (v *State) IsOracle(addr common.Address) (bool, error) {
	oracles, err := v.Oracles(false)
	if err != nil || len(oracles) == 0 {
		return false, fmt.Errorf("cannot check authorization against a nil or empty oracle list")
	}
	return func() bool {
		for _, oracle := range oracles {
			if oracle == addr {
				return true
			}
		}
		return false
	}(), nil
}

// RemoveValidator removes a tendermint validator identified by its address
func (v *State) RemoveValidator(address []byte) error {
	v.Tx.Lock()
	defer v.Tx.Unlock()
	validators, err := v.Tx.SubTree(StateTreeCfg(TreeValidators))
	if err != nil {
		return err
	}
	if _, err := validators.Get(address); errors.Is(err, arbo.ErrKeyNotFound) {
		return fmt.Errorf("validator not found: %w", err)
	} else if err != nil {
		return err
	}
	return validators.Set(address, nil)
}

// Validators returns a list of the chain validators
// When committed is false, the operation is executed also on not yet commited
// data from the currently open StateDB transaction.
// When committed is true, the operation is executed on the last commited version.
func (v *State) Validators(committed bool) ([]*models.Validator, error) {
	if !committed {
		v.Tx.RLock()
		defer v.Tx.RUnlock()
	}

	validatorsTree, err := v.mainTreeViewer(committed).SubTree(StateTreeCfg(TreeValidators))
	if err != nil {
		return nil, err
	}

	var validators []*models.Validator
	var callbackErr error
	if err := validatorsTree.Iterate(func(key, value []byte) bool {
		// removed validators are still in the tree but with value set
		// to nil
		if len(value) == 0 {
			return true
		}
		validator := &models.Validator{}
		if err := proto.Unmarshal(value, validator); err != nil {
			callbackErr = err
			return false
		}
		validators = append(validators, validator)
		return true
	}); err != nil {
		return nil, err
	}
	if callbackErr != nil {
		return nil, callbackErr
	}
	return validators, nil
}

// AddProcessKeys adds the keys to the process
func (v *State) AddProcessKeys(tx *models.AdminTx) error {
	if tx.ProcessId == nil || tx.KeyIndex == nil {
		return fmt.Errorf("no processId or keyIndex provided on AddProcessKeys")
	}
	process, err := v.Process(tx.ProcessId, false)
	if err != nil {
		return err
	}
	if tx.EncryptionPublicKey != nil {
		process.EncryptionPublicKeys[tx.GetKeyIndex()] = fmt.Sprintf("%x", tx.EncryptionPublicKey)
		log.Debugf("added encryption key %d for process %x: %x",
			tx.GetKeyIndex(), tx.ProcessId, tx.EncryptionPublicKey)
	}
	if process.KeyIndex == nil {
		process.KeyIndex = new(uint32)
	}
	*process.KeyIndex++
	if err := v.updateProcess(process, tx.ProcessId); err != nil {
		return err
	}
	for _, l := range v.eventListeners {
		l.OnProcessKeys(tx.ProcessId, fmt.Sprintf("%x", tx.EncryptionPublicKey), v.TxCounter())
	}
	return nil
}

// RevealProcessKeys reveals the keys of a process
func (v *State) RevealProcessKeys(tx *models.AdminTx) error {
	if tx.ProcessId == nil || tx.KeyIndex == nil {
		return fmt.Errorf("no processId or keyIndex provided on AddProcessKeys")
	}
	process, err := v.Process(tx.ProcessId, false)
	if err != nil {
		return err
	}
	if process.KeyIndex == nil || *process.KeyIndex < 1 {
		return fmt.Errorf("no keys to reveal, keyIndex is < 1")
	}
	ekey := ""
	if tx.EncryptionPrivateKey != nil {
		ekey = fmt.Sprintf("%x", tx.EncryptionPrivateKey)
		process.EncryptionPrivateKeys[tx.GetKeyIndex()] = ekey
		log.Debugf("revealed encryption key %d for process %x: %x",
			tx.GetKeyIndex(), tx.ProcessId, tx.EncryptionPrivateKey)
	}
	*process.KeyIndex--
	if err := v.updateProcess(process, tx.ProcessId); err != nil {
		return err
	}
	for _, l := range v.eventListeners {
		l.OnRevealKeys(tx.ProcessId, ekey, v.TxCounter())
	}
	return nil
}

// pathProcessIDsByStartBlock is the db path used to store ProcessIDs indexed
// by their StartBlock.
const pathProcessIDsByStartBlock = "pidByStartBlock"

// keyProcessIDsByStartBlock returns the db key where ProcessesIDs with
// startBlock are stored.
func keyProcessIDsByStartBlock(startBlock uint32) []byte {
	key := make([]byte, 4)
	binary.LittleEndian.PutUint32(key, startBlock)
	return []byte(path.Join(pathProcessIDsByStartBlock, string(key)))
}

// processIDsByStartBlock returns the ProcessIDs of processes with startBlock.
func (v *State) processIDsByStartBlock(startBlock uint32) ([][]byte, error) {
	noState := v.Tx.NoState()
	pidsBytes, err := noState.Get(keyProcessIDsByStartBlock(startBlock))
	if err == db.ErrKeyNotFound {
		return [][]byte{}, nil
	} else if err != nil {
		return nil, err
	}
	var pids models.ProcessIdList
	if err := proto.Unmarshal(pidsBytes, &pids); err != nil {
		return nil, fmt.Errorf("cannot proto.Unmarshal pids: %w", err)
	}
	return pids.ProcessIds, nil
}

// setProcessIDByStartBlock indexes the processIDs to by its processes
// startBlock.
func (v *State) setProcessIDByStartBlock(processID []byte, startBlock uint32) error {
	noState := v.Tx.NoState()
	var pids models.ProcessIdList
	if pidsBytes, err := noState.Get(keyProcessIDsByStartBlock(startBlock)); err == db.ErrKeyNotFound {
		// no pids indexed by startBlock, so we build upon an empty pids
	} else if err != nil {
		return err
	} else {
		if err := proto.Unmarshal(pidsBytes, &pids); err != nil {
			return fmt.Errorf("cannot proto.Unmarshal pids: %w", err)
		}
	}
	pids.ProcessIds = append(pids.ProcessIds, processID)
	pidsBytes, err := proto.Marshal(&pids)
	if err != nil {
		return err
	}
	return noState.Set(keyProcessIDsByStartBlock(startBlock), pidsBytes)
}

// setRollingCensusSize loads all processes from pids, and for those that are
// rolling, it sets the RollingCensusSize parameter.
func (v *State) setRollingCensusSize(pids [][]byte) error {
	mainTreeView := v.mainTreeViewer(false)
	for _, pid := range pids {
		process, err := getProcess(mainTreeView, pid)
		if err != nil {
			return err
		}
		if !process.Mode.PreRegister {
			continue
		}
		censusSize, err := getRollingCensusSize(mainTreeView, pid)
		if err != nil {
			return err
		}
		process.RollingCensusSize = &censusSize
		if err := updateProcess(&v.Tx, process, pid); err != nil {
			return err
		}

	}
	return nil
}

// Save persistent save of vochain mem trees
func (v *State) Save() ([]byte, error) {
	height := v.CurrentHeight()
	var pidsStartNextBlock [][]byte
	v.Tx.Lock()
	err := func() error {
		var err error
		pidsStartNextBlock, err = v.processIDsByStartBlock(height + 1)
		if err != nil {
			return fmt.Errorf("cannot get processIDs by StartBlock: %w", err)
		}
		if err = v.setRollingCensusSize(pidsStartNextBlock); err != nil {
			return fmt.Errorf("cannot set rollingCensusSize for processes")
		}

		if err := v.Tx.Commit(height); err != nil {
			return fmt.Errorf("cannot commit statedb tx: %w", err)
		}
		if v.Tx.TreeTx, err = v.Store.BeginTx(); err != nil {
			return fmt.Errorf("cannot begin statedb tx: %w", err)
		}
		return nil
	}()
	v.Tx.Unlock()
	if err != nil {
		return nil, err
	}
	mainTreeView, err := v.Store.TreeView(nil)
	if err != nil {
		return nil, fmt.Errorf("cannot get statdeb mainTreeView: %w", err)
	}
	v.setMainTreeView(mainTreeView)
	// Notify listeners about processes that start in the next block.
	if len(pidsStartNextBlock) > 0 {
		for _, l := range v.eventListeners {
			l.OnProcessesStart(pidsStartNextBlock)
		}
	}
	// Notify listeners about the commit state
	for _, l := range v.eventListeners {
		if err := l.Commit(height); err != nil {
			if _, fatal := err.(ErrHaltVochain); fatal {
				return nil, err
			}
			log.Warnf("event callback error on commit: %v", err)
		}
	}

	// TODO: Purge rolling censuses from all processes that start now
	// for _, pid := range pids {
	//   v.PurgeRollingCensus(pid)
	// }

	return v.Store.Hash()
}

// Rollback rollbacks to the last persistent db data version
func (v *State) Rollback() {
	for _, l := range v.eventListeners {
		l.Rollback()
	}
	v.Tx.Lock()
	defer v.Tx.Unlock()
	v.Tx.Discard()
	var err error
	if v.Tx.TreeTx, err = v.Store.BeginTx(); err != nil {
		log.Fatalf("cannot begin statedb tx: %s", err)
	}
	atomic.StoreInt32(&v.txCounter, 0)
}

func (v *State) Close() error {
	v.Tx.Lock()
	v.Tx.Discard()
	v.Tx.Unlock()

	return v.db.Close()
}

// LastHeight returns the last commited height (block count).  We match the
// StateDB Version with the height via the Commits done in Save.
func (v *State) LastHeight() (uint32, error) {
	return v.Store.Version()
}

// CurrentHeight returns the current state height (block count).
func (v *State) CurrentHeight() uint32 {
	return atomic.LoadUint32(&v.currentHeight)
}

// SetHeight sets the height for the current block.
func (v *State) SetHeight(height uint32) {
	atomic.StoreUint32(&v.currentHeight, height)
}

// WorkingHash returns the hash of the vochain StateDB (mainTree.Root)
func (v *State) WorkingHash() []byte {
	v.Tx.RLock()
	defer v.Tx.RUnlock()
	hash, err := v.Tx.Root()
	if err != nil {
		panic(fmt.Sprintf("cannot get statedb mainTree root: %s", err))
	}
	return hash
}

// TxCounterAdd adds to the atomic transaction counter
func (v *State) TxCounterAdd() {
	atomic.AddInt32(&v.txCounter, 1)
}

// TxCounter returns the current tx count
func (v *State) TxCounter() int32 {
	return atomic.LoadInt32(&v.txCounter)
}

package vochain

import (
	"bytes"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"errors"
	"fmt"
	"path"
	"path/filepath"
	"sync"
	"sync/atomic"
	"time"

	"github.com/ethereum/go-ethereum/common"
	lru "github.com/hashicorp/golang-lru"
	tmcrypto "github.com/tendermint/tendermint/crypto"
	ed25519 "github.com/tendermint/tendermint/crypto/ed25519"
	"github.com/vocdoni/arbo"
	"go.vocdoni.io/dvote/crypto/ethereum"
	"go.vocdoni.io/dvote/db"
	"go.vocdoni.io/dvote/db/metadb"
	"go.vocdoni.io/dvote/log"
	"go.vocdoni.io/dvote/statedb"

	"go.vocdoni.io/dvote/types"
	models "go.vocdoni.io/proto/build/go/models"
	"google.golang.org/protobuf/proto"
)

// defaultHashLen is the most common default hash length for Arbo hashes.  This
// is the value of arbo.HashFunctionSha256.Len(), arbo.HashFunctionPoseidon.Len() and
// arbo.HashFunctionBlake2b.Len()
const defaultHashLen = 32

// TxTypeCostToStateKey translates models.TxType to a string which the State uses
// as a key internally under the Extra tree
var TxTypeCostToStateKey = map[models.TxType]string{
	models.TxType_SET_PROCESS_STATUS:         "c_setProcessStatus",
	models.TxType_SET_PROCESS_CENSUS:         "c_setProcessCensus",
	models.TxType_SET_PROCESS_QUESTION_INDEX: "c_setProcessResults",
	models.TxType_SET_PROCESS_RESULTS:        "c_setProcessQuestionIndex",
	models.TxType_REGISTER_VOTER_KEY:         "c_registerKey",
	models.TxType_NEW_PROCESS:                "c_newProcess",
	models.TxType_SEND_TOKENS:                "c_sendTokens",
	models.TxType_SET_ACCOUNT_INFO:           "c_setAccountInfo",
	models.TxType_ADD_DELEGATE_FOR_ACCOUNT:   "c_addDelegateForAccount",
	models.TxType_DEL_DELEGATE_FOR_ACCOUNT:   "c_delDelegateForAccount",
	models.TxType_COLLECT_FAUCET:             "c_collectFaucet",
}

// rootLeafGetRoot is the GetRootFn function for a leaf that is the root
// itself.
func rootLeafGetRoot(value []byte) ([]byte, error) {
	if len(value) != defaultHashLen {
		return nil, fmt.Errorf("len(value) = %v != %v", len(value), defaultHashLen)
	}
	return value, nil
}

// rootLeafSetRoot is the SetRootFn function for a leaf that is the root
// itself.
func rootLeafSetRoot(value []byte, root []byte) ([]byte, error) {
	if len(value) != defaultHashLen {
		return nil, fmt.Errorf("len(value) = %v != %v", len(value), defaultHashLen)
	}
	return root, nil
}

// processGetCensusRoot is the GetRootFn function to get the rolling census
// root of a process leaf.
func processGetCensusRoot(value []byte) ([]byte, error) {
	var sdbProc models.StateDBProcess
	if err := proto.Unmarshal(value, &sdbProc); err != nil {
		return nil, fmt.Errorf("cannot unmarshal StateDBProcess: %w", err)
	}
	if len(sdbProc.Process.RollingCensusRoot) != defaultHashLen {
		return nil, fmt.Errorf("len(sdbProc.Process.RollingCensusRoot) != %v",
			defaultHashLen)
	}
	return sdbProc.Process.RollingCensusRoot, nil
}

// processSetCensusRoot is the SetRootFn function to set the rolling census
// root of a process leaf.
func processSetCensusRoot(value []byte, root []byte) ([]byte, error) {
	var sdbProc models.StateDBProcess
	if err := proto.Unmarshal(value, &sdbProc); err != nil {
		return nil, fmt.Errorf("cannot unmarshal StateDBProcess: %w", err)
	}
	sdbProc.Process.RollingCensusRoot = root
	newValue, err := proto.Marshal(&sdbProc)
	if err != nil {
		return nil, fmt.Errorf("cannot marshal StateDBProcess: %w", err)
	}
	return newValue, nil
}

// processGetPreRegisterNullifiersRoot is the GetRootFn function to get the nullifiers
// root of a process leaf.
func processGetPreRegisterNullifiersRoot(value []byte) ([]byte, error) {
	var sdbProc models.StateDBProcess
	if err := proto.Unmarshal(value, &sdbProc); err != nil {
		return nil, fmt.Errorf("cannot unmarshal StateDBProcess: %w", err)
	}
	if len(sdbProc.Process.NullifiersRoot) != defaultHashLen {
		return nil, fmt.Errorf("len(sdbProc.Process.NullifiersRoot) != %v",
			defaultHashLen)
	}
	return sdbProc.Process.NullifiersRoot, nil
}

// processSetPreRegisterNullifiersRoot is the SetRootFn function to set the nullifiers
// root of a process leaf.
func processSetPreRegisterNullifiersRoot(value []byte, root []byte) ([]byte, error) {
	var sdbProc models.StateDBProcess
	if err := proto.Unmarshal(value, &sdbProc); err != nil {
		return nil, fmt.Errorf("cannot unmarshal StateDBProcess: %w", err)
	}
	sdbProc.Process.NullifiersRoot = root
	newValue, err := proto.Marshal(&sdbProc)
	if err != nil {
		return nil, fmt.Errorf("cannot marshal StateDBProcess: %w", err)
	}
	return newValue, nil
}

// processGetVotesRoot is the GetRootFn function to get the votes root of a
// process leaf.
func processGetVotesRoot(value []byte) ([]byte, error) {
	var sdbProc models.StateDBProcess
	if err := proto.Unmarshal(value, &sdbProc); err != nil {
		return nil, fmt.Errorf("cannot unmarshal StateDBProcess: %w", err)
	}
	if len(sdbProc.VotesRoot) != defaultHashLen {
		return nil, fmt.Errorf("len(sdbProc.VotesRoot) != %v", defaultHashLen)
	}
	return sdbProc.VotesRoot, nil
}

// processSetVotesRoot is the SetRootFn function to set the votes root of a
// process leaf.
func processSetVotesRoot(value []byte, root []byte) ([]byte, error) {
	var sdbProc models.StateDBProcess
	if err := proto.Unmarshal(value, &sdbProc); err != nil {
		return nil, fmt.Errorf("cannot unmarshal StateDBProcess: %w", err)
	}
	sdbProc.VotesRoot = root
	newValue, err := proto.Marshal(&sdbProc)
	if err != nil {
		return nil, fmt.Errorf("cannot marshal StateDBProcess: %w", err)
	}
	return newValue, nil
}

// StateDB Tree hierarchy
// - Main
//   - Extra (key: string, value: []byte)
//   - Oracles (key: address, value: []byte{1} if exists)
//   - Validators (key: address, value: models.Validator)
//   - Accounts (key: address, value: models.Account)
//   - Processes (key: ProcessId, value: models.StateDBProcess)
//     - CensusPoseidon (key: sequential index 64 bits little endian, value: zkCensusKey)
//     - Nullifiers (key: pre-census user nullifier, value: weight used)
//     - Votes (key: VoteId, value: models.StateDBVote)

var (
	// TreasurerKey is the key representing the Treasurer entry on the Extra subtree
	TreasurerKey = "treasurer"
	// ExtraCfg is the Extra subTree configuration.
	ExtraCfg = statedb.NewTreeSingletonConfig(statedb.TreeParams{
		HashFunc:          arbo.HashFunctionSha256,
		KindID:            "xtra",
		MaxLevels:         256,
		ParentLeafGetRoot: rootLeafGetRoot,
		ParentLeafSetRoot: rootLeafSetRoot,
	})

	// OraclesCfg is the Oracles subTree configuration.
	OraclesCfg = statedb.NewTreeSingletonConfig(statedb.TreeParams{
		HashFunc:          arbo.HashFunctionSha256,
		KindID:            "oracs",
		MaxLevels:         256,
		ParentLeafGetRoot: rootLeafGetRoot,
		ParentLeafSetRoot: rootLeafSetRoot,
	})

	// ValidatorsCfg is the Validators subTree configuration.
	ValidatorsCfg = statedb.NewTreeSingletonConfig(statedb.TreeParams{
		HashFunc:          arbo.HashFunctionSha256,
		KindID:            "valids",
		MaxLevels:         256,
		ParentLeafGetRoot: rootLeafGetRoot,
		ParentLeafSetRoot: rootLeafSetRoot,
	})

	// ProcessesCfg is the Processes subTree configuration.
	ProcessesCfg = statedb.NewTreeSingletonConfig(statedb.TreeParams{
		HashFunc:          arbo.HashFunctionSha256,
		KindID:            "procs",
		MaxLevels:         256,
		ParentLeafGetRoot: rootLeafGetRoot,
		ParentLeafSetRoot: rootLeafSetRoot,
	})

	// CensusCfg is the Rolling census subTree (found under a Process leaf)
	// configuration for a process that supports non-anonymous voting with
	// rolling census.
	CensusCfg = statedb.NewTreeNonSingletonConfig(statedb.TreeParams{
		HashFunc:          arbo.HashFunctionSha256,
		KindID:            "cen",
		MaxLevels:         256,
		ParentLeafGetRoot: processGetCensusRoot,
		ParentLeafSetRoot: processSetCensusRoot,
	})

	// CensusPoseidonCfg is the Rolling census subTree (found under a
	// Process leaf) configuration when the process supports anonymous
	// voting with rolling census.  This Census subTree uses the SNARK
	// friendly hash function Poseidon.
	CensusPoseidonCfg = statedb.NewTreeNonSingletonConfig(statedb.TreeParams{
		HashFunc:          arbo.HashFunctionPoseidon,
		KindID:            "cenPos",
		MaxLevels:         64,
		ParentLeafGetRoot: processGetCensusRoot,
		ParentLeafSetRoot: processSetCensusRoot,
	})
	// PreRegisterNullifiersCfg is the Nullifiers subTree (found under a
	// Process leaf) configuration when the process supports anonymous
	// voting with rolling census.  This tree contains the pre-census
	// nullifiers that have pre-registered.
	PreRegisterNullifiersCfg = statedb.NewTreeNonSingletonConfig(statedb.TreeParams{
		HashFunc:          arbo.HashFunctionSha256,
		KindID:            "prNul",
		MaxLevels:         256,
		ParentLeafGetRoot: processGetPreRegisterNullifiersRoot,
		ParentLeafSetRoot: processSetPreRegisterNullifiersRoot,
	})

	// VotesCfg is the Votes subTree (found under a Process leaf) configuration.
	VotesCfg = statedb.NewTreeNonSingletonConfig(statedb.TreeParams{
		HashFunc:          arbo.HashFunctionSha256,
		KindID:            "votes",
		MaxLevels:         256,
		ParentLeafGetRoot: processGetVotesRoot,
		ParentLeafSetRoot: processSetVotesRoot,
	})

	// AccountsCfg is the Accounts subTree configuration.
	AccountsCfg = statedb.NewTreeSingletonConfig(statedb.TreeParams{
		HashFunc:          arbo.HashFunctionSha256,
		KindID:            "balan",
		MaxLevels:         256,
		ParentLeafGetRoot: rootLeafGetRoot,
		ParentLeafSetRoot: rootLeafSetRoot,
	})
)

// EventListener is an interface used for executing custom functions during the
// events of the block creation process.
// The order in which events are executed is: Rollback, OnVote, Onprocess, On..., Commit.
// The process is concurrency safe, meaning that there cannot be two sequences
// happening in parallel.
//
// If Commit() returns ErrHaltVochain, the error is considered a consensus
// failure and the blockchain will halt.
//
// If OncProcessResults() returns an error, the results transaction won't be included
// in the blockchain. This event relays on the event handlers to decide if results are
// valid or not since the Vochain State do not validate results.
type EventListener interface {
	OnVote(vote *models.Vote, txIndex int32)
	OnNewTx(hash []byte, blockHeight uint32, txIndex int32)
	OnProcess(pid, eid []byte, censusRoot, censusURI string, txIndex int32)
	OnProcessStatusChange(pid []byte, status models.ProcessStatus, txIndex int32)
	OnCancel(pid []byte, txIndex int32)
	OnProcessKeys(pid []byte, encryptionPub string, txIndex int32)
	OnRevealKeys(pid []byte, encryptionPriv string, txIndex int32)
	OnProcessResults(pid []byte, results *models.ProcessResult, txIndex int32) error
	OnProcessesStart(pids [][]byte)
	Commit(height uint32) (err error)
	Rollback()
}

type ErrHaltVochain struct {
	reason error
}

func (e ErrHaltVochain) Error() string { return fmt.Sprintf("halting vochain: %v", e.reason) }
func (e ErrHaltVochain) Unwrap() error { return e.reason }

// treeTxWithMutex is a wrapper over TreeTx with a mutex for convenient
// RWLocking.
type treeTxWithMutex struct {
	*statedb.TreeTx
	sync.RWMutex
}

// State represents the state of the vochain application
type State struct {
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
	Tx                  treeTxWithMutex
	mainTreeViewValue   atomic.Value
	DisableVoteCache    atomic.Value
	voteCache           *lru.Cache
	mempoolRemoveTxKeys func([][32]byte, bool)
	txCounter           int32
	eventListeners      []EventListener
	// currentHeight is the height of the current started block
	currentHeight uint32
}

// NewState creates a new State
func NewState(dbType, dataDir string) (*State, error) {
	database, err := metadb.New(dbType, filepath.Join(dataDir, "vcstate"))
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
		db:        database,
		Store:     sdb,
		Tx:        treeTxWithMutex{TreeTx: tx},
		voteCache: voteCache,
	}
	s.DisableVoteCache.Store(false)
	s.setMainTreeView(mainTreeView)
	return s, nil
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
	if err := update.Add(ExtraCfg.Key(),
		make([]byte, ExtraCfg.HashFunc().Len())); err != nil {
		return nil, err
	}
	if _, err := update.SubTree(ExtraCfg); err != nil {
		return nil, err
	}
	if err := update.Add(OraclesCfg.Key(),
		make([]byte, OraclesCfg.HashFunc().Len())); err != nil {
		return nil, err
	}
	if _, err := update.SubTree(OraclesCfg); err != nil {
		return nil, err
	}
	if err := update.Add(ValidatorsCfg.Key(),
		make([]byte, ValidatorsCfg.HashFunc().Len())); err != nil {
		return nil, err
	}
	if _, err := update.SubTree(ValidatorsCfg); err != nil {
		return nil, err
	}
	if err := update.Add(ProcessesCfg.Key(),
		make([]byte, ProcessesCfg.HashFunc().Len())); err != nil {
		return nil, err
	}
	if _, err := update.SubTree(ProcessesCfg); err != nil {
		return nil, err
	}
	if err := update.Add(AccountsCfg.Key(),
		make([]byte, AccountsCfg.HashFunc().Len())); err != nil {
		return nil, err
	}

	return sdb, update.Commit(0)
}

// MainTreeView is a thread-safe function to obtain a pointer to the last
// opened mainTree as a TreeView.
func (v *State) MainTreeView() *statedb.TreeView {
	return v.mainTreeViewValue.Load().(*statedb.TreeView)
}

// setMainTreeView is a thread-safe function to store a pointer to the last
// opened mainTree as TreeView.
func (v *State) setMainTreeView(treeView *statedb.TreeView) {
	v.mainTreeViewValue.Store(treeView)
}

// mainTreeViewer returns the mainTree as a treeViewer.  When isQuery is false,
// the mainTree returned is the not yet commited one from the currently open
// StateDB transaction.  When isQuery is true, the mainTree returned is the
// last commited version.
func (v *State) mainTreeViewer(isQuery bool) statedb.TreeViewer {
	if isQuery {
		return v.MainTreeView()
	}
	return v.Tx.AsTreeView()
}

// AddEventListener adds a new event listener, to receive method calls on block
// events as documented in EventListener.
func (v *State) AddEventListener(l EventListener) {
	v.eventListeners = append(v.eventListeners, l)
}

var exist = []byte{1}

// AddOracle adds a trusted oracle given its address if not exists
func (v *State) AddOracle(address common.Address) error {
	v.Tx.Lock()
	defer v.Tx.Unlock()
	return v.Tx.DeepSet(address.Bytes(), exist, OraclesCfg)
}

// RemoveOracle removes a trusted oracle given its address if exists
func (v *State) RemoveOracle(address common.Address) error {
	v.Tx.Lock()
	defer v.Tx.Unlock()
	oracles, err := v.Tx.SubTree(OraclesCfg)
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

// Oracles returns the current oracle list
func (v *State) Oracles(isQuery bool) ([]common.Address, error) {
	if !isQuery {
		v.Tx.RLock()
		defer v.Tx.RUnlock()
	}

	oraclesTree, err := v.mainTreeViewer(isQuery).SubTree(OraclesCfg)
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

// setTreasurer saves the Treasurer address to the state
func (v *State) setTreasurer(address common.Address) error {
	tBytes, err := proto.Marshal(
		&models.Treasurer{
			Address: address.Bytes(),
			Nonce:   0,
		},
	)
	if err != nil {
		return err
	}
	v.Tx.Lock()
	defer v.Tx.Unlock()
	return v.Tx.DeepSet([]byte(TreasurerKey), tBytes, ExtraCfg)
}

// Treasurer returns the address and the Treasurer nonce
func (v *State) Treasurer(isQuery bool) (*models.Treasurer, error) {
	if !isQuery {
		v.Tx.RLock()
		defer v.Tx.RUnlock()
	}
	extraTree, err := v.mainTreeViewer(isQuery).SubTree(ExtraCfg)
	if err != nil {
		return nil, err
	}
	var rawTreasurer []byte
	if rawTreasurer, err = extraTree.Get([]byte(TreasurerKey)); err != nil {
		return nil, err
	}
	var t models.Treasurer
	if err := proto.Unmarshal(rawTreasurer, &t); err != nil {
		return nil, err
	}
	return &t, nil
}

// IsTreasurer returns true if the given address matches the Treasurer address
func (v *State) IsTreasurer(addr common.Address) (bool, error) {
	t, err := v.Treasurer(false)
	if err != nil {
		return false, err
	}
	tAddr := common.BytesToAddress(t.Address)
	return addr == tAddr, nil
}

// incrementTreasurerNonce increments the treasurer nonce
func (v *State) incrementTreasurerNonce() error {
	t, err := v.Treasurer(false)
	if err != nil {
		return err
	}
	t.Nonce++
	tBytes, err := proto.Marshal(t)
	if err != nil {
		return err
	}
	v.Tx.Lock()
	defer v.Tx.Unlock()
	return v.Tx.DeepSet([]byte(TreasurerKey), tBytes, ExtraCfg)
}

// SetTxCost sets the given transaction cost
func (v *State) SetTxCost(txType models.TxType, cost uint64) error {
	key, ok := TxTypeCostToStateKey[txType]
	if !ok {
		return fmt.Errorf("txType %v shouldn't cost anything", txType)
	}
	v.Tx.Lock()
	defer v.Tx.Unlock()

	costBytes := [8]byte{}
	binary.LittleEndian.PutUint64(costBytes[:], cost)
	return v.Tx.DeepSet([]byte(key), costBytes[:], ExtraCfg)
}

// TxCost returns the cost of a given transaction
func (v *State) TxCost(txType models.TxType, isQuery bool) (uint64, error) {
	key, ok := TxTypeCostToStateKey[txType]
	if !ok {
		return 0, fmt.Errorf("txType %v shouldn't cost anything", txType)
	}
	if !isQuery {
		v.Tx.RLock()
		defer v.Tx.RUnlock()
	}
	extraTree, err := v.mainTreeViewer(isQuery).SubTree(ExtraCfg)
	if err != nil {
		return 0, err
	}
	var cost []byte
	if cost, err = extraTree.Get([]byte(key)); err != nil {
		return 0, err
	}
	return binary.LittleEndian.Uint64(cost), nil
}

// hexPubKeyToTendermintEd25519 decodes a pubKey string to a ed25519 pubKey
func hexPubKeyToTendermintEd25519(pubKey string) (tmcrypto.PubKey, error) {
	var tmkey ed25519.PubKey
	pubKeyBytes, err := hex.DecodeString(pubKey)
	if err != nil {
		return nil, err
	}
	if len(pubKeyBytes) != 32 {
		return nil, fmt.Errorf("pubKey length is invalid")
	}
	copy(tmkey[:], pubKeyBytes[:])
	return tmkey, nil
}

// AddValidator adds a tendemint validator if it is not already added
func (v *State) AddValidator(validator *models.Validator) error {
	v.Tx.Lock()
	defer v.Tx.Unlock()
	validatorBytes, err := proto.Marshal(validator)
	if err != nil {
		return err
	}
	return v.Tx.DeepSet(validator.Address, validatorBytes, ValidatorsCfg)
}

// RemoveValidator removes a tendermint validator identified by its address
func (v *State) RemoveValidator(address []byte) error {
	v.Tx.Lock()
	defer v.Tx.Unlock()
	validators, err := v.Tx.SubTree(ValidatorsCfg)
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

// Validators returns a list of the validators saved on persistent storage
func (v *State) Validators(isQuery bool) ([]*models.Validator, error) {
	if !isQuery {
		v.Tx.RLock()
		defer v.Tx.RUnlock()
	}

	validatorsTree, err := v.mainTreeViewer(isQuery).SubTree(ValidatorsCfg)
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
		process.EncryptionPublicKeys[*tx.KeyIndex] = fmt.Sprintf("%x", tx.EncryptionPublicKey)
		log.Debugf("added encryption key %d for process %x: %x",
			*tx.KeyIndex, tx.ProcessId, tx.EncryptionPublicKey)
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
		process.EncryptionPrivateKeys[*tx.KeyIndex] = ekey
		log.Debugf("revealed encryption key %d for process %x: %x",
			*tx.KeyIndex, tx.ProcessId, tx.EncryptionPrivateKey)
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

// VoteCount return the global vote count.
func (v *State) VoteCount(isQuery bool) (uint64, error) {
	if !isQuery {
		v.Tx.RLock()
		defer v.Tx.RUnlock()
	}
	noState := v.mainTreeViewer(isQuery).NoState()
	voteCountLE, err := noState.Get(voteCountKey)
	if errors.Is(err, db.ErrKeyNotFound) {
		return 0, nil
	} else if err != nil {
		return 0, err
	}
	return binary.LittleEndian.Uint64(voteCountLE), nil
}

// voteCountInc increases by 1 the global vote count.
func (v *State) voteCountInc() error {
	noState := v.Tx.NoState()
	voteCountLE, err := noState.Get(voteCountKey)
	if errors.Is(err, db.ErrKeyNotFound) {
		voteCountLE = make([]byte, 8)
	} else if err != nil {
		return err
	}
	voteCount := binary.LittleEndian.Uint64(voteCountLE)
	voteCount++
	binary.LittleEndian.PutUint64(voteCountLE, voteCount)
	return noState.Set(voteCountKey, voteCountLE)
}

// AddVote adds a new vote to a process and call the even listeners to OnVote.
// This method does not check if the vote already exist!
func (v *State) AddVote(vote *models.Vote) error {
	vid, err := v.voteID(vote.ProcessId, vote.Nullifier)
	if err != nil {
		return err
	}
	// save block number
	vote.Height = v.CurrentHeight()
	voteBytes, err := proto.Marshal(vote)
	if err != nil {
		return fmt.Errorf("cannot marshal vote: %w", err)
	}
	// TO-DO (pau): Why are we storing processID and nullifier?
	sdbVote := models.StateDBVote{
		VoteHash:  ethereum.HashRaw(voteBytes),
		ProcessId: vote.ProcessId,
		Nullifier: vote.Nullifier,
	}
	sdbVoteBytes, err := proto.Marshal(&sdbVote)
	if err != nil {
		return fmt.Errorf("cannot marshal sdbVote: %w", err)
	}
	v.Tx.Lock()
	err = func() error {
		if err := v.Tx.DeepAdd(vid, sdbVoteBytes,
			ProcessesCfg, VotesCfg.WithKey(vote.ProcessId)); err != nil {
			return err
		}
		return v.voteCountInc()
	}()
	v.Tx.Unlock()
	if err != nil {
		return err
	}
	for _, l := range v.eventListeners {
		l.OnVote(vote, v.TxCounter())
	}
	return nil
}

// NOTE(Edu): Changed this from byte(processID+nullifier) to
// hash(processID+nullifier) to allow using it as a key in Arbo tree.
// voteID = hash(processID+nullifier)
func (v *State) voteID(pid, nullifier []byte) ([]byte, error) {
	if len(pid) != types.ProcessIDsize {
		return nil, fmt.Errorf("wrong processID size %d", len(pid))
	}
	if len(nullifier) != types.VoteNullifierSize {
		return nil, fmt.Errorf("wrong nullifier size %d", len(nullifier))
	}
	vid := sha256.New()
	vid.Write(pid)
	vid.Write(nullifier)
	return vid.Sum(nil), nil
}

// Envelope returns the hash of a stored vote if exists.
func (v *State) Envelope(processID, nullifier []byte, isQuery bool) (_ []byte, err error) {
	vid, err := v.voteID(processID, nullifier)
	if err != nil {
		return nil, err
	}
	if !isQuery {
		v.Tx.RLock()
		defer v.Tx.RUnlock() // needs to be deferred due to the recover above
	}
	votesTree, err := v.mainTreeViewer(isQuery).DeepSubTree(
		ProcessesCfg, VotesCfg.WithKey(processID))
	if errors.Is(err, arbo.ErrKeyNotFound) {
		return nil, ErrProcessNotFound
	} else if err != nil {
		return nil, err
	}
	sdbVoteBytes, err := votesTree.Get(vid)
	if errors.Is(err, arbo.ErrKeyNotFound) {
		return nil, ErrVoteDoesNotExist
	} else if err != nil {
		return nil, err
	}
	var sdbVote models.StateDBVote
	if err := proto.Unmarshal(sdbVoteBytes, &sdbVote); err != nil {
		return nil, fmt.Errorf("cannot unmarshal sdbVote: %w", err)
	}
	return sdbVote.VoteHash, nil
}

// EnvelopeExists returns true if the envelope identified with voteID exists
func (v *State) EnvelopeExists(processID, nullifier []byte, isQuery bool) (bool, error) {
	_, err := v.Envelope(processID, nullifier, isQuery)
	if errors.Is(err, ErrProcessNotFound) {
		return false, nil
	} else if errors.Is(err, ErrVoteDoesNotExist) {
		return false, nil
	} else if err != nil {
		return false, err
	}
	return true, nil
}

// iterateVotes iterates fn over state tree entries with the processID prefix.
// if isQuery, the IAVL tree is used, otherwise the AVL tree is used.
func (v *State) iterateVotes(processID []byte,
	fn func(vid []byte, sdbVote *models.StateDBVote) bool, isQuery bool) error {
	if !isQuery {
		v.Tx.RLock()
		defer v.Tx.RUnlock()
	}
	votesTree, err := v.mainTreeViewer(isQuery).DeepSubTree(
		ProcessesCfg, VotesCfg.WithKey(processID))
	if err != nil {
		return err
	}
	var callbackErr error
	if err := votesTree.Iterate(func(key, value []byte) bool {
		var sdbVote models.StateDBVote
		if err := proto.Unmarshal(value, &sdbVote); err != nil {
			callbackErr = err
			return true
		}
		return fn(key, &sdbVote)
	}); err != nil {
		return err
	}
	if callbackErr != nil {
		return callbackErr
	}
	return nil
}

// CountVotes returns the number of votes registered for a given process id
func (v *State) CountVotes(processID []byte, isQuery bool) uint32 {
	var count uint32
	// TODO: Once statedb.TreeView.Size() works, replace this by that.
	v.iterateVotes(processID, func(vid []byte, sdbVote *models.StateDBVote) bool {
		count++
		return false
	}, isQuery)
	return count
}

// EnvelopeList returns a list of registered envelopes nullifiers given a processId
func (v *State) EnvelopeList(processID []byte, from, listSize int,
	isQuery bool) (nullifiers [][]byte) {
	idx := 0
	v.iterateVotes(processID, func(vid []byte, sdbVote *models.StateDBVote) bool {
		if idx >= from+listSize {
			return true
		}
		if idx >= from {
			nullifiers = append(nullifiers, sdbVote.Nullifier)
		}
		idx++
		return false
	}, isQuery)
	return nullifiers
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
	for _, l := range v.eventListeners {
		if err := l.Commit(height); err != nil {
			if _, fatal := err.(ErrHaltVochain); fatal {
				return nil, err
			}
			log.Warnf("event callback error on commit: %v", err)
		}
	}

	// Notify listeners about processes that start in the next block.
	if len(pidsStartNextBlock) > 0 {
		for _, l := range v.eventListeners {
			l.OnProcessesStart(pidsStartNextBlock)
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

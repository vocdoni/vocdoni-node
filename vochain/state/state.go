package state

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"
	"time"

	"github.com/ethereum/go-ethereum/common"
	lru "github.com/hashicorp/golang-lru/v2"
	"go.vocdoni.io/dvote/db"
	"go.vocdoni.io/dvote/db/metadb"
	"go.vocdoni.io/dvote/log"
	"go.vocdoni.io/dvote/statedb"
	"go.vocdoni.io/dvote/vochain/state/electionprice"

	"go.vocdoni.io/proto/build/go/models"
)

const (
	voteCachePurgeThreshold = uint32(180) // in blocks about 30 minutes
	voteCacheSize           = 100000

	// defaultHashLen is the most common default hash length for Arbo hashes.  This
	// is the value of arbo.HashFunctionSha256.Len(), arbo.HashFunctionPoseidon.Len() and
	// arbo.HashFunctionBlake2b.Len()
	defaultHashLen = 32
	// storageDirectory is the final directory name where all files are stored.
	storageDirectory = "vcstate"
	// snapshotsDirectory is the final directory name where the snapshots are stored.
	snapshotsDirectory = "snapshots"
)

var BurnAddress = common.HexToAddress("0xffffffffffffffffffffffffffffffffffffffff")

// _________________________ CENSUS ORIGINS __________________________

type CensusProperties struct {
	Name              string
	AllowCensusUpdate bool
	NeedsDownload     bool
	NeedsIndexSlot    bool
	NeedsURI          bool
	WeightedSupport   bool
}

var CensusOrigins = map[models.CensusOrigin]CensusProperties{
	models.CensusOrigin_OFF_CHAIN_TREE: {Name: "offchain tree",
		NeedsDownload: true, NeedsURI: true, AllowCensusUpdate: true},
	models.CensusOrigin_OFF_CHAIN_TREE_WEIGHTED: {
		Name: "offchain weighted tree", NeedsDownload: true, NeedsURI: true,
		WeightedSupport: true, AllowCensusUpdate: true,
	},
	models.CensusOrigin_ERC20: {Name: "erc20", NeedsDownload: true,
		WeightedSupport: true, NeedsIndexSlot: true},
	models.CensusOrigin_OFF_CHAIN_CA: {Name: "ca", WeightedSupport: true,
		NeedsURI: true, AllowCensusUpdate: true},
}

// State represents the state of the vochain application
type State struct {
	// data directory for storing files
	dataDir string
	// db is the underlying key-value database used by the StateDB
	db db.Database
	// Store contains the StateDB.  We match every StateDB commit version
	// with the block height.
	store          *statedb.StateDB
	eventListeners []EventListener
	// Tx must always be accessed via mutex because there will be
	// concurrent operations on it.  In particular, while the Tx is being
	// written serially via Tendermint DeliverTx (which updates the StateDB
	// by processing Vochain transactions), parallel calls of Tendermint
	// CheckTx happen (which read the temporary state kept in the Tx to
	// validate Vochain transactions).
	tx                treeTxWithMutex
	mainTreeViewValue atomic.Pointer[statedb.TreeView]
	DisableVoteCache  atomic.Bool
	voteCache         *lru.Cache[string, *Vote]
	txCounter         atomic.Int32
	// currentHeight is the height of the current started block
	currentHeight atomic.Uint32
	// chainID identifies the blockchain
	chainID string
	// electionPriceCalc is the calculator for the election price
	ElectionPriceCalc    *electionprice.Calculator
	ProcessBlockRegistry *ProcessBlockRegistry

	validSIKRoots    [][]byte
	mtxValidSIKRoots *sync.Mutex
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
	voteCache, err := lru.New[string, *Vote](voteCacheSize)
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
		dataDir:           dataDir,
		db:                database,
		store:             sdb,
		tx:                treeTxWithMutex{TreeTx: tx},
		voteCache:         voteCache,
		ElectionPriceCalc: &electionprice.Calculator{Disable: true},
	}
	s.DisableVoteCache.Store(false)
	s.setMainTreeView(mainTreeView)

	s.ProcessBlockRegistry = &ProcessBlockRegistry{
		db:    s.NoState(true),
		state: s,
	}
	s.mtxValidSIKRoots = &sync.Mutex{}
	if err := s.FetchValidSIKRoots(); err != nil {
		return nil, fmt.Errorf("cannot update valid SIK roots: %w", err)
	}
	return s, os.MkdirAll(filepath.Join(dataDir, storageDirectory, snapshotsDirectory), 0750)
}

// initStateDB initializes the StateDB with the default subTrees
func initStateDB(database db.Database) (*statedb.StateDB, error) {
	log.Infof("initializing StateDB")
	sdb := statedb.NewStateDB(database)
	startTime := time.Now()
	defer log.Infof("state database load took %s", time.Since(startTime))
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
	// Create the Extra, Validators and Processes subtrees (from
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
	treeCfg = StateTreeCfg(TreeSIK)
	if err := update.Add(treeCfg.Key(),
		make([]byte, treeCfg.HashFunc().Len())); err != nil {
		return nil, err
	}
	if _, err := update.SubTree(treeCfg); err != nil {
		return nil, err
	}

	return sdb, update.Commit(0)
}

// EventListeners returns the list of subscribed event listeners.
func (v *State) EventListeners() []EventListener {
	return v.eventListeners
}

// SetChainID sets the state chainID (blockchain identifier)
func (v *State) SetChainID(chID string) {
	v.chainID = chID
}

// ChainID gets the state chainID (blockchain identifier)
func (v *State) ChainID() string {
	return v.chainID
}

// SetElectionPriceCalc sets the election price calculator with the current network capacity and base price.
func (v *State) SetElectionPriceCalc() error {
	// initialize election price calculator
	electionBasePrice, err := v.TxBaseCost(models.TxType_NEW_PROCESS, false)
	if err != nil {
		if errors.Is(err, ErrTxCostNotFound) {
			electionBasePrice = 0
		} else {
			return fmt.Errorf("cannot fetch election base price: %w", err)
		}
	}
	v.ElectionPriceCalc = electionprice.NewElectionPriceCalculator(electionprice.DefaultElectionPriceFactors)
	v.ElectionPriceCalc.SetBasePrice(electionBasePrice)
	capacity, err := v.NetworkCapacity()
	if err != nil {
		return fmt.Errorf("cannot fetch network capacity: %w", err)
	}
	v.ElectionPriceCalc.SetCapacity(capacity)
	log.Infow("election price calculator initialized", "basePrice", electionBasePrice, "capacity", capacity)
	return nil
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
	if err := v.UpdateProcess(process, tx.ProcessId); err != nil {
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
	if err := v.UpdateProcess(process, tx.ProcessId); err != nil {
		return err
	}
	for _, l := range v.eventListeners {
		l.OnRevealKeys(tx.ProcessId, ekey, v.TxCounter())
	}
	return nil
}

// PrepareCommit prepares the state for commit. It returns the new root hash.
func (v *State) PrepareCommit() ([]byte, error) {
	v.tx.Lock()
	defer v.tx.Unlock()
	if err := v.tx.CommitOnTx(v.CurrentHeight()); err != nil {
		return nil, err
	}
	return v.mainTreeViewer(false).Root()
}

// Save persistent save of vochain mem trees. It returns the new root hash. It also notifies the event listeners.
// Save should usually be called after PrepareCommit().
func (v *State) Save() ([]byte, error) {
	height := v.CurrentHeight()
	var pidsStartNextBlock [][]byte

	// Notify listeners about processes that start in the next block.
	if len(pidsStartNextBlock) > 0 {
		for _, l := range v.eventListeners {
			l.OnProcessesStart(pidsStartNextBlock)
		}
	}
	// Notify listeners about the commit state
	for _, l := range v.eventListeners {
		if err := l.Commit(height); err != nil {
			log.Warnf("event callback error on commit: %v", err)
		}
	}

	// Commit the statedb tx
	// Note that we need to commit the tx after calling listeners, because
	// the listeners may need to get the previous (not committed) state.
	// Update the SIK merkle-tree roots
	if err := v.UpdateSIKRoots(); err != nil {
		return nil, fmt.Errorf("cannot update SIK roots: %w", err)
	}
	v.tx.Lock()
	defer v.tx.Unlock()
	err := func() error {
		var err error
		if err := v.tx.SaveWithoutCommit(); err != nil {
			return fmt.Errorf("cannot commit statedb tx: %w", err)
		}
		if v.tx.TreeTx, err = v.store.BeginTx(); err != nil {
			return fmt.Errorf("cannot begin statedb tx: %w", err)
		}
		v.txCounter.Store(0)
		return nil
	}()
	if err != nil {
		return nil, err
	}

	// Update the main state tree
	mainTreeView, err := v.store.TreeView(nil)
	if err != nil {
		return nil, fmt.Errorf("cannot get statedb mainTreeView: %w", err)
	}
	v.setMainTreeView(mainTreeView)
	return mainTreeView.Root()
}

// Rollback rollbacks to the last persistent db data version
func (v *State) Rollback() {
	for _, l := range v.eventListeners {
		l.Rollback()
	}
	v.tx.Lock()
	defer v.tx.Unlock()
	v.tx.Discard()
	v.store.NoStateWriteTx.Discard()
	var err error
	if v.tx.TreeTx, err = v.store.BeginTx(); err != nil {
		log.Errorf("cannot begin statedb tx: %s", err)
		return
	}
	v.txCounter.Store(0)
}

// Close closes the vochain StateDB.
func (v *State) Close() error {
	v.tx.Lock()
	v.tx.Discard()
	v.store.NoStateWriteTx.Discard()
	v.tx.Unlock()

	return v.db.Close()
}

// LastHeight returns the last committed height (block count).  We match the
// StateDB Version with the height via the Commits done in Save.
func (v *State) LastHeight() (uint32, error) {
	return v.store.Version()
}

// CurrentHeight returns the current state height (block count).
func (v *State) CurrentHeight() uint32 {
	return v.currentHeight.Load()
}

// SetHeight sets the height for the current (not committed) block.
func (v *State) SetHeight(height uint32) {
	v.currentHeight.Store(height)
}

// CommittedHash returns the hash of the last committed vochain StateDB
func (v *State) CommittedHash() []byte {
	hash, err := v.mainTreeViewer(true).Root()
	if err != nil {
		panic(fmt.Sprintf("cannot get statedb mainTree root: %s", err))
	}
	return hash
}

// TxCounterAdd adds to the atomic transaction counter
func (v *State) TxCounterAdd() {
	v.txCounter.Add(1)
}

// TxCounter returns the current tx count
func (v *State) TxCounter() int32 {
	return v.txCounter.Load()
}

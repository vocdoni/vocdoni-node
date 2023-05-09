package vochain

import (
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"path/filepath"
	"sync"
	"sync/atomic"
	"time"

	abcitypes "github.com/cometbft/cometbft/abci/types"
	crypto256k1 "github.com/cometbft/cometbft/crypto/secp256k1"
	"github.com/cometbft/cometbft/libs/service"
	"github.com/cometbft/cometbft/node"
	tmcli "github.com/cometbft/cometbft/rpc/client/local"
	ctypes "github.com/cometbft/cometbft/rpc/core/types"
	tmtypes "github.com/cometbft/cometbft/types"
	ethcommon "github.com/ethereum/go-ethereum/common"
	"go.vocdoni.io/dvote/crypto/zk/circuit"
	"go.vocdoni.io/dvote/vochain/genesis"
	"go.vocdoni.io/dvote/vochain/ist"
	vstate "go.vocdoni.io/dvote/vochain/state"
	"go.vocdoni.io/dvote/vochain/transaction"
	"go.vocdoni.io/dvote/vochain/transaction/vochaintx"

	"go.vocdoni.io/dvote/db/lru"
	"go.vocdoni.io/dvote/log"
	"go.vocdoni.io/proto/build/go/models"
)

const (
	// MempoolLaunchPruneIntervalBlocks is the number of blocks after which the mempool is pruned.
	MempoolLaunchPruneIntervalBlocks = 12 // 2 minutes
	// MempoolTxTTLBlocks is the maximum live time in blocks for a transaction before is pruned from the mempool.
	MempoolTxTTLBlocks = 60 // 10 minutes
)

var (
	// ErrTransactionNotFound is returned when the transaction is not found in the blockstore.
	ErrTransactionNotFound = fmt.Errorf("transaction not found")
)

// BaseApplication reflects the ABCI application implementation.
type BaseApplication struct {
	State              *vstate.State
	Istc               *ist.Controller
	Service            service.Service
	Node               *tmcli.Local
	TransactionHandler *transaction.TransactionHandler
	isSynchronizingFn  func() bool
	// tendermint WaitSync() function is racy, we need to use a mutex in order to avoid
	// data races when querying about the sync status of the blockchain.
	isSynchronizing atomic.Bool

	// mempoolTxRef is a map of tx hashes to the block height when they were added to the mempool.
	mempoolTxRef map[[32]byte]uint32
	// mempoolTxRefLock is a mutex to protect the mempoolTxRef map.
	mempoolTxRefLock sync.Mutex
	// mempoolTxRefToGC is a slice of tx hashes to be removed from the mempoolTxRef map on Commit().
	mempoolTxRefToGC [][32]byte

	// Callback blockchain functions
	fnGetBlockByHeight func(height int64) *tmtypes.Block
	fnGetBlockByHash   func(hash []byte) *tmtypes.Block
	fnSendTx           func(tx []byte) (*ctypes.ResultBroadcastTx, error)
	fnGetTx            func(height uint32, txIndex int32) (*models.SignedTx, error)
	fnGetTxHash        func(height uint32, txIndex int32) (*models.SignedTx, []byte, error)
	fnMempoolSize      func() int
	fnMempoolPrune     func(txKey [32]byte) error
	fnBeginBlock       func(req abcitypes.RequestBeginBlock) abcitypes.ResponseBeginBlock
	fnEndBlock         func(req abcitypes.RequestEndBlock) abcitypes.ResponseEndBlock

	blockCache *lru.AtomicCache
	// height of the last ended block
	height atomic.Uint32
	// endBlockTimestamp is the last block end timestamp calculated from local time.
	endBlockTimestamp atomic.Int64
	// startBlockTimestamp is the current block timestamp from tendermint's
	// abcitypes.RequestBeginBlock.Header.Time
	startBlockTimestamp atomic.Int64
	chainID             string
	circuitConfigTag    string
	dataDir             string
	genesisInfo         *tmtypes.GenesisDoc
}

// Ensure that BaseApplication implements abcitypes.Application.
var _ abcitypes.Application = (*BaseApplication)(nil)

// NewBaseApplication creates a new BaseApplication given a name and a DB backend.
// Node still needs to be initialized with SetNode.
// Callback functions still need to be initialized.
func NewBaseApplication(dbType, dbpath string) (*BaseApplication, error) {
	state, err := vstate.NewState(dbType, dbpath)
	if err != nil {
		return nil, fmt.Errorf("cannot create state: (%v)", err)
	}
	istc := ist.NewISTC(state)

	// Create the transaction handler for checking and processing transactions
	transactionHandler, err := transaction.NewTransactionHandler(
		state,
		istc,
		filepath.Join(dbpath, "txHandler"),
	)
	if err != nil {
		return nil, fmt.Errorf("cannot create transaction handler: (%v)", err)
	}
	// Load or download the zk verification keys
	if err := transactionHandler.LoadZkCircuit(circuit.DefaultCircuitConfigurationTag); err != nil {
		return nil, fmt.Errorf("cannot load zk circuit: %w", err)
	}
	return &BaseApplication{
		State:              state,
		Istc:               istc,
		TransactionHandler: transactionHandler,
		blockCache:         lru.NewAtomic(32),
		dataDir:            dbpath,
		chainID:            "test",
		circuitConfigTag:   circuit.DefaultCircuitConfigurationTag,
		genesisInfo:        &tmtypes.GenesisDoc{},
		mempoolTxRef:       make(map[[32]byte]uint32),
	}, nil
}

// BeginBlock is called at the beginning of every block.
func (app *BaseApplication) BeginBlock(req abcitypes.RequestBeginBlock) abcitypes.ResponseBeginBlock {
	if app.isSynchronizingFn != nil {
		if app.isSynchronizingFn() {
			app.isSynchronizing.Store(true)
		} else {
			app.isSynchronizing.Store(false)
		}
	}
	if app.Height()%MempoolLaunchPruneIntervalBlocks == 0 {
		// remove all expired txs from mempool
		count := 0
		app.mempoolTxRefLock.Lock()
		for txKey, height := range app.mempoolTxRef {
			if height+MempoolTxTTLBlocks > app.Height() {
				if app.fnMempoolPrune != nil {
					if err := app.fnMempoolPrune(txKey); err != nil {
						log.Warnw("mempool prune", "err", err.Error(), "tx", hex.EncodeToString(txKey[:]))
					}
				}
				count++
				delete(app.mempoolTxRef, txKey)
			}
		}
		app.mempoolTxRefLock.Unlock()
		if count > 0 {
			log.Infow("mempool prune", "txs", count, "height", app.Height())
		}
	}

	app.mempoolTxRefLock.Lock()
	app.mempoolTxRefToGC = [][32]byte{}
	app.mempoolTxRefLock.Unlock()
	return app.fnBeginBlock(req)
}

// EndBlock is called at the end of every block.
func (app *BaseApplication) EndBlock(req abcitypes.RequestEndBlock) abcitypes.ResponseEndBlock {
	return app.fnEndBlock(req)
}

// Info Return information about the application state.
// Used to sync Tendermint with the application during a handshake that happens on startup.
// The returned AppVersion will be included in the Header of every block.
// Tendermint expects LastBlockAppHash and LastBlockHeight to be updated during Commit,
// ensuring that Commit is never called twice for the same block height.
func (app *BaseApplication) Info(req abcitypes.RequestInfo) abcitypes.ResponseInfo {
	// print some basic version info about tendermint components
	log.Infof("tendermint Core version: %s", req.Version)
	log.Infof("tendermint P2P protocol version: %d", req.P2PVersion)
	log.Infof("tendermint Block protocol version: %d", req.BlockVersion)
	lastHeight, err := app.State.LastHeight()
	if err != nil {
		log.Fatalf("cannot get State.LastHeight: %v", err)
	}
	appHash, err := app.State.Store.Hash()
	if err != nil {
		log.Fatalf("cannot get Store.Hash: %v", err)
	}
	log.Infof("replaying blocks. Current height %d, current APP hash %x",
		lastHeight, appHash)
	return abcitypes.ResponseInfo{
		LastBlockHeight:  int64(lastHeight),
		LastBlockAppHash: appHash,
	}
}

// InitChain called once upon genesis
// ResponseInitChain can return a list of validators. If the list is empty,
// Tendermint will use the validators loaded in the genesis file.
func (app *BaseApplication) InitChain(req abcitypes.RequestInitChain) abcitypes.ResponseInitChain {
	// setting the app initial state with validators, height = 0 and empty apphash
	// unmarshal app state from genesis
	var genesisAppState genesis.GenesisAppState
	err := json.Unmarshal(req.AppStateBytes, &genesisAppState)
	if err != nil {
		fmt.Printf("%s\n", req.AppStateBytes)
		log.Fatalf("cannot unmarshal app state bytes: %v", err)
	}
	// create accounts
	for _, acc := range genesisAppState.Accounts {
		addr := ethcommon.BytesToAddress(acc.Address)
		if err := app.State.CreateAccount(addr, "", nil, acc.Balance); err != nil {
			if err != vstate.ErrAccountAlreadyExists {
				log.Fatalf("cannot create acount %x %v", addr, err)
			}
			if err := app.State.InitChainMintBalance(addr, acc.Balance); err != nil {
				log.Fatal(err)
			}
		}
		log.Infow("created account", "addr", addr.Hex(), "tokens", acc.Balance)
	}
	// get validators
	// TODO pau: unify this code with the one on apputils.go that essentially does the same
	tendermintValidators := []abcitypes.ValidatorUpdate{}
	for i := 0; i < len(genesisAppState.Validators); i++ {
		log.Infow("add genesis validator",
			"signingAddress", genesisAppState.Validators[i].Address.String(),
			"consensusPubKey", genesisAppState.Validators[i].PubKey.String(),
			"power", genesisAppState.Validators[i].Power,
			"name", genesisAppState.Validators[i].Name,
			"keyIndex", genesisAppState.Validators[i].KeyIndex,
		)

		v := &models.Validator{
			Address:  genesisAppState.Validators[i].Address,
			PubKey:   genesisAppState.Validators[i].PubKey,
			Power:    genesisAppState.Validators[i].Power,
			KeyIndex: uint32(genesisAppState.Validators[i].KeyIndex),
		}
		if err = app.State.AddValidator(v); err != nil {
			log.Fatal(err)
		}
		tendermintValidators = append(tendermintValidators,
			abcitypes.UpdateValidator(
				genesisAppState.Validators[i].PubKey,
				int64(genesisAppState.Validators[i].Power),
				crypto256k1.KeyType,
			))
	}

	// set treasurer address
	log.Infof("adding genesis treasurer %x", genesisAppState.Treasurer)
	if err := app.State.SetTreasurer(ethcommon.BytesToAddress(genesisAppState.Treasurer), 0); err != nil {
		log.Fatalf("could not set State.Treasurer from genesis file: %s", err)
	}

	// add tx costs
	for k, v := range genesisAppState.TxCost.AsMap() {
		err = app.State.SetTxCost(k, v)
		if err != nil {
			log.Fatalf("could not set tx cost %q to value %q from genesis file to the State", k, v)
		}
	}

	// create burn account
	if err := app.State.SetAccount(vstate.BurnAddress, &vstate.Account{}); err != nil {
		log.Fatal("unable to set burn address")
	}

	// set max election size
	if err := app.State.SetMaxProcessSize(genesisAppState.MaxElectionSize); err != nil {
		log.Fatal("unable to set max election size")
	}

	// commit state and get hash
	hash, err := app.State.Save()
	if err != nil {
		log.Fatalf("cannot save state: %s", err)
	}
	return abcitypes.ResponseInitChain{
		Validators: tendermintValidators,
		AppHash:    hash,
	}
}

// CheckTx unmarshals req.Tx and checks its validity
func (app *BaseApplication) CheckTx(req abcitypes.RequestCheckTx) abcitypes.ResponseCheckTx {
	var response *transaction.TransactionResponse
	var err error
	height := app.Height()
	if req.Type == abcitypes.CheckTxType_Recheck {
		return abcitypes.ResponseCheckTx{Code: 0}
	}
	tx := new(vochaintx.Tx)
	if err = tx.Unmarshal(req.Tx, app.ChainID()); err == nil {
		if response, err = app.TransactionHandler.CheckTx(tx, false); err != nil {
			if errors.Is(err, transaction.ErrorAlreadyExistInCache) {
				return abcitypes.ResponseCheckTx{Code: 0}
			}
			log.Errorw(err, "checkTx")
			return abcitypes.ResponseCheckTx{Code: 1, Data: []byte("checkTx " + err.Error())}
		}
	} else {
		return abcitypes.ResponseCheckTx{Code: 1, Data: []byte("unmarshalTx " + err.Error())}
	}
	// add tx to mempool reference map for recheck prunning
	app.mempoolTxRefLock.Lock()
	app.mempoolTxRef[tx.TxID] = height
	app.mempoolTxRefLock.Unlock()

	return abcitypes.ResponseCheckTx{
		Code: 0,
		Data: response.Data,
		Info: fmt.Sprintf("%x", response.TxHash),
		Log:  response.Log,
	}
}

// DeliverTx unmarshals req.Tx and adds it to the State if it is valid
func (app *BaseApplication) DeliverTx(req abcitypes.RequestDeliverTx) abcitypes.ResponseDeliverTx {
	var response *transaction.TransactionResponse
	var err error
	// Increase Tx counter on return since the index 0 is valid
	defer app.State.TxCounterAdd()
	tx := new(vochaintx.Tx)
	if err = tx.Unmarshal(req.Tx, app.ChainID()); err == nil {
		log.Debugw("deliver tx",
			"hash", fmt.Sprintf("%x", tx.TxID),
			"type", tx.TxModelType,
			"height", app.Height(),
			"tx", tx.Tx,
		)
		// add tx to mempool reference map for prunning on Commit()
		app.mempoolTxRefLock.Lock()
		app.mempoolTxRefToGC = append(app.mempoolTxRefToGC, tx.TxID)
		app.mempoolTxRefLock.Unlock()
		// check tx is correct on the current state
		if response, err = app.TransactionHandler.CheckTx(tx, true); err != nil {
			log.Errorw(err, "rejected tx")
			return abcitypes.ResponseDeliverTx{Code: 1, Data: []byte(err.Error())}
		}
		// call event listeners
		for _, e := range app.State.EventListeners() {
			e.OnNewTx(tx, app.Height()+1, app.State.TxCounter())
		}
	} else {
		return abcitypes.ResponseDeliverTx{Code: 1, Data: []byte(err.Error())}
	}
	return abcitypes.ResponseDeliverTx{
		Code: 0,
		Data: response.Data,
		Info: fmt.Sprintf("%x", response.TxHash),
		Log:  response.Log,
	}
}

// Commit saves the current vochain state and returns a commit hash
func (app *BaseApplication) Commit() abcitypes.ResponseCommit {
	// execute internal state transition commit
	if err := app.Istc.Commit(app.Height(), app.IsSynchronizing()); err != nil {
		log.Fatalf("cannot execute ISTC commit: %v", err)
	}
	// save state
	data, err := app.State.Save()
	if err != nil {
		log.Fatalf("cannot save state: %v", err)
	}
	// perform state snapshot (DISABLED)
	if false && app.Height()%50000 == 0 && !app.IsSynchronizing() { // DISABLED
		startTime := time.Now()
		log.Infof("performing a state snapshot on block %d", app.Height())
		if _, err := app.State.Snapshot(); err != nil {
			log.Fatalf("cannot make state snapshot: %v", err)
		}
		log.Infof("snapshot created successfully, took %s", time.Since(startTime))
		log.Debugf("%+v", app.State.ListSnapshots())
	}
	// prune pending mempool tx references
	app.mempoolTxRefLock.Lock()
	for _, txID := range app.mempoolTxRefToGC {
		delete(app.mempoolTxRef, txID)
	}
	app.mempoolTxRefLock.Unlock()
	return abcitypes.ResponseCommit{
		Data: data,
	}
}

// GetBlockByHeight retrieves a full block indexed by its height.
// This method uses an LRU cache for the blocks so in general it is more
// convenient for high load operations than GetBlockByHash(), which does not use cache.
func (app *BaseApplication) GetBlockByHeight(height int64) *tmtypes.Block {
	if app.fnGetBlockByHeight == nil {
		log.Errorw(fmt.Errorf("method not assigned"), "getBlockByHeight")
		return nil
	}
	cachedBlock := app.blockCache.GetAndUpdate(height, func(prev interface{}) interface{} {
		if prev != nil {
			// If it's already in the cache, use it as-is.
			return prev
		}
		return app.fnGetBlockByHeight(height)
	})
	return cachedBlock.(*tmtypes.Block)
}

// GetBlockByHash retreies a full block indexed by its Hash
func (app *BaseApplication) GetBlockByHash(hash []byte) *tmtypes.Block {
	if app.fnGetBlockByHash == nil {
		log.Errorw(fmt.Errorf("method not assigned"), "getBlockByHash")
		return nil
	}
	return app.fnGetBlockByHash(hash)
}

// GetTx retrieves a vochain transaction from the blockstore
func (app *BaseApplication) GetTx(height uint32, txIndex int32) (*models.SignedTx, error) {
	return app.fnGetTx(height, txIndex)
}

// GetTxHash retrieves a vochain transaction, with its hash, from the blockstore
func (app *BaseApplication) GetTxHash(height uint32, txIndex int32) (*models.SignedTx, []byte, error) {
	return app.fnGetTxHash(height, txIndex)
}

// SendTx sends a transaction to the mempool (sync)
func (app *BaseApplication) SendTx(tx []byte) (*ctypes.ResultBroadcastTx, error) {
	if app.fnSendTx == nil {
		log.Errorw(fmt.Errorf("method not assigned"), "sendTx")
		return nil, nil
	}
	return app.fnSendTx(tx)
}

// Query does nothing
func (app *BaseApplication) Query(req abcitypes.RequestQuery) abcitypes.ResponseQuery {
	return abcitypes.ResponseQuery{}
}

// PrepareProposal does nothing
func (app *BaseApplication) PrepareProposal(req abcitypes.RequestPrepareProposal) abcitypes.ResponsePrepareProposal {
	return abcitypes.ResponsePrepareProposal{
		Txs: req.GetTxs(),
	}
}

// ProcessProposal does nothing
func (app *BaseApplication) ProcessProposal(req abcitypes.RequestProcessProposal) abcitypes.ResponseProcessProposal {
	return abcitypes.ResponseProcessProposal{
		Status: abcitypes.ResponseProcessProposal_ACCEPT,
	}
}

func (app *BaseApplication) ApplySnapshotChunk(
	req abcitypes.RequestApplySnapshotChunk) abcitypes.ResponseApplySnapshotChunk {
	return abcitypes.ResponseApplySnapshotChunk{}
}

func (app *BaseApplication) ListSnapshots(
	req abcitypes.RequestListSnapshots) abcitypes.ResponseListSnapshots {
	return abcitypes.ResponseListSnapshots{}
}

func (app *BaseApplication) LoadSnapshotChunk(
	req abcitypes.RequestLoadSnapshotChunk) abcitypes.ResponseLoadSnapshotChunk {
	return abcitypes.ResponseLoadSnapshotChunk{}
}

func (app *BaseApplication) OfferSnapshot(
	req abcitypes.RequestOfferSnapshot) abcitypes.ResponseOfferSnapshot {
	return abcitypes.ResponseOfferSnapshot{}
}

// ChainID returns the Node ChainID
func (app *BaseApplication) ChainID() string {
	return app.chainID
}

// SetChainID sets the app and state chainID
func (app *BaseApplication) SetChainID(chainId string) {
	app.chainID = chainId
	app.State.SetChainID(chainId)
}

// Genesis returns the tendermint genesis information
func (app *BaseApplication) Genesis() *tmtypes.GenesisDoc {
	return app.genesisInfo
}

// SetCircuitConfigTag sets the current BaseApplication circuit config tag
// attribute to the provided one and loads the circuit configuration based on
// it. The available circuit config tags are defined in
// /crypto/zk/circuit/config.go
func (app *BaseApplication) SetCircuitConfigTag(tag string) error {
	// Update the loaded circuit of the current app transactionHandler
	if err := app.TransactionHandler.LoadZkCircuit(tag); err != nil {
		return fmt.Errorf("cannot load zk circuit: %w", err)
	}
	app.circuitConfigTag = tag
	return nil
}

// CircuitConfigurationTag returns the Node CircuitConfigurationTag
func (app *BaseApplication) CircuitConfigurationTag() string {
	return app.circuitConfigTag
}

// IsSynchronizing informes if the blockchain is synchronizing or not.
func (app *BaseApplication) isSynchronizingTendermint() bool {
	if app.Service == nil {
		return true
	}
	return app.Service.(*node.Node).ConsensusReactor().WaitSync()
}

// IsSynchronizing informes if the blockchain is synchronizing or not.
// The value is updated every new block.
func (app *BaseApplication) IsSynchronizing() bool {
	return app.isSynchronizing.Load()
}

// Height returns the current blockchain height
func (app *BaseApplication) Height() uint32 {
	return app.height.Load()
}

// Timestamp returns the last block end timestamp
func (app *BaseApplication) Timestamp() int64 {
	return app.endBlockTimestamp.Load()
}

// TimestampStartBlock returns the current block start timestamp
func (app *BaseApplication) TimestampStartBlock() int64 {
	return app.startBlockTimestamp.Load()
}

// TimestampFromBlock returns the timestamp for a specific block height.
// If the block is not found, it returns nil.
// If the block is the current block, it returns the current block start timestamp.
func (app *BaseApplication) TimestampFromBlock(height int64) *time.Time {
	if int64(app.Height()) == height {
		t := time.Unix(app.TimestampStartBlock(), 0)
		return &t
	}
	blk := app.fnGetBlockByHeight(height)
	if blk == nil {
		return nil
	}
	return &blk.Time
}

// MempoolSize returns the size of the transaction mempool
func (app *BaseApplication) MempoolSize() int {
	return app.fnMempoolSize()
}

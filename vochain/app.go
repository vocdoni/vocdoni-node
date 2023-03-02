package vochain

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"path/filepath"
	"strconv"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	ethcommon "github.com/ethereum/go-ethereum/common"
	abcitypes "github.com/tendermint/tendermint/abci/types"
	"github.com/tendermint/tendermint/libs/service"
	tmprototypes "github.com/tendermint/tendermint/proto/tendermint/types"
	tmcli "github.com/tendermint/tendermint/rpc/client/local"
	ctypes "github.com/tendermint/tendermint/rpc/coretypes"
	tmtypes "github.com/tendermint/tendermint/types"
	"go.vocdoni.io/dvote/crypto/zk/circuit"
	vstate "go.vocdoni.io/dvote/vochain/state"
	"go.vocdoni.io/dvote/vochain/transaction"
	"go.vocdoni.io/dvote/vochain/transaction/vochaintx"
	"google.golang.org/protobuf/proto"

	"go.vocdoni.io/dvote/config"
	"go.vocdoni.io/dvote/db/lru"
	"go.vocdoni.io/dvote/db/metadb"
	"go.vocdoni.io/dvote/log"
	"go.vocdoni.io/dvote/test/testcommon/testutil"
	models "go.vocdoni.io/proto/build/go/models"
)

var (
	// ErrTransactionNotFound is returned when the transaction is not found in the blockstore.
	ErrTransactionNotFound = fmt.Errorf("transaction not found")
)

// BaseApplication reflects the ABCI application implementation.
type BaseApplication struct {
	State              *vstate.State
	Service            service.Service
	Node               *tmcli.Local
	TransactionHandler *transaction.TransactionHandler
	IsSynchronizing    func() bool
	// tendermint WaitSync() function is racy, we need to use a mutex in order to avoid
	// data races when querying about the sync status of the blockchain.
	isSyncLock sync.Mutex

	// Callback blockchain functions
	fnGetBlockByHeight func(height int64) *tmtypes.Block
	fnGetBlockByHash   func(hash []byte) *tmtypes.Block
	fnSendTx           func(tx []byte) (*ctypes.ResultBroadcastTx, error)
	fnGetTx            func(height uint32, txIndex int32) (*models.SignedTx, error)
	fnGetTxHash        func(height uint32, txIndex int32) (*models.SignedTx, []byte, error)
	fnMempoolSize      func() int
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
	// Create the transaction handler for checking and processing transactions
	transactionHandler, err := transaction.NewTransactionHandler(
		state,
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
		TransactionHandler: transactionHandler,
		blockCache:         lru.NewAtomic(32),
		dataDir:            dbpath,
		chainID:            "test",
		circuitConfigTag:   circuit.DefaultCircuitConfigurationTag,
		genesisInfo:        &tmtypes.GenesisDoc{},
	}, nil
}

// TestBaseApplication creates a new BaseApplication for testing purposes.
// It initializes the State, TransactionHandler and all the callback functions.
// Once the application is create, it is the caller's responsibility to call
// app.AdvanceTestBlock() to advance the block height and commit the state.
func TestBaseApplication(tb testing.TB) *BaseApplication {
	app, err := NewBaseApplication(metadb.ForTest(), tb.TempDir())
	if err != nil {
		tb.Fatal(err)
	}
	app.SetTestingMethods()
	genesisDoc, err := NewTemplateGenesisFile(tb.TempDir(), 4)
	if err != nil {
		tb.Fatal(err)
	}
	app.InitChain(abcitypes.RequestInitChain{
		Time:          time.Now(),
		ChainId:       "test",
		Validators:    []abcitypes.ValidatorUpdate{},
		AppStateBytes: genesisDoc.AppState,
	})
	// TODO: should this be a Close on the entire BaseApplication?
	tb.Cleanup(func() {
		if err := app.State.Close(); err != nil {
			tb.Error(err)
		}
	})
	return app
}

func (app *BaseApplication) SetNode(vochaincfg *config.VochainCfg, genesis []byte) error {
	var err error
	if app.Service, err = newTendermint(app, vochaincfg, genesis); err != nil {
		return fmt.Errorf("could not set tendermint node service: %s", err)
	}
	if vochaincfg.IsSeedNode {
		return nil
	}
	if app.Node, err = tmcli.New(app.Service.(tmcli.NodeService)); err != nil {
		return fmt.Errorf("could not start tendermint node client: %w", err)
	}
	nodeGenesis, err := app.Node.Genesis(context.TODO())
	if err != nil {
		log.Fatal(err)
	}
	app.genesisInfo = nodeGenesis.Genesis
	return nil
}

// SetDefaultMethods assigns fnGetBlockByHash, fnGetBlockByHeight, fnSendTx to use the
// BlockStore from app.Node to load blocks. Assumes app.Node has been set.
func (app *BaseApplication) SetDefaultMethods() {
	app.SetFnGetBlockByHash(func(hash []byte) *tmtypes.Block {
		resblock, err := app.Node.BlockByHash(context.Background(), hash)
		if err != nil {
			log.Warnf("cannot fetch block by hash: %v", err)
			return nil
		}
		return resblock.Block
	})

	app.SetFnGetBlockByHeight(func(height int64) *tmtypes.Block {
		resblock, err := app.Node.Block(context.Background(), &height)
		if err != nil {
			log.Warnf("cannot fetch block by height: %v", err)
			return nil
		}
		return resblock.Block
	})

	app.IsSynchronizing = app.isSynchronizingTendermint
	app.SetFnGetTx(app.getTxTendermint)
	app.SetFnGetTxHash(app.getTxHashTendermint)
	app.SetFnMempoolSize(func() int {
		// TODO: find the way to return correctly the mempool size
		return 0
	})
	app.SetFnBeginBlock(app.fnBeginBlockDefault)
	app.SetFnEndBlock(app.fnEndBlockDefault)
	app.SetFnSendTx(func(tx []byte) (*ctypes.ResultBroadcastTx, error) {
		return app.Node.BroadcastTxSync(context.Background(), tx)
	})
}

// SetTestingMethods assigns fnGetBlockByHash, fnGetBlockByHeight, fnSendTx to use mockBlockStore
func (app *BaseApplication) SetTestingMethods() {
	mockBlockStore := new(testutil.MockBlockStore)
	mockBlockStore.Init()
	app.SetFnGetBlockByHash(mockBlockStore.GetByHash)
	app.SetFnGetBlockByHeight(mockBlockStore.Get)
	app.SetFnGetTx(func(height uint32, txIndex int32) (*models.SignedTx, error) {
		blk := mockBlockStore.Get(int64(height))
		if blk == nil {
			return nil, fmt.Errorf("block not found")
		}
		if len(blk.Txs) <= int(txIndex) {
			return nil, fmt.Errorf("txIndex out of range")
		}
		stx := models.SignedTx{}
		return &stx, proto.Unmarshal(blk.Txs[txIndex], &stx)
	})
	app.SetFnGetTxHash(func(height uint32, txIndex int32) (*models.SignedTx, []byte, error) {
		blk := mockBlockStore.Get(int64(height))
		if blk == nil {
			return nil, nil, fmt.Errorf("block not found")
		}
		if len(blk.Txs) <= int(txIndex) {
			return nil, nil, fmt.Errorf("txIndex out of range")
		}
		stx := models.SignedTx{}
		tx := blk.Txs[txIndex]
		return &stx, tx.Hash(), proto.Unmarshal(blk.Txs[txIndex], &stx)
	})
	app.SetFnSendTx(func(tx []byte) (*ctypes.ResultBroadcastTx, error) {
		resp := app.DeliverTx(abcitypes.RequestDeliverTx{Tx: tx})
		if resp.Code == 0 {
			mockBlockStore.AddTxToBlock(tx)
		}
		return &ctypes.ResultBroadcastTx{
			Hash: tmtypes.Tx(tx).Hash(),
			Code: resp.Code,
			Data: resp.Data,
		}, nil
	})
	app.SetFnMempoolSize(func() int { return 0 })
	app.IsSynchronizing = func() bool { return false }
	app.SetFnEndBlock(func(req abcitypes.RequestEndBlock) abcitypes.ResponseEndBlock {
		height := mockBlockStore.EndBlock()
		app.endBlock(height, time.Now())
		//app.State.SetHeight(uint32(height))
		return abcitypes.ResponseEndBlock{}
	})
	app.SetFnBeginBlock(func(req abcitypes.RequestBeginBlock) abcitypes.ResponseBeginBlock {
		mockBlockStore.NewBlock(req.Header.Height)
		app.fnBeginBlockDefault(req)
		return abcitypes.ResponseBeginBlock{}
	})
	app.State.SetHeight(0)
	app.endBlockTimestamp.Store(time.Now().Unix())
}

// IsSynchronizing informes if the blockchain is synchronizing or not.
func (app *BaseApplication) isSynchronizingTendermint() bool {
	app.isSyncLock.Lock()
	defer app.isSyncLock.Unlock()
	status, err := app.Node.Status(context.Background())
	if err != nil {
		log.Warnf("error retrieving node status information: %v", err)
		return true
	}
	return status.SyncInfo.CatchingUp
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

// ChainID returns the Node ChainID
func (app *BaseApplication) ChainID() string {
	return app.chainID
}

// CircuitConfigurationTag returns the Node CircuitConfigurationTag
func (app *BaseApplication) CircuitConfigurationTag() string {
	return app.circuitConfigTag
}

// MempoolSize returns the size of the transaction mempool
func (app *BaseApplication) MempoolSize() int {
	return app.fnMempoolSize()
}

// GetBlockByHeight retrieves a full Tendermint block indexed by its height.
// This method uses an LRU cache for the blocks so in general it is more
// convinient for high load operations than GetBlockByHash(), which does not use cache.
func (app *BaseApplication) GetBlockByHeight(height int64) *tmtypes.Block {
	if app.fnGetBlockByHeight == nil {
		log.Error("application getBlockByHeight method not assigned")
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

// GetBlockByHash retreies a full Tendermint block indexed by its Hash
func (app *BaseApplication) GetBlockByHash(hash []byte) *tmtypes.Block {
	if app.fnGetBlockByHash == nil {
		log.Error("application getBlockByHash method not assigned")
		return nil
	}
	return app.fnGetBlockByHash(hash)
}

// GetTx retrieves a vochain transaction from the blockstore
func (app *BaseApplication) GetTx(height uint32, txIndex int32) (*models.SignedTx, error) {
	return app.fnGetTx(height, txIndex)
}

func (app *BaseApplication) getTxTendermint(height uint32, txIndex int32) (*models.SignedTx, error) {
	block := app.GetBlockByHeight(int64(height))
	if block == nil {
		return nil, ErrTransactionNotFound
	}
	if int32(len(block.Txs)) <= txIndex {
		return nil, ErrTransactionNotFound
	}
	tx := &models.SignedTx{}
	return tx, proto.Unmarshal(block.Txs[txIndex], tx)
}

// GetTxHash retrieves a vochain transaction, with its hash, from the blockstore
func (app *BaseApplication) GetTxHash(height uint32, txIndex int32) (*models.SignedTx, []byte, error) {
	return app.fnGetTxHash(height, txIndex)
}

func (app *BaseApplication) getTxHashTendermint(height uint32, txIndex int32) (*models.SignedTx, []byte, error) {
	block := app.GetBlockByHeight(int64(height))
	if block == nil {
		return nil, nil, ErrTransactionNotFound
	}
	if int32(len(block.Txs)) <= txIndex {
		return nil, nil, ErrTransactionNotFound
	}
	tx := &models.SignedTx{}
	return tx, block.Txs[txIndex].Hash(), proto.Unmarshal(block.Txs[txIndex], tx)
}

// SendTx sends a transaction to the mempool (sync)
func (app *BaseApplication) SendTx(tx []byte) (*ctypes.ResultBroadcastTx, error) {
	if app.fnSendTx == nil {
		log.Error("application sendTx method not assigned")
		return nil, nil
	}
	return app.fnSendTx(tx)
}

// BeginBlock is called at the beginning of every block.
func (app *BaseApplication) BeginBlock(req abcitypes.RequestBeginBlock) abcitypes.ResponseBeginBlock {
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
	// setting the app initial state with validators, oracles, height = 0 and empty apphash
	// unmarshal app state from genesis
	var genesisAppState GenesisAppState
	err := json.Unmarshal(req.AppStateBytes, &genesisAppState)
	if err != nil {
		fmt.Printf("%s\n", req.AppStateBytes)
		log.Fatalf("cannot unmarshal app state bytes: %v", err)
	}
	// get oracles
	for _, v := range genesisAppState.Oracles {
		log.Infof("adding genesis oracle %x", v)
		addr := ethcommon.BytesToAddress(v)
		if err := app.State.AddOracle(addr); err != nil {
			log.Fatalf("cannot add oracles: %v", err)
		}
		app.State.CreateAccount(addr, "", nil, 0)
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
		log.Infof("created acccount %x with %d tokens", addr, acc.Balance)
	}
	// get validators
	for i := 0; i < len(genesisAppState.Validators); i++ {
		log.Infof("adding genesis validator %x", genesisAppState.Validators[i].Address)
		pwr, err := strconv.ParseUint(genesisAppState.Validators[i].Power, 10, 64)
		if err != nil {
			log.Fatalf("cannot decode validator power: %s", err)
		}
		v := &models.Validator{
			Address: genesisAppState.Validators[i].Address,
			PubKey:  genesisAppState.Validators[i].PubKey.Value,
			Power:   pwr,
		}
		if err = app.State.AddValidator(v); err != nil {
			log.Fatal(err)
		}
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

	// Is this save needed?
	if _, err := app.State.Save(); err != nil {
		log.Fatalf("cannot save state: %s", err)
	}
	// TBD: using empty list here, should return validatorsUpdate to use the validators obtained here
	return abcitypes.ResponseInitChain{}
}

// fnBeginBlockDefault signals the beginning of a new block. Called prior to any DeliverTxs.
// The header contains the height, timestamp, and more - it exactly matches the
// Tendermint block header.
// The LastCommitInfo and ByzantineValidators can be used to determine rewards and
// punishments for the validators.
func (app *BaseApplication) fnBeginBlockDefault(
	req abcitypes.RequestBeginBlock) abcitypes.ResponseBeginBlock {
	app.State.Rollback()
	app.startBlockTimestamp.Store(req.Header.GetTime().Unix())
	height := uint32(req.Header.GetHeight())
	app.State.SetHeight(height)
	go app.State.CachePurge(height)

	return abcitypes.ResponseBeginBlock{}
}

// AdvanceTestBlock commits the current state, ends the current block and starts a new one.
// Advances the block height and timestamp.
func (app *BaseApplication) AdvanceTestBlock() {
	app.Commit()
	endingHeight := int64(app.Height())
	app.EndBlock(abcitypes.RequestEndBlock{Height: endingHeight})

	// The next block begins a second later.
	nextHeight := endingHeight + 1
	nextStartTime := time.Now()

	app.BeginBlock(abcitypes.RequestBeginBlock{Header: tmprototypes.Header{
		Time:   nextStartTime,
		Height: nextHeight,
	}})
}

// CheckTx unmarshals req.Tx and checks its validity
func (app *BaseApplication) CheckTx(req abcitypes.RequestCheckTx) abcitypes.ResponseCheckTx {
	var response *transaction.TransactionResponse
	var err error
	if req.Type == abcitypes.CheckTxType_Recheck {
		return abcitypes.ResponseCheckTx{Code: 0}
	}
	tx := new(vochaintx.VochainTx)
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
	tx := new(vochaintx.VochainTx)
	if err = tx.Unmarshal(req.Tx, app.ChainID()); err == nil {
		log.Debugw("deliver tx",
			"hash", fmt.Sprintf("%x", tx.TxID),
			"type", tx.TxModelType,
			"height", app.Height(),
			"tx", tx.Tx,
		)
		if response, err = app.TransactionHandler.CheckTx(tx, true); err != nil {
			log.Errorw(err, "rejected tx")
			return abcitypes.ResponseDeliverTx{Code: 1, Data: []byte(err.Error())}
		}
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
	data, err := app.State.Save()
	if err != nil {
		log.Fatalf("cannot save state: %v", err)
	}
	if false && app.Height()%50000 == 0 && !app.IsSynchronizing() { // DISABLED
		startTime := time.Now()
		log.Infof("performing a state snapshot on block %d", app.Height())
		if _, err := app.State.Snapshot(); err != nil {
			log.Fatalf("cannot make state snapshot: %v", err)
		}
		log.Infof("snapshot created successfully, took %s", time.Since(startTime))
		log.Debugf("%+v", app.State.ListSnapshots())
	}
	return abcitypes.ResponseCommit{
		Data: data,
	}
}

// Query does nothing
func (app *BaseApplication) Query(req abcitypes.RequestQuery) abcitypes.ResponseQuery {
	return abcitypes.ResponseQuery{}
}

// fnEndBlockDefault updates the app height and timestamp at the end of the current block
func (app *BaseApplication) fnEndBlockDefault(req abcitypes.RequestEndBlock) abcitypes.ResponseEndBlock {
	app.endBlock(req.Height, time.Now())
	return abcitypes.ResponseEndBlock{}
}

func (app *BaseApplication) endBlock(height int64, timestamp time.Time) abcitypes.ResponseEndBlock {
	app.height.Store(uint32(height))
	app.endBlockTimestamp.Store(timestamp.Unix())
	return abcitypes.ResponseEndBlock{}
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

// SetFnGetBlockByHash sets the getter for blocks by hash
func (app *BaseApplication) SetFnGetBlockByHash(fn func(hash []byte) *tmtypes.Block) {
	app.fnGetBlockByHash = fn
}

// SetFnGetBlockByHeight sets the getter for blocks by height
func (app *BaseApplication) SetFnGetBlockByHeight(fn func(height int64) *tmtypes.Block) {
	app.fnGetBlockByHeight = fn
}

// SetFnSendTx sets the sendTx method
func (app *BaseApplication) SetFnSendTx(fn func(tx []byte) (*ctypes.ResultBroadcastTx, error)) {
	app.fnSendTx = fn
}

// SetFnGetTx sets the getTx method
func (app *BaseApplication) SetFnGetTx(fn func(height uint32, txIndex int32) (*models.SignedTx, error)) {
	app.fnGetTx = fn
}

// SetFnGetTxHash sets the getTxHash method
func (app *BaseApplication) SetFnGetTxHash(fn func(height uint32, txIndex int32) (*models.SignedTx, []byte, error)) {
	app.fnGetTxHash = fn
}

// SetFnMempoolSize sets the mempool size method method
func (app *BaseApplication) SetFnMempoolSize(fn func() int) {
	app.fnMempoolSize = fn
}

func (app *BaseApplication) SetFnBeginBlock(fn func(req abcitypes.RequestBeginBlock) abcitypes.ResponseBeginBlock) {
	app.fnBeginBlock = fn
}

func (app *BaseApplication) SetFnEndBlock(fn func(req abcitypes.RequestEndBlock) abcitypes.ResponseEndBlock) {
	app.fnEndBlock = fn
}

// SetChainID sets the app and state chainID
func (app *BaseApplication) SetChainID(chainId string) {
	app.chainID = chainId
	app.State.SetChainID(chainId)
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

// Genesis returns the tendermint genesis information
func (app *BaseApplication) Genesis() *tmtypes.GenesisDoc {
	return app.genesisInfo
}

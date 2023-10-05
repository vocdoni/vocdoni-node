package vochain

import (
	"fmt"
	"path/filepath"
	"sync"
	"sync/atomic"
	"time"

	abcitypes "github.com/cometbft/cometbft/abci/types"
	tmnode "github.com/cometbft/cometbft/node"
	tmcli "github.com/cometbft/cometbft/rpc/client/local"
	ctypes "github.com/cometbft/cometbft/rpc/core/types"
	tmtypes "github.com/cometbft/cometbft/types"
	ethcommon "github.com/ethereum/go-ethereum/common"
	lru "github.com/hashicorp/golang-lru/v2"
	"go.vocdoni.io/dvote/crypto/zk/circuit"
	"go.vocdoni.io/dvote/test/testcommon/testutil"
	"go.vocdoni.io/dvote/vochain/ist"
	vstate "go.vocdoni.io/dvote/vochain/state"
	"go.vocdoni.io/dvote/vochain/transaction"
	"go.vocdoni.io/dvote/vochain/transaction/vochaintx"

	"go.vocdoni.io/dvote/log"
	"go.vocdoni.io/proto/build/go/models"
)

const (
	// recheckTxHeightInterval is the number of blocks after which the mempool is
	// checked for transactions to be rechecked.
	recheckTxHeightInterval = 6 * 5 // 5 minutes
	// transactionBlocksTTL is the number of blocks after which a transaction is
	// removed from the mempool.
	transactionBlocksTTL = 6 * 10 // 10 minutes
)

var (
	// ErrTransactionNotFound is returned when the transaction is not found in the blockstore.
	ErrTransactionNotFound = fmt.Errorf("transaction not found")
	// Ensure that BaseApplication implements abcitypes.Application.
	_ abcitypes.Application = (*BaseApplication)(nil)
)

// BaseApplication reflects the ABCI application implementation.
type BaseApplication struct {
	State              *vstate.State
	Istc               *ist.Controller
	Node               *tmnode.Node
	NodeClient         *tmcli.Local
	NodeAddress        ethcommon.Address
	TransactionHandler *transaction.TransactionHandler
	isSynchronizingFn  func() bool
	// tendermint WaitSync() function is racy, we need to use a mutex in order to avoid
	// data races when querying about the sync status of the blockchain.
	isSynchronizing atomic.Bool

	// Callback blockchain functions
	fnGetBlockByHeight func(height int64) *tmtypes.Block
	fnGetBlockByHash   func(hash []byte) *tmtypes.Block
	fnSendTx           func(tx []byte) (*ctypes.ResultBroadcastTx, error)
	fnGetTx            func(height uint32, txIndex int32) (*models.SignedTx, error)
	fnGetTxHash        func(height uint32, txIndex int32) (*models.SignedTx, []byte, error)
	fnMempoolSize      func() int
	fnMempoolPrune     func(txKey [32]byte) error
	blockCache         *lru.Cache[int64, *tmtypes.Block]
	// txTTLReferences is a map of tx hashes to the block height where they failed.
	txTTLReferences sync.Map
	// endBlockTimestamp is the last block end timestamp calculated from local time.
	endBlockTimestamp atomic.Int64
	// startBlockTimestamp is the current block timestamp from tendermint's
	// abcitypes.RequestBeginBlock.Header.Time
	startBlockTimestamp atomic.Int64
	chainID             string
	circuitConfigTag    string
	dataDir             string
	genesisInfo         *tmtypes.GenesisDoc

	// prepareProposalLock is used to avoid concurrent calls between Prepare/Process Proposal and FinalizeBlock
	prepareProposalLock sync.Mutex

	// testMockBlockStore is used for testing purposes only
	testMockBlockStore *testutil.MockBlockStore
}

// DeliverTxResponse is the response returned by DeliverTx after executing the transaction.
type DeliverTxResponse struct {
	Code uint32
	Log  string
	Info string
	Data []byte
}

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
	blockCache, err := lru.New[int64, *tmtypes.Block](32)
	if err != nil {
		return nil, err
	}
	return &BaseApplication{
		State:              state,
		Istc:               istc,
		TransactionHandler: transactionHandler,
		blockCache:         blockCache,
		dataDir:            dbpath,
		circuitConfigTag:   circuit.DefaultCircuitConfigurationTag,
		genesisInfo:        &tmtypes.GenesisDoc{},
	}, nil
}

// ExecuteBlock delivers a block of transactions to the Application.
// It modifies the state according to the transactions and returns the resulting Merkle root hash.
// It returns a list of ResponseDeliverTx, one for each transaction in the block.
func (app *BaseApplication) ExecuteBlock(txs [][]byte, height uint32, blockTime time.Time) ([]*DeliverTxResponse, []byte, error) {
	result := []*DeliverTxResponse{}
	app.beginBlock(blockTime, height)
	for _, tx := range txs {
		resp := app.deliverTx(tx)
		if resp.Code != 0 {
			log.Warnw("deliverTx failed",
				"code", resp.Code,
				"data", string(resp.Data),
				"info", resp.Info,
				"log", resp.Log)
		}
		result = append(result, resp)
	}
	// execute internal state transition commit
	if err := app.Istc.Commit(height); err != nil {
		return nil, nil, fmt.Errorf("cannot execute ISTC commit: %w", err)
	}
	app.endBlock(blockTime, height)
	root, err := app.State.PrepareCommit()
	if err != nil {
		return nil, nil, fmt.Errorf("cannot prepare commit: %w", err)
	}
	return result, root, nil
}

// CommitState saves the state to persistent storage and returns the hash.
func (app *BaseApplication) CommitState() ([]byte, error) {
	// Commit the state and get the hash
	if app.State.TxCounter() > 0 {
		log.Infow("commit block", "height", app.Height(), "txs", app.State.TxCounter())
	}
	hash, err := app.State.Save()
	if err != nil {
		return nil, fmt.Errorf("cannot save state: %w", err)
	}
	// perform state snapshot (DISABLED)
	if false && app.Height()%50000 == 0 && !app.IsSynchronizing() { // DISABLED
		startTime := time.Now()
		log.Infof("performing a state snapshot on block %d", app.Height())
		if _, err := app.State.Snapshot(); err != nil {
			return hash, fmt.Errorf("cannot make state snapshot: %w", err)
		}
		log.Infof("snapshot created successfully, took %s", time.Since(startTime))
		log.Debugf("%+v", app.State.ListSnapshots())
	}
	return hash, err
}

// deliverTx unmarshals req.Tx and adds it to the State if it is valid
func (app *BaseApplication) deliverTx(rawTx []byte) *DeliverTxResponse {
	// Increase Tx counter on return since the index 0 is valid
	defer app.State.TxCounterAdd()
	tx := new(vochaintx.Tx)
	if err := tx.Unmarshal(rawTx, app.ChainID()); err != nil {
		return &DeliverTxResponse{Code: 1, Data: []byte(err.Error())}
	}
	log.Debugw("deliver tx",
		"hash", fmt.Sprintf("%x", tx.TxID),
		"type", tx.TxModelType,
		"height", app.Height(),
		"tx", tx.Tx,
	)
	// check tx is correct on the current state
	response, err := app.TransactionHandler.CheckTx(tx, true)
	if err != nil {
		log.Errorw(err, "rejected tx")
		return &DeliverTxResponse{Code: 1, Data: []byte(err.Error())}
	}
	app.txTTLReferences.Delete(tx.TxID)
	// call event listeners
	for _, e := range app.State.EventListeners() {
		e.OnNewTx(tx, app.Height(), app.State.TxCounter())
	}
	return &DeliverTxResponse{
		Code: 0,
		Data: response.Data,
		Info: fmt.Sprintf("%x", response.TxHash),
		Log:  response.Log,
	}
}

// beginBlock is called at the beginning of every block.
func (app *BaseApplication) beginBlock(t time.Time, height uint32) {
	if app.isSynchronizingFn != nil {
		if app.isSynchronizingFn() {
			app.isSynchronizing.Store(true)
		} else {
			app.isSynchronizing.Store(false)
		}
	}
	app.State.Rollback()
	app.startBlockTimestamp.Store(t.Unix())
	app.State.SetHeight(height)
	go app.State.CachePurge(height)
	app.State.OnBeginBlock(vstate.BeginBlock{
		Height: int64(height),
		Time:   t,
	})
}

// endBlock is called at the end of every block.
func (app *BaseApplication) endBlock(t time.Time, h uint32) {
	app.endBlockTimestamp.Store(t.Unix())
}

// GetBlockByHeight retrieves a full block indexed by its height.
// This method uses an LRU cache for the blocks so in general it is more
// convenient for high load operations than GetBlockByHash(), which does not use cache.
func (app *BaseApplication) GetBlockByHeight(height int64) *tmtypes.Block {
	if app.fnGetBlockByHeight == nil {
		log.Errorw(fmt.Errorf("method not assigned"), "getBlockByHeight")
		return nil
	}
	if block, ok := app.blockCache.Get(height); ok {
		return block
	}
	block := app.fnGetBlockByHeight(height)
	// Don't add nil entries to the block cache.
	// If a block is fetched before it's available, we don't want to cache the failure,
	// as otherwise we might keep returning a nil block even after the blockstore has it.
	// This means that we only cache blockstore hits, but that seems okay.
	//
	// TODO: we could cache blockstore misses as long as we remove a block's cache entry
	// when a block appears in the chain.
	if block == nil {
		return nil
	}
	app.blockCache.Add(height, block)
	return block
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

// ChainID returns the Node ChainID
func (app *BaseApplication) ChainID() string {
	return app.chainID
}

// SetChainID sets the app and state chainID
func (app *BaseApplication) SetChainID(chainID string) {
	app.chainID = chainID
	app.State.SetChainID(chainID)
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
	if app.Node == nil {
		return true
	}
	return app.Node.ConsensusReactor().WaitSync()
}

// IsSynchronizing informes if the blockchain is synchronizing or not.
// The value is updated every new block.
func (app *BaseApplication) IsSynchronizing() bool {
	return app.isSynchronizing.Load()
}

// Height returns the current blockchain height, including the latest (under construction) block.
func (app *BaseApplication) Height() uint32 {
	return app.State.CurrentHeight()
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
	blk := app.GetBlockByHeight(height)
	if blk == nil {
		return nil
	}
	return &blk.Time
}

// MempoolSize returns the size of the transaction mempool
func (app *BaseApplication) MempoolSize() int {
	return app.fnMempoolSize()
}

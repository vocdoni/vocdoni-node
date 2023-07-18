package vochain

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"path/filepath"
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
	lru "github.com/hashicorp/golang-lru/v2"
	"go.vocdoni.io/dvote/crypto/zk/circuit"
	"go.vocdoni.io/dvote/test/testcommon/testutil"
	"go.vocdoni.io/dvote/vochain/genesis"
	"go.vocdoni.io/dvote/vochain/ist"
	"go.vocdoni.io/dvote/vochain/state"
	vstate "go.vocdoni.io/dvote/vochain/state"
	"go.vocdoni.io/dvote/vochain/transaction"
	"go.vocdoni.io/dvote/vochain/transaction/vochaintx"

	"go.vocdoni.io/dvote/log"
	"go.vocdoni.io/proto/build/go/models"
)

const (
	// recheckTxHeightInterval is the number of blocks after which the mempool is
	// checked for transactions to be rechecked.
	recheckTxHeightInterval = 12
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
	Service            service.Service
	Node               *tmcli.Local
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

// Info Return information about the application state.
// Used to sync Tendermint with the application during a handshake that happens on startup.
// The returned AppVersion will be included in the Header of every block.
// Tendermint expects LastBlockAppHash and LastBlockHeight to be updated during Commit,
// ensuring that Commit is never called twice for the same block height.
//
// We use this method to initialize some state variables.
func (app *BaseApplication) Info(ctx context.Context,
	req *abcitypes.RequestInfo) (*abcitypes.ResponseInfo, error) {
	lastHeight, err := app.State.LastHeight()
	if err != nil {
		return nil, fmt.Errorf("cannot get State.LastHeight: %w", err)
	}
	appHash, err := app.State.Store.Hash()
	if err != nil {
		return nil, fmt.Errorf("cannot get Store.Hash: %w", err)
	}
	if err := app.State.SetElectionPriceCalc(); err != nil {
		return nil, fmt.Errorf("cannot set election price calc: %w", err)
	}
	// print some basic version info about tendermint components
	log.Infow("cometbft info", "cometVersion", req.Version, "p2pVersion",
		req.P2PVersion, "blockVersion", req.BlockVersion, "lastHeight",
		lastHeight, "appHash", hex.EncodeToString(appHash))

	return &abcitypes.ResponseInfo{
		LastBlockHeight:  int64(lastHeight),
		LastBlockAppHash: appHash,
	}, nil
}

// InitChain called once upon genesis
// ResponseInitChain can return a list of validators. If the list is empty,
// Tendermint will use the validators loaded in the genesis file.
func (app *BaseApplication) InitChain(ctx context.Context,
	req *abcitypes.RequestInitChain) (*abcitypes.ResponseInitChain, error) {
	// setting the app initial state with validators, height = 0 and empty apphash
	// unmarshal app state from genesis
	var genesisAppState genesis.GenesisAppState
	err := json.Unmarshal(req.AppStateBytes, &genesisAppState)
	if err != nil {
		return nil, fmt.Errorf("cannot unmarshal app state bytes: %w", err)
	}
	// create accounts
	for _, acc := range genesisAppState.Accounts {
		addr := ethcommon.BytesToAddress(acc.Address)
		if err := app.State.CreateAccount(addr, "", nil, acc.Balance); err != nil {
			if err != vstate.ErrAccountAlreadyExists {
				return nil, fmt.Errorf("cannot create acount %x: %w", addr, err)
			}
			if err := app.State.InitChainMintBalance(addr, acc.Balance); err != nil {
				return nil, fmt.Errorf("cannot initialize chain minintg balance: %w", err)
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
			return nil, fmt.Errorf("cannot add validator %s: %w", log.FormatProto(v), err)
		}
		tendermintValidators = append(tendermintValidators,
			abcitypes.UpdateValidator(
				genesisAppState.Validators[i].PubKey,
				int64(genesisAppState.Validators[i].Power),
				crypto256k1.KeyType,
			))
	}

	// set treasurer address
	if genesisAppState.Treasurer != nil {
		log.Infof("adding genesis treasurer %x", genesisAppState.Treasurer)
		if err := app.State.SetTreasurer(ethcommon.BytesToAddress(genesisAppState.Treasurer), 0); err != nil {
			return nil, fmt.Errorf("could not set State.Treasurer from genesis file: %w", err)
		}
	}

	// add tx costs
	for k, v := range genesisAppState.TxCost.AsMap() {
		err = app.State.SetTxBaseCost(k, v)
		if err != nil {
			return nil, fmt.Errorf("could not set tx cost %q to value %q from genesis file to the State", k, v)
		}
	}

	// create burn account
	if err := app.State.SetAccount(vstate.BurnAddress, &vstate.Account{}); err != nil {
		return nil, fmt.Errorf("unable to set burn address")
	}

	// set max election size
	if err := app.State.SetMaxProcessSize(genesisAppState.MaxElectionSize); err != nil {
		return nil, fmt.Errorf("unable to set max election size")
	}

	// set network capacity
	if err := app.State.SetNetworkCapacity(genesisAppState.NetworkCapacity); err != nil {
		return nil, fmt.Errorf("unable to set  network capacity")
	}

	// initialize election price calc
	if err := app.State.SetElectionPriceCalc(); err != nil {
		return nil, fmt.Errorf("cannot set election price calc: %w", err)
	}

	// commit state and get hash
	hash, err := app.State.Save()
	if err != nil {
		return nil, fmt.Errorf("cannot save state: %w", err)
	}
	return &abcitypes.ResponseInitChain{
		Validators: tendermintValidators,
		AppHash:    hash,
	}, nil
}

// CheckTx unmarshals req.Tx and checks its validity
func (app *BaseApplication) CheckTx(ctx context.Context,
	req *abcitypes.RequestCheckTx) (*abcitypes.ResponseCheckTx, error) {
	height := app.Height()
	if req.Type == abcitypes.CheckTxType_Recheck {
		if height%recheckTxHeightInterval != 0 {
			return &abcitypes.ResponseCheckTx{Code: 0}, nil
		}
	}
	tx := new(vochaintx.Tx)
	if err := tx.Unmarshal(req.Tx, app.ChainID()); err != nil {
		return &abcitypes.ResponseCheckTx{Code: 1, Data: []byte("unmarshalTx " + err.Error())}, err
	}
	response, err := app.TransactionHandler.CheckTx(tx, false)
	if err != nil {
		if errors.Is(err, transaction.ErrorAlreadyExistInCache) {
			return &abcitypes.ResponseCheckTx{Code: 0}, nil
		}
		log.Errorw(err, "checkTx")
		return &abcitypes.ResponseCheckTx{Code: 1, Data: []byte("checkTx " + err.Error())}, err
	}
	return &abcitypes.ResponseCheckTx{
		Code: 0,
		Data: response.Data,
		Info: fmt.Sprintf("%x", response.TxHash),
		Log:  response.Log,
	}, nil
}

// FinalizeBlock It delivers a decided block to the Application. The Application must execute
// the transactions in the block deterministically and update its state accordingly.
// Cryptographic commitments to the block and transaction results, returned via the corresponding
// parameters in ResponseFinalizeBlock, are included in the header of the next block.
// CometBFT calls it when a new block is decided.
func (app *BaseApplication) FinalizeBlock(ctx context.Context,
	req *abcitypes.RequestFinalizeBlock) (*abcitypes.ResponseFinalizeBlock, error) {
	app.beginBlock(req.GetTime(), uint32(req.GetHeight()))
	txResults := make([]*abcitypes.ExecTxResult, len(req.Txs))
	for i, tx := range req.Txs {
		resp := app.deliverTx(tx)
		if resp.Code != 0 {
			log.Warnw("deliverTx failed",
				"code", resp.Code,
				"data", string(resp.Data),
				"info", resp.Info,
				"log", resp.Log)
		}
		txResults[i] = &abcitypes.ExecTxResult{
			Code: resp.Code,
			Data: resp.Data,
			Log:  resp.Log,
		}
	}
	// execute internal state transition commit
	if err := app.Istc.Commit(app.Height(), app.IsSynchronizing()); err != nil {
		return nil, fmt.Errorf("cannot execute ISTC commit: %w", err)
	}
	app.endBlock(req.GetTime(), uint32(req.GetHeight()))
	return &abcitypes.ResponseFinalizeBlock{
		AppHash:   app.State.WorkingHash(),
		TxResults: txResults, // TODO: check if we can remove this
	}, nil
}

// Commit saves the current vochain state and returns a commit hash
func (app *BaseApplication) Commit(ctx context.Context, req *abcitypes.RequestCommit) (*abcitypes.ResponseCommit, error) {
	// save state
	_, err := app.State.Save()
	if err != nil {
		return nil, fmt.Errorf("cannot save state: %w", err)
	}
	// perform state snapshot (DISABLED)
	if false && app.Height()%50000 == 0 && !app.IsSynchronizing() { // DISABLED
		startTime := time.Now()
		log.Infof("performing a state snapshot on block %d", app.Height())
		if _, err := app.State.Snapshot(); err != nil {
			return nil, fmt.Errorf("cannot make state snapshot: %w", err)
		}
		log.Infof("snapshot created successfully, took %s", time.Since(startTime))
		log.Debugf("%+v", app.State.ListSnapshots())
	}
	if app.State.TxCounter() > 0 {
		log.Infow("commit block", "height", app.Height(), "txs", app.State.TxCounter())
	}
	return &abcitypes.ResponseCommit{
		RetainHeight: 0, // When snapshot sync enabled, we can start to remove old blocks
	}, nil
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
	// call event listeners
	for _, e := range app.State.EventListeners() {
		e.OnNewTx(tx, app.Height()+1, app.State.TxCounter())
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
	app.height.Store(height)
	go app.State.CachePurge(height)
	app.State.OnBeginBlock(state.BeginBlock{
		Height: int64(height),
		Time:   t,
		// TODO: remove data hash from this event call
	})
}

// endBlock is called at the end of every block.
func (app *BaseApplication) endBlock(t time.Time, height uint32) {
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

// Query does nothing
func (app *BaseApplication) Query(ctx context.Context,
	req *abcitypes.RequestQuery) (*abcitypes.ResponseQuery, error) {
	return &abcitypes.ResponseQuery{}, nil
}

// PrepareProposal allows the block proposer to perform application-dependent work in a block before
// proposing it. This enables, for instance, batch optimizations to a block, which has been empirically
// demonstrated to be a key component for improved performance. Method PrepareProposal is called every
// time CometBFT is about to broadcast a Proposal message and validValue is nil. CometBFT gathers
// outstanding transactions from the mempool, generates a block header, and uses them to create a block
// to propose. Then, it calls RequestPrepareProposal with the newly created proposal, called raw proposal.
// The Application can make changes to the raw proposal, such as modifying the set of transactions or the
// order in which they appear, and returns the (potentially) modified proposal, called prepared proposal in
// the ResponsePrepareProposal call. The logic modifying the raw proposal MAY be non-deterministic.
func (app *BaseApplication) PrepareProposal(ctx context.Context,
	req *abcitypes.RequestPrepareProposal) (*abcitypes.ResponsePrepareProposal, error) {
	validTxs := [][]byte{}
	for _, tx := range req.GetTxs() {
		resp, err := app.CheckTx(ctx, &abcitypes.RequestCheckTx{
			Tx: tx, Type: abcitypes.CheckTxType_New})
		if err != nil || resp.Code != 0 {
			log.Warnw("discard invalid tx on prepare proposal",
				"err", err,
				"code", resp.Code,
				"data", string(resp.Data),
				"info", resp.Info,
				"log", resp.Log)
			continue
		}
		validTxs = append(validTxs, tx)
	}

	return &abcitypes.ResponsePrepareProposal{
		Txs: validTxs,
	}, nil
}

// ProcessProposal allows a validator to perform application-dependent work in a proposed block. This enables
// features such as immediate block execution, and allows the Application to reject invalid blocks.
// CometBFT calls it when it receives a proposal and validValue is nil. The Application cannot modify the
// proposal at this point but can reject it if invalid. If that is the case, the consensus algorithm will
// prevote nil on the proposal, which has strong liveness implications for CometBFT. As a general rule, the
// Application SHOULD accept a prepared proposal passed via ProcessProposal, even if a part of the proposal
// is invalid (e.g., an invalid transaction); the Application can ignore the invalid part of the prepared
// proposal at block execution time. The logic in ProcessProposal MUST be deterministic.
func (app *BaseApplication) ProcessProposal(ctx context.Context,
	req *abcitypes.RequestProcessProposal) (*abcitypes.ResponseProcessProposal, error) {
	return &abcitypes.ResponseProcessProposal{
		Status: abcitypes.ResponseProcessProposal_ACCEPT,
	}, nil
}

// ListSnapshots returns a list of available snapshots.
func (app *BaseApplication) ListSnapshots(context.Context,
	*abcitypes.RequestListSnapshots) (*abcitypes.ResponseListSnapshots, error) {
	return &abcitypes.ResponseListSnapshots{}, nil
}

// OfferSnapshot returns the response to a snapshot offer.
func (app *BaseApplication) OfferSnapshot(context.Context,
	*abcitypes.RequestOfferSnapshot) (*abcitypes.ResponseOfferSnapshot, error) {
	return &abcitypes.ResponseOfferSnapshot{}, nil
}

// LoadSnapshotChunk returns the response to a snapshot chunk loading request.
func (app *BaseApplication) LoadSnapshotChunk(context.Context,
	*abcitypes.RequestLoadSnapshotChunk) (*abcitypes.ResponseLoadSnapshotChunk, error) {
	return &abcitypes.ResponseLoadSnapshotChunk{}, nil
}

// ApplySnapshotChunk returns the response to a snapshot chunk applying request.
func (app *BaseApplication) ApplySnapshotChunk(context.Context,
	*abcitypes.RequestApplySnapshotChunk) (*abcitypes.ResponseApplySnapshotChunk, error) {
	return &abcitypes.ResponseApplySnapshotChunk{}, nil
}

// ExtendVote creates application specific vote extension
func (app *BaseApplication) ExtendVote(context.Context,
	*abcitypes.RequestExtendVote) (*abcitypes.ResponseExtendVote, error) {
	return &abcitypes.ResponseExtendVote{}, nil
}

// VerifyVoteExtension verifies application's vote extension data
func (app *BaseApplication) VerifyVoteExtension(context.Context,
	*abcitypes.RequestVerifyVoteExtension) (*abcitypes.ResponseVerifyVoteExtension, error) {
	return &abcitypes.ResponseVerifyVoteExtension{}, nil
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

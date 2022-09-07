package vochain

import (
	"context"
	"encoding/hex"
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
	mempl "github.com/tendermint/tendermint/mempool"
	nm "github.com/tendermint/tendermint/node"
	tmprototypes "github.com/tendermint/tendermint/proto/tendermint/types"

	// ctypes "github.com/tendermint/tendermint/rpc/coretypes" // TENDERMINT 0.35
	ctypes "github.com/tendermint/tendermint/rpc/core/types"

	tmtypes "github.com/tendermint/tendermint/types"
	snarkTypes "github.com/vocdoni/go-snark/types"
	zkartifacts "go.vocdoni.io/dvote/crypto/zk/artifacts"
	"google.golang.org/protobuf/proto"

	"go.vocdoni.io/dvote/config"
	"go.vocdoni.io/dvote/crypto/zk"
	"go.vocdoni.io/dvote/db/lru"
	"go.vocdoni.io/dvote/db/metadb"
	"go.vocdoni.io/dvote/log"
	"go.vocdoni.io/dvote/test/testcommon/testutil"
	models "go.vocdoni.io/proto/build/go/models"
)

// BaseApplication reflects the ABCI application implementation.
type BaseApplication struct {
	State *State
	// Service service.Service TENDERMINT 0.35
	// Node    *tmcli.Local TENDERMINT 0.35
	Node *nm.Node

	IsSynchronizing func() bool
	// tendermint WaitSync() function is racy, we need to use a mutex in order to avoid
	// data races when querying about the sync status of the blockchain.
	isSyncLock sync.Mutex

	fnGetBlockByHeight func(height int64) *tmtypes.Block
	fnGetBlockByHash   func(hash []byte) *tmtypes.Block
	fnSendTx           func(tx []byte) (*ctypes.ResultBroadcastTx, error)
	fnGetTx            func(height uint32, txIndex int32) (*models.SignedTx, error)
	fnGetTxHash        func(height uint32, txIndex int32) (*models.SignedTx, []byte, error)
	fnMempoolSize      func() int

	blockCache *lru.AtomicCache
	// height of the last ended block
	height uint32
	// endBlockTimestamp is the last block end timestamp calculated from local time.
	endBlockTimestamp int64
	// startBlockTimestamp is the current block timestamp from tendermint's
	// abcitypes.RequestBeginBlock.Header.Time
	startBlockTimestamp int64
	chainId             string
	dataDir             string
	// ZkVKs contains the VerificationKey for each circuit parameters index
	ZkVKs []*snarkTypes.Vk
}

// Ensure that BaseApplication implements abcitypes.Application.
var _ abcitypes.Application = (*BaseApplication)(nil)

// NewBaseApplication creates a new BaseApplication given a name and a DB backend.
// Node still needs to be initialized with SetNode
// Callback functions still need to be initialized
func NewBaseApplication(dbType, dbpath string) (*BaseApplication, error) {
	state, err := NewState(dbType, dbpath)
	if err != nil {
		return nil, fmt.Errorf("cannot create vochain state: (%s)", err)
	}

	return &BaseApplication{
		State:      state,
		blockCache: lru.NewAtomic(32),
		dataDir:    dbpath,
	}, nil
}

func TestBaseApplication(tb testing.TB) *BaseApplication {
	app, err := NewBaseApplication(metadb.ForTest(), tb.TempDir())
	if err != nil {
		tb.Fatal(err)
	}
	app.SetTestingMethods()

	// TODO: should this be a Close on the entire BaseApplication?
	tb.Cleanup(func() { app.State.Close() })
	return app
}

// LoadZkVKs loads the Zero Knowledge Verification Keys for the given
// ChainID into the BaseApplication, downloading them if necessary, and
// verifying their cryptographic hahes.
func (app *BaseApplication) LoadZkVKs(ctx context.Context) error {
	app.ZkVKs = []*snarkTypes.Vk{}
	var circuits []zkartifacts.CircuitConfig
	if genesis, ok := Genesis[app.chainId]; ok {
		circuits = genesis.CircuitsConfig
	} else {
		log.Info("using dev genesis zkSnarks circuits")
		circuits = Genesis["dev"].CircuitsConfig
	}
	for i, cc := range circuits {
		log.Infof("downloading zk-circuits-artifacts index: %d", i)

		// download VKs from CircuitsConfig
		ctx, cancel := context.WithTimeout(context.Background(), time.Minute*1)
		defer cancel()
		cc.LocalDir = filepath.Join(app.dataDir, cc.LocalDir)
		if err := zkartifacts.DownloadVKFile(ctx, cc); err != nil {
			return err
		}

		// parse VK and store it into vnode.ZkVKs
		log.Infof("parse VK from file into memory. CircuitArtifact index: %d", i)
		vk, err := zk.LoadVkFromFile(filepath.Join(cc.LocalDir, cc.CircuitPath, zkartifacts.FilenameVK))
		if err != nil {
			return err
		}
		app.ZkVKs = append(app.ZkVKs, vk)
	}
	return nil
}

/* TENDERMINT 0.35
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
	return nil
}

// SetDefaultMethods assigns fnGetBlockByHash, fnGetBlockByHeight, fnSendTx to use the
// BlockStore from app.Node to load blocks. Assumes app.Node has been set.
func (app *BaseApplication) SetDefaultMethods() {
	if app.Node == nil {
		return
	}

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

	app.SetFnSendTx(func(tx []byte) (*ctypes.ResultBroadcastTx, error) {
		r, err := return app.Node.BroadcastTxSync(context.Background(), tx)
		txHash, err := hex.DecodeString(r.Info)
		if err != nil {
			return nil, fmt.Errorf("no tx hash received")
		}
		return &ctypes.ResultBroadcastTx{
			Code: r.Code,
			Data: r.Data,
			Log:  r.Log,
			Hash: txHash,
		}, nil
	})
}

// SetTestingMethods assigns fnGetBlockByHash, fnGetBlockByHeight, fnSendTx to use mockBlockStore
func (app *BaseApplication) SetTestingMethods() {
	mockBlockStore := new(testutil.MockBlockStore)
	mockBlockStore.Init()
	app.SetFnGetBlockByHash(mockBlockStore.GetByHash)
	app.SetFnGetBlockByHeight(mockBlockStore.Get)
	app.SetFnGetTx(app.getTxTendermint)
	app.SetFnGetTxHash(app.getTxHashTendermint)
	app.SetFnSendTx(func(tx []byte) (*ctypes.ResultBroadcastTx, error) {
		mockBlockStore.Add(&tmtypes.Block{Data: tmtypes.Data{Txs: []tmtypes.Tx{tx}}})
		return nil, nil
	})
	app.SetFnMempoolSize(func() int { return 0 })
	app.IsSynchronizing = func() bool { return false }

	// For tests, we begin at block 2.
	// The last block we finished was 1, and we did so 10s ago.
	app.height = 1
	now := time.Now()
	app.endBlockTimestamp = now.Add(-10 * time.Second).Unix()
	app.BeginBlock(abcitypes.RequestBeginBlock{Header: tmprototypes.Header{
		Time:   now,
		Height: 2,
	}})
}

// IsSynchronizing informes if the blockchain is synchronizing or not.
func (app *BaseApplication) isSynchronizingTendermint() bool {
	app.isSyncLock.Lock()
	defer app.isSyncLock.Unlock()
	status, err := app.Node.Status(context.Background())
	if err != nil {
		log.Warnf("error retriving node status information: %v", err)
		return true
	}
	return status.SyncInfo.CatchingUp
}
*/

// isSynchronizingTendermint informes if the blockchain is synchronizing or not.
func (app *BaseApplication) isSynchronizingTendermint() bool {
	app.isSyncLock.Lock()
	defer app.isSyncLock.Unlock()
	return app.Node.ConsensusReactor().WaitSync()
}

func (app *BaseApplication) SetNode(vochaincfg *config.VochainCfg, genesis []byte) error {
	var err error
	if app.Node, err = newTendermint(app, vochaincfg, genesis); err != nil {
		return fmt.Errorf("could not set application Node: %s", err)
	}
	app.chainId = app.Node.GenesisDoc().ChainID
	return nil
}

// SetDefaultMethods assigns fnGetBlockByHash, fnGetBlockByHeight, fnSendTx to use the
// BlockStore from app.Node to load blocks. Assumes app.Node has been set.
func (app *BaseApplication) SetDefaultMethods() {
	if app.Node == nil {
		log.Error("Cannot assign block getters: App Node is not initialized")
		return
	}
	app.SetFnGetBlockByHash(app.Node.BlockStore().LoadBlockByHash)
	app.SetFnGetBlockByHeight(app.Node.BlockStore().LoadBlock)
	app.IsSynchronizing = app.isSynchronizingTendermint
	app.SetFnGetTx(app.getTxTendermint)
	app.SetFnGetTxHash(app.getTxHashTendermint)
	app.SetFnMempoolSize(app.Node.Mempool().Size)
	app.SetFnSendTx(func(tx []byte) (*ctypes.ResultBroadcastTx, error) {
		resCh := make(chan *abcitypes.Response, 1)
		defer close(resCh)
		err := app.Node.Mempool().CheckTx(tx, func(res *abcitypes.Response) {
			resCh <- res
		}, mempl.TxInfo{})
		if err != nil {
			return nil, err
		}
		res := <-resCh
		r := res.GetCheckTx()
		txHash, err := hex.DecodeString(r.Info)
		if err != nil {
			return nil, fmt.Errorf("no tx hash received")
		}
		return &ctypes.ResultBroadcastTx{
			Code: r.Code,
			Data: r.Data,
			Log:  r.Log,
			Hash: txHash,
		}, nil
	})
}

// SetTestingMethods assigns fnGetBlockByHash, fnGetBlockByHeight, fnSendTx to use mockBlockStore
func (app *BaseApplication) SetTestingMethods() {
	mockBlockStore := new(testutil.MockBlockStore)
	mockBlockStore.Init()
	app.SetFnGetBlockByHash(mockBlockStore.GetByHash)
	app.SetFnGetBlockByHeight(mockBlockStore.Get)
	app.SetFnGetTx(app.getTxTendermint)
	app.SetFnGetTxHash(app.getTxHashTendermint)
	app.SetFnSendTx(func(tx []byte) (*ctypes.ResultBroadcastTx, error) {
		mockBlockStore.Add(&tmtypes.Block{Data: tmtypes.Data{Txs: []tmtypes.Tx{tx}}})
		return nil, nil
	})
	app.SetFnMempoolSize(func() int { return 0 })
	app.IsSynchronizing = func() bool { return false }

	// For tests, we begin at block 2.
	// The last block we finished was 1, and we did so 10s ago.
	app.height = 1
	now := time.Now()
	app.endBlockTimestamp = now.Add(-10 * time.Second).Unix()
	app.BeginBlock(abcitypes.RequestBeginBlock{Header: tmprototypes.Header{
		Time:   now,
		Height: 2,
	}})
}

// Height returns the current blockchain height
func (app *BaseApplication) Height() uint32 {
	return atomic.LoadUint32(&app.height)
}

// Timestamp returns the last block end timestamp
func (app *BaseApplication) Timestamp() int64 {
	return atomic.LoadInt64(&app.endBlockTimestamp)
}

// TimestampStartBlock returns the current block start timestamp
func (app *BaseApplication) TimestampStartBlock() int64 {
	return atomic.LoadInt64(&app.startBlockTimestamp)
}

// TimestampFromBlock returns the timestamp for a specific block height
func (app *BaseApplication) TimestampFromBlock(height int64) *time.Time {
	blk := app.fnGetBlockByHeight(height)
	if blk == nil {
		return nil
	}
	return &blk.Time
}

// ChainID returns the Node ChainID
func (app *BaseApplication) ChainID() string {
	return app.chainId
}

// MempoolSize returns the size of the transaction mempool
func (app *BaseApplication) MempoolSize() int {
	return app.fnMempoolSize()
}

// ONLY TENDERMINT 0.34
// MempoolRemoveTx removes a transaction (identifier by its vochain.TxKey() hash)
// from the Tendermint mempool.
func (app *BaseApplication) MempoolRemoveTx(txKey [32]byte) {
	app.Node.Mempool().(*mempl.CListMempool).RemoveTxByKey(txKey, false)
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
		return nil, fmt.Errorf("unable to get block by height: %d", height)
	}
	if int32(len(block.Txs)) <= txIndex {
		return nil, fmt.Errorf("txIndex overflow on GetTx (height: %d, txIndex:%d)", height, txIndex)
	}
	tx := &models.SignedTx{}
	return tx, proto.Unmarshal(block.Txs[txIndex], tx)
}

// GetTxHash retrieves a vochain transaction, with its hash, from the blockstore
func (app *BaseApplication) GetTxHash(height uint32,
	txIndex int32) (*models.SignedTx, []byte, error) {
	return app.fnGetTxHash(height, txIndex)
}

func (app *BaseApplication) getTxHashTendermint(height uint32,
	txIndex int32) (*models.SignedTx, []byte, error) {
	block := app.GetBlockByHeight(int64(height))
	if block == nil {
		return nil, nil, fmt.Errorf("unable to get block by height: %d", height)
	}
	if int32(len(block.Txs)) <= txIndex {
		return nil, nil, fmt.Errorf("txIndex overflow on GetTx (height: %d, txIndex:%d)", height, txIndex)
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
		app.State.CreateAccount(addr, "", nil, 100)
	}
	// create accounts
	for _, acc := range genesisAppState.Accounts {
		addr := ethcommon.BytesToAddress(acc.Address)
		if err := app.State.CreateAccount(addr, "", nil, acc.Balance); err != nil {
			if err != ErrAccountAlreadyExists {
				log.Fatalf("cannot create acount %x %v", addr, err)
			}
			if err := app.State.MintBalance(addr, acc.Balance); err != nil {
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
	if err := app.State.SetAccount(BurnAddress, &Account{}); err != nil {
		log.Fatal("unable to set burn address")
	}

	// Is this save needed?
	if _, err := app.State.Save(); err != nil {
		log.Fatalf("cannot save state: %s", err)
	}
	// TBD: using empty list here, should return validatorsUpdate to use the validators obtained here
	return abcitypes.ResponseInitChain{}
}

// BeginBlock signals the beginning of a new block. Called prior to any DeliverTxs.
// The header contains the height, timestamp, and more - it exactly matches the
// Tendermint block header.
// The LastCommitInfo and ByzantineValidators can be used to determine rewards and
// punishments for the validators.
func (app *BaseApplication) BeginBlock(
	req abcitypes.RequestBeginBlock) abcitypes.ResponseBeginBlock {
	app.State.Rollback()
	atomic.StoreInt64(&app.startBlockTimestamp, req.Header.GetTime().Unix())
	height := uint32(req.Header.GetHeight())
	app.State.SetHeight(height)
	go app.State.CachePurge(height)

	return abcitypes.ResponseBeginBlock{}
}

func (app *BaseApplication) AdvanceTestBlock() {
	// Each block spans tens seconds.
	endingHeight := int64(app.Height()) + 1
	endingTime := time.Unix(app.TimestampStartBlock(), 0).Add(10 * time.Second)
	app.endBlock(endingHeight, endingTime)

	app.Commit()

	// The next block begins a second later.
	nextHeight := endingHeight + 1
	nextStartTime := endingTime.Add(2 * time.Second)
	app.BeginBlock(abcitypes.RequestBeginBlock{Header: tmprototypes.Header{
		Time:   nextStartTime,
		Height: nextHeight,
	}})
}

// ONLY FOR TENDERMINT 0.34
// SetOption does nothing
func (*BaseApplication) SetOption(req abcitypes.RequestSetOption) abcitypes.ResponseSetOption {
	return abcitypes.ResponseSetOption{}
}

// CheckTx unmarshals req.Tx and checks its validity
func (app *BaseApplication) CheckTx(req abcitypes.RequestCheckTx) abcitypes.ResponseCheckTx {
	var response *AddTxResponse
	var err error
	if req.Type == abcitypes.CheckTxType_Recheck {
		return abcitypes.ResponseCheckTx{Code: 0}
	}
	tx := new(VochainTx)
	if err = tx.Unmarshal(req.Tx, app.ChainID()); err == nil {
		if response, err = app.AddTx(tx, false); err != nil {
			if errors.Is(err, ErrorAlreadyExistInCache) {
				return abcitypes.ResponseCheckTx{Code: 0}
			}
			log.Debugf("checkTx error: %v", err)
			return abcitypes.ResponseCheckTx{Code: 1, Data: []byte("addTx " + err.Error())}
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
	var response *AddTxResponse
	var err error
	// Increase Tx counter on return since the index 0 is valid
	defer app.State.TxCounterAdd()
	tx := new(VochainTx)
	if err = tx.Unmarshal(req.Tx, app.ChainID()); err == nil {
		log.Debugf("deliver tx: %s", log.FormatProto(tx.Tx))
		if response, err = app.AddTx(tx, true); err != nil {
			log.Debugf("rejected tx: %v", err)
			return abcitypes.ResponseDeliverTx{Code: 1, Data: []byte(err.Error())}
		}
		for _, e := range app.State.eventListeners {
			e.OnNewTx(tmtypes.Tx(req.Tx).Hash(), app.Height()+1, app.State.TxCounter())
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
	if app.Height()%50000 == 0 && !app.IsSynchronizing() {
		startTime := time.Now()
		log.Infof("performing a state snapshot on block %d", app.Height())
		if err := app.State.Snapshot(); err != nil {
			log.Fatalf("cannot make state snapshot: %v", err)
		}
		log.Infof("snapshot created successfully, took %s", time.Since(startTime))
	}
	return abcitypes.ResponseCommit{
		Data: data,
	}
}

// Query does nothing
func (app *BaseApplication) Query(req abcitypes.RequestQuery) abcitypes.ResponseQuery {
	return abcitypes.ResponseQuery{}
}

// EndBlock updates the app height and timestamp at the end of the current block
func (app *BaseApplication) EndBlock(req abcitypes.RequestEndBlock) abcitypes.ResponseEndBlock {
	app.endBlock(req.Height, time.Now())
	return abcitypes.ResponseEndBlock{}
}

func (app *BaseApplication) endBlock(height int64, timestamp time.Time) abcitypes.ResponseEndBlock {
	atomic.StoreUint32(&app.height, uint32(height))
	atomic.StoreInt64(&app.endBlockTimestamp, timestamp.Unix())
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

// SetChainID sets the app chainID
func (app *BaseApplication) SetChainID(chainId string) {
	app.chainId = chainId
}

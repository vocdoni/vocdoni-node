package vochain

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"strconv"
	"sync"
	"sync/atomic"
	"time"

	ethcommon "github.com/ethereum/go-ethereum/common"
	abcitypes "github.com/tendermint/tendermint/abci/types"
	mempl "github.com/tendermint/tendermint/mempool"
	nm "github.com/tendermint/tendermint/node"
	ctypes "github.com/tendermint/tendermint/rpc/core/types"
	tmtypes "github.com/tendermint/tendermint/types"
	"google.golang.org/protobuf/proto"

	"go.vocdoni.io/dvote/config"
	"go.vocdoni.io/dvote/db/lru"
	"go.vocdoni.io/dvote/log"
	"go.vocdoni.io/dvote/test/testcommon/testutil"
	models "go.vocdoni.io/proto/build/go/models"
)

// BaseApplication reflects the ABCI application implementation.
type BaseApplication struct {
	State *State
	Node  *nm.Node

	// tendermint WaitSync() function is racy, we need to use a mutex in order to avoid
	// data races when querying about the sync status of the blockchain.
	isSyncLock sync.Mutex

	fnGetBlockByHeight func(height int64) *tmtypes.Block
	fnGetBlockByHash   func(hash []byte) *tmtypes.Block
	fnSendTx           func(tx []byte) (*ctypes.ResultBroadcastTx, error)
	blockCache         *lru.AtomicCache
	height             uint32
	timestamp          int64
	chainId            string
}

var _ abcitypes.Application = (*BaseApplication)(nil)

// NewBaseApplication creates a new BaseApplication given a name an a DB backend.
// Node still needs to be initialized with SetNode
// Callback functions still need to be initialized
func NewBaseApplication(dbpath string) (*BaseApplication, error) {
	state, err := NewState(dbpath)
	if err != nil {
		return nil, fmt.Errorf("cannot create vochain state: (%s)", err)
	}
	return &BaseApplication{
		State:      state,
		blockCache: lru.NewAtomic(128),
	}, nil
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
		return &ctypes.ResultBroadcastTx{
			Code: r.Code,
			Data: r.Data,
			Log:  r.Log,
			Hash: tmtypes.Tx(tx).Hash(),
		}, nil
	})
}

// SetTetingMethods assigns fnGetBlockByHash, fnGetBlockByHeight, fnSendTx to use mockBlockStore
func (app *BaseApplication) SetTestingMethods() {
	mockBlockStore := new(testutil.MockBlockStore)
	mockBlockStore.Init()
	app.SetFnGetBlockByHash(mockBlockStore.GetByHash)
	app.SetFnGetBlockByHeight(mockBlockStore.Get)
	app.SetFnSendTx(func(tx []byte) (*ctypes.ResultBroadcastTx, error) {
		mockBlockStore.Add(&tmtypes.Block{Data: tmtypes.Data{Txs: []tmtypes.Tx{tx}}})
		return nil, nil
	})
}

// IsSynchronizing informes if the blockchain is synchronizing or not.
func (app *BaseApplication) IsSynchronizing() bool {
	app.isSyncLock.Lock()
	defer app.isSyncLock.Unlock()
	return app.Node.ConsensusReactor().WaitSync()
}

// Height returns the current blockchain height
func (app *BaseApplication) Height() uint32 {
	return atomic.LoadUint32(&app.height)
}

// Timestamp returns the last block timestamp
func (app *BaseApplication) Timestamp() int64 {
	return atomic.LoadInt64(&app.timestamp)
}

// ChainID returns the Node ChainID
func (app *BaseApplication) ChainID() string {
	return app.chainId
}

// MempoolRemoveTx removes a transaction (identifier by its vochain.TxKey() hash)
// from the Tendermint mempool.
func (app *BaseApplication) MempoolRemoveTx(txKey [32]byte) {
	app.Node.Mempool().(*mempl.CListMempool).RemoveTxByKey(txKey, false)
}

// GetBlockByHeight retreies a full Tendermint block indexed by its height.
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

// GetTx retreives a vochain transaction from the blockstore
func (app *BaseApplication) GetTx(height uint32, txIndex int32) (*models.SignedTx, error) {
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

// GetTxHash retreives a vochain transaction, with its hash, from the blockstore
func (app *BaseApplication) GetTxHash(height uint32,
	txIndex int32) (*models.SignedTx, []byte, error) {
	block := app.GetBlockByHeight(int64(height))
	if block == nil {
		return nil, nil, fmt.Errorf("unable to get block by height: %d", height)
	}
	if int32(len(block.Txs)) <= txIndex {
		return nil, nil, fmt.Errorf("txIndex overflow on GetTx (height: %d, txIndex:%d)", height, txIndex)
	}
	tx := &models.SignedTx{}
	return tx, tmtypes.Tx(block.Txs[txIndex]).Hash(), proto.Unmarshal(block.Txs[txIndex], tx)
}

// SendTX sends a transaction to the mempool (sync)
func (app *BaseApplication) SendTx(tx []byte) (*ctypes.ResultBroadcastTx, error) {
	if app.fnGetBlockByHeight == nil {
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
	header := app.State.Header(false)
	log.Infof("replaying blocks. Current height %d, current APP hash %x",
		header.Height, header.AppHash)
	return abcitypes.ResponseInfo{
		LastBlockHeight:  header.Height,
		LastBlockAppHash: header.AppHash,
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
		log.Infof("adding genesis oracle %s", v)
		if err := app.State.AddOracle(ethcommon.HexToAddress(v)); err != nil {
			log.Fatalf("cannot add oracles: %v", err)
		}
	}
	// get validators
	for i := 0; i < len(genesisAppState.Validators); i++ {
		log.Infof("adding genesis validator %x", genesisAppState.Validators[i].Address)
		pwr, err := strconv.Atoi(genesisAppState.Validators[i].Power)
		if err != nil {
			log.Fatal("cannot decode validator power: %s", err)
		}
		v := &models.Validator{
			Address: genesisAppState.Validators[i].Address,
			PubKey:  genesisAppState.Validators[i].PubKey.Value,
			Power:   uint64(pwr),
		}
		if err = app.State.AddValidator(v); err != nil {
			log.Fatal(err)
		}
	}

	var header models.TendermintHeader
	header.Height = 0
	header.AppHash = []byte{}
	header.ChainId = req.ChainId
	headerBytes, err := proto.Marshal(&header)
	if err != nil {
		log.Fatalf("cannot marshal header: %s", err)
	}
	app.State.Lock()
	if err := app.State.Tx.Set(headerKey, headerBytes); err != nil {
		log.Fatal(err)
	}
	app.State.Unlock()
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
	header := &models.TendermintHeader{
		Height:         req.Header.GetHeight(),
		ConsensusHash:  req.Header.GetConsensusHash(),
		AppHash:        req.Header.GetAppHash(),
		BlockID:        req.Header.LastBlockId.GetHash(),
		DataHash:       req.Header.GetDataHash(),
		LastCommitHash: req.Header.GetLastCommitHash(),
		Timestamp:      req.Header.GetTime().Unix(),
	}
	headerBytes, err := proto.Marshal(header)
	if err != nil {
		log.Warnf("cannot marshal header in BeginBlock")
	}
	// reset app state to latest persistent data
	app.State.Rollback()
	app.State.Lock()
	if err = app.State.Tx.Set(headerKey, headerBytes); err != nil {
		log.Fatal(err)
	}
	app.State.Unlock()
	go app.State.CachePurge(uint32(header.Height))

	return abcitypes.ResponseBeginBlock{}
}

// SetOption does nothing
func (*BaseApplication) SetOption(req abcitypes.RequestSetOption) abcitypes.ResponseSetOption {
	return abcitypes.ResponseSetOption{}
}

// CheckTx unmarshals req.Tx and checks its validity
func (app *BaseApplication) CheckTx(req abcitypes.RequestCheckTx) abcitypes.ResponseCheckTx {
	var data []byte
	var err error
	var tx *models.Tx
	var signature []byte
	var txBytes []byte
	if req.Type == abcitypes.CheckTxType_Recheck {
		return abcitypes.ResponseCheckTx{Code: 0, Data: data}
	}
	if tx, txBytes, signature, err = UnmarshalTx(req.Tx); err == nil {
		if data, err = AddTx(tx, txBytes, signature, app.State, TxKey(req.Tx), false); err != nil {
			log.Debugf("checkTx error: %s", err)
			return abcitypes.ResponseCheckTx{Code: 1, Data: []byte("addTx " + err.Error())}
		}
	} else {
		return abcitypes.ResponseCheckTx{Code: 1, Data: []byte("unmarshalTx " + err.Error())}
	}
	return abcitypes.ResponseCheckTx{Code: 0, Data: data}
}

// DeliverTx unmarshals req.Tx and adds it to the State if it is valid
func (app *BaseApplication) DeliverTx(req abcitypes.RequestDeliverTx) abcitypes.ResponseDeliverTx {
	var data []byte
	var err error
	var tx *models.Tx
	var signature []byte
	var txBytes []byte
	// Increase Tx counter on return since the index 0 is valid
	defer app.State.TxCounterAdd()
	if tx, txBytes, signature, err = UnmarshalTx(req.Tx); err == nil {
		log.Debugf("deliver tx: %s", log.FormatProto(tx))
		if data, err = AddTx(tx, txBytes, signature, app.State, TxKey(req.Tx), true); err != nil {
			log.Debugf("rejected tx: %v", err)
			return abcitypes.ResponseDeliverTx{Code: 1, Data: []byte(err.Error())}
		}
		for _, e := range app.State.eventListeners {
			e.OnNewTx(app.Height()+1, app.State.TxCounter())
		}
	} else {
		return abcitypes.ResponseDeliverTx{Code: 1, Data: []byte(err.Error())}
	}
	return abcitypes.ResponseDeliverTx{Code: 0, Data: data}
}

// Commit saves the current vochain state and returns a commit hash
func (app *BaseApplication) Commit() abcitypes.ResponseCommit {
	data, err := app.State.Save()
	if err != nil {
		log.Fatalf("cannot save state: %s", err)
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
	atomic.StoreUint32(&app.height, uint32(req.Height))
	atomic.StoreInt64(&app.timestamp, time.Now().Unix())
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

// TxKey computes the checksum of the tx
func TxKey(tx tmtypes.Tx) [32]byte {
	return sha256.Sum256(tx)
}

// SetFnGetBlockByHash sets the getter for blocks by hash
func (app *BaseApplication) SetFnGetBlockByHash(fn func(hash []byte) *tmtypes.Block) {
	app.fnGetBlockByHash = fn
}

// SetFnGetBlockByHash sets the getter for blocks by height
func (app *BaseApplication) SetFnGetBlockByHeight(fn func(height int64) *tmtypes.Block) {
	app.fnGetBlockByHeight = fn
}

// SetFnGetBlockByHash sets the sendTx method
func (app *BaseApplication) SetFnSendTx(fn func(tx []byte) (*ctypes.ResultBroadcastTx, error)) {
	app.fnSendTx = fn
}

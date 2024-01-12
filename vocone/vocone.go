package vocone

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	abcitypes "github.com/cometbft/cometbft/abci/types"
	"github.com/cometbft/cometbft/crypto/tmhash"
	tmcoretypes "github.com/cometbft/cometbft/rpc/core/types"
	tmtypes "github.com/cometbft/cometbft/types"
	"github.com/ethereum/go-ethereum/common"
	"github.com/google/uuid"
	"go.vocdoni.io/dvote/api"
	"go.vocdoni.io/dvote/config"
	"go.vocdoni.io/dvote/crypto/ethereum"
	"go.vocdoni.io/dvote/db"
	"go.vocdoni.io/dvote/db/metadb"
	"go.vocdoni.io/dvote/httprouter"
	"go.vocdoni.io/dvote/log"
	"go.vocdoni.io/dvote/service"
	"go.vocdoni.io/dvote/vochain"
	"go.vocdoni.io/dvote/vochain/genesis"
	"go.vocdoni.io/dvote/vochain/indexer"
	"go.vocdoni.io/dvote/vochain/keykeeper"
	"go.vocdoni.io/dvote/vochain/state"
	"go.vocdoni.io/dvote/vochain/vochaininfo"
	"go.vocdoni.io/proto/build/go/models"
	"google.golang.org/protobuf/proto"
)

const (
	DefaultTxsPerBlock     = 500
	DefaultBlockTimeTarget = time.Second * 5
	DefaultTxCosts         = 10
	mempoolSize            = 100 << 10
)

// Vocone is an implementation of the Vocdoni protocol run by a single (atomic) node.
type Vocone struct {
	service.VocdoniService

	mempool         chan []byte // a buffered channel acts like a FIFO with a fixed size
	blockStore      db.Database
	height          atomic.Int64
	blockTimestamps sync.Map
	lastBlockTime   time.Time
	txsPerBlock     int
	// vcMtx is a lock on modification to the app state.
	// this enables direct calls to vochain functions from the vocone
	//  without causing race conditions
	vcMtx sync.Mutex
}

// NewVocone returns a ready Vocone instance.
func NewVocone(dataDir string, keymanager *ethereum.SignKeys, disableIPFS bool, connectKey string, connectPeers []string) (*Vocone, error) {
	var err error

	vc := &Vocone{}
	vc.Config = &config.VochainCfg{}
	vc.Config.DataDir = dataDir
	vc.Config.DBType = db.TypePebble
	vc.App, err = vochain.NewBaseApplication(vc.Config)
	if err != nil {
		return nil, err
	}
	vc.mempool = make(chan []byte, mempoolSize)
	vc.txsPerBlock = DefaultTxsPerBlock
	version, err := vc.App.State.LastHeight()
	if err != nil {
		return nil, err
	}
	vc.height.Store(int64(version))
	if vc.blockStore, err = metadb.New(db.TypePebble,
		filepath.Join(dataDir, "blockstore")); err != nil {
		return nil, err
	}

	vc.setDefaultMethods()
	vc.App.State.SetHeight(uint32(vc.height.Load()))
	vc.App.SetBlockTimeTarget(DefaultBlockTimeTarget)

	// Create indexer
	if vc.Indexer, err = indexer.New(vc.App, indexer.Options{
		DataDir: filepath.Join(dataDir, "indexer"),
	}); err != nil {
		return nil, err
	}

	// Create key keeper
	if err := vc.SetKeyKeeper(keymanager); err != nil {
		return nil, fmt.Errorf("could not create keykeeper: %w", err)
	}

	// Create vochain metrics collector
	vc.Stats = vochaininfo.NewVochainInfo(vc.App)
	go vc.Stats.Start(10)

	// Create the IPFS storage layer
	if !disableIPFS {
		vc.Storage, err = vc.IPFS(&config.IPFSCfg{
			ConfigPath:   filepath.Join(dataDir, "ipfs"),
			ConnectKey:   connectKey,
			ConnectPeers: connectPeers,
		})
		if err != nil {
			return nil, err
		}

		// Create the data downloader and offchain data handler
		if err := vc.OffChainDataHandler(); err != nil {
			return nil, err
		}
	}

	return vc, err
}

// EnableAPI starts the HTTP API server. It is not enabled by default.
func (vc *Vocone) EnableAPI(host string, port int, URLpath string) (*api.API, error) {
	vc.Router = new(httprouter.HTTProuter)
	if err := vc.Router.Init(host, port); err != nil {
		return nil, err
	}
	uAPI, err := api.NewAPI(vc.Router, URLpath, vc.Config.DataDir, db.TypePebble)
	if err != nil {
		return nil, err
	}
	uAPI.Attach(
		vc.App,
		vc.Stats,
		vc.Indexer,
		vc.Storage,
		vc.CensusDB,
	)
	adminToken := uuid.New()
	log.Warnw("new admin token generated", "token", adminToken.String())
	uAPI.Endpoint.SetAdminToken(adminToken.String())

	return uAPI, uAPI.EnableHandlers(
		api.ElectionHandler,
		api.VoteHandler,
		api.ChainHandler,
		api.WalletHandler,
		api.AccountHandler,
		api.CensusHandler,
		api.SIKHandler,
	)
}

// Start starts the Vocone node. This function is blocking.
func (vc *Vocone) Start() {
	vc.lastBlockTime = time.Now()
	go vochainPrintInfo(10*time.Second, vc.Stats)
	if vc.App.Height() == 0 {
		log.Infof("initializing new blockchain")
		genesisAppData, err := json.Marshal(&genesis.AppState{
			MaxElectionSize: 1000000,
			NetworkCapacity: uint64(vc.txsPerBlock),
			TxCost:          defaultTxCosts(),
		})
		if err != nil {
			panic(err)
		}
		if _, err = vc.App.InitChain(context.Background(), &abcitypes.RequestInitChain{
			ChainId:       vc.App.ChainID(),
			AppStateBytes: genesisAppData,
			Time:          time.Now(),
		}); err != nil {
			panic(err)
		}
		time.Sleep(1 * time.Second)
	}
	for {
		// Begin block
		vc.vcMtx.Lock()
		startTime := time.Now()
		height := vc.height.Load()

		// Create and execute block
		resp, err := vc.App.ExecuteBlock(vc.prepareBlock(), uint32(height), startTime)
		if err != nil {
			log.Error(err, "execute block error")
			continue
		}
		if _, err := vc.App.CommitState(); err != nil {
			log.Fatalf("could not commit state: %v", err)
		}
		log.Debugw("block committed",
			"timestamp", startTime.Unix(),
			"height", height,
			"hash", hex.EncodeToString(resp.Root),
			"took", time.Since(startTime),
		)
		vc.vcMtx.Unlock()

		// Waiting time
		sinceLast := time.Since(vc.lastBlockTime)
		if sinceLast < vc.App.BlockTimeTarget() {
			time.Sleep(vc.App.BlockTimeTarget() - sinceLast)
		}
		vc.lastBlockTime = time.Now()
		vc.height.Add(1)
	}
}

// SetBlockSize configures the maximum number of transactions per block.
func (vc *Vocone) SetBlockSize(txsCount int) {
	vc.txsPerBlock = txsCount
}

// CreateAccount creates a new account in the state.
func (vc *Vocone) CreateAccount(key common.Address, acc *state.Account) error {
	vc.vcMtx.Lock()
	defer vc.vcMtx.Unlock()
	if err := vc.App.State.SetAccount(key, acc); err != nil {
		return err
	}
	if _, err := vc.Commit(); err != nil {
		return err
	}
	return nil
}

// Commit saves the current state and returns the hash.
func (vc *Vocone) Commit() ([]byte, error) {
	hash, err := vc.App.State.PrepareCommit()
	if err != nil {
		return nil, err
	}
	if _, err := vc.App.State.Save(); err != nil {
		return nil, err
	}
	return hash, nil
}

// SetKeyKeeper adds a keykeeper to the application.
func (vc *Vocone) SetKeyKeeper(key *ethereum.SignKeys) error {
	// Create key keeper
	// we need to add a validator, so the keykeeper functions are allowed
	vc.vcMtx.Lock()
	defer vc.vcMtx.Unlock()
	// remove existing validators before we add the new one (to avoid keyindex collision)
	validators, err := vc.App.State.Validators(true)
	if err != nil {
		return err
	}
	for _, v := range validators {
		if err := vc.App.State.RemoveValidator(v); err != nil {
			log.Warnf("could not remove validator %x", v.Address)
		}
	}
	// add the new validator
	if err := vc.App.State.AddValidator(&models.Validator{
		Address:  key.Address().Bytes(),
		Power:    100,
		Name:     "vocone-solo-validator",
		KeyIndex: 1,
	}); err != nil {
		return err
	}
	log.Infow("adding validator", "address", key.Address().Hex(), "keyIndex", 1)
	if _, err := vc.Commit(); err != nil {
		return err
	}
	vc.KeyKeeper, err = keykeeper.NewKeyKeeper(
		filepath.Join(vc.Config.DataDir, "keykeeper"),
		vc.App,
		key,
		1)
	return err
}

// SetTxCost configures the transaction cost for the given tx type.
func (vc *Vocone) SetTxCost(txType models.TxType, cost uint64) error {
	vc.vcMtx.Lock()
	defer vc.vcMtx.Unlock()
	if err := vc.App.State.SetTxBaseCost(txType, cost); err != nil {
		return err
	}
	if _, err := vc.Commit(); err != nil {
		return err
	}
	return nil
}

// SetBulkTxCosts sets the transaction cost for all existing transaction types.
// It is useful to bootstrap the blockchain by set the transaction cost for all
// transaction types at once. If force is enabld the cost is set for all tx types.
// If force is disabled, the cost is set only for tx types that have not been set.
func (vc *Vocone) SetBulkTxCosts(txCost uint64, force bool) error {
	vc.vcMtx.Lock()
	defer vc.vcMtx.Unlock()
	for k := range state.TxTypeCostToStateKey {
		if !force {
			_, err := vc.App.State.TxBaseCost(k, true)
			if err == nil || errors.Is(err, state.ErrTxCostNotFound) {
				continue
			}
			// If error is not ErrTxCostNotFound, return it
			return err
		}
		log.Infow("setting tx base cost", "txtype", models.TxType_name[int32(k)], "cost", txCost)
		if err := vc.App.State.SetTxBaseCost(k, txCost); err != nil {
			return err
		}
	}
	if _, err := vc.Commit(); err != nil {
		return err
	}
	return nil
}

// SetElectionPrice sets the election price.
func (vc *Vocone) SetElectionPrice() error {
	vc.vcMtx.Lock()
	defer vc.vcMtx.Unlock()
	if err := vc.App.State.SetElectionPriceCalc(); err != nil {
		return err
	}
	return nil
}

func (vc *Vocone) setDefaultMethods() {
	// first set the default methods, then override some of them
	vc.App.SetDefaultMethods()
	vc.App.SetFnIsSynchronizing(func() bool { return false })
	vc.App.SetFnSendTx(vc.addTx)
	vc.App.SetFnGetTx(vc.getTx)
	vc.App.SetFnGetBlockByHeight(vc.getBlock)
	vc.App.SetFnGetTxHash(vc.getTxWithHash)
	vc.App.SetFnMempoolSize(vc.mempoolSize)
	vc.App.SetFnMempoolPrune(nil)
}

func (vc *Vocone) addTx(tx []byte) (*tmcoretypes.ResultBroadcastTx, error) {
	resp, err := vc.App.CheckTx(context.Background(), &abcitypes.RequestCheckTx{Tx: tx})
	if err != nil {
		return nil, err
	}
	if resp.Code == 0 {
		select {
		case vc.mempool <- tx:
		default: // mempool is full
			return &tmcoretypes.ResultBroadcastTx{
				Code: 1,
				Data: []byte("mempool is full"),
			}, fmt.Errorf("mempool is full")
		}
	} else {
		log.Debugf("checkTx failed: %s", resp.Data)
	}
	return &tmcoretypes.ResultBroadcastTx{
		Code: resp.Code,
		Data: resp.Data,
		Hash: tmhash.Sum(tx),
	}, nil
}

// prepareBlock prepares a block with transactions from the mempool and returns the list of transactions.
func (vc *Vocone) prepareBlock() [][]byte {
	defer vc.blockTimestamps.Store(vc.height.Load(), time.Now())
	blockStoreTx := vc.blockStore.WriteTx()
	defer blockStoreTx.Discard()
	var transactions [][]byte
txLoop:
	for txCount := 0; txCount < vc.txsPerBlock; {
		var tx []byte
		select {
		case tx = <-vc.mempool:
		default: // mempool is empty
			break txLoop
		}
		// ensure all txs are still valid valid
		resp, err := vc.App.CheckTx(context.Background(), &abcitypes.RequestCheckTx{Tx: tx})
		if err != nil {
			log.Errorw(err, "error on check tx")
			continue
		}
		if resp.Code == 0 {
			if err := blockStoreTx.Set(
				[]byte(fmt.Sprintf("%d_%d", vc.height.Load(), txCount)),
				tx,
			); err != nil {
				log.Errorw(err, "error on store tx")
				continue
			}
			transactions = append(transactions, tx)
		} else {
			log.Warnw("check tx failed", "code", resp.Code, "data", string(resp.Data))
		}
	}
	if len(transactions) > 0 {
		log.Infow("stored transactions on block", "count", len(transactions), "height", vc.height.Load())
	}
	return transactions
}

// TO-DO: improve this function
func (vc *Vocone) getBlock(height int64) *tmtypes.Block {
	blk := new(tmtypes.Block)
	blk.Height = height
	blk.ChainID = vc.App.ChainID()
	v, found := vc.blockTimestamps.Load(height)
	if found {
		if t, ok := v.(time.Time); ok {
			blk.Time = t
		}
	}
	for i := int32(0); ; i++ {
		tx, err := vc.getTx(uint32(height), i)
		if err != nil {
			break
		}
		txb, err := proto.Marshal(tx)
		if err != nil {
			log.Warnf("error marshaling tx: %v", err)
			continue
		}
		blk.Data.Txs = append(blk.Data.Txs, txb)
	}
	return blk
}

func (vc *Vocone) getTx(height uint32, txIndex int32) (*models.SignedTx, error) {
	tx, err := vc.blockStore.Get([]byte(fmt.Sprintf("%d_%d", height, txIndex)))
	if err != nil {
		return nil, err
	}
	stx := &models.SignedTx{}
	return stx, proto.Unmarshal(tx, stx)
}

func (vc *Vocone) mempoolSize() int {
	return len(vc.mempool)
}

func (vc *Vocone) getTxWithHash(height uint32, txIndex int32) (*models.SignedTx, []byte, error) {
	tx, err := vc.blockStore.Get([]byte(fmt.Sprintf("%d_%d", height, txIndex)))
	if err != nil {
		return nil, nil, err
	}
	stx := &models.SignedTx{}
	return stx, ethereum.HashRaw(tx), proto.Unmarshal(tx, stx)
}

// VochainPrintInfo initializes the Vochain statistics recollection
func vochainPrintInfo(interval time.Duration, vi *vochaininfo.VochainInfo) {
	var h uint64
	var p, v uint64
	var m, vc, vxm uint64
	var b strings.Builder
	for {
		b.Reset()
		a := vi.BlockTimes()
		if a[0] > 0 {
			fmt.Fprintf(&b, "1m:%s", a[0].Truncate(time.Millisecond))
		}
		if a[1] > 0 {
			fmt.Fprintf(&b, " 10m:%s", a[1].Truncate(time.Millisecond))
		}
		if a[2] > 0 {
			fmt.Fprintf(&b, " 1h:%s", a[2].Truncate(time.Millisecond))
		}
		if a[3] > 0 {
			fmt.Fprintf(&b, " 6h:%s", a[3].Truncate(time.Millisecond))
		}
		if a[4] > 0 {
			fmt.Fprintf(&b, " 24h:%s", a[4].Truncate(time.Millisecond))
		}
		h = vi.Height()
		m = vi.MempoolSize()
		p, v, vxm = vi.TreeSizes()
		vc = vi.VoteCacheSize()
		log.Monitor("[vochain info]", map[string]any{
			"height":       h,
			"mempool":      m,
			"processes":    p,
			"votes":        v,
			"vxm":          vxm,
			"voteCache":    vc,
			"blockPeriod":  b.String(),
			"blocksMinute": fmt.Sprintf("%.2f", vi.BlocksLastMinute()),
		})
		time.Sleep(interval)
	}
}

func defaultTxCosts() genesis.TransactionCosts {
	return genesis.TransactionCosts{
		SetProcessStatus:        1,
		SetProcessCensus:        1,
		SetProcessQuestionIndex: 1,
		RegisterKey:             1,
		NewProcess:              10,
		SendTokens:              1,
		SetAccountInfoURI:       5,
		CreateAccount:           1,
		AddDelegateForAccount:   1,
		DelDelegateForAccount:   1,
		CollectFaucet:           1,
	}
}

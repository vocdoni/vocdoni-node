package vocone

import (
	"errors"
	"fmt"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	abcitypes "github.com/cometbft/cometbft/abci/types"
	"github.com/cometbft/cometbft/crypto/tmhash"
	tmprototypes "github.com/cometbft/cometbft/proto/tendermint/types"
	tmcoretypes "github.com/cometbft/cometbft/rpc/core/types"
	tmtypes "github.com/cometbft/cometbft/types"
	"github.com/ethereum/go-ethereum/common"
	"go.vocdoni.io/dvote/api"
	"go.vocdoni.io/dvote/api/censusdb"
	"go.vocdoni.io/dvote/config"
	"go.vocdoni.io/dvote/crypto/ethereum"
	"go.vocdoni.io/dvote/data"
	"go.vocdoni.io/dvote/data/downloader"
	"go.vocdoni.io/dvote/db"
	"go.vocdoni.io/dvote/db/metadb"
	"go.vocdoni.io/dvote/httprouter"
	"go.vocdoni.io/dvote/log"
	"go.vocdoni.io/dvote/service"
	"go.vocdoni.io/dvote/vochain"
	"go.vocdoni.io/dvote/vochain/indexer"
	"go.vocdoni.io/dvote/vochain/keykeeper"
	"go.vocdoni.io/dvote/vochain/offchaindatahandler"
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
	sc              *indexer.Indexer
	kk              *keykeeper.KeyKeeper
	mempool         chan []byte // a buffered channel acts like a FIFO with a fixed size
	blockStore      db.Database
	dataDir         string
	height          atomic.Int64
	appInfo         *vochaininfo.VochainInfo
	app             *vochain.BaseApplication
	storage         data.Storage
	censusdb        *censusdb.CensusDB
	lastBlockTime   time.Time
	blockTimeTarget time.Duration
	txsPerBlock     int
	// vcMtx is a lock on modification to the app state.
	// this enables direct calls to vochain functions from the vocone
	//  without causing race conditions
	vcMtx sync.Mutex
}

// NewVocone returns a ready Vocone instance.
func NewVocone(dataDir string, keymanager *ethereum.SignKeys) (*Vocone, error) {
	vc := &Vocone{}
	var err error
	vc.dataDir = dataDir
	vc.app, err = vochain.NewBaseApplication(db.TypePebble, dataDir)
	if err != nil {
		return nil, err
	}
	vc.mempool = make(chan []byte, mempoolSize)
	vc.blockTimeTarget = DefaultBlockTimeTarget
	vc.txsPerBlock = DefaultTxsPerBlock
	version, err := vc.app.State.Store.Version()
	if err != nil {
		return nil, err
	}
	vc.height.Store(int64(version))
	if vc.blockStore, err = metadb.New(db.TypePebble,
		filepath.Join(dataDir, "blockstore")); err != nil {
		return nil, err
	}

	vc.setDefaultMethods()
	vc.app.State.SetHeight(uint32(vc.height.Load()))

	// Set max process size to 1M
	if err := vc.app.State.SetMaxProcessSize(1000000); err != nil {
		return nil, err
	}

	// Create burn account
	if err := vc.CreateAccount(state.BurnAddress, &state.Account{}); err != nil {
		return nil, err
	}

	// Create indexer
	if vc.sc, err = indexer.NewIndexer(
		filepath.Join(dataDir, "indexer"),
		vc.app,
		true,
	); err != nil {
		return nil, err
	}

	// Create key keeper
	if err := vc.SetKeyKeeper(keymanager); err != nil {
		return nil, fmt.Errorf("could not create keykeeper: %w", err)
	}

	// Create vochain metrics collector
	vc.appInfo = vochaininfo.NewVochainInfo(vc.app)
	go vc.appInfo.Start(10)

	// Create the IPFS storage layer (we use the Vocdoni general service)
	srv := service.VocdoniService{}
	if vc.storage, err = srv.IPFS(&config.IPFSCfg{
		ConfigPath: filepath.Join(dataDir, "ipfs"),
	}); err != nil {
		return nil, err
	}

	// Create the census database for storing census data
	cdb, err := metadb.New(db.TypePebble, filepath.Join(dataDir, "census"))
	if err != nil {
		return nil, err
	}
	vc.censusdb = censusdb.NewCensusDB(cdb)

	// Create the data downloader and offchain data handler
	offchaindatahandler.NewOffChainDataHandler(
		vc.app,
		downloader.NewDownloader(vc.storage),
		vc.censusdb,
		false,
	)

	return vc, err
}

// EnableAPI starts the HTTP API server. It is not enabled by default.
func (vc *Vocone) EnableAPI(host string, port int, URLpath string) (*api.API, error) {
	var httpRouter httprouter.HTTProuter
	if err := httpRouter.Init(host, port); err != nil {
		return nil, err
	}
	uAPI, err := api.NewAPI(&httpRouter, URLpath, vc.dataDir)
	if err != nil {
		return nil, err
	}
	uAPI.Attach(
		vc.app,
		vc.appInfo,
		vc.sc,
		vc.storage,
		vc.censusdb,
	)
	return uAPI, uAPI.EnableHandlers(
		api.ElectionHandler,
		api.VoteHandler,
		api.ChainHandler,
		api.WalletHandler,
		api.AccountHandler,
		api.CensusHandler,
	)
}

// Start initializes the block production. This method should be run async.
func (vc *Vocone) Start() {
	vc.lastBlockTime = time.Now()
	go vochainPrintInfo(10, vc.appInfo)

	for {
		// Begin block
		vc.vcMtx.Lock()
		bblock := abcitypes.RequestBeginBlock{
			Header: tmprototypes.Header{
				Time:   time.Now(),
				Height: vc.height.Load(),
			},
		}
		vc.app.BeginBlock(bblock)
		// Commit block
		vc.commitBlock()
		comres := vc.app.Commit()
		log.Debugf("commit hash for block %d: %x", bblock.Header.Height, comres.Data)
		vc.app.EndBlock(abcitypes.RequestEndBlock{Height: bblock.Header.Height})
		vc.vcMtx.Unlock()

		// Waiting time
		sinceLast := time.Since(vc.lastBlockTime)
		if sinceLast < vc.blockTimeTarget {
			time.Sleep(vc.blockTimeTarget - sinceLast)
		}
		vc.lastBlockTime = time.Now()
		vc.height.Add(1)
	}
}

// SetBlockTimeTarget configures the time window in which blocks will be created.
func (vc *Vocone) SetBlockTimeTarget(targetTime time.Duration) {
	vc.blockTimeTarget = targetTime
}

// SetBlockSize configures the maximum number of transactions per block.
func (vc *Vocone) SetBlockSize(txsCount int) {
	vc.txsPerBlock = txsCount
}

// CreateAccount creates a new account in the state.
func (vc *Vocone) CreateAccount(key common.Address, acc *state.Account) error {
	vc.vcMtx.Lock()
	defer vc.vcMtx.Unlock()
	if err := vc.app.State.SetAccount(key, acc); err != nil {
		return err
	}
	if _, err := vc.app.State.Save(); err != nil {
		return err
	}
	return nil
}

// SetTreasurer configures the vocone treasurer account address
func (vc *Vocone) SetTreasurer(treasurer common.Address) error {
	vc.vcMtx.Lock()
	defer vc.vcMtx.Unlock()
	if err := vc.app.State.SetTreasurer(treasurer, 0); err != nil {
		return err
	}
	if _, err := vc.app.State.Save(); err != nil {
		return err
	}
	return nil
}

// SetKeyKeeper adds a keykeper to the application.
func (vc *Vocone) SetKeyKeeper(key *ethereum.SignKeys) error {
	// Create key keeper
	// we need to add a validator, so the keykeeper functions are allowed
	vc.vcMtx.Lock()
	defer vc.vcMtx.Unlock()
	// remove existing validators before we add the new one (to avoid keyindex collision)
	validators, err := vc.app.State.Validators(true)
	if err != nil {
		return err
	}
	for _, v := range validators {
		if err := vc.app.State.RemoveValidator(v.Address); err != nil {
			log.Warnf("could not remove validator %x", v.Address)
		}
	}
	// add the new validator
	if err := vc.app.State.AddValidator(&models.Validator{
		Address:  key.Address().Bytes(),
		Power:    100,
		Name:     "vocone-solo-validator",
		KeyIndex: 1,
	}); err != nil {
		return err
	}
	log.Infow("adding validator", "address", key.Address().Hex(), "keyIndex", 1)
	if _, err := vc.app.State.Save(); err != nil {
		return err
	}
	vc.kk, err = keykeeper.NewKeyKeeper(
		filepath.Join(vc.dataDir, "keykeeper"),
		vc.app,
		key,
		1)
	return err
}

// MintTokens mints tokens to the given account address
func (vc *Vocone) MintTokens(to common.Address, amount uint64) error {
	vc.vcMtx.Lock()
	defer vc.vcMtx.Unlock()
	if err := vc.app.State.InitChainMintBalance(to, amount); err != nil {
		return err
	}
	if err := vc.app.State.IncrementTreasurerNonce(); err != nil {
		return err
	}
	if _, err := vc.app.State.Save(); err != nil {
		return err
	}
	return nil
}

// SetTxCost configures the transaction cost for the given tx type.
func (vc *Vocone) SetTxCost(txType models.TxType, cost uint64) error {
	vc.vcMtx.Lock()
	defer vc.vcMtx.Unlock()
	if err := vc.app.State.SetTxBaseCost(txType, cost); err != nil {
		return err
	}
	if err := vc.app.State.IncrementTreasurerNonce(); err != nil {
		return err
	}
	if _, err := vc.app.State.Save(); err != nil {
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
			_, err := vc.app.State.TxBaseCost(k, true)
			if err == nil || errors.Is(err, state.ErrTxCostNotFound) {
				continue
			}
			// If error is not ErrTxCostNotFound, return it
			return err
		}
		log.Infow("setting tx base cost", "txtype", models.TxType_name[int32(k)], "cost", txCost)
		if err := vc.app.State.SetTxBaseCost(k, txCost); err != nil {
			return err
		}
	}
	if _, err := vc.app.State.Save(); err != nil {
		return err
	}
	return nil
}

// SetElectionPrice sets the election price.
func (vc *Vocone) SetElectionPrice() error {
	vc.vcMtx.Lock()
	defer vc.vcMtx.Unlock()
	if err := vc.app.State.SetElectionPriceCalc(); err != nil {
		return err
	}
	return nil
}

func (vc *Vocone) setDefaultMethods() {
	// first set the default methods, then override some of them
	vc.app.SetDefaultMethods()
	vc.app.SetFnIsSynchronizing(func() bool { return false })
	vc.app.SetFnSendTx(vc.addTx)
	vc.app.SetFnGetTx(vc.getTx)
	vc.app.SetFnGetBlockByHeight(vc.getBlock)
	vc.app.SetFnGetTxHash(vc.getTxWithHash)
	vc.app.SetFnMempoolSize(vc.mempoolSize)
	vc.app.SetFnMempoolPrune(nil)
}

func (vc *Vocone) addTx(tx []byte) (*tmcoretypes.ResultBroadcastTx, error) {
	resp := vc.app.CheckTx(abcitypes.RequestCheckTx{Tx: tx})
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

func (vc *Vocone) commitBlock() {
	blockStoreTx := vc.blockStore.WriteTx()
	defer blockStoreTx.Discard()
	var txCount int
txLoop:
	for txCount = 0; txCount < vc.txsPerBlock; {
		var tx []byte
		select {
		case tx = <-vc.mempool:
		default: // mempool is empty
			break txLoop
		}
		resp := vc.app.DeliverTx(abcitypes.RequestDeliverTx{Tx: tx})
		if resp.Code == 0 {
			blockStoreTx.Set(
				[]byte(fmt.Sprintf("%d_%d", vc.height.Load(), txCount)),
				tx,
			)
			txCount++
			log.Debugf("deliver tx succeed %s", resp.Info)
		} else {
			log.Warnf("deliver tx failed: %s", resp.Data)
		}
	}
	if txCount > 0 {
		log.Infof("stored %d transactions on block %d", txCount, vc.height.Load())
		if err := blockStoreTx.Commit(); err != nil {
			log.Errorf("cannot commit to blockstore: %v", err)
		}
	}
}

// TO-DO: improve this function
func (vc *Vocone) getBlock(height int64) *tmtypes.Block {
	blk := new(tmtypes.Block)
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
	rtx := vc.blockStore.ReadTx()
	tx, err := rtx.Get([]byte(fmt.Sprintf("%d_%d", height, txIndex)))
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
	rtx := vc.blockStore.ReadTx()
	tx, err := rtx.Get([]byte(fmt.Sprintf("%d_%d", height, txIndex)))
	if err != nil {
		return nil, nil, err
	}
	stx := &models.SignedTx{}
	return stx, ethereum.HashRaw(tx), proto.Unmarshal(tx, stx)
}

// VochainPrintInfo initializes the Vochain statistics recollection
func vochainPrintInfo(sleepSecs int64, vi *vochaininfo.VochainInfo) {
	var a *[5]int32
	var h int64
	var p, v uint64
	var m, vc, vxm int
	var b strings.Builder
	for {
		b.Reset()
		a = vi.BlockTimes()
		if a[0] > 0 {
			fmt.Fprintf(&b, "1m:%.2f", float32(a[0])/1000)
		}
		if a[1] > 0 {
			fmt.Fprintf(&b, " 10m:%.2f", float32(a[1])/1000)
		}
		if a[2] > 0 {
			fmt.Fprintf(&b, " 1h:%.2f", float32(a[2])/1000)
		}
		if a[3] > 0 {
			fmt.Fprintf(&b, " 6h:%.2f", float32(a[3])/1000)
		}
		if a[4] > 0 {
			fmt.Fprintf(&b, " 24h:%.2f", float32(a[4])/1000)
		}
		h = vi.Height()
		m = vi.MempoolSize()
		p, v, vxm = vi.TreeSizes()
		vc = vi.VoteCacheSize()
		log.Infof("[vochain info] height:%d mempool:%d "+
			"processes:%d votes:%d vxm:%d voteCache:%d blockTime:{%s}",
			h, m, p, v, vxm, vc, b.String(),
		)
		time.Sleep(time.Duration(sleepSecs) * time.Second)
	}
}

// SetChainID sets the chainID for the vocone instance
func (vc *Vocone) SetChainID(chainID string) {
	vc.app.SetChainID(chainID)
}

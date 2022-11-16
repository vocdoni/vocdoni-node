package vocone

import (
	"errors"
	"fmt"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/enriquebris/goconcurrentqueue"
	"github.com/ethereum/go-ethereum/common"
	abcitypes "github.com/tendermint/tendermint/abci/types"
	"github.com/tendermint/tendermint/crypto/tmhash"
	tmprototypes "github.com/tendermint/tendermint/proto/tendermint/types"
	tmcoretypes "github.com/tendermint/tendermint/rpc/coretypes"
	tmtypes "github.com/tendermint/tendermint/types"
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
	"go.vocdoni.io/dvote/oracle"
	"go.vocdoni.io/dvote/service"
	"go.vocdoni.io/dvote/vochain"
	"go.vocdoni.io/dvote/vochain/keykeeper"
	"go.vocdoni.io/dvote/vochain/offchaindatahandler"
	"go.vocdoni.io/dvote/vochain/scrutinizer"
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
	sc              *scrutinizer.Scrutinizer
	kk              *keykeeper.KeyKeeper
	oracle          *oracle.Oracle
	mempool         *goconcurrentqueue.FixedFIFO
	blockStore      db.Database
	dataDir         string
	height          int64
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
func NewVocone(dataDir string, keymanager *ethereum.SignKeys, disableIpfs bool) (*Vocone, error) {
	vc := &Vocone{}
	var err error
	vc.dataDir = dataDir
	vc.app, err = vochain.NewBaseApplication(db.TypePebble, dataDir)
	if err != nil {
		return nil, err
	}
	vc.mempool = goconcurrentqueue.NewFixedFIFO(mempoolSize)
	vc.blockTimeTarget = DefaultBlockTimeTarget
	vc.txsPerBlock = DefaultTxsPerBlock
	version, err := vc.app.State.Store.Version()
	if err != nil {
		return nil, err
	}
	vc.height = int64(version)
	if vc.blockStore, err = metadb.New(db.TypePebble,
		filepath.Join(dataDir, "blockstore")); err != nil {
		return nil, err
	}

	vc.setDefaultMethods()
	vc.app.State.SetHeight(uint32(vc.height))

	// Create burn account
	if err := vc.CreateAccount(vochain.BurnAddress, &vochain.Account{}); err != nil {
		return nil, err
	}

	// Create scrutinizer
	if vc.sc, err = scrutinizer.NewScrutinizer(
		filepath.Join(dataDir, "scrutinizer"),
		vc.app,
		true,
	); err != nil {
		return nil, err
	}

	// Create key keeper and the oracle (same key for both)
	if err := vc.AddOracle(keymanager); err != nil {
		return nil, err
	}
	vc.kk, err = keykeeper.NewKeyKeeper(
		filepath.Join(dataDir, "keykeeper"),
		vc.app,
		keymanager,
		1)
	if err != nil {
		return nil, err
	}
	if vc.oracle, err = oracle.NewOracle(vc.app, keymanager); err != nil {
		return nil, err
	}
	vc.oracle.EnableResults(vc.sc)

	// Create vochain metrics collector
	vc.appInfo = vochaininfo.NewVochainInfo(vc.app)
	go vc.appInfo.Start(10)

	// Create the IPFS storage layer
	if vc.storage, err = service.IPFS(&config.IPFSCfg{
		ConfigPath: filepath.Join(dataDir, "ipfs"), NoInit: disableIpfs,
	}, nil, nil); err != nil {
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
func (vc *Vocone) EnableAPI(host string, port int, URLpath string) error {
	var httpRouter httprouter.HTTProuter
	if err := httpRouter.Init(host, port); err != nil {
		return err
	}
	uAPI, err := api.NewAPI(&httpRouter, URLpath, vc.dataDir)
	if err != nil {
		return err
	}
	uAPI.Attach(
		vc.app,
		vc.appInfo,
		vc.sc,
		vc.storage,
		vc.censusdb,
	)
	return uAPI.EnableHandlers(
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
				Height: atomic.LoadInt64(&vc.height),
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
		atomic.AddInt64(&vc.height, 1)
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

// AddOracle adds a new oracle to the state. If oracle exists, does nothing.
func (vc *Vocone) AddOracle(oracleKey *ethereum.SignKeys) error {
	oracleList, err := vc.app.State.Oracles(true)
	if err != nil {
		return err
	}
	oracleExist := false
	for _, o := range oracleList {
		if oracleKey.Address() == o {
			oracleExist = true
			break
		}
	}
	if !oracleExist {
		log.Infof("adding new oracle %s", oracleKey.Address())
		vc.vcMtx.Lock()
		vc.app.State.AddOracle(oracleKey.Address())
		if _, err := vc.app.State.Save(); err != nil {
			vc.vcMtx.Unlock()
			return err
		}
		vc.vcMtx.Unlock()
	}
	// Create the account and assign balance if does not exist or balance too low
	oAcc, err := vc.app.State.GetAccount(oracleKey.Address(), true)
	if err != nil {
		return err
	}
	if oAcc == nil || oAcc.Balance < 10000 {
		vc.CreateAccount(oracleKey.Address(), &vochain.Account{Account: models.Account{
			Balance: 100000,
		}})
	}
	return nil
}

// CreateAccount creates a new account in the state.
func (vc *Vocone) CreateAccount(key common.Address, acc *vochain.Account) error {
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

// MintTokens mints tokens to the given account address
func (vc *Vocone) MintTokens(to common.Address, amount uint64) error {
	vc.vcMtx.Lock()
	defer vc.vcMtx.Unlock()
	if err := vc.app.State.MintBalance(to, amount); err != nil {
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
	if err := vc.app.State.SetTxCost(txType, cost); err != nil {
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
	for k := range vochain.TxTypeCostToStateKey {
		if !force {
			_, err := vc.app.State.TxCost(k, true)
			if err == nil || errors.Is(err, vochain.ErrTxCostNotFound) {
				continue
			}
			// If error is not ErrTxCostNotFound, return it
			return err
		}
		log.Debugf("setting tx cost for txtype %s", models.TxType_name[int32(k)])
		if err := vc.app.State.SetTxCost(k, txCost); err != nil {
			return err
		}
	}
	if _, err := vc.app.State.Save(); err != nil {
		return err
	}
	return nil
}

func (vc *Vocone) setDefaultMethods() {
	vc.app.IsSynchronizing = func() bool { return false }
	vc.app.SetFnSendTx(vc.addTx)
	vc.app.SetFnGetTx(vc.getTx)
	vc.app.SetFnGetBlockByHeight(vc.getBlock)
	vc.app.SetFnGetTxHash(vc.getTxWithHash)
	vc.app.SetFnMempoolSize(vc.mempoolSize)
}

func (vc *Vocone) addTx(tx []byte) (*tmcoretypes.ResultBroadcastTx, error) {
	resp := vc.app.CheckTx(abcitypes.RequestCheckTx{Tx: tx})
	if resp.Code == 0 {
		if err := vc.mempool.Enqueue(tx); err != nil {
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
	for txCount = 0; txCount < vc.txsPerBlock; {
		tx, err := vc.mempool.Dequeue()
		if err != nil {
			break
		}
		resp := vc.app.DeliverTx(abcitypes.RequestDeliverTx{Tx: tx.([]byte)})
		if resp.Code == 0 {
			blockStoreTx.Set(
				[]byte(fmt.Sprintf("%d_%d", vc.height, txCount)),
				tx.([]byte),
			)
			txCount++
			log.Debugf("deliver tx succeed %s", resp.Info)
		} else {
			log.Warnf("deliver tx failed: %s", resp.Data)
		}
	}
	if txCount > 0 {
		log.Infof("stored %d transactions on block %d", txCount, vc.height)
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
	return vc.mempool.GetLen()
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

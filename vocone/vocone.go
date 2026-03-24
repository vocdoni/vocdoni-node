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

	cometabcitypes "github.com/cometbft/cometbft/abci/types"
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
	"go.vocdoni.io/dvote/vochain/transaction/proofs/farcasterproof"
	"go.vocdoni.io/dvote/vochain/vochaininfo"
	"go.vocdoni.io/proto/build/go/models"
)

const (
	DefaultTxsPerBlock     = 500
	DefaultBlockTimeTarget = time.Second * 5
	DefaultTxCosts         = 10
	DefaultMempoolSize     = 8192 // maximum number of pending transactions

	// Key prefixes for the block store KV database.
	prefixTx        = "tx/"        // tx/<height_8B>/<txIndex_4B> → raw tx bytes
	prefixMeta      = "meta/"      // meta/<height_8B> → blockMeta JSON
	prefixBlockHash = "blockhash/" // blockhash/<hash> → height (8B BE)

	// Key prefixes for the mempool KV database.
	prefixMempool = "mp/"   // mp/<seq_8B> → raw tx bytes
	keyMempoolSeq = "mpseq" // last sequence number (8B BE)
)

// Vocone is an implementation of the Vocdoni protocol run by a single (atomic) node.
type Vocone struct {
	service.VocdoniService

	blockStore  db.Database
	mempoolDB   db.Database
	height      atomic.Int64
	txsPerBlock int
	closed      atomic.Bool

	// mempoolMtx protects mempoolKeys and mempoolSeq.
	mempoolMtx  sync.Mutex
	mempoolKeys [][]byte // ordered list of db keys for pending txs
	mempoolSeq  uint64   // monotonic sequence for mempool key generation

	// vcMtx serializes block production and state modifications.
	vcMtx           sync.Mutex
	lastBlockTime   time.Time
	proposerAddress []byte // address of the solo validator
}

// NewVocone creates and returns a ready Vocone instance.
func NewVocone(dataDir string, keymanager *ethereum.SignKeys, disableIPFS bool,
	connectKey string, connectPeers []string,
) (*Vocone, error) {
	vc := &Vocone{}
	vc.Config = &config.VochainCfg{}
	vc.Config.DataDir = dataDir
	vc.Config.DBType = db.TypePebble

	var err error
	vc.App, err = vochain.NewBaseApplication(vc.Config)
	if err != nil {
		return nil, fmt.Errorf("could not create base application: %w", err)
	}
	vc.txsPerBlock = DefaultTxsPerBlock

	// Recover height from persisted state.
	lastHeight, err := vc.App.State.LastHeight()
	if err != nil {
		return nil, fmt.Errorf("could not get last height: %w", err)
	}
	vc.height.Store(int64(lastHeight))

	// Open the block store database.
	vc.blockStore, err = metadb.New(db.TypePebble, filepath.Join(dataDir, "blockstore"))
	if err != nil {
		return nil, fmt.Errorf("could not open blockstore: %w", err)
	}

	// Open the mempool database and reload pending transactions.
	vc.mempoolDB, err = metadb.New(db.TypePebble, filepath.Join(dataDir, "mempool"))
	if err != nil {
		return nil, fmt.Errorf("could not open mempool db: %w", err)
	}
	if err := vc.loadMempool(); err != nil {
		return nil, fmt.Errorf("could not load mempool: %w", err)
	}

	vc.setDefaultMethods()
	vc.App.State.SetHeight(uint32(vc.height.Load()))
	vc.App.SetBlockTimeTarget(DefaultBlockTimeTarget)

	// Create indexer.
	vc.Indexer, err = indexer.New(vc.App, indexer.Options{
		DataDir: filepath.Join(dataDir, "indexer"),
	})
	if err != nil {
		return nil, fmt.Errorf("could not create indexer: %w", err)
	}

	// Create key keeper (also adds the validator).
	if err := vc.SetKeyKeeper(keymanager); err != nil {
		return nil, fmt.Errorf("could not create keykeeper: %w", err)
	}

	// Create vochain metrics collector.
	vc.Stats = vochaininfo.NewVochainInfo(vc.App)
	go vc.Stats.Start(10)

	// Create the IPFS storage layer.
	if !disableIPFS {
		vc.Storage, err = vc.IPFS(&config.IPFSCfg{
			ConfigPath:   filepath.Join(dataDir, "ipfs"),
			ConnectKey:   connectKey,
			ConnectPeers: connectPeers,
		})
		if err != nil {
			return nil, fmt.Errorf("could not create IPFS storage: %w", err)
		}
		if err := vc.OffChainDataHandler(); err != nil {
			return nil, fmt.Errorf("could not create offchain data handler: %w", err)
		}
	}

	// Disable election ID verification on the farcaster proof for testing purposes.
	farcasterproof.DisableElectionIDVerification = true

	return vc, nil
}

// EnableAPI starts the HTTP API server. It is not enabled by default.
func (vc *Vocone) EnableAPI(host string, port int, urlPath string) (*api.API, error) {
	vc.Router = new(httprouter.HTTProuter)
	if err := vc.Router.Init(host, port); err != nil {
		return nil, err
	}
	uAPI, err := api.NewAPI(vc.Router, urlPath, vc.Config.DataDir, db.TypePebble)
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

// Start runs the block production loop. It blocks until ctx is cancelled.
// Returns an error if initialization fails; block production errors are logged and skipped.
func (vc *Vocone) Start(ctx context.Context) error {
	vc.lastBlockTime = time.Now()
	go vochainPrintInfo(ctx, 10*time.Second, vc.Stats)

	if vc.App.Height() == 0 {
		log.Infow("initializing new blockchain")
		genesisAppData, err := json.Marshal(&genesis.AppState{
			MaxElectionSize: 1000000,
			NetworkCapacity: uint64(vc.txsPerBlock),
			TxCost:          defaultTxCosts(),
		})
		if err != nil {
			return fmt.Errorf("could not marshal genesis app state: %w", err)
		}
		if _, err = vc.App.InitChain(ctx, &cometabcitypes.InitChainRequest{
			ChainId:       vc.App.ChainID(),
			AppStateBytes: genesisAppData,
			Time:          time.Now(),
		}); err != nil {
			return fmt.Errorf("could not init chain: %w", err)
		}
		time.Sleep(1 * time.Second)
	}

	for {
		select {
		case <-ctx.Done():
			log.Infow("stopping block production", "height", vc.height.Load())
			return nil
		default:
		}

		if err := vc.produceBlock(); err != nil {
			log.Errorw(err, "block production error")
		}

		// Wait until the block time target has elapsed.
		sinceLast := time.Since(vc.lastBlockTime)
		if wait := vc.App.BlockTimeTarget() - sinceLast; wait > 0 {
			select {
			case <-ctx.Done():
				return nil
			case <-time.After(wait):
			}
		}
	}
}

// produceBlock creates and commits a single block.
func (vc *Vocone) produceBlock() error {
	vc.vcMtx.Lock()
	defer vc.vcMtx.Unlock()

	startTime := time.Now()
	height := vc.height.Load()

	txs := vc.prepareBlock()

	resp, err := vc.App.ExecuteBlock(txs, uint32(height), startTime)
	if err != nil {
		return fmt.Errorf("execute block at height %d: %w", height, err)
	}

	// Build the block to compute its hash (needed for the indexer and hash index).
	blk := vc.buildBlock(height, startTime, txs, resp.Root)
	blockHash := blk.Hash()

	// Retrieve the previous block hash for chain linkage.
	var lastBlockHash []byte
	if height > 0 {
		if prevMeta, err := vc.loadBlockMeta(height - 1); err == nil {
			lastBlockHash = prevMeta.Hash
		}
	}

	// Persist block metadata and hash→height index.
	if err := vc.storeBlockMeta(height, startTime, int32(len(txs)), resp.Root,
		blockHash, vc.proposerAddress, lastBlockHash, blk.DataHash); err != nil {
		return fmt.Errorf("store block meta at height %d: %w", height, err)
	}

	if _, err := vc.App.CommitState(); err != nil {
		return fmt.Errorf("commit state at height %d: %w", height, err)
	}

	log.Debugw("block committed",
		"timestamp", startTime.Unix(),
		"height", height,
		"hash", hex.EncodeToString(blockHash),
		"txs", len(txs),
		"took", time.Since(startTime),
	)

	vc.lastBlockTime = time.Now()
	vc.height.Add(1)
	return nil
}

// Close releases all resources held by Vocone.
func (vc *Vocone) Close() error {
	vc.closed.Store(true)
	var errs []error
	if vc.blockStore != nil {
		if err := vc.blockStore.Close(); err != nil {
			errs = append(errs, fmt.Errorf("close blockstore: %w", err))
		}
	}
	if vc.mempoolDB != nil {
		if err := vc.mempoolDB.Close(); err != nil {
			errs = append(errs, fmt.Errorf("close mempool db: %w", err))
		}
	}
	return errors.Join(errs...)
}

// setDefaultMethods configures the vochain callback functions for single-node operation.
func (vc *Vocone) setDefaultMethods() {
	vc.App.SetDefaultMethods()
	vc.App.SetFnIsSynchronizing(func() bool { return false })
	vc.App.SetFnSendTx(vc.addTx)
	vc.App.SetFnGetTx(vc.getTx)
	vc.App.SetFnGetBlockByHeight(vc.getBlock)
	vc.App.SetFnGetBlockByHash(vc.getBlockByHash)
	vc.App.SetFnGetTxHash(vc.getTxWithHash)
	vc.App.SetFnMempoolSize(vc.mempoolSize)
	vc.App.SetFnMempoolPrune(vc.mempoolPrune)
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
	_, err := vc.commit()
	return err
}

// commit saves the current state and returns the hash. Must be called with vcMtx held.
func (vc *Vocone) commit() ([]byte, error) {
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
	vc.vcMtx.Lock()
	defer vc.vcMtx.Unlock()
	// Remove existing validators before adding the new one (avoid keyindex collision).
	validators, err := vc.App.State.Validators(true)
	if err != nil {
		return err
	}
	for _, v := range validators {
		if err := vc.App.State.RemoveValidator(v); err != nil {
			log.Warnw("could not remove validator", "address", fmt.Sprintf("%x", v.Address))
		}
	}
	if err := vc.App.State.AddValidator(&models.Validator{
		Address:  key.Address().Bytes(),
		Power:    100,
		Name:     "vocone-solo-validator",
		KeyIndex: 1,
	}); err != nil {
		return err
	}
	log.Infow("adding validator", "address", key.Address().Hex(), "keyIndex", 1)
	vc.proposerAddress = key.Address().Bytes()
	if _, err := vc.commit(); err != nil {
		return err
	}
	vc.KeyKeeper, err = keykeeper.NewKeyKeeper(vc.App, key, 1)
	return err
}

// SetTxCost configures the transaction cost for the given tx type.
func (vc *Vocone) SetTxCost(txType models.TxType, cost uint64) error {
	vc.vcMtx.Lock()
	defer vc.vcMtx.Unlock()
	if err := vc.App.State.SetTxBaseCost(txType, cost); err != nil {
		return err
	}
	_, err := vc.commit()
	return err
}

// SetBulkTxCosts sets the transaction cost for all existing transaction types.
// If force is enabled the cost is set for all tx types.
// If force is disabled, the cost is set only for tx types that have not been set yet.
func (vc *Vocone) SetBulkTxCosts(txCost uint64, force bool) error {
	vc.vcMtx.Lock()
	defer vc.vcMtx.Unlock()
	for k := range state.TxTypeCostToStateKey {
		if !force {
			_, err := vc.App.State.TxBaseCost(k, true)
			if err == nil {
				// Cost already set, skip.
				continue
			}
			if !errors.Is(err, state.ErrTxCostNotFound) {
				return err
			}
			// Cost not found — fall through to set it.
		}
		log.Infow("setting tx base cost", "txtype", models.TxType_name[int32(k)], "cost", txCost)
		if err := vc.App.State.SetTxBaseCost(k, txCost); err != nil {
			return err
		}
	}
	_, err := vc.commit()
	return err
}

// SetElectionPrice sets the election price calculator.
func (vc *Vocone) SetElectionPrice() error {
	vc.vcMtx.Lock()
	defer vc.vcMtx.Unlock()
	return vc.App.State.SetElectionPriceCalc()
}

// vochainPrintInfo periodically logs chain statistics.
func vochainPrintInfo(ctx context.Context, interval time.Duration, vi *vochaininfo.VochainInfo) {
	var b strings.Builder
	for {
		select {
		case <-ctx.Done():
			return
		case <-time.After(interval):
		}

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

		h := vi.Height()
		m := vi.MempoolSize()
		p, v, vxm := vi.TreeSizes()
		vc := vi.VoteCacheSize()
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

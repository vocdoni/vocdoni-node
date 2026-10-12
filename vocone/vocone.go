package vocone

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"go.vocdoni.io/dvote/api"
	"go.vocdoni.io/dvote/config"
	"go.vocdoni.io/dvote/crypto/ethereum"
	"go.vocdoni.io/dvote/db"
	"go.vocdoni.io/dvote/db/metadb"
	"go.vocdoni.io/dvote/httprouter"
	"go.vocdoni.io/dvote/log"
	"go.vocdoni.io/dvote/service"
	"go.vocdoni.io/dvote/vochain"
	"go.vocdoni.io/dvote/vochain/blocktypes"
	"go.vocdoni.io/dvote/vochain/genesis"
	"go.vocdoni.io/dvote/vochain/indexer"
	"go.vocdoni.io/dvote/vochain/keykeeper"
	"go.vocdoni.io/dvote/vochain/state"
	"go.vocdoni.io/dvote/vochain/vochaininfo"
	"go.vocdoni.io/proto/build/go/models"
)

const (
	DefaultTxsPerBlock     = 500
	DefaultBlockTimeTarget = time.Second * 5
	DefaultTxCosts         = 10
	DefaultMempoolSize     = 8192 // maximum number of pending transactions

	// GenesisFile is the file, inside the data directory, where the genesis of the chain is persisted.
	GenesisFile = "genesis.json"

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

	// AdminToken is the bearer token required by the admin API endpoints.
	// If empty when EnableAPI is called, the admin endpoints are disabled.
	AdminToken string

	blockStore  db.Database
	mempoolDB   db.Database
	height      atomic.Int64
	txsPerBlock int
	closed      atomic.Bool

	// imported is set when the chain state was produced by another node, see ImportChain.
	imported *importRecord

	// wg tracks goroutines started by Start that access databases so that
	// Close can wait for them to finish before releasing those databases.
	wg sync.WaitGroup

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
func NewVocone(dataDir string, keymanager *ethereum.SignKeys) (*Vocone, error) {
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

	// Open the block store database (before height calc so we can check for committed blocks).
	vc.blockStore, err = metadb.New(db.TypePebble, filepath.Join(dataDir, "blockstore"))
	if err != nil {
		return nil, fmt.Errorf("could not open blockstore: %w", err)
	}

	// Recover height from persisted state and block store.
	if err := vc.loadImportRecord(); err != nil {
		return nil, err
	}
	if err := vc.recoverHeight(); err != nil {
		return nil, err
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

	// Load the persisted genesis, if the chain was already initialized.
	if err := vc.loadGenesis(); err != nil {
		return nil, err
	}

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
	)
	if vc.AdminToken != "" {
		uAPI.Endpoint.SetAdminToken(vc.AdminToken)
	} else {
		log.Infow("admin token not set, admin API endpoints are disabled")
	}

	return uAPI, uAPI.EnableHandlers(
		api.ElectionHandler,
		api.VoteHandler,
		api.ChainHandler,
		api.WalletHandler,
		api.AccountHandler,
	)
}

// Start runs the block production loop. It blocks until ctx is cancelled.
// Returns an error if initialization fails; block production errors are logged and skipped.
func (vc *Vocone) Start(ctx context.Context) error {
	vc.lastBlockTime = time.Now()
	go vochainPrintInfo(ctx, 10*time.Second, vc.Stats)

	if !vc.Initialized() {
		if err := vc.InitChain(vc.App.ChainID(), DefaultGenesisAppState(vc.txsPerBlock)); err != nil {
			return err
		}
	}
	// Reveal the encryption keys of ended elections that could not be revealed
	// while the node was stopped.
	if vc.KeyKeeper != nil {
		vc.wg.Add(1)
		go func() {
			defer vc.wg.Done()
			vc.KeyKeeper.RevealUnpublished()
		}()
	}
	// Rebuild the live results of the open elections and repair the indexed blocks.
	if vc.Indexer != nil {
		vc.wg.Add(1)
		go func() {
			defer vc.wg.Done()
			vc.Indexer.AfterSyncBootstrap(false)
		}()
	}
	// Load the election price calculator from the current state (tx costs and capacity).
	if err := vc.SetElectionPrice(); err != nil {
		return fmt.Errorf("could not set election price calculator: %w", err)
	}

	for {
		select {
		case <-ctx.Done():
			log.Infow("stopping block production", "height", vc.height.Load())
			return nil
		default:
		}
		// Close() may run before the context is canceled (the test harness
		// defers Close after cancel, so it executes first); producing against
		// released databases would panic.
		if vc.closed.Load() {
			log.Infow("stopping block production, vocone closed", "height", vc.height.Load())
			return nil
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

	// Checked under vcMtx: Close() sets the flag before taking this lock, so
	// either it waits for an in-flight block to finish, or we observe the flag
	// here and never touch the already-released databases. The pre-lock check
	// in Start is advisory only; this one is what makes shutdown race-free.
	if vc.closed.Load() {
		return nil
	}

	startTime := time.Now()
	height := vc.height.Load()

	txs, includedKeys, discardedKeys := vc.prepareBlock()

	resp, err := vc.App.ExecuteBlock(txs, uint32(height), startTime)
	if err != nil {
		return fmt.Errorf("execute block at height %d: %w", height, err)
	}

	// Build the block to compute its hash (needed for the indexer and hash index).
	blk := vc.buildBlock(height, startTime, txs, resp.Root)
	blockHash := blk.Hash()

	// Retrieve the previous block hash for chain linkage.
	lastBlockHash := vc.previousBlockHash(height)

	// Persist block metadata and hash→height index.
	if err := vc.storeBlockMeta(height, startTime, int32(len(txs)), resp.Root,
		blockHash, vc.proposerAddress, lastBlockHash, blk.DataHash); err != nil {
		return fmt.Errorf("store block meta at height %d: %w", height, err)
	}

	// Store transactions in the blockstore (before CommitState so they're
	// available if the indexer queries getBlock during commit processing).
	if len(txs) > 0 {
		wTx := vc.blockStore.WriteTx()
		defer wTx.Discard()
		for i, tx := range txs {
			if err := wTx.Set(txKey(height, int32(i)), tx); err != nil {
				return fmt.Errorf("store tx %d in blockstore at height %d: %w", i, height, err)
			}
		}
		if err := wTx.Commit(); err != nil {
			return fmt.Errorf("commit blockstore txs at height %d: %w", height, err)
		}
	}

	if _, err := vc.App.CommitState(); err != nil {
		return fmt.Errorf("commit state at height %d: %w", height, err)
	}

	// Build the set of invalid tx IDs from failed deliverTx responses.
	// Txs that fail delivery are NOT consumed from the mempool so they can be
	// retried in a subsequent block once the relevant account state advances
	// (e.g. a future-nonce tx becomes valid after a lower-nonce tx is mined).
	// This mirrors cometbft's post-commit recheck behaviour.
	invalidTxIDs := make(map[[32]byte]bool, len(resp.InvalidTransactions))
	for _, txID := range resp.InvalidTransactions {
		invalidTxIDs[txID] = true
	}

	// consumedKeys = txs that CheckTx-rejected + txs that delivered successfully.
	// Txs that failed deliverTx keep their mempool slot and are tried again.
	consumedKeys := discardedKeys
	for i, tx := range txs {
		if !invalidTxIDs[blocktypes.Tx(tx).Key()] {
			consumedKeys = append(consumedKeys, includedKeys[i])
		}
	}

	// Only clean up the mempool after a fully successful commit.
	vc.commitMempoolCleanup(consumedKeys)

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
// It signals goroutines to stop, waits for the bootstrap goroutines started
// by Start to finish, then acquires both vcMtx and mempoolMtx to ensure no
// concurrent block production or mempool operation is in progress before
// closing databases.
func (vc *Vocone) Close() error {
	vc.closed.Store(true)
	// Wait for the bootstrap goroutines (RevealUnpublished, AfterSyncBootstrap)
	// to finish before releasing the databases they access.
	vc.wg.Wait()
	vc.vcMtx.Lock()
	defer vc.vcMtx.Unlock()
	vc.mempoolMtx.Lock()
	defer vc.mempoolMtx.Unlock()

	var errs []error
	if vc.Indexer != nil {
		if err := vc.Indexer.Close(); err != nil {
			errs = append(errs, fmt.Errorf("close indexer: %w", err))
		}
	}
	if vc.Stats != nil {
		vc.Stats.Close()
	}
	if vc.App != nil && vc.App.State != nil {
		if err := vc.App.State.Close(); err != nil {
			errs = append(errs, fmt.Errorf("close state: %w", err))
		}
	}
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
	vc.App.SetFnIsSynchronizing(func() bool { return false })
	vc.App.SetFnBlockStoreRange(vc.blockStoreRange)
	vc.App.SetFnSendTx(vc.addTx)
	vc.App.SetFnGetTx(vc.getTx)
	vc.App.SetFnGetBlockByHeight(vc.getBlock)
	vc.App.SetFnGetBlockByHash(vc.getBlockByHash)
	vc.App.SetFnGetTxHash(vc.getTxWithHash)
	vc.App.SetFnMempoolSize(vc.mempoolSize)
	vc.App.SetFnMempoolPrune(vc.mempoolPrune)
}

// Initialized reports whether the chain has a genesis, either persisted in the data
// directory or implied by already produced blocks.
func (vc *Vocone) Initialized() bool {
	return vc.App.Genesis().ChainID != "" || vc.height.Load() > 0
}

// InitChain initializes a new chain with the given chainID and genesis app state,
// and persists the genesis in the data directory so restarts keep the same chain.
// If blocks were already produced (with no persisted genesis), the state is left
// untouched and only the genesis identity is persisted.
func (vc *Vocone) InitChain(chainID string, appState *genesis.AppState) error {
	vc.vcMtx.Lock()
	defer vc.vcMtx.Unlock()
	if vc.App.Genesis().ChainID != "" {
		return fmt.Errorf("chain already initialized with chainID %q", vc.App.Genesis().ChainID)
	}
	vc.App.SetChainID(chainID)
	genesisTime := time.Now().UTC()
	doc := &genesis.Doc{
		GenesisTime: genesisTime,
		ChainID:     chainID,
	}
	if vc.height.Load() > 0 {
		log.Warnw("persisting genesis of an existing chain", "chainID", chainID, "height", vc.height.Load())
		if meta, err := vc.loadBlockMeta(0); err == nil {
			doc.GenesisTime = time.Unix(0, meta.Timestamp).UTC()
		}
	} else {
		log.Infow("initializing new blockchain", "chainID", chainID)
		if _, err := vc.App.InitChain(appState, genesisTime); err != nil {
			return fmt.Errorf("could not init chain: %w", err)
		}
		appStateBytes, err := json.Marshal(appState)
		if err != nil {
			return fmt.Errorf("could not marshal genesis app state: %w", err)
		}
		doc.AppState = appStateBytes
	}
	if err := doc.SaveAs(filepath.Join(vc.Config.DataDir, GenesisFile)); err != nil {
		return fmt.Errorf("could not save genesis: %w", err)
	}
	vc.App.SetGenesis(doc)
	return nil
}

// loadGenesis loads the persisted genesis, if any, and sets the chainID from it.
func (vc *Vocone) loadGenesis() error {
	path := filepath.Join(vc.Config.DataDir, GenesisFile)
	if _, err := os.Stat(path); errors.Is(err, os.ErrNotExist) {
		return nil
	}
	doc, err := genesis.LoadFromFile(path)
	if err != nil {
		return fmt.Errorf("could not load genesis: %w", err)
	}
	vc.App.SetChainID(doc.ChainID)
	vc.App.SetGenesis(doc)
	log.Infow("loaded genesis", "chainID", doc.ChainID, "genesisTime", doc.GenesisTime)
	return nil
}

// DefaultGenesisAppState returns the genesis app state used when no other is provided.
func DefaultGenesisAppState(txsPerBlock int) *genesis.AppState {
	return &genesis.AppState{
		MaxElectionSize: 1000000,
		NetworkCapacity: uint64(txsPerBlock),
		TxCost:          defaultTxCosts(),
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
	_, err := vc.commit()
	return err
}

// MintBalance adds amount to the balance of an account, creating it if it does not
// exist, and commits the state. It is used to fund accounts explicitly, for instance
// when a chain is imported (see ImportChain), which applies no genesis.
func (vc *Vocone) MintBalance(addr common.Address, amount uint64) error {
	vc.vcMtx.Lock()
	defer vc.vcMtx.Unlock()
	acc, err := vc.App.State.GetAccount(addr, false)
	if err != nil {
		return fmt.Errorf("cannot get account %s: %w", addr.Hex(), err)
	}
	if acc == nil {
		if err := vc.App.State.CreateAccount(addr, "", nil, amount); err != nil {
			return fmt.Errorf("cannot create account %s: %w", addr.Hex(), err)
		}
	} else if err := vc.App.State.InitChainMintBalance(addr, amount); err != nil {
		return fmt.Errorf("cannot mint balance for %s: %w", addr.Hex(), err)
	}
	_, err = vc.commit()
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

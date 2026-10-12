package vocone

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	// sqlite driver used to inspect the imported indexer database
	_ "github.com/mattn/go-sqlite3"

	"go.vocdoni.io/dvote/db"
	"go.vocdoni.io/dvote/db/metadb"
	"go.vocdoni.io/dvote/log"
	"go.vocdoni.io/dvote/types"
	"go.vocdoni.io/dvote/vochain"
	"go.vocdoni.io/dvote/vochain/genesis"
	vstate "go.vocdoni.io/dvote/vochain/state"
)

// keyImport is the block store key holding the importRecord of a chain whose state
// was produced by another node (see ImportChain).
const keyImport = "import"

// importRecord describes the point at which an existing chain was imported. Blocks
// below Base are not in the vocone block store; their indexed data (blocks,
// transactions, votes) stays available through the indexer.
type importRecord struct {
	// Base is the first height produced by vocone, the imported state height plus one.
	Base int64 `json:"base"`
	// LastBlockHash is the hash of the last imported block (Base-1), taken from the
	// indexer, used to link the first vocone block to the imported chain.
	LastBlockHash types.HexBytes `json:"lastBlockHash,omitempty"`
	// ChainID is the chainID of the imported chain.
	ChainID string `json:"chainId"`
	// ImportedAt is the time the import was performed.
	ImportedAt time.Time `json:"importedAt"`
}

// ImportResult summarizes a completed ImportChain.
type ImportResult struct {
	ChainID           string
	StateHeight       int64
	FirstHeight       int64
	IndexerHeight     int64
	IndexerBlockCount int64
}

// cometGenesis holds the fields read from a cometbft genesis.json. initial_height
// is encoded by cometbft as a string, hence the custom decoding.
type cometGenesis struct {
	GenesisTime   time.Time       `json:"genesis_time"`
	ChainID       string          `json:"chain_id"`
	InitialHeight json.RawMessage `json:"initial_height"`
	AppState      json.RawMessage `json:"app_state,omitempty"`
}

// ParseImportGenesis decodes the genesis.json of the chain to import. It accepts both
// the cometbft format (initial_height as a string) and the vocone format.
func ParseImportGenesis(data []byte) (*genesis.Doc, error) {
	var cg cometGenesis
	if err := json.Unmarshal(data, &cg); err != nil {
		return nil, fmt.Errorf("cannot decode genesis: %w", err)
	}
	doc := &genesis.Doc{
		GenesisTime: cg.GenesisTime,
		ChainID:     cg.ChainID,
		AppState:    cg.AppState,
	}
	if len(cg.InitialHeight) > 0 && string(cg.InitialHeight) != "null" {
		raw := strings.Trim(string(cg.InitialHeight), `"`)
		if raw != "" {
			h, err := strconv.ParseInt(raw, 10, 64)
			if err != nil {
				return nil, fmt.Errorf("invalid initial_height %q: %w", raw, err)
			}
			doc.InitialHeight = h
		}
	}
	if err := doc.Validate(); err != nil {
		return nil, err
	}
	return doc, nil
}

// ImportChain prepares dataDir so that vocone continues an existing chain whose state
// (vcstate) and, optionally, indexer database (indexer/db.sqlite) were produced by
// another node and copied into dataDir. It must run before NewVocone.
//
// The genesis is not applied: the chainID, genesis time and initial height are taken
// from doc and persisted as the vocone genesis, so restarts resume the imported chain.
// Block production continues at the state height plus one; that height is recorded
// as the base of the vocone block store. No account is created or funded.
//
// ImportChain fails if dataDir already holds a vocone chain (persisted genesis or
// produced blocks), if the state is empty, or if the indexer is ahead of the state.
func ImportChain(dataDir string, doc *genesis.Doc) (*ImportResult, error) {
	if doc == nil {
		return nil, fmt.Errorf("missing genesis to import")
	}
	if err := doc.Validate(); err != nil {
		return nil, err
	}
	if _, err := os.Stat(filepath.Join(dataDir, GenesisFile)); err == nil {
		return nil, fmt.Errorf("data directory %s already holds a vocone chain (%s exists)", dataDir, GenesisFile)
	} else if !errors.Is(err, os.ErrNotExist) {
		return nil, err
	}
	if _, err := os.Stat(filepath.Join(dataDir, vochain.StateDataDir)); err != nil {
		return nil, fmt.Errorf("no state to import in %s: %w", dataDir, err)
	}

	state, err := vstate.New(db.TypePebble, filepath.Join(dataDir, vochain.StateDataDir))
	if err != nil {
		return nil, fmt.Errorf("cannot open state: %w", err)
	}
	lastHeight, err := state.LastHeight()
	if closeErr := state.Close(); closeErr != nil && err == nil {
		err = closeErr
	}
	if err != nil {
		return nil, fmt.Errorf("cannot read state height: %w", err)
	}
	if lastHeight == 0 {
		return nil, fmt.Errorf("state in %s is empty, nothing to import", dataDir)
	}
	res := &ImportResult{
		ChainID:     doc.ChainID,
		StateHeight: int64(lastHeight),
		FirstHeight: int64(lastHeight) + 1,
	}

	rec := importRecord{
		Base:       res.FirstHeight,
		ChainID:    doc.ChainID,
		ImportedAt: time.Now().UTC(),
	}
	indexerDB := filepath.Join(dataDir, "indexer", "db.sqlite")
	if _, err := os.Stat(indexerDB); err == nil {
		info, err := inspectIndexer(indexerDB, res.StateHeight)
		if err != nil {
			return nil, err
		}
		res.IndexerHeight, res.IndexerBlockCount = info.maxHeight, info.blockCount
		if info.maxHeight > res.StateHeight {
			return nil, fmt.Errorf("indexer is ahead of the state (indexer height %d, state height %d): "+
				"copy the indexer before the state, or from a stopped node", info.maxHeight, res.StateHeight)
		}
		if info.maxHeight < res.StateHeight {
			log.Warnw("indexer is behind the state, the missing blocks will not be indexed",
				"indexerHeight", info.maxHeight, "stateHeight", res.StateHeight)
		}
		if info.chainID != "" && info.chainID != doc.ChainID {
			return nil, fmt.Errorf("indexer chainID %q differs from genesis chainID %q", info.chainID, doc.ChainID)
		}
		rec.LastBlockHash = info.lastHash
	} else if errors.Is(err, os.ErrNotExist) {
		log.Warnw("no indexer database to import, the chain history will not be served", "path", indexerDB)
	} else {
		return nil, err
	}

	blockStore, err := metadb.New(db.TypePebble, filepath.Join(dataDir, "blockstore"))
	if err != nil {
		return nil, fmt.Errorf("could not open blockstore: %w", err)
	}
	defer func() {
		if err := blockStore.Close(); err != nil {
			log.Warnw("could not close blockstore", "err", err)
		}
	}()
	if err := checkEmptyBlockStore(blockStore); err != nil {
		return nil, err
	}
	data, err := json.Marshal(&rec)
	if err != nil {
		return nil, err
	}
	wTx := blockStore.WriteTx()
	defer wTx.Discard()
	if err := wTx.Set([]byte(keyImport), data); err != nil {
		return nil, err
	}
	if err := wTx.Commit(); err != nil {
		return nil, fmt.Errorf("could not store import record: %w", err)
	}

	// The genesis is persisted last: its presence marks the chain as initialized.
	if err := doc.SaveAs(filepath.Join(dataDir, GenesisFile)); err != nil {
		return nil, fmt.Errorf("could not save genesis: %w", err)
	}
	log.Infow("imported chain", "chainID", doc.ChainID, "stateHeight", res.StateHeight,
		"firstHeight", res.FirstHeight, "indexerHeight", res.IndexerHeight)
	return res, nil
}

// checkEmptyBlockStore fails if the block store already holds blocks or an import record.
func checkEmptyBlockStore(blockStore db.Database) error {
	if _, err := blockStore.Get([]byte(keyImport)); err == nil {
		return fmt.Errorf("block store already holds an import record")
	} else if !errors.Is(err, db.ErrKeyNotFound) {
		return err
	}
	found := false
	if err := blockStore.Iterate([]byte(prefixMeta), func(_, _ []byte) bool {
		found = true
		return false
	}); err != nil {
		return err
	}
	if found {
		return fmt.Errorf("block store already holds blocks")
	}
	return nil
}

type indexerInfo struct {
	maxHeight  int64
	blockCount int64
	chainID    string
	lastHash   []byte
}

// inspectIndexer reads the indexed block range of the indexer database at path, and
// the chainID and hash of the block at stateHeight, if indexed.
func inspectIndexer(path string, stateHeight int64) (*indexerInfo, error) {
	sqlDB, err := sql.Open("sqlite3", fmt.Sprintf("file:%s?mode=ro&_journal_mode=wal", path))
	if err != nil {
		return nil, fmt.Errorf("cannot open indexer database: %w", err)
	}
	defer func() {
		if err := sqlDB.Close(); err != nil {
			log.Warnw("could not close indexer database", "err", err)
		}
	}()
	info := &indexerInfo{}
	var maxHeight sql.NullInt64
	if err := sqlDB.QueryRow("SELECT MAX(height), COUNT(*) FROM blocks").Scan(&maxHeight, &info.blockCount); err != nil {
		return nil, fmt.Errorf("cannot read indexed blocks: %w", err)
	}
	info.maxHeight = maxHeight.Int64
	err = sqlDB.QueryRow("SELECT chain_id, hash FROM blocks WHERE height = ?", stateHeight).Scan(&info.chainID, &info.lastHash)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return nil, fmt.Errorf("cannot read indexed block %d: %w", stateHeight, err)
	}
	return info, nil
}

// loadImportRecord loads the import record from the block store, if any.
func (vc *Vocone) loadImportRecord() error {
	data, err := vc.blockStore.Get([]byte(keyImport))
	if errors.Is(err, db.ErrKeyNotFound) {
		return nil
	}
	if err != nil {
		return err
	}
	rec := &importRecord{}
	if err := json.Unmarshal(data, rec); err != nil {
		return fmt.Errorf("cannot decode import record: %w", err)
	}
	vc.imported = rec
	return nil
}

// recoverHeight sets the height of the next block to produce from the persisted
// state and block store.
func (vc *Vocone) recoverHeight() error {
	// LastHeight returns the last committed height. If that block was already
	// fully committed (metadata exists in blockstore), we must start at the
	// next height to avoid re-executing an already-committed block.
	lastHeight, err := vc.App.State.LastHeight()
	if err != nil {
		return fmt.Errorf("could not get last height: %w", err)
	}
	nextHeight := int64(lastHeight)
	if _, err := vc.loadBlockMeta(nextHeight); err == nil {
		// Block at lastHeight was already committed, advance to the next one.
		nextHeight++
	}
	// On an imported chain the blocks below the import base were committed by the
	// node that produced the state and are not in the block store.
	if vc.imported != nil && nextHeight < vc.imported.Base {
		nextHeight = vc.imported.Base
	}
	vc.height.Store(nextHeight)
	return nil
}

// ImportBase returns the first height produced by vocone on an imported chain, or
// zero if the chain was not imported.
func (vc *Vocone) ImportBase() int64 {
	if vc.imported == nil {
		return 0
	}
	return vc.imported.Base
}

// previousBlockHash returns the hash of the block before height, from the block
// store or, for the first block after an import, from the import record.
func (vc *Vocone) previousBlockHash(height int64) []byte {
	if height <= 0 {
		return nil
	}
	if prevMeta, err := vc.loadBlockMeta(height - 1); err == nil && len(prevMeta.Hash) > 0 {
		return prevMeta.Hash
	}
	if vc.imported != nil && height == vc.imported.Base {
		return vc.imported.LastBlockHash
	}
	return nil
}

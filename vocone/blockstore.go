package vocone

import (
	"encoding/binary"
	"encoding/json"
	"errors"
	"time"

	"go.vocdoni.io/dvote/crypto/ethereum"
	"go.vocdoni.io/dvote/db"
	"go.vocdoni.io/dvote/log"
	"go.vocdoni.io/dvote/vochain/blocktypes"
	"go.vocdoni.io/proto/build/go/models"
	"google.golang.org/protobuf/proto"
)

// blockMeta holds persisted metadata for each block.
type blockMeta struct {
	Timestamp       int64  `json:"t"`
	TxCount         int32  `json:"n"`
	StateRoot       []byte `json:"r,omitempty"`
	Hash            []byte `json:"h,omitempty"`
	ProposerAddress []byte `json:"p,omitempty"`
	LastBlockHash   []byte `json:"l,omitempty"`
	DataHash        []byte `json:"d,omitempty"`
}

// storeBlockMeta persists block metadata and a hash→height reverse index.
func (vc *Vocone) storeBlockMeta(height int64, timestamp time.Time, txCount int32,
	stateRoot, blockHash, proposerAddr, lastBlockHash, dataHash []byte,
) error {
	meta := blockMeta{
		Timestamp:       timestamp.UnixNano(),
		TxCount:         txCount,
		StateRoot:       stateRoot,
		Hash:            blockHash,
		ProposerAddress: proposerAddr,
		LastBlockHash:   lastBlockHash,
		DataHash:        dataHash,
	}
	data, err := json.Marshal(&meta)
	if err != nil {
		return err
	}
	wTx := vc.blockStore.WriteTx()
	defer wTx.Discard()
	if prevData, err := vc.blockStore.Get(metaKey(height)); err == nil {
		var prev blockMeta
		if jsonErr := json.Unmarshal(prevData, &prev); jsonErr == nil && len(prev.Hash) > 0 {
			if len(blockHash) == 0 || string(prev.Hash) != string(blockHash) {
				if err := wTx.Delete(blockHashKey(prev.Hash)); err != nil {
					return err
				}
			}
		}
	} else if !errors.Is(err, db.ErrKeyNotFound) {
		return err
	}
	if err := wTx.Set(metaKey(height), data); err != nil {
		return err
	}
	// Store hash→height reverse index for GetBlockByHash lookups.
	if len(blockHash) > 0 {
		heightBytes := make([]byte, 8)
		binary.BigEndian.PutUint64(heightBytes, uint64(height))
		if err := wTx.Set(blockHashKey(blockHash), heightBytes); err != nil {
			return err
		}
	}
	return wTx.Commit()
}

// loadBlockMeta reads block metadata from the store.
func (vc *Vocone) loadBlockMeta(height int64) (*blockMeta, error) {
	data, err := vc.blockStore.Get(metaKey(height))
	if err != nil {
		return nil, err
	}
	var meta blockMeta
	if err := json.Unmarshal(data, &meta); err != nil {
		return nil, err
	}
	return &meta, nil
}

// buildBlock constructs the block produced at the given height, linked to the
// previous block hash if available.
func (vc *Vocone) buildBlock(height int64, timestamp time.Time, txs [][]byte, appHash []byte) *blocktypes.Block {
	blk := &blocktypes.Block{
		Header: blocktypes.Header{
			ChainID:         vc.App.ChainID(),
			Height:          height,
			Time:            timestamp,
			ProposerAddress: vc.proposerAddress,
			AppHash:         appHash,
		},
	}
	if prevHash := vc.previousBlockHash(height); len(prevHash) > 0 {
		blk.LastBlockID = blocktypes.BlockID{Hash: prevHash}
	}
	blk.Txs = make([]blocktypes.Tx, len(txs))
	for i, tx := range txs {
		blk.Txs[i] = tx
	}
	blk.DataHash = blk.Data.Hash()
	return blk
}

// getBlock reconstructs a block from the persistent store.
// Returns nil if the block is not available.
func (vc *Vocone) getBlock(height int64) *blocktypes.Block {
	if vc.closed.Load() {
		return nil
	}
	meta, err := vc.loadBlockMeta(height)
	if err != nil {
		return nil
	}

	blk := &blocktypes.Block{
		Header: blocktypes.Header{
			ChainID:         vc.App.ChainID(),
			Height:          height,
			Time:            time.Unix(0, meta.Timestamp),
			ProposerAddress: meta.ProposerAddress,
			AppHash:         meta.StateRoot,
			DataHash:        meta.DataHash,
			LastBlockID:     blocktypes.BlockID{Hash: meta.LastBlockHash},
		},
	}

	// Read exactly txCount transactions
	for i := int32(0); i < meta.TxCount; i++ {
		txData, err := vc.blockStore.Get(txKey(height, i))
		if err != nil {
			log.Warnw("missing tx in block store",
				"height", height, "index", i, "err", err)
			break
		}
		blk.Txs = append(blk.Txs, txData)
	}
	return blk
}

// blockStoreRange returns the lowest and highest heights of the committed blocks.
// On an imported chain the lowest height is the import base: older blocks are only
// available through the indexer.
func (vc *Vocone) blockStoreRange() (base, height int64) {
	return vc.ImportBase(), vc.height.Load() - 1
}

// getBlockByHash looks up a block by its hash using the reverse index.
func (vc *Vocone) getBlockByHash(hash []byte) *blocktypes.Block {
	if vc.closed.Load() {
		return nil
	}
	heightBytes, err := vc.blockStore.Get(blockHashKey(hash))
	if err != nil {
		return nil
	}
	if len(heightBytes) != 8 {
		return nil
	}
	height := int64(binary.BigEndian.Uint64(heightBytes))
	return vc.getBlock(height)
}

// getTx retrieves a single transaction from the block store.
func (vc *Vocone) getTx(height uint32, txIndex int32) (*models.SignedTx, error) {
	txData, err := vc.blockStore.Get(txKey(int64(height), txIndex))
	if err != nil {
		return nil, err
	}
	stx := &models.SignedTx{}
	return stx, proto.Unmarshal(txData, stx)
}

// getTxWithHash retrieves a transaction and its hash from the block store.
func (vc *Vocone) getTxWithHash(height uint32, txIndex int32) (*models.SignedTx, []byte, error) {
	txData, err := vc.blockStore.Get(txKey(int64(height), txIndex))
	if err != nil {
		return nil, nil, err
	}
	stx := &models.SignedTx{}
	return stx, ethereum.HashRaw(txData), proto.Unmarshal(txData, stx)
}

// txKey builds the db key for a transaction: "tx/" + height (8 bytes BE) + "/" + txIndex (4 bytes BE).
func txKey(height int64, txIndex int32) []byte {
	key := make([]byte, len(prefixTx)+8+1+4)
	copy(key, prefixTx)
	binary.BigEndian.PutUint64(key[len(prefixTx):], uint64(height))
	key[len(prefixTx)+8] = '/'
	binary.BigEndian.PutUint32(key[len(prefixTx)+9:], uint32(txIndex))
	return key
}

// metaKey builds the db key for block metadata: "meta/" + height (8 bytes BE).
func metaKey(height int64) []byte {
	key := make([]byte, len(prefixMeta)+8)
	copy(key, prefixMeta)
	binary.BigEndian.PutUint64(key[len(prefixMeta):], uint64(height))
	return key
}

// blockHashKey builds the db key for the hash→height reverse index: "blockhash/" + hash.
func blockHashKey(hash []byte) []byte {
	key := make([]byte, len(prefixBlockHash)+len(hash))
	copy(key, prefixBlockHash)
	copy(key[len(prefixBlockHash):], hash)
	return key
}

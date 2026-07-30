package indexer

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"

	"go.vocdoni.io/dvote/crypto/ethereum"
	"go.vocdoni.io/dvote/log"
	"go.vocdoni.io/dvote/types"
	indexerdb "go.vocdoni.io/dvote/vochain/indexer/db"
	"go.vocdoni.io/dvote/vochain/indexer/indexertypes"
	"go.vocdoni.io/dvote/vochain/transaction/vochaintx"
	"go.vocdoni.io/proto/build/go/models"
	"google.golang.org/protobuf/proto"
)

// ErrTransactionNotFound is returned if the transaction is not found.
var ErrTransactionNotFound = fmt.Errorf("transaction not found")

// CountTotalTransactions returns the number of transactions indexed
func (idx *Indexer) CountTotalTransactions() (uint64, error) {
	count, err := idx.readOnlyQuery.CountTransactions(context.TODO())
	return uint64(count), err
}

// CountTransactionsByType returns the number of indexed transactions of each type,
// keyed by the transaction type as reported by SearchTransactions. Types with no
// transaction indexed are absent from the map rather than reported as zero.
func (idx *Indexer) CountTransactionsByType() (map[string]uint64, error) {
	rows, err := idx.readOnlyQuery.CountTransactionsByType(context.TODO())
	if err != nil {
		return nil, err
	}
	counts := make(map[string]uint64, len(rows))
	for _, row := range rows {
		counts[row.Type] = uint64(row.Count)
	}
	return counts, nil
}

// CountTransactionsByHeight returns the number of transactions indexed for a given height
func (idx *Indexer) CountTransactionsByHeight(height int64) (int64, error) {
	return idx.readOnlyQuery.CountTransactionsByHeight(context.TODO(), height)
}

// GetTxMetadataByHash fetches the tx metadata for the given tx hash
func (idx *Indexer) GetTxMetadataByHash(hash types.HexBytes) (*indexertypes.TransactionMetadata, error) {
	sqlTxRef, err := idx.readOnlyQuery.GetTransactionByHash(context.TODO(), hash)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrTransactionNotFound
		}
		return nil, fmt.Errorf("tx hash %x not found: %v", hash, err)
	}
	return indexertypes.TransactionMetadataFromDB(&sqlTxRef), nil
}

// GetTransactionByHash fetches the full tx for the given tx hash
func (idx *Indexer) GetTransactionByHash(hash types.HexBytes) (*indexertypes.Transaction, error) {
	sqlTxRef, err := idx.readOnlyQuery.GetTransactionByHash(context.TODO(), hash)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrTransactionNotFound
		}
		return nil, fmt.Errorf("tx hash %x not found: %v", hash, err)
	}
	return indexertypes.TransactionFromDB(&sqlTxRef), nil
}

// GetTransactionByHeightAndIndex fetches the full tx for the given tx height and block tx index
func (idx *Indexer) GetTransactionByHeightAndIndex(blockHeight, blockIndex int64) (*indexertypes.Transaction, error) {
	sqlTxRef, err := idx.readOnlyQuery.GetTransactionByHeightAndIndex(context.TODO(), indexerdb.GetTransactionByHeightAndIndexParams{
		BlockHeight: blockHeight,
		BlockIndex:  blockIndex,
	})
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrTransactionNotFound
		}
		return nil, fmt.Errorf("tx at block %d and index %d not found: %v", blockHeight, blockIndex, err)
	}
	return indexertypes.TransactionFromDB(&sqlTxRef), nil
}

// SearchTransactions returns the list of transactions indexed.
// blockHeight, hash, txType, txSubtype and txSigner are optional, if declared as zero-value will be ignored.
// hash matches substrings.
// The first one returned is the newest, so they are in descending order.
func (idx *Indexer) SearchTransactions(limit, offset int, blockHeight uint64, txHash, txType, txSubtype, txSigner string) ([]*indexertypes.TransactionMetadata, uint64, error) {
	if offset < 0 {
		return nil, 0, fmt.Errorf("invalid value: offset cannot be %d", offset)
	}
	if limit <= 0 {
		return nil, 0, fmt.Errorf("invalid value: limit cannot be %d", limit)
	}
	results, err := idx.readOnlyQuery.SearchTransactions(context.TODO(), indexerdb.SearchTransactionsParams{
		Limit:       int64(limit),
		Offset:      int64(offset),
		HashSubstr:  txHash,
		BlockHeight: blockHeight,
		TxType:      txType,
		TxSubtype:   txSubtype,
		TxSigner:    txSigner,
	})
	if err != nil {
		return nil, 0, err
	}
	list := []*indexertypes.TransactionMetadata{}
	for _, row := range results {
		list = append(list, indexertypes.TransactionMetadataFromDBRow(&row))
	}
	if len(results) == 0 {
		return list, 0, nil
	}
	return list, uint64(results[0].TotalCount), nil
}

func (idx *Indexer) OnNewTx(tx *vochaintx.Tx, blockHeight uint32, txIndex int32) {
	idx.blockMu.Lock()
	defer idx.blockMu.Unlock()

	idx.indexTx(tx, blockHeight, txIndex)
}

func (idx *Indexer) indexTx(tx *vochaintx.Tx, blockHeight uint32, txIndex int32) {
	rawtx, err := proto.Marshal(tx.Tx)
	if err != nil {
		log.Errorw(err, "indexer cannot marshal transaction")
		return
	}

	signer := []byte{}
	if len(tx.Signature) > 0 { // not all txs are signed, for example zk ones
		addr, err := ethereum.AddrFromSignature(tx.SignedBody, tx.Signature)
		if err != nil {
			log.Errorw(err, "indexer cannot recover signer from signature")
			return
		}
		signer = addr.Bytes()
	}

	queries := idx.blockTxQueries()
	if _, err := queries.CreateTransaction(context.TODO(), indexerdb.CreateTransactionParams{
		Hash:        tx.TxID[:],
		BlockHeight: int64(blockHeight),
		BlockIndex:  int64(txIndex),
		Type:        tx.TxModelType,
		Subtype:     strings.ToLower(tx.TxSubtype()),
		RawTx:       rawtx,
		Signature:   nonNullBytes(tx.Signature),
		Signer:      nonNullBytes(signer),
	}); err != nil {
		log.Errorw(err, "cannot index transaction")
	}

	idx.indexKeyRevealUnsafe(queries, tx, blockHeight)
}

// Transaction type and subtype, as stored by indexTx, of the transaction which
// reveals the encryption keys of an election.
const (
	txTypeAdmin             = "admin"
	txSubtypeRevealProcKeys = "reveal_process_keys"
)

// indexKeyRevealUnsafe records on the process where its encryption keys were
// revealed, when the given transaction is the one revealing them. Doing it here
// rather than from the state event listener is what makes the transaction hash
// available, and makes a block reindex repair the columns for free.
//
// Assumes idx.blockMu is locked, as its caller does.
func (idx *Indexer) indexKeyRevealUnsafe(queries *indexerdb.Queries, tx *vochaintx.Tx, blockHeight uint32) {
	if tx.TxModelType != txTypeAdmin || strings.ToLower(tx.TxSubtype()) != txSubtypeRevealProcKeys {
		return
	}
	pid := tx.Tx.GetAdmin().GetProcessId()
	if len(pid) == 0 {
		log.Warnw("reveal process keys transaction with no process id", "txHash", tx.TxID[:])
		return
	}
	if _, err := queries.SetProcessKeyReveal(context.TODO(), indexerdb.SetProcessKeyRevealParams{
		ID:              pid,
		KeyRevealHeight: int64(blockHeight),
		KeyRevealTxHash: tx.TxID[:],
	}); err != nil {
		log.Errorw(err, "cannot index process key reveal")
	}
}

// bootstrapKeyReveals fills the key reveal columns of the elections whose keys
// were revealed before those columns existed, by re-reading the stored body of
// the transactions that revealed them.
//
// It repairs only what the transaction table can answer for: a transaction
// indexed before its raw body was stored has nothing to parse, and one never
// indexed at all is invisible here. Those are recovered instead by ReindexBlocks,
// which re-runs indexTx over the blockstore, and so only as far back as the
// blockstore reaches.
func (idx *Indexer) bootstrapKeyReveals() {
	rows, err := idx.readOnlyQuery.ListTransactionsByTypeAndSubtype(context.TODO(),
		indexerdb.ListTransactionsByTypeAndSubtypeParams{
			TxType:    txTypeAdmin,
			TxSubtype: txSubtypeRevealProcKeys,
		})
	if err != nil {
		log.Errorw(err, "could not list key reveal transactions")
		return
	}
	if len(rows) == 0 {
		return
	}
	queries := indexerdb.New(idx.readWriteDB)
	filled := 0
	for _, row := range rows {
		if len(row.RawTx) == 0 {
			continue // indexed before the raw body was stored
		}
		protoTx := new(models.Tx)
		if err := proto.Unmarshal(row.RawTx, protoTx); err != nil {
			log.Warnw("cannot unmarshal stored key reveal transaction", "txHash", row.Hash, "err", err.Error())
			continue
		}
		pid := protoTx.GetAdmin().GetProcessId()
		if len(pid) == 0 {
			continue
		}
		if _, err := queries.SetProcessKeyReveal(context.TODO(), indexerdb.SetProcessKeyRevealParams{
			ID:              pid,
			KeyRevealHeight: row.BlockHeight,
			KeyRevealTxHash: row.Hash,
		}); err != nil {
			log.Errorw(err, "could not backfill process key reveal")
			continue
		}
		filled++
	}
	log.Infow("indexer key reveal bootstrap finished", "transactions", len(rows), "updated", filled)
}

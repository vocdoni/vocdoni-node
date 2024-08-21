package indexer

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"go.vocdoni.io/dvote/log"
	"go.vocdoni.io/dvote/types"
	indexerdb "go.vocdoni.io/dvote/vochain/indexer/db"
	"go.vocdoni.io/dvote/vochain/indexer/indexertypes"
	"go.vocdoni.io/dvote/vochain/transaction/vochaintx"
)

// ErrTransactionNotFound is returned if the transaction is not found.
var ErrTransactionNotFound = fmt.Errorf("transaction not found")

// CountTotalTransactions returns the number of transactions indexed
func (idx *Indexer) CountTotalTransactions() (uint64, error) {
	count, err := idx.readOnlyQuery.CountTransactions(context.TODO())
	return uint64(count), err
}

// GetTxReferenceByBlockHeightAndBlockIndex fetches the txReference for the given tx height and block tx index
func (idx *Indexer) GetTxReferenceByBlockHeightAndBlockIndex(blockHeight, blockIndex int64) (*indexertypes.Transaction, error) {
	sqlTxRef, err := idx.readOnlyQuery.GetTxReferenceByBlockHeightAndBlockIndex(context.TODO(), indexerdb.GetTxReferenceByBlockHeightAndBlockIndexParams{
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

// GetTxHashReference fetches the txReference for the given tx hash
func (idx *Indexer) GetTxHashReference(hash types.HexBytes) (*indexertypes.Transaction, error) {
	sqlTxRef, err := idx.readOnlyQuery.GetTransactionByHash(context.TODO(), hash)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrTransactionNotFound
		}
		return nil, fmt.Errorf("tx hash %x not found: %v", hash, err)
	}
	return indexertypes.TransactionFromDB(&sqlTxRef), nil
}

// SearchTransactions returns the list of transactions indexed.
// height and txType are optional, if declared as zero-value will be ignored.
// The first one returned is the newest, so they are in descending order.
func (idx *Indexer) SearchTransactions(limit, offset int, blockHeight uint64, txType string) ([]*indexertypes.Transaction, uint64, error) {
	if offset < 0 {
		return nil, 0, fmt.Errorf("invalid value: offset cannot be %d", offset)
	}
	if limit <= 0 {
		return nil, 0, fmt.Errorf("invalid value: limit cannot be %d", limit)
	}
	results, err := idx.readOnlyQuery.SearchTransactions(context.TODO(), indexerdb.SearchTransactionsParams{
		Limit:       int64(limit),
		Offset:      int64(offset),
		BlockHeight: blockHeight,
		TxType:      txType,
	})
	if err != nil {
		return nil, 0, err
	}
	list := []*indexertypes.Transaction{}
	for _, row := range results {
		list = append(list, &indexertypes.Transaction{
			Hash:         row.Hash,
			BlockHeight:  uint32(row.BlockHeight),
			TxBlockIndex: int32(row.BlockIndex),
			TxType:       row.Type,
		})
	}
	if len(results) == 0 {
		return list, 0, nil
	}
	return list, uint64(results[0].TotalCount), nil
}

func (idx *Indexer) OnNewTx(tx *vochaintx.Tx, blockHeight uint32, txIndex int32) {
	idx.blockMu.Lock()
	defer idx.blockMu.Unlock()
	queries := idx.blockTxQueries()
	if _, err := queries.CreateTransaction(context.TODO(), indexerdb.CreateTransactionParams{
		Hash:        tx.TxID[:],
		BlockHeight: int64(blockHeight),
		BlockIndex:  int64(txIndex),
		Type:        tx.TxModelType,
	}); err != nil {
		log.Errorw(err, "cannot index new transaction")
	}
}

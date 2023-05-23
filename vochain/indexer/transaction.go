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

var (
	// ErrTransactionNotFound is returned if the transaction is not found.
	ErrTransactionNotFound = fmt.Errorf("transaction not found")
)

// TransactionCount returns the number of transactions indexed
func (idx *Indexer) TransactionCount() (uint64, error) {
	count, err := idx.oneQuery.CountTransactions(context.TODO())
	return uint64(count), err
}

// GetTransaction fetches the txReference for the given tx height
func (idx *Indexer) GetTransaction(id uint64) (*indexertypes.Transaction, error) {
	sqlTxRef, err := idx.oneQuery.GetTransaction(context.TODO(), int64(id))
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrTransactionNotFound
		}
		return nil, fmt.Errorf("tx wiht id %d not found: %v", id, err)
	}
	return indexertypes.TransactionFromDB(&sqlTxRef), nil
}

// GetTxReferenceByBlockHeightAndBlockIndex fetches the txReference for the given tx height and block tx index
func (idx *Indexer) GetTxReferenceByBlockHeightAndBlockIndex(blockHeight, blockIndex int64) (*indexertypes.Transaction, error) {
	sqlTxRef, err := idx.oneQuery.GetTxReferenceByBlockHeightAndBlockIndex(context.TODO(), indexerdb.GetTxReferenceByBlockHeightAndBlockIndexParams{
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
	sqlTxRef, err := idx.oneQuery.GetTransactionByHash(context.TODO(), hash)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrTransactionNotFound
		}
		return nil, fmt.Errorf("tx hash %x not found: %v", hash, err)
	}
	return indexertypes.TransactionFromDB(&sqlTxRef), nil
}

// GetLastTransactions fetches a number of the latest indexed transactions.
// The first one returned is the newest, so they are in descending order.
func (idx *Indexer) GetLastTransactions(limit, offset int32) ([]*indexertypes.Transaction, error) {
	sqlTxRefs, err := idx.oneQuery.GetLastTransactions(context.TODO(), indexerdb.GetLastTransactionsParams{
		Limit:  limit,
		Offset: offset,
	})
	if err != nil || len(sqlTxRefs) == 0 {
		if errors.Is(err, sql.ErrNoRows) || len(sqlTxRefs) == 0 {
			return nil, ErrTransactionNotFound
		}
		return nil, fmt.Errorf("could not get last %d tx refs: %v", limit, err)
	}
	txRefs := make([]*indexertypes.Transaction, len(sqlTxRefs))
	for i, sqlTxRef := range sqlTxRefs {
		txRefs[i] = indexertypes.TransactionFromDB(&sqlTxRef)
	}
	return txRefs, nil
}

func (idx *Indexer) OnNewTx(tx *vochaintx.Tx, blockHeight uint32, txIndex int32) {
	idx.lockPool.Lock()
	defer idx.lockPool.Unlock()
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

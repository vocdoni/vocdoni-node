package indexer

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

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
	queries, ctx, cancel := idx.timeoutQueries()
	defer cancel()
	count, err := queries.CountTxReferences(ctx)
	return uint64(count), err
}

// GetTxReference fetches the txReference for the given tx height
func (idx *Indexer) GetTxReference(height uint64) (*indexertypes.TxReference, error) {
	queries, ctx, cancel := idx.timeoutQueries()
	defer cancel()
	sqlTxRef, err := queries.GetTxReference(ctx, int64(height))
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrTransactionNotFound
		}
		return nil, fmt.Errorf("tx height %d not found: %v", height, err)
	}
	return indexertypes.TxReferenceFromDB(&sqlTxRef), nil
}

// GetTxHashReference fetches the txReference for the given tx hash
func (idx *Indexer) GetTxHashReference(hash types.HexBytes) (*indexertypes.TxReference, error) {
	queries, ctx, cancel := idx.timeoutQueries()
	defer cancel()
	sqlTxRef, err := queries.GetTxReferenceByHash(ctx, hash)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrTransactionNotFound
		}
		return nil, fmt.Errorf("tx hash %x not found: %v", hash, err)
	}
	return indexertypes.TxReferenceFromDB(&sqlTxRef), nil
}

// GetLastTxReferences fetches a number of the latest indexed transactions.
// The first one returned is the newest, so they are in descending order.
func (idx *Indexer) GetLastTxReferences(limit, offset int32) ([]*indexertypes.TxReference, error) {
	queries, ctx, cancel := idx.timeoutQueries()
	defer cancel()
	sqlTxRefs, err := queries.GetLastTxReferences(ctx, indexerdb.GetLastTxReferencesParams{
		Limit:  limit,
		Offset: offset,
	})
	if err != nil || len(sqlTxRefs) == 0 {
		if errors.Is(err, sql.ErrNoRows) || len(sqlTxRefs) == 0 {
			return nil, ErrTransactionNotFound
		}
		return nil, fmt.Errorf("could not get last %d tx refs: %v", limit, err)
	}
	txRefs := make([]*indexertypes.TxReference, len(sqlTxRefs))
	for i, sqlTxRef := range sqlTxRefs {
		txRefs[i] = indexertypes.TxReferenceFromDB(&sqlTxRef)
	}
	return txRefs, nil
}

// OnNewTx stores the transaction reference in the indexer database
func (idx *Indexer) OnNewTx(tx *vochaintx.VochainTx, blockHeight uint32, txIndex int32) {
	if err := idx.indexNewTx(tx, blockHeight, txIndex); err != nil {
		log.Errorw(err, "cannot index new transaction")
	}
}

func (idx *Indexer) indexNewTx(tx *vochaintx.VochainTx, blockHeight uint32, txIndex int32) error {
	idx.lockPool.Lock()
	defer idx.lockPool.Unlock()

	queries := idx.blockTxQueries()
	ctx, cancel := context.WithTimeout(context.TODO(), time.Minute)
	defer cancel()

	if _, err := queries.CreateTxReference(ctx, indexerdb.CreateTxReferenceParams{
		Hash:         tx.TxID[:],
		BlockHeight:  int64(blockHeight),
		TxBlockIndex: int64(txIndex),
		TxType:       tx.TxModelType,
	}); err != nil {
		return err
	}
	return nil
}

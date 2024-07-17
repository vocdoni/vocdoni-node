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
	"google.golang.org/protobuf/proto"
)

// ErrTransactionNotFound is returned if the transaction is not found.
var ErrTransactionNotFound = fmt.Errorf("transaction not found")

// CountTotalTransactions returns the number of transactions indexed
func (idx *Indexer) CountTotalTransactions() (uint64, error) {
	count, err := idx.readOnlyQuery.CountTransactions(context.TODO())
	return uint64(count), err
}

// CountTransactionsByHeight returns the number of transactions indexed for a given height
func (idx *Indexer) CountTransactionsByHeight(height int64) (int64, error) {
	return idx.readOnlyQuery.CountTransactionsByHeight(context.TODO(), height)
}

// GetTransaction fetches the txReference for the given tx height
func (idx *Indexer) GetTransaction(id uint64) (*indexertypes.Transaction, error) {
	sqlTxRef, err := idx.readOnlyQuery.GetTransaction(context.TODO(), int64(id))
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrTransactionNotFound
		}
		return nil, fmt.Errorf("tx with id %d not found: %v", id, err)
	}
	return indexertypes.TransactionFromDB(&sqlTxRef), nil
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
			Index:        uint64(row.ID),
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

// TransactionListByHeight fetches the indexed transactions for a given height.
// The first one returned has TxBlockIndex=0, so they are in ascending order.
func (idx *Indexer) TransactionListByHeight(height, limit, offset int64) ([]*indexertypes.Transaction, uint64, error) {
	results, err := idx.readOnlyQuery.SearchTransactions(context.TODO(), indexerdb.SearchTransactionsParams{
		BlockHeight: height,
		Limit:       limit,
		Offset:      offset,
	})
	if err != nil || len(results) == 0 {
		if errors.Is(err, sql.ErrNoRows) || len(results) == 0 {
			return nil, 0, ErrTransactionNotFound
		}
		return nil, 0, fmt.Errorf("could not get %d txs for height %d: %v", limit, height, err)
	}
	list := make([]*indexertypes.Transaction, len(results))
	for i, row := range results {
		list[i] = indexertypes.TransactionFromDBRow(&row)
	}
	return list, uint64(results[0].TotalCount), nil
}

func (idx *Indexer) OnNewTx(tx *vochaintx.Tx, blockHeight uint32, txIndex int32) {
	idx.blockMu.Lock()
	defer idx.blockMu.Unlock()

	rawtx, err := proto.Marshal(tx.Tx)
	if err != nil {
		log.Errorw(err, "indexer cannot marshal new transaction")
		return
	}

	queries := idx.blockTxQueries()
	if _, err := queries.CreateTransaction(context.TODO(), indexerdb.CreateTransactionParams{
		Hash:        tx.TxID[:],
		BlockHeight: int64(blockHeight),
		BlockIndex:  int64(txIndex),
		Type:        tx.TxModelType,
		RawTx:       rawtx,
	}); err != nil {
		log.Errorw(err, "cannot index new transaction")
	}
}

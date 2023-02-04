package indexer

import (
	"fmt"

	"go.vocdoni.io/dvote/log"
	"go.vocdoni.io/dvote/types"
	indexerdb "go.vocdoni.io/dvote/vochain/indexer/db"
	"go.vocdoni.io/dvote/vochain/indexer/indexertypes"
	"go.vocdoni.io/dvote/vochain/transaction/vochaintx"
)

// TransactionCount returns the number of transactions indexed
func (s *Indexer) TransactionCount() (uint64, error) {
	queries, ctx, cancel := s.timeoutQueries()
	defer cancel()
	count, err := queries.CountTxReferences(ctx)
	return uint64(count), err
}

// GetTxReference fetches the txReference for the given tx height
func (s *Indexer) GetTxReference(height uint64) (*indexertypes.TxReference, error) {
	queries, ctx, cancel := s.timeoutQueries()
	defer cancel()
	sqlTxRef, err := queries.GetTxReference(ctx, int64(height))
	if err != nil {
		return nil, fmt.Errorf("tx height %d not found: %v", height, err)
	}
	return indexertypes.TxReferenceFromDB(&sqlTxRef), nil
}

// GetTxReference fetches the txReference for the given tx hash
func (s *Indexer) GetTxHashReference(hash types.HexBytes) (*indexertypes.TxReference, error) {
	queries, ctx, cancel := s.timeoutQueries()
	defer cancel()
	sqlTxRef, err := queries.GetTxReferenceByHash(ctx, hash)
	if err != nil {
		return nil, fmt.Errorf("tx hash %x not found: %v", hash, err)
	}
	return indexertypes.TxReferenceFromDB(&sqlTxRef), nil
}

// GetLastTxReferences fetches a number of the latest indexed transactions.
// The first one returned is the newest, so they are in descending order.
func (s *Indexer) GetLastTxReferences(limit, offset int32) ([]*indexertypes.TxReference, error) {
	queries, ctx, cancel := s.timeoutQueries()
	defer cancel()
	sqlTxRefs, err := queries.GetLastTxReferences(ctx, indexerdb.GetLastTxReferencesParams{
		Limit:  limit,
		Offset: offset,
	})
	if err != nil {
		return nil, fmt.Errorf("could not get last %d tx refs: %v", limit, err)
	}
	txRefs := make([]*indexertypes.TxReference, len(sqlTxRefs))
	for i, sqlTxRef := range sqlTxRefs {
		txRefs[i] = indexertypes.TxReferenceFromDB(&sqlTxRef)
	}
	return txRefs, nil
}

// OnNewTx stores the transaction reference in the indexer database
func (s *Indexer) OnNewTx(tx *vochaintx.VochainTx, blockHeight uint32, txIndex int32) {
	s.lockPool.Lock()
	defer s.lockPool.Unlock()
	s.newTxPool = append(s.newTxPool, &indexertypes.TxReference{
		Hash:         types.HexBytes(tx.TxID[:]),
		BlockHeight:  blockHeight,
		TxBlockIndex: txIndex,
		TxType:       tx.TxModelType,
	})
}

// indexNewTxs indexes the txs pending in the newTxPool and updates the transaction count
// this function should only be called within Commit(), on a new block.
func (s *Indexer) indexNewTxs(txList []*indexertypes.TxReference) {
	defer s.liveGoroutines.Add(-1)
	if len(txList) == 0 {
		return
	}
	if s.cancelCtx.Err() != nil {
		return // closing
	}

	// TODO(mvdan): use a single transaction
	queries, ctx, cancel := s.timeoutQueries()
	defer cancel()
	for _, tx := range txList {
		if s.cancelCtx.Err() != nil {
			return // closing
		}
		if _, err := queries.CreateTxReference(ctx, indexerdb.CreateTxReferenceParams{
			Hash:         tx.Hash,
			BlockHeight:  int64(tx.BlockHeight),
			TxBlockIndex: int64(tx.TxBlockIndex),
			TxType:       tx.TxType,
		}); err != nil {
			log.Errorf("cannot store tx at height %d: %v", tx.Index, err)
			return
		}
	}
}

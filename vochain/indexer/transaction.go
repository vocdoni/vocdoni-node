package indexer

import (
	"fmt"
	"sync/atomic"

	"github.com/timshannon/badgerhold/v3"
	"go.vocdoni.io/dvote/log"
	"go.vocdoni.io/dvote/types"
	indexerdb "go.vocdoni.io/dvote/vochain/indexer/db"
	"go.vocdoni.io/dvote/vochain/indexer/indexertypes"
	"go.vocdoni.io/proto/build/go/models"
	"google.golang.org/protobuf/proto"
)

// TransactionCount returns the number of transactions indexed
func (s *Indexer) TransactionCount() (uint64, error) {
	if !enableBadgerhold {
		queries, ctx, cancel := s.timeoutQueries()
		defer cancel()
		count, err := queries.CountTxReferences(ctx)
		return uint64(count), err
	}
	txCountStore := &indexertypes.CountStore{}
	if err := s.db.Get(indexertypes.CountStoreTransactions, txCountStore); err != nil {
		return txCountStore.Count, err
	}
	return txCountStore.Count, nil
}

// GetTxReference fetches the txReference for the given tx height
func (s *Indexer) GetTxReference(height uint64) (*indexertypes.TxReference, error) {
	if !enableBadgerhold {
		queries, ctx, cancel := s.timeoutQueries()
		defer cancel()
		sqlTxRef, err := queries.GetTxReference(ctx, int64(height))
		if err != nil {
			return nil, fmt.Errorf("tx height %d not found: %v", height, err)
		}
		return indexertypes.TxReferenceFromDB(&sqlTxRef), nil
	}
	txReference := &indexertypes.TxReference{}
	err := s.db.FindOne(txReference, badgerhold.Where(badgerhold.Key).Eq(height))
	if err != nil {
		return nil, fmt.Errorf("tx height %d not found: %v", height, err)
	}
	return txReference, nil
}

// GetTxReference fetches the txReference for the given tx hash
func (s *Indexer) GetTxHashReference(hash types.HexBytes) (*indexertypes.TxReference, error) {
	if !enableBadgerhold {
		queries, ctx, cancel := s.timeoutQueries()
		defer cancel()
		sqlTxRef, err := queries.GetTxReferenceByHash(ctx, hash)
		if err != nil {
			return nil, fmt.Errorf("tx hash %x not found: %v", hash, err)
		}
		return indexertypes.TxReferenceFromDB(&sqlTxRef), nil
	}
	txReference := &indexertypes.TxReference{}
	err := s.db.FindOne(txReference, badgerhold.Where("Hash").Eq(hash).Index("Hash"))
	if err != nil {
		return nil, fmt.Errorf("tx hash %x not found: %v", hash, err)
	}
	return txReference, nil
}

// GetLastTxReferences fetches a number of the latest indexed transactions.
// The first one returned is the newest, so they are in descending order.
func (s *Indexer) GetLastTxReferences(limit int32) ([]*indexertypes.TxReference, error) {
	queries, ctx, cancel := s.timeoutQueries()
	defer cancel()
	sqlTxRefs, err := queries.GetLastTxReferences(ctx, limit)
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
func (s *Indexer) OnNewTx(hash []byte, blockHeight uint32, txIndex int32) {
	s.lockPool.Lock()
	defer s.lockPool.Unlock()
	signedTx, err := s.App.GetTx(blockHeight, txIndex)
	if err != nil {
		log.Warnf("could not get tx %x from block %d: %v", hash, blockHeight, err)
		return
	}
	tx := models.Tx{}
	if err := proto.Unmarshal(signedTx.GetTx(), &tx); err != nil {
		log.Warnf("could not unmarshal tx %x from block %d: %v", hash, blockHeight, err)
		return
	}
	s.newTxPool = append(s.newTxPool, &indexertypes.TxReference{
		Hash:         types.HexBytes(hash),
		BlockHeight:  blockHeight,
		TxBlockIndex: txIndex,
		TxType:       fmt.Sprintf("%s", tx.ProtoReflect().Type()),
	})
}

// indexNewTxs indexes the txs pending in the newTxPool and updates the transaction count
// this function should only be called within Commit(), on a new block.
func (s *Indexer) indexNewTxs(txList []*indexertypes.TxReference) {
	defer atomic.AddInt64(&s.liveGoroutines, -1)
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
		}); err != nil {
			log.Errorf("cannot store tx at height %d: %v", tx.Index, err)
			return
		}
	}

	if !enableBadgerhold {
		return
	}
	s.txIndexLock.Lock()
	defer s.txIndexLock.Unlock()
	txCount := &indexertypes.CountStore{}
	if err := s.db.Get(indexertypes.CountStoreTransactions, txCount); err != nil {
		log.Errorf("could not get tx count: %v", err)
		return
	}
	for i, tx := range txList {
		if s.cancelCtx.Err() != nil {
			return // closing
		}
		// Add confirmed txs to transaction count
		tx.Index = txCount.Count + uint64(i) + 1 // Start indexing at 1
		if err := s.db.Insert(tx.Index, tx); err != nil {
			log.Errorf("cannot store tx at height %d: %v", tx.Index, err)
			return
		}
	}
	// Store new transaction count
	txCount.Count += uint64(len(txList))
	if err := s.db.Upsert(indexertypes.CountStoreTransactions, txCount); err != nil {
		log.Errorf("could not update tx count: %v", err)
	}
}

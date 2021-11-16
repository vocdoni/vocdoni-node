package scrutinizer

import (
	"fmt"

	"github.com/timshannon/badgerhold/v3"
	"go.vocdoni.io/dvote/log"
	"go.vocdoni.io/dvote/types"
	"go.vocdoni.io/dvote/vochain/scrutinizer/indexertypes"
)

// TransactionCount returns the number of transactions indexed
func (s *Scrutinizer) TransactionCount() (uint64, error) {
	txCountStore := &indexertypes.CountStore{}
	if err := s.db.Get(indexertypes.CountStoreTransactions, txCountStore); err != nil {
		return txCountStore.Count, err
	}
	return txCountStore.Count, nil
}

// GetTxReference fetches the txReference for the given tx height
func (s *Scrutinizer) GetTxReference(height uint64) (*indexertypes.TxReference, error) {
	txReference := &indexertypes.TxReference{}
	err := s.db.FindOne(txReference, badgerhold.Where(badgerhold.Key).Eq(height))
	if err != nil {
		return nil, fmt.Errorf("tx height %d not found: %v", height, err)
	}
	return txReference, nil
}

// GetTxReference fetches the txReference for the given tx hash
func (s *Scrutinizer) GetTxHashReference(hash types.HexBytes) (*indexertypes.TxReference, error) {
	txReference := &indexertypes.TxReference{}
	err := s.db.FindOne(txReference, badgerhold.Where("Hash").Eq(hash).Index("Hash"))
	if err != nil {
		return nil, fmt.Errorf("tx hash %x not found: %v", hash, err)
	}
	return txReference, nil
}

// OnNewTx stores the transaction reference in the indexer database
func (s *Scrutinizer) OnNewTx(hash []byte, blockHeight uint32, txIndex int32) {
	s.newTxPool = append(s.newTxPool, &indexertypes.TxReference{
		Hash:         types.HexBytes(hash),
		BlockHeight:  blockHeight,
		TxBlockIndex: txIndex,
	})
}

// indexNewTxs indexes the txs pending in the newTxPool and updates the transaction count
// this function should only be called within Commit(), on a new block.
func (s *Scrutinizer) indexNewTxs(txList []*indexertypes.TxReference) {
	if len(txList) == 0 {
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

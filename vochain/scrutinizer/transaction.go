package scrutinizer

import (
	"fmt"

	"github.com/timshannon/badgerhold/v3"
	"go.vocdoni.io/dvote/vochain/scrutinizer/indexertypes"
)

// TransactionCount returns the number of transactions indexed
func (s *Scrutinizer) TransactionCount() (uint64, error) {
	txCountStore := &indexertypes.CountStore{}
	if err := s.db.Get(indexertypes.CountStore_Transactions, txCountStore); err != nil {
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

// OnNewTx stores the transaction reference in the indexer database
func (s *Scrutinizer) OnNewTx(blockHeight uint32, txIndex int32) {
	s.newTxPool = append(s.newTxPool, &indexertypes.TxReference{
		BlockHeight:  blockHeight,
		TxBlockIndex: txIndex,
	})
}

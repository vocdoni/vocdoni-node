package scrutinizer

import (
	"sync/atomic"

	"github.com/timshannon/badgerhold/v3"
	"go.vocdoni.io/dvote/log"
	"go.vocdoni.io/dvote/vochain/scrutinizer/indexertypes"
)

// TransactionCount returns the number of transactions indexed
func (s *Scrutinizer) TransactionCount() uint64 {
	return atomic.LoadUint64(&s.countTotalTransactions)
}

// GetTxReference fetches the txReference for the given tx height
func (s *Scrutinizer) GetTxReference(height uint64) (*indexertypes.TxReference, error) {
	txReference := &indexertypes.TxReference{}
	err := s.db.FindOne(txReference, badgerhold.Where(badgerhold.Key).Eq(height))
	if err != nil {
		return nil, err
	}
	return txReference, nil
}

// OnNewTx stores the transaction reference in the indexer database
func (s *Scrutinizer) OnNewTx(blockHeight, txIndex uint32) {
	txCount := atomic.AddUint64(&s.countTotalTransactions, 1)
	log.Debugf("Storing tx %d: block %d tx %d", txCount, blockHeight, txIndex)
	err := s.db.Insert(txCount, &indexertypes.TxReference{
		Index:        txCount,
		BlockHeight:  blockHeight,
		TxBlockIndex: txIndex,
	})
	if err != nil {
		log.Errorf("cannot store tx at height %d: %v", txCount, err)
	}
}

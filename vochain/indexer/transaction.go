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
	"google.golang.org/protobuf/proto"
)

// ErrTransactionNotFound is returned if the transaction is not found.
var ErrTransactionNotFound = fmt.Errorf("transaction not found")

// CountTotalTransactions returns the number of transactions indexed
func (idx *Indexer) CountTotalTransactions() (uint64, error) {
	count, err := idx.readOnlyQuery.CountTotalTransactions(context.TODO())
	return uint64(count), err
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

// TransactionList returns the list of transactions indexed.
// blockHeight, hash, txType, txSubtype and txSigner are optional, if declared as zero-value will be ignored.
// hash matches substrings.
// The first one returned is the newest, so they are in descending order.
func (idx *Indexer) TransactionList(limit, offset int, blockHeight uint64, txHash, txType, txSubtype, txSigner string) ([]*indexertypes.TransactionMetadata, uint64, error) {
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
		list = append(list, indexertypes.TransactionMetadataFromDB(&row))
	}
	count, err := idx.readOnlyQuery.CountTransactions(context.TODO(), indexerdb.CountTransactionsParams{
		HashSubstr:  txHash,
		BlockHeight: blockHeight,
		TxType:      txType,
		TxSubtype:   txSubtype,
		TxSigner:    txSigner,
	})
	if err != nil {
		return nil, 0, err
	}
	return list, uint64(count), nil
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
}

package vocone

import (
	"context"
	"encoding/binary"
	"errors"
	"fmt"

	cometabcitypes "github.com/cometbft/cometbft/abci/types"
	comettmhash "github.com/cometbft/cometbft/crypto/tmhash"
	cometcoretypes "github.com/cometbft/cometbft/rpc/core/types"
	"go.vocdoni.io/dvote/db"
	"go.vocdoni.io/dvote/log"
)

// loadMempool rebuilds the in-memory mempool index from the persistent database.
// Called once at startup.
func (vc *Vocone) loadMempool() error {
	vc.mempoolMtx.Lock()
	defer vc.mempoolMtx.Unlock()

	// Load the last sequence number.
	seqBytes, err := vc.mempoolDB.Get([]byte(keyMempoolSeq))
	if err != nil && !errors.Is(err, db.ErrKeyNotFound) {
		return fmt.Errorf("could not read mempool sequence: %w", err)
	}
	if len(seqBytes) == 8 {
		vc.mempoolSeq = binary.BigEndian.Uint64(seqBytes)
	}

	// Iterate over all pending transactions and rebuild the key list.
	vc.mempoolKeys = nil
	if err := vc.mempoolDB.Iterate([]byte(prefixMempool), func(key, _ []byte) bool {
		keyCopy := make([]byte, len(key))
		copy(keyCopy, key)
		vc.mempoolKeys = append(vc.mempoolKeys, keyCopy)
		return true
	}); err != nil {
		return fmt.Errorf("could not iterate mempool: %w", err)
	}

	if len(vc.mempoolKeys) > 0 {
		log.Infow("recovered pending transactions from mempool", "count", len(vc.mempoolKeys))
	}
	return nil
}

// addTx validates and adds a transaction to the persistent mempool.
func (vc *Vocone) addTx(tx []byte) (*cometcoretypes.ResultBroadcastTx, error) {
	resp, err := vc.App.CheckTx(context.Background(), &cometabcitypes.CheckTxRequest{Tx: tx})
	if err != nil {
		return nil, err
	}
	if resp.Code != 0 {
		log.Debugw("checkTx failed", "data", string(resp.Data))
		return &cometcoretypes.ResultBroadcastTx{
			Code: resp.Code,
			Data: resp.Data,
			Hash: comettmhash.Sum(tx),
		}, nil
	}

	vc.mempoolMtx.Lock()
	defer vc.mempoolMtx.Unlock()

	if len(vc.mempoolKeys) >= DefaultMempoolSize {
		return &cometcoretypes.ResultBroadcastTx{
			Code: 1,
			Data: []byte("mempool is full"),
		}, fmt.Errorf("mempool is full")
	}

	// Generate a unique key using a monotonic sequence.
	vc.mempoolSeq++
	key := mempoolKey(vc.mempoolSeq)

	wTx := vc.mempoolDB.WriteTx()
	defer wTx.Discard()
	if err := wTx.Set(key, tx); err != nil {
		return nil, fmt.Errorf("could not store tx in mempool: %w", err)
	}
	// Persist the sequence counter so it survives restarts.
	seqBytes := make([]byte, 8)
	binary.BigEndian.PutUint64(seqBytes, vc.mempoolSeq)
	if err := wTx.Set([]byte(keyMempoolSeq), seqBytes); err != nil {
		return nil, fmt.Errorf("could not store mempool seq: %w", err)
	}
	if err := wTx.Commit(); err != nil {
		return nil, fmt.Errorf("could not commit mempool tx: %w", err)
	}
	vc.mempoolKeys = append(vc.mempoolKeys, key)

	return &cometcoretypes.ResultBroadcastTx{
		Code: resp.Code,
		Data: resp.Data,
		Hash: comettmhash.Sum(tx),
	}, nil
}

// mempoolSize returns the number of pending transactions.
func (vc *Vocone) mempoolSize() int {
	vc.mempoolMtx.Lock()
	defer vc.mempoolMtx.Unlock()
	return len(vc.mempoolKeys)
}

// mempoolPrune removes a transaction from the mempool by its hash.
func (vc *Vocone) mempoolPrune(txHash [32]byte) error {
	vc.mempoolMtx.Lock()
	defer vc.mempoolMtx.Unlock()

	for i, key := range vc.mempoolKeys {
		txData, err := vc.mempoolDB.Get(key)
		if err != nil {
			continue
		}
		if [32]byte(comettmhash.Sum(txData)) == txHash {
			wTx := vc.mempoolDB.WriteTx()
			defer wTx.Discard()
			if err := wTx.Delete(key); err != nil {
				return err
			}
			if err := wTx.Commit(); err != nil {
				return err
			}
			vc.mempoolKeys = append(vc.mempoolKeys[:i], vc.mempoolKeys[i+1:]...)
			return nil
		}
	}
	return nil
}

// mempoolKey builds a db key for a mempool entry from a sequence number.
func mempoolKey(seq uint64) []byte {
	key := make([]byte, len(prefixMempool)+8)
	copy(key, prefixMempool)
	binary.BigEndian.PutUint64(key[len(prefixMempool):], seq)
	return key
}

// prepareBlock drains transactions from the mempool, re-validates them,
// and returns the raw tx list along with the mempool keys that were consumed.
// It does NOT modify the mempool or blockstore — the caller is responsible
// for persisting writes and cleaning up the mempool after a successful commit.
func (vc *Vocone) prepareBlock() (txs [][]byte, consumedKeys [][]byte) {
	vc.mempoolMtx.Lock()
	// Take up to txsPerBlock keys from the front of the queue.
	count := min(vc.txsPerBlock, len(vc.mempoolKeys))
	pendingKeys := make([][]byte, count)
	copy(pendingKeys, vc.mempoolKeys[:count])
	vc.mempoolMtx.Unlock()

	if len(pendingKeys) == 0 {
		return nil, nil
	}

	for _, key := range pendingKeys {
		txData, err := vc.mempoolDB.Get(key)
		if err != nil {
			// Key disappeared (pruned concurrently), skip.
			consumedKeys = append(consumedKeys, key)
			continue
		}

		// Re-validate before including in the block.
		resp, err := vc.App.CheckTx(context.Background(), &cometabcitypes.CheckTxRequest{Tx: txData})
		if err != nil {
			log.Errorw(err, "error on check tx during block preparation")
			consumedKeys = append(consumedKeys, key)
			continue
		}
		if resp.Code != 0 {
			log.Warnw("check tx failed during block preparation",
				"code", resp.Code, "data", string(resp.Data))
			consumedKeys = append(consumedKeys, key)
			continue
		}

		txs = append(txs, txData)
		consumedKeys = append(consumedKeys, key)
	}

	if len(txs) > 0 {
		log.Infow("prepared block transactions",
			"count", len(txs), "height", vc.height.Load())
	}
	return txs, consumedKeys
}

// commitMempoolCleanup removes consumed keys from the persistent mempool
// and the in-memory index. Must be called after a successful block commit.
func (vc *Vocone) commitMempoolCleanup(consumedKeys [][]byte) {
	if len(consumedKeys) == 0 {
		return
	}
	wTx := vc.mempoolDB.WriteTx()
	defer wTx.Discard()
	for _, key := range consumedKeys {
		if err := wTx.Delete(key); err != nil {
			log.Errorw(err, "could not delete mempool entry")
		}
	}
	if err := wTx.Commit(); err != nil {
		log.Errorw(err, "could not commit mempool cleanup")
	}

	// Update the in-memory index.
	vc.mempoolMtx.Lock()
	vc.mempoolKeys = vc.mempoolKeys[len(consumedKeys):]
	vc.mempoolMtx.Unlock()
}

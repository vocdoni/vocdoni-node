package indexer

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"go.vocdoni.io/dvote/log"
	indexerdb "go.vocdoni.io/dvote/vochain/indexer/db"
	"go.vocdoni.io/dvote/vochain/state"
)

// ErrBlockNotFound is returned if the block is not found in the indexer database.
var ErrBlockNotFound = fmt.Errorf("block not found")

func (idx *Indexer) OnBeginBlock(bb state.BeginBlock) {
	idx.blockMu.Lock()
	defer idx.blockMu.Unlock()
	queries := idx.blockTxQueries()
	if _, err := queries.CreateBlock(context.TODO(), indexerdb.CreateBlockParams{
		Height:   bb.Height,
		Time:     bb.Time,
		DataHash: nonNullBytes(bb.DataHash),
	}); err != nil {
		log.Errorw(err, "cannot index new block")
	}
}

// BlockTimestamp returns the timestamp of the block at the given height
func (idx *Indexer) BlockTimestamp(height int64) (time.Time, error) {
	block, err := idx.readOnlyQuery.GetBlock(context.TODO(), height)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return time.Time{}, ErrBlockNotFound
		}
		return time.Time{}, err
	}
	return block.Time, nil
}

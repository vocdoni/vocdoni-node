package indexer

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"go.vocdoni.io/dvote/vochain/indexer/indexertypes"
)

// ErrBlockNotFound is returned if the block is not found in the indexer database.
var ErrBlockNotFound = fmt.Errorf("block not found")

// BlockTimestamp returns the timestamp of the block at the given height
func (idx *Indexer) BlockTimestamp(height int64) (time.Time, error) {
	block, err := idx.BlockByHeight(height)
	if err != nil {
		return time.Time{}, err
	}
	return block.Time, nil
}

// BlockByHeight returns the available information of the block at the given height
func (idx *Indexer) BlockByHeight(height int64) (*indexertypes.Block, error) {
	block, err := idx.readOnlyQuery.GetBlockByHeight(context.TODO(), height)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrBlockNotFound
		}
		return nil, err
	}
	return indexertypes.BlockFromDB(&block), nil
}

// BlockByHeight returns the available information of the block with the given hash
func (idx *Indexer) BlockByHash(hash []byte) (*indexertypes.Block, error) {
	block, err := idx.readOnlyQuery.GetBlockByHash(context.TODO(), hash)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrBlockNotFound
		}
		return nil, err
	}
	return indexertypes.BlockFromDB(&block), nil
}

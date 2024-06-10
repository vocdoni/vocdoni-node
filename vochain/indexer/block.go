package indexer

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"
)

// ErrBlockNotFound is returned if the block is not found in the indexer database.
var ErrBlockNotFound = fmt.Errorf("block not found")

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

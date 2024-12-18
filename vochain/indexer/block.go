package indexer

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	indexerdb "go.vocdoni.io/dvote/vochain/indexer/db"
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

// BlockByHash returns the available information of the block with the given hash
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

// BlockList returns the list of blocks indexed.
// chainID, hash, proposerAddress are optional, if declared as zero-value will be ignored.
// The first one returned is the newest, so they are in descending order.
func (idx *Indexer) BlockList(limit, offset int, chainID, hash, proposerAddress string) ([]*indexertypes.Block, uint64, error) {
	if offset < 0 {
		return nil, 0, fmt.Errorf("invalid value: offset cannot be %d", offset)
	}
	if limit <= 0 {
		return nil, 0, fmt.Errorf("invalid value: limit cannot be %d", limit)
	}
	results, err := idx.readOnlyQuery.SearchBlocks(context.TODO(), indexerdb.SearchBlocksParams{
		Limit:           int64(limit),
		Offset:          int64(offset),
		ChainID:         chainID,
		HashSubstr:      hash,
		ProposerAddress: proposerAddress,
	})
	if err != nil {
		return nil, 0, err
	}
	list := []*indexertypes.Block{}
	for _, row := range results {
		list = append(list, indexertypes.BlockFromDBRow(&row))
	}
	count, err := idx.CountBlocks(chainID, hash, proposerAddress)
	if err != nil {
		return nil, 0, err
	}
	return list, count, nil
}

// CountBlocks returns how many blocks are indexed.
// If all args passed are empty ("") it will return the last block height, as an optimization.
func (idx *Indexer) CountBlocks(chainID, hash, proposerAddress string) (uint64, error) {
	if chainID == "" && hash == "" && proposerAddress == "" {
		count, err := idx.readOnlyQuery.LastBlockHeight(context.TODO())
		if err != nil {
			return 0, err
		}
		return uint64(count), nil
	}
	count, err := idx.readOnlyQuery.CountBlocks(context.TODO(), indexerdb.CountBlocksParams{
		ChainID:         chainID,
		HashSubstr:      hash,
		ProposerAddress: proposerAddress,
	})
	if err != nil {
		return 0, err
	}
	return uint64(count), nil
}

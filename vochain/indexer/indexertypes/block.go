package indexertypes

import (
	"time"

	"go.vocdoni.io/dvote/types"
	indexerdb "go.vocdoni.io/dvote/vochain/indexer/db"
)

// Block represents a block handled by the Vochain.
// The indexer Block data type is different from the vochain state data type
// since it is optimized for querying purposes and not for keeping a shared consensus state.
type Block struct {
	ChainID         string         `json:"chainId"`
	Height          int64          `json:"height"`
	Time            time.Time      `json:"time"`
	Hash            types.HexBytes `json:"hash"`
	ProposerAddress types.HexBytes `json:"proposer"`
	LastBlockHash   types.HexBytes `json:"lastBlockHash"`
	TxCount         int64          `json:"txCount"`
}

// BlockFromDB converts the indexerdb.Block into a Block
func BlockFromDB(dbblock *indexerdb.Block) *Block {
	return &Block{
		ChainID:         dbblock.ChainID,
		Height:          dbblock.Height,
		Time:            dbblock.Time,
		Hash:            nonEmptyBytes(dbblock.Hash),
		ProposerAddress: nonEmptyBytes(dbblock.ProposerAddress),
		LastBlockHash:   nonEmptyBytes(dbblock.LastBlockHash),
	}
}

// BlockFromDBRow converts the indexerdb.SearchBlocksRow into a Block
func BlockFromDBRow(row *indexerdb.SearchBlocksRow) *Block {
	return &Block{
		ChainID:         row.ChainID,
		Height:          row.Height,
		Time:            row.Time,
		Hash:            nonEmptyBytes(row.Hash),
		ProposerAddress: nonEmptyBytes(row.ProposerAddress),
		LastBlockHash:   nonEmptyBytes(row.LastBlockHash),
		TxCount:         row.TxCount,
	}
}

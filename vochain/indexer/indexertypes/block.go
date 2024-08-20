package indexertypes

import (
	"time"

	"go.vocdoni.io/dvote/types"
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

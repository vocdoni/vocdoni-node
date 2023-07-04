package indexer

import (
	"context"

	"go.vocdoni.io/dvote/log"
	indexerdb "go.vocdoni.io/dvote/vochain/indexer/db"
	"go.vocdoni.io/dvote/vochain/state"
)

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

package testutil

import (
	"bytes"
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	tmtypes "github.com/cometbft/cometbft/types"
	"go.vocdoni.io/dvote/log"
)

type MockBlockStore struct {
	store *sync.Map
	count atomic.Int64
}

func (b *MockBlockStore) Init() {
	log.Info("init mock block store")
	b.store = new(sync.Map)
	b.count.Store(0)
}

func (b *MockBlockStore) Height() int64 {
	return b.count.Load()
}

func (b *MockBlockStore) AddTxToBlock(tx []byte) {
	count := b.count.Load()
	log.Infow("add tx to block", "height", count)
	b.Get(count).Data.Txs = append(b.Get(count).Data.Txs, tx)
}

func (b *MockBlockStore) NewBlock(height int64) {
	if count := b.count.Load(); height != count {
		panic(fmt.Sprintf("height is not the expected one (got:%d expected:%d)", height, count))
	}
	log.Infow("new block", "height", height)
	b.set(height, &tmtypes.Block{
		Header: tmtypes.Header{Height: height, Time: time.Now(), ChainID: "test"},
		Data:   tmtypes.Data{Txs: make([]tmtypes.Tx, 0)}},
	)
}

func (b *MockBlockStore) EndBlock() int64 {
	log.Infow("end block", "height", b.count.Load())
	return b.count.Add(1)
}

func (b *MockBlockStore) Get(height int64) *tmtypes.Block {
	val, ok := b.store.Load(height)
	if !ok {
		return nil
	}
	return val.(*tmtypes.Block)
}

func (b *MockBlockStore) GetByHash(hash []byte) *tmtypes.Block {
	var block *tmtypes.Block
	b.store.Range(func(key, value any) bool {
		if bytes.Equal(value.(*tmtypes.Block).Hash().Bytes(), hash) {
			block = value.(*tmtypes.Block)
			return false
		}
		return true
	})
	return block
}

func (b *MockBlockStore) set(height int64, block *tmtypes.Block) {
	b.store.Store(height, block)
}

package testutil

import (
	"bytes"
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	tmtypes "github.com/tendermint/tendermint/types"
	"go.vocdoni.io/dvote/log"
)

type MockBlockStore struct {
	store *sync.Map
	count int64
}

func (b *MockBlockStore) Init() {
	log.Info("init mock block store")
	b.store = new(sync.Map)
	b.count = 0
}

func (b *MockBlockStore) Height() int64 {
	return b.count
}

func (b *MockBlockStore) AddTxToBlock(tx []byte) {
	log.Infow("add tx to block", map[string]interface{}{"height": b.count})
	b.Get(b.count).Data.Txs = append(b.Get(b.count).Data.Txs, tx)
}

func (b *MockBlockStore) NewBlock(height int64) {
	log.Infow("new block", map[string]interface{}{"height": b.count})
	if height != b.count {
		panic(fmt.Sprintf("height is not the expected one (got:%d expected:%d)", height, b.count))
	}
	b.set(b.count, &tmtypes.Block{
		Header: tmtypes.Header{Height: b.count, Time: time.Now(), ChainID: "test"},
		Data:   tmtypes.Data{Txs: make([]tmtypes.Tx, 0)}},
	)
}

func (b *MockBlockStore) EndBlock() int64 {
	log.Infow("end block", map[string]interface{}{"height": b.count})
	return atomic.AddInt64(&b.count, 1)
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
	b.store.Range(func(key, value interface{}) bool {
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

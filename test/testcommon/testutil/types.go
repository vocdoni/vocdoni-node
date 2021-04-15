package testutil

import (
	"bytes"
	"sync"

	"sync/atomic"

	tmtypes "github.com/tendermint/tendermint/types"
)

type MockBlockStore struct {
	store *sync.Map
	count int64
}

func (b *MockBlockStore) Init() {
	b.store = new(sync.Map)
	b.count = 0
}

func (b *MockBlockStore) Add(block *tmtypes.Block) {
	c := atomic.AddInt64(&b.count, 1)
	b.set(c-1, block)
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

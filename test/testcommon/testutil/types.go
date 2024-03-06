package testutil

import (
	"bytes"
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	comettypes "github.com/cometbft/cometbft/types"
	"go.vocdoni.io/dvote/log"
)

type MockBlockStore struct {
	blockByHeight sync.Map // map[int64]*tmtypes.Block
	height        atomic.Int64
}

func (b *MockBlockStore) Init() {
	log.Info("init mock block store")
}

func (b *MockBlockStore) Height() int64 {
	return b.height.Load()
}

func (b *MockBlockStore) AddTxToBlock(tx []byte) {
	count := b.height.Load()
	log.Infow("add tx to block", "height", count)
	block := b.Get(count)
	// Note that this append is not safe to do concurrently.
	block.Txs = append(block.Txs, tx)
}

func (b *MockBlockStore) NewBlock(height int64, timestamp time.Time) {
	if count := b.height.Load(); height != count {
		panic(fmt.Sprintf("height is not the expected one (got:%d expected:%d)", height, count))
	}
	log.Infow("new block", "height", height, "timestamp", timestamp)
	b.set(height, &comettypes.Block{
		Header: comettypes.Header{Height: height, Time: time.Now(), ChainID: "test"},
		Data:   comettypes.Data{Txs: make([]comettypes.Tx, 0)},
	},
	)
}

func (b *MockBlockStore) EndBlock() int64 {
	log.Infow("end block", "height", b.height.Load())
	return b.height.Add(1)
}

func (b *MockBlockStore) Get(height int64) *comettypes.Block {
	val, ok := b.blockByHeight.Load(height)
	if !ok {
		return nil
	}
	return val.(*comettypes.Block)
}

func (b *MockBlockStore) GetByHash(hash []byte) *comettypes.Block {
	var block *comettypes.Block
	b.blockByHeight.Range(func(key, value any) bool {
		if bytes.Equal(value.(*comettypes.Block).Hash().Bytes(), hash) {
			block = value.(*comettypes.Block)
			return false
		}
		return true
	})
	return block
}

func (b *MockBlockStore) set(height int64, block *comettypes.Block) {
	b.blockByHeight.Store(height, block)
}

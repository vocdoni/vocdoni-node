package vochain

import (
	"context"
	"fmt"
	"testing"
	"time"

	abcitypes "github.com/cometbft/cometbft/abci/types"
	ctypes "github.com/cometbft/cometbft/rpc/core/types"
	tmtypes "github.com/cometbft/cometbft/types"
	"go.vocdoni.io/dvote/db/metadb"
	"go.vocdoni.io/dvote/test/testcommon/testutil"
	"go.vocdoni.io/proto/build/go/models"
	"google.golang.org/protobuf/proto"
)

// TestBaseApplication creates a new BaseApplication for testing purposes.
// It initializes the State, TransactionHandler and all the callback functions.
// Once the application is create, it is the caller's responsibility to call
// app.AdvanceTestBlock() to advance the block height and commit the state.
func TestBaseApplication(tb testing.TB) *BaseApplication {
	app, err := NewBaseApplication(metadb.ForTest(), tb.TempDir())
	if err != nil {
		tb.Fatal(err)
	}
	app.SetTestingMethods()
	genesisDoc, err := NewTemplateGenesisFile(tb.TempDir(), 4)
	if err != nil {
		tb.Fatal(err)
	}
	_, err = app.InitChain(context.TODO(), &abcitypes.RequestInitChain{
		Time:          time.Now(),
		ChainId:       "test",
		Validators:    []abcitypes.ValidatorUpdate{},
		AppStateBytes: genesisDoc.AppState,
	})
	if err != nil {
		tb.Fatal(err)
	}
	// TODO: should this be a Close on the entire BaseApplication?
	tb.Cleanup(func() {
		if err := app.State.Close(); err != nil {
			tb.Error(err)
		}
	})
	return app
}

// SetTestingMethods assigns fnGetBlockByHash, fnGetBlockByHeight, fnSendTx to use mockBlockStore
func (app *BaseApplication) SetTestingMethods() {
	app.testMockBlockStore = new(testutil.MockBlockStore)
	app.testMockBlockStore.Init()
	app.SetFnGetBlockByHash(app.testMockBlockStore.GetByHash)
	app.SetFnGetBlockByHeight(app.testMockBlockStore.Get)
	app.SetFnGetTx(func(height uint32, txIndex int32) (*models.SignedTx, error) {
		blk := app.testMockBlockStore.Get(int64(height))
		if blk == nil {
			return nil, fmt.Errorf("block not found")
		}
		if len(blk.Txs) <= int(txIndex) {
			return nil, fmt.Errorf("txIndex out of range")
		}
		stx := models.SignedTx{}
		return &stx, proto.Unmarshal(blk.Txs[txIndex], &stx)
	})
	app.SetFnGetTxHash(func(height uint32, txIndex int32) (*models.SignedTx, []byte, error) {
		blk := app.testMockBlockStore.Get(int64(height))
		if blk == nil {
			return nil, nil, fmt.Errorf("block not found")
		}
		if len(blk.Txs) <= int(txIndex) {
			return nil, nil, fmt.Errorf("txIndex out of range")
		}
		stx := models.SignedTx{}
		tx := blk.Txs[txIndex]
		return &stx, tx.Hash(), proto.Unmarshal(blk.Txs[txIndex], &stx)
	})
	app.SetFnSendTx(func(tx []byte) (*ctypes.ResultBroadcastTx, error) {
		resp := app.deliverTx(tx)
		if resp.Code == 0 {
			app.testMockBlockStore.AddTxToBlock(tx)
		}
		return &ctypes.ResultBroadcastTx{
			Hash: tmtypes.Tx(tx).Hash(),
			Code: resp.Code,
			Data: resp.Data,
		}, nil
	})
	app.SetFnMempoolSize(func() int { return 0 })
	app.isSynchronizingFn = func() bool { return false }
	app.beginBlock(time.Now(), 0)
	app.State.SetHeight(0)
	app.endBlockTimestamp.Store(time.Now().Unix())
}

// AdvanceTestBlock commits the current state, ends the current block and starts a new one.
// Advances the block height and timestamp.
func (app *BaseApplication) AdvanceTestBlock() {
	_, err := app.Commit(context.TODO(), nil)
	if err != nil {
		panic(err)
	}
	app.endBlock(time.Now(), uint32(app.testMockBlockStore.Height()))
	newHeight := app.testMockBlockStore.EndBlock()
	// The next block begins 50ms later
	time.Sleep(time.Millisecond * 50)
	nextStartTime := time.Now()
	app.testMockBlockStore.NewBlock(newHeight)
	app.beginBlock(nextStartTime, uint32(newHeight))
}

package vochain

import (
	"fmt"
	"testing"
	"time"

	abcitypes "github.com/cometbft/cometbft/abci/types"
	tmprototypes "github.com/cometbft/cometbft/proto/tendermint/types"
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
	app.InitChain(abcitypes.RequestInitChain{
		Time:          time.Now(),
		ChainId:       "test",
		Validators:    []abcitypes.ValidatorUpdate{},
		AppStateBytes: genesisDoc.AppState,
	})
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
	mockBlockStore := new(testutil.MockBlockStore)
	mockBlockStore.Init()
	app.SetFnGetBlockByHash(mockBlockStore.GetByHash)
	app.SetFnGetBlockByHeight(mockBlockStore.Get)
	app.SetFnGetTx(func(height uint32, txIndex int32) (*models.SignedTx, error) {
		blk := mockBlockStore.Get(int64(height))
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
		blk := mockBlockStore.Get(int64(height))
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
		resp := app.DeliverTx(abcitypes.RequestDeliverTx{Tx: tx})
		if resp.Code == 0 {
			mockBlockStore.AddTxToBlock(tx)
		}
		return &ctypes.ResultBroadcastTx{
			Hash: tmtypes.Tx(tx).Hash(),
			Code: resp.Code,
			Data: resp.Data,
		}, nil
	})
	app.SetFnMempoolSize(func() int { return 0 })
	app.isSynchronizingFn = func() bool { return false }
	app.SetFnEndBlock(func(req abcitypes.RequestEndBlock) abcitypes.ResponseEndBlock {
		height := mockBlockStore.EndBlock()
		app.endBlock(height, time.Now())
		//app.State.SetHeight(uint32(height))
		return abcitypes.ResponseEndBlock{}
	})
	app.SetFnBeginBlock(func(req abcitypes.RequestBeginBlock) abcitypes.ResponseBeginBlock {
		mockBlockStore.NewBlock(req.Header.Height)
		app.fnBeginBlockDefault(req)
		return abcitypes.ResponseBeginBlock{}
	})
	app.State.SetHeight(0)
	app.endBlockTimestamp.Store(time.Now().Unix())
}

// AdvanceTestBlock commits the current state, ends the current block and starts a new one.
// Advances the block height and timestamp.
func (app *BaseApplication) AdvanceTestBlock() {
	app.Commit()
	endingHeight := int64(app.Height())
	app.EndBlock(abcitypes.RequestEndBlock{Height: endingHeight})

	// The next block begins a second later.
	nextHeight := endingHeight + 1
	nextStartTime := time.Now()

	app.BeginBlock(abcitypes.RequestBeginBlock{Header: tmprototypes.Header{
		Time:   nextStartTime,
		Height: nextHeight,
	}})
}

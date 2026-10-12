package vochain

import (
	"fmt"
	"testing"
	"time"

	"go.vocdoni.io/dvote/config"
	"go.vocdoni.io/dvote/crypto/ethereum"
	"go.vocdoni.io/dvote/db/metadb"
	"go.vocdoni.io/dvote/vochain/blocktypes"
	"go.vocdoni.io/dvote/vochain/genesis"
	"go.vocdoni.io/proto/build/go/models"
	"google.golang.org/protobuf/proto"
)

// TestBaseApplication creates a new BaseApplication for testing purposes.
// It initializes the State, TransactionHandler and all the callback functions.
// Once the application is create, it is the caller's responsibility to call
// app.AdvanceTestBlock() to advance the block height and commit the state.
func TestBaseApplication(tb testing.TB) *BaseApplication {
	return TestBaseApplicationWithChainID(tb, "test")
}

// TestBaseApplicationWithChainID creates a new BaseApplication for testing purposes.
// It initializes the State, TransactionHandler and all the callback functions.
// Once the application is create, it is the caller's responsibility to call
// app.AdvanceTestBlock() to advance the block height and commit the state.
func TestBaseApplicationWithChainID(tb testing.TB, chainID string) *BaseApplication {
	app, err := NewBaseApplication(&config.VochainCfg{
		DBType:  metadb.ForTest(),
		DataDir: tb.TempDir(),
	})
	if err != nil {
		tb.Fatal(err)
	}
	app.SetTestingMethods()
	app.SetChainID(chainID)
	appState, err := testGenesisAppState(4)
	if err != nil {
		tb.Fatal(err)
	}
	if _, err := app.InitChain(appState, time.Unix(0, 0)); err != nil {
		tb.Fatal(err)
	}
	app.State.ElectionPriceCalc.SetBasePrice(1)
	app.State.ElectionPriceCalc.SetCapacity(10000)
	// TODO: should this be a Close on the entire BaseApplication?
	tb.Cleanup(func() {
		if err := app.State.Close(); err != nil {
			tb.Error(err)
		}
	})
	// Set the initial timestamp
	if err := app.State.SetTimestamp(0); err != nil {
		tb.Fatal(err)
	}
	return app
}

// testGenesisAppState returns a genesis app state with a funded faucet account
// and the given number of validators, all with random keys.
func testGenesisAppState(validators int) (*genesis.AppState, error) {
	faucet := ethereum.SignKeys{}
	if err := faucet.Generate(); err != nil {
		return nil, err
	}
	appState := &genesis.AppState{
		Accounts: []genesis.Account{
			{
				Address: faucet.Address().Bytes(),
				Balance: 100000,
			},
		},
		TxCost:          genesis.DefaultTransactionCosts(),
		MaxElectionSize: 100000,
	}
	for i := 0; i < validators; i++ {
		signer := ethereum.SignKeys{}
		if err := signer.Generate(); err != nil {
			return nil, err
		}
		appState.Validators = append(appState.Validators, genesis.AppStateValidators{
			Address:  signer.Address().Bytes(),
			PubKey:   signer.PublicKey(),
			Power:    10,
			Name:     fmt.Sprintf("validator%d", i),
			KeyIndex: uint8(i + 1), // zero is reserved for disabling validator key keeper capabilities
		})
	}
	return appState, nil
}

// SetTestingMethods assigns fnGetBlockByHash, fnGetBlockByHeight, fnSendTx to use mockBlockStore
func (app *BaseApplication) SetTestingMethods() {
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
		return &stx, tx.Hash(), proto.Unmarshal(tx, &stx)
	})
	app.SetFnSendTx(func(tx []byte) (*SendTxResponse, error) {
		resp := app.deliverTx(tx)
		if resp.Code == 0 {
			app.testMockBlockStore.AddTxToBlock(tx)
		}
		return &SendTxResponse{
			Hash: blocktypes.Tx(tx).Hash(),
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
	height := uint32(app.testMockBlockStore.Height())
	ts, err := app.State.Timestamp(false)
	if err != nil {
		panic(err)
	}
	// execute internal state transition commit
	if err := app.Istc.Commit(height, ts); err != nil {
		panic(err)
	}
	// finalize block
	app.endBlock(time.Unix(int64(ts), 0), height)
	// save the state
	if _, err = app.State.PrepareCommit(); err != nil {
		panic(err)
	}
	_, err = app.CommitState()
	if err != nil {
		panic(err)
	}
	// The next block begins 0.00005 seconds later
	newHeight := app.testMockBlockStore.EndBlock()
	time.Sleep(time.Microsecond * 50)
	// nextStartTime is the previous block timestamp + second
	nextStartTime := time.Unix(int64(ts+1), 0)
	app.testMockBlockStore.NewBlock(newHeight, nextStartTime)
	app.beginBlock(nextStartTime, uint32(newHeight))
}

// AdvanceTestBlocksUntilHeight loops over AdvanceTestBlock
// until reaching height n.
func (app *BaseApplication) AdvanceTestBlocksUntilHeight(n uint32) {
	for {
		if uint32(app.testMockBlockStore.Height()) >= n {
			return
		}
		app.AdvanceTestBlock()
	}
}

// AdvanceTestBlocksUntilTimestamp loops over AdvanceTestBlock
// until reaching timestamp ts.
func (app *BaseApplication) AdvanceTestBlocksUntilTimestamp(ts uint32) {
	for {
		if uint32(app.Timestamp()) >= ts {
			return
		}
		app.AdvanceTestBlock()
	}
}

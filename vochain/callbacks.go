package vochain

import (
	"go.vocdoni.io/dvote/vochain/blocktypes"
	"go.vocdoni.io/proto/build/go/models"
)

// SetFnGetBlockByHash sets the getter for blocks by hash
func (app *BaseApplication) SetFnGetBlockByHash(fn func(hash []byte) *blocktypes.Block) {
	app.fnGetBlockByHash = fn
}

// SetFnGetBlockByHeight sets the getter for blocks by height
func (app *BaseApplication) SetFnGetBlockByHeight(fn func(height int64) *blocktypes.Block) {
	app.fnGetBlockByHeight = fn
}

// SetFnBlockStoreRange sets the getter for the lowest and highest block heights in the block store
func (app *BaseApplication) SetFnBlockStoreRange(fn func() (base, height int64)) {
	app.fnBlockStoreRange = fn
}

// SetFnSendTx sets the sendTx method
func (app *BaseApplication) SetFnSendTx(fn func(tx []byte) (*SendTxResponse, error)) {
	app.fnSendTx = fn
}

// SetFnGetTx sets the getTx method
func (app *BaseApplication) SetFnGetTx(fn func(height uint32, txIndex int32) (*models.SignedTx, error)) {
	app.fnGetTx = fn
}

// SetFnIsSynchronizing sets the is synchronizing method
func (app *BaseApplication) SetFnIsSynchronizing(fn func() bool) {
	app.isSynchronizingFn = fn
}

// SetFnGetTxHash sets the getTxHash method
func (app *BaseApplication) SetFnGetTxHash(fn func(height uint32, txIndex int32) (*models.SignedTx, []byte, error)) {
	app.fnGetTxHash = fn
}

// SetFnMempoolSize sets the mempool size method
func (app *BaseApplication) SetFnMempoolSize(fn func() int) {
	app.fnMempoolSize = fn
}

// SetFnMempoolPrune sets the mempool prune method for a transaction.
func (app *BaseApplication) SetFnMempoolPrune(fn func([32]byte) error) {
	app.fnMempoolPrune = fn
}

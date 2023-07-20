package vochain

import (
	"context"
	"fmt"

	"github.com/cometbft/cometbft/node"
	tmcli "github.com/cometbft/cometbft/rpc/client/local"
	ctypes "github.com/cometbft/cometbft/rpc/core/types"
	tmtypes "github.com/cometbft/cometbft/types"
	"go.vocdoni.io/dvote/config"
	"go.vocdoni.io/dvote/log"
	"go.vocdoni.io/proto/build/go/models"
	"google.golang.org/protobuf/proto"
)

// SetNode initializes the cometbft consensus node service and client.
func (app *BaseApplication) SetNode(vochaincfg *config.VochainCfg, genesis []byte) error {
	var err error
	if app.Service, err = newTendermint(app, vochaincfg, genesis); err != nil {
		return fmt.Errorf("could not set tendermint node service: %s", err)
	}
	if vochaincfg.IsSeedNode {
		return nil
	}
	if app.Node = tmcli.New(app.Service.(*node.Node)); err != nil {
		return fmt.Errorf("could not start tendermint node client: %w", err)
	}
	nodeGenesis, err := app.Node.Genesis(context.TODO())
	if err != nil {
		return err
	}
	app.genesisInfo = nodeGenesis.Genesis
	return nil
}

// SetDefaultMethods assigns fnGetBlockByHash, fnGetBlockByHeight, fnSendTx to use the
// BlockStore from app.Node to load blocks. Assumes app.Node has been set.
func (app *BaseApplication) SetDefaultMethods() {
	app.SetFnGetBlockByHash(func(hash []byte) *tmtypes.Block {
		resblock, err := app.Node.BlockByHash(context.Background(), hash)
		if err != nil {
			log.Warnf("cannot fetch block by hash: %v", err)
			return nil
		}
		return resblock.Block
	})

	app.SetFnGetBlockByHeight(func(height int64) *tmtypes.Block {
		resblock, err := app.Node.Block(context.Background(), &height)
		if err != nil {
			log.Warnf("cannot fetch block by height: %v", err)
			return nil
		}
		return resblock.Block
	})

	app.isSynchronizingFn = app.isSynchronizingTendermint
	app.SetFnGetTx(app.getTxTendermint)
	app.SetFnGetTxHash(app.getTxHashTendermint)
	app.SetFnMempoolSize(func() int {
		return app.Service.(*node.Node).Mempool().Size()
	})
	app.SetFnMempoolPrune(app.fnMempoolRemoveTxTendermint)
	app.SetFnSendTx(func(tx []byte) (*ctypes.ResultBroadcastTx, error) {
		result, err := app.Node.BroadcastTxSync(context.Background(), tx)
		log.Debugw("broadcast tx",
			"size", len(tx),
			"result",
			func() int {
				if result == nil {
					return -1
				}
				return int(result.Code)
			}(),
			"error",
			func() string {
				if err != nil {
					return err.Error()
				}
				return ""
			}(),
		)
		return result, err
	})
}

func (app *BaseApplication) getTxTendermint(height uint32, txIndex int32) (*models.SignedTx, error) {
	block := app.GetBlockByHeight(int64(height))
	if block == nil {
		return nil, ErrTransactionNotFound
	}
	if int32(len(block.Data.Txs)) <= txIndex {
		return nil, ErrTransactionNotFound
	}
	tx := &models.SignedTx{}
	return tx, proto.Unmarshal(block.Data.Txs[txIndex], tx)
}

func (app *BaseApplication) getTxHashTendermint(height uint32, txIndex int32) (*models.SignedTx, []byte, error) {
	block := app.GetBlockByHeight(int64(height))
	if block == nil {
		return nil, nil, ErrTransactionNotFound
	}
	if int32(len(block.Txs)) <= txIndex {
		return nil, nil, ErrTransactionNotFound
	}
	tx := &models.SignedTx{}
	return tx, block.Txs[txIndex].Hash(), proto.Unmarshal(block.Txs[txIndex], tx)
}

// SetFnGetBlockByHash sets the getter for blocks by hash
func (app *BaseApplication) SetFnGetBlockByHash(fn func(hash []byte) *tmtypes.Block) {
	app.fnGetBlockByHash = fn
}

// SetFnGetBlockByHeight sets the getter for blocks by height
func (app *BaseApplication) SetFnGetBlockByHeight(fn func(height int64) *tmtypes.Block) {
	app.fnGetBlockByHeight = fn
}

// SetFnSendTx sets the sendTx method
func (app *BaseApplication) SetFnSendTx(fn func(tx []byte) (*ctypes.ResultBroadcastTx, error)) {
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

// SetFnMempoolSize sets the mempool size method method
func (app *BaseApplication) SetFnMempoolSize(fn func() int) {
	app.fnMempoolSize = fn
}

// SetFnMempoolPrune sets the mempool prune method for a transaction.
func (app *BaseApplication) SetFnMempoolPrune(fn func([32]byte) error) {
	app.fnMempoolPrune = fn
}

// fnMempoolRemoveTxTendermint removes a transaction (identifier by its vochain.TxKey() hash)
// from the Tendermint mempool.
func (app *BaseApplication) fnMempoolRemoveTxTendermint(txKey [tmtypes.TxKeySize]byte) error {
	if app.Service == nil {
		log.Errorw(fmt.Errorf("method not assigned"), "mempoolRemoveTxTendermint")
		return nil
	}
	return app.Service.(*node.Node).Mempool().RemoveTxByKey(txKey)
}

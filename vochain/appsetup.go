package vochain

import (
	"context"
	"fmt"

	cometcli "github.com/cometbft/cometbft/rpc/client/local"
	cometcoretypes "github.com/cometbft/cometbft/rpc/core/types"
	comettypes "github.com/cometbft/cometbft/types"
	"go.vocdoni.io/dvote/config"
	"go.vocdoni.io/dvote/log"
	"go.vocdoni.io/proto/build/go/models"
	"google.golang.org/protobuf/proto"
)

// SetNode initializes the cometbft consensus node service and client.
func (app *BaseApplication) SetNode(vochaincfg *config.VochainCfg, genesis []byte) error {
	var err error
	if app.Node, err = newTendermint(app, vochaincfg, genesis); err != nil {
		return fmt.Errorf("could not set tendermint node service: %s", err)
	}
	if vochaincfg.IsSeedNode {
		return nil
	}
	// Note that cometcli.New logs any error rather than returning it.
	app.NodeClient = cometcli.New(app.Node)
	nodeGenesis, err := app.NodeClient.Genesis(context.TODO())
	if err != nil {
		return err
	}
	app.genesisInfo = nodeGenesis.Genesis
	return nil
}

// SetDefaultMethods assigns fnGetBlockByHash, fnGetBlockByHeight, fnSendTx to use the
// BlockStore from app.Node to load blocks. Assumes app.Node has been set.
func (app *BaseApplication) SetDefaultMethods() {
	app.SetFnGetBlockByHash(func(hash []byte) *comettypes.Block {
		resblock, err := app.NodeClient.BlockByHash(context.Background(), hash)
		if err != nil {
			log.Warnf("cannot fetch block by hash: %v", err)
			return nil
		}
		return resblock.Block
	})

	app.SetFnGetBlockByHeight(func(height int64) *comettypes.Block {
		resblock, err := app.NodeClient.Block(context.Background(), &height)
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
		return app.Node.Mempool().Size()
	})
	app.SetFnMempoolPrune(app.fnMempoolRemoveTxTendermint)
	app.SetFnSendTx(func(tx []byte) (*cometcoretypes.ResultBroadcastTx, error) {
		result, err := app.NodeClient.BroadcastTxSync(context.Background(), tx)
		if err != nil {
			return nil, err
		}
		log.Debugw("broadcast tx",
			"size", len(tx),
			"result",
			func() int {
				if result == nil {
					return -1
				}
				return int(result.Code)
			}(),
			"data", result.Data,
		)
		if result == nil {
			return nil, fmt.Errorf("no result returned from broadcast tx")
		}
		if result.Code != 0 {
			errmsg := string(result.Data)
			err = fmt.Errorf("%s", errmsg)
			return result, err
		}
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
func (app *BaseApplication) SetFnGetBlockByHash(fn func(hash []byte) *comettypes.Block) {
	app.fnGetBlockByHash = fn
}

// SetFnGetBlockByHeight sets the getter for blocks by height
func (app *BaseApplication) SetFnGetBlockByHeight(fn func(height int64) *comettypes.Block) {
	app.fnGetBlockByHeight = fn
}

// SetFnSendTx sets the sendTx method
func (app *BaseApplication) SetFnSendTx(fn func(tx []byte) (*cometcoretypes.ResultBroadcastTx, error)) {
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

// fnMempoolRemoveTxTendermint removes a transaction (identifier by its vochain.TxKey() hash)
// from the Tendermint mempool.
func (app *BaseApplication) fnMempoolRemoveTxTendermint(txKey [comettypes.TxKeySize]byte) error {
	if app.Node == nil {
		log.Errorw(fmt.Errorf("method not assigned"), "mempoolRemoveTxTendermint")
		return nil
	}
	return app.Node.Mempool().RemoveTxByKey(txKey)
}

package vochain

import (
	"context"
	"fmt"

	abcitypes "github.com/cometbft/cometbft/abci/types"
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
	log.Infof("setting default Tendermint methods for blockchain interaction")
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
	app.SetFnBeginBlock(app.fnBeginBlockDefault)
	app.SetFnEndBlock(app.fnEndBlockDefault)
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
	if int32(len(block.Txs)) <= txIndex {
		return nil, ErrTransactionNotFound
	}
	tx := &models.SignedTx{}
	return tx, proto.Unmarshal(block.Txs[txIndex], tx)
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

// fnBeginBlockDefault signals the beginning of a new block. Called prior to any DeliverTxs.
// The header contains the height, timestamp, and more - it exactly matches the
// Tendermint block header.
// The LastCommitInfo and ByzantineValidators can be used to determine rewards and
// punishments for the validators.
func (app *BaseApplication) fnBeginBlockDefault(
	req abcitypes.RequestBeginBlock) abcitypes.ResponseBeginBlock {
	app.State.Rollback()
	app.startBlockTimestamp.Store(req.Header.GetTime().Unix())
	height := uint32(req.Header.GetHeight())
	app.State.SetHeight(height)
	go app.State.CachePurge(height)

	return abcitypes.ResponseBeginBlock{}
}

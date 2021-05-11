package router

import (
	"fmt"

	tmtypes "github.com/tendermint/tendermint/types"
	"go.vocdoni.io/dvote/api"
	"go.vocdoni.io/dvote/log"
	"go.vocdoni.io/dvote/types"
	"go.vocdoni.io/dvote/vochain/scrutinizer/indexertypes"
	models "go.vocdoni.io/proto/build/go/models"
	"google.golang.org/protobuf/proto"
)

func (r *Router) getStats(request routerRequest) {
	var err error
	stats := new(api.VochainStats)
	stats.BlockHeight = r.vocapp.Height()
	stats.BlockTimeStamp = int32(r.vocapp.State.Header(true).Timestamp)
	stats.EntityCount = r.Scrutinizer.EntityCount()
	if stats.EnvelopeCount, err = r.Scrutinizer.GetEnvelopeHeight([]byte{}); err != nil {
		log.Warnf("could not count vote envelopes: %s", err)
	}
	stats.ProcessCount = r.Scrutinizer.ProcessCount([]byte{})
	vals, _ := r.vocapp.State.Validators(true)
	stats.ValidatorCount = len(vals)
	stats.BlockTime = *r.vocinfo.BlockTimes()
	stats.ChainID = r.vocapp.ChainID()
	stats.GenesisTimeStamp = r.vocapp.Node.GenesisDoc().GenesisTime
	stats.Syncing = r.vocapp.IsSynchronizing()

	var response api.MetaResponse
	response.Stats = stats
	if err != nil {
		log.Errorf("could not marshal vochainStats: %s", err)
	}
	request.Send(r.buildReply(request, &response))
}

func (r *Router) getEnvelopeList(request routerRequest) {
	var response api.MetaResponse
	max := request.ListSize
	if max > MaxListSize || max <= 0 {
		max = MaxListSize
	}
	if request.ListSize > MaxListSize {
		r.sendError(request, fmt.Sprintf("listSize overflow, maximum is %d", MaxListSize))
		return
	}
	var err error
	if response.Envelopes, err = r.Scrutinizer.GetEnvelopes(request.ProcessID, request.ListSize, request.From, request.SearchTerm); err != nil {
		r.sendError(request, fmt.Sprintf("cannot get envelope list: (%s)", err))
		return
	}
	request.Send(r.buildReply(request, &response))
}

func (r *Router) getValidatorList(request routerRequest) {
	var response api.MetaResponse
	var err error
	if response.ValidatorList, err = r.vocapp.State.Validators(true); err != nil {
		r.sendError(request, fmt.Sprintf("cannot get validator list: %v", err))
		return
	}
	request.Send(r.buildReply(request, &response))
}

func (r *Router) getBlock(request routerRequest) {
	var response api.MetaResponse
	if request.Height > r.vocapp.Height() {
		r.sendError(request, fmt.Sprintf("block height %d not valid for vochain with height %d", request.Height, r.vocapp.Height()))
		return
	}
	if response.Block = indexertypes.BlockMetadataFromBlockModel(r.Scrutinizer.App.GetBlockByHeight(int64(request.Height))); response.Block == nil {
		r.sendError(request, fmt.Sprintf("cannot get block: no block with height %d", request.Height))
		return
	}
	request.Send(r.buildReply(request, &response))
}

func (r *Router) getBlockByHash(request routerRequest) {
	var response api.MetaResponse
	response.Block = indexertypes.BlockMetadataFromBlockModel(r.Scrutinizer.App.GetBlockByHash(request.Hash))
	if response.Block == nil {
		r.sendError(request, fmt.Sprintf("cannot get block: no block with hash %x", request.Hash))
		return
	}
	request.Send(r.buildReply(request, &response))
}

// TODO improve this function
func (r *Router) getBlockList(request routerRequest) {
	var response api.MetaResponse
	for i := 0; i < request.ListSize; i++ {
		if uint32(request.From)+uint32(i) > r.vocapp.Height() {
			break
		}
		response.BlockList = append(response.BlockList,
			indexertypes.BlockMetadataFromBlockModel(
				r.Scrutinizer.App.GetBlockByHeight(int64(request.From)+int64(i))))
	}
	request.Send(r.buildReply(request, &response))
}

func (r *Router) getTx(request routerRequest) {
	var response api.MetaResponse
	tx, hash, err := r.Scrutinizer.App.GetTxHash(request.Height, request.TxIndex)
	if err != nil {
		r.sendError(request, fmt.Sprintf("cannot get tx: %v", err))
		return
	}
	response.Tx = &indexertypes.TxPackage{
		Tx:          tx.Tx,
		BlockHeight: request.Height,
		Index:       request.TxIndex,
		Hash:        hash,
		Signature:   tx.Signature,
	}
	request.Send(r.buildReply(request, &response))
}

func (r *Router) getTxListForBlock(request routerRequest) {
	var response api.MetaResponse
	block := r.vocapp.Node.BlockStore().LoadBlock(int64(request.Height))
	if block == nil {
		r.sendError(request, "cannot get tx list: block does not exist")
		return
	}
	if request.ListSize > MaxListSize || request.ListSize <= 0 {
		request.ListSize = MaxListSize
	}
	maxIndex := request.From + request.ListSize
	for i := request.From; i < maxIndex && i < len(block.Txs); i++ {
		signedTx := new(models.SignedTx)
		tx := new(models.Tx)
		var err error
		if err = proto.Unmarshal(block.Txs[i], signedTx); err != nil {
			r.sendError(request, fmt.Sprintf("cannot get signed tx: %v", err))
			return
		}
		if err = proto.Unmarshal(signedTx.Tx, tx); err != nil {
			r.sendError(request, fmt.Sprintf("cannot get tx: %v", err))
			return
		}
		var txType string
		switch tx.Payload.(type) {
		case *models.Tx_Vote:
			txType = types.TxVote
		case *models.Tx_NewProcess:
			txType = types.TxNewProcess
		case *models.Tx_Admin:
			txType = tx.Payload.(*models.Tx_Admin).Admin.GetTxtype().String()
		case *models.Tx_SetProcess:
			txType = types.TxSetProcess
		default:
			txType = "unknown"
		}
		response.TxList = append(response.TxList, &indexertypes.TxMetadata{
			Type:        txType,
			BlockHeight: request.Height,
			Index:       int32(i),
			Hash:        tmtypes.Tx(block.Txs[i]).Hash(),
		})
	}
	request.Send(r.buildReply(request, &response))
}

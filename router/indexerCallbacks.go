package router

import (
	"encoding/hex"
	"fmt"

	tmtypes "github.com/tendermint/tendermint/types"
	"go.vocdoni.io/dvote/api"
	"go.vocdoni.io/dvote/log"
	"go.vocdoni.io/dvote/types"
	"go.vocdoni.io/dvote/vochain/scrutinizer/indexertypes"
	models "go.vocdoni.io/proto/build/go/models"
	"google.golang.org/protobuf/proto"
)

func (r *Router) getStats(request RouterRequest) {
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
	if err := request.Send(r.BuildReply(request, &response)); err != nil {
		log.Warnf("error sending response: %s", err)
	}
}

func (r *Router) getEnvelopeList(request RouterRequest) {
	var response api.MetaResponse
	max := request.ListSize
	if max > MaxListSize || max <= 0 {
		max = MaxListSize
	}
	var err error
	if response.Envelopes, err = r.Scrutinizer.GetEnvelopes(
		request.ProcessID, max, request.From, request.SearchTerm); err != nil {
		r.SendError(request, fmt.Sprintf("cannot get envelope list: (%s)", err))
		return
	}
	if err := request.Send(r.BuildReply(request, &response)); err != nil {
		log.Warnf("error sending response: %s", err)
	}
}

func (r *Router) getValidatorList(request RouterRequest) {
	var response api.MetaResponse
	var err error
	if response.ValidatorList, err = r.vocapp.State.Validators(true); err != nil {
		r.SendError(request, fmt.Sprintf("cannot get validator list: %v", err))
		return
	}
	if err := request.Send(r.BuildReply(request, &response)); err != nil {
		log.Warnf("error sending response: %s", err)
	}
}

func (r *Router) getBlock(request RouterRequest) {
	var response api.MetaResponse
	if request.Height > r.vocapp.Height() {
		r.SendError(request, fmt.Sprintf(
			"block height %d not valid for vochain with height %d", request.Height, r.vocapp.Height()))
		return
	}
	if response.Block = blockMetadataFromBlockModel(
		r.Scrutinizer.App.GetBlockByHeight(int64(request.Height)), false, true); response.Block == nil {
		r.SendError(request, fmt.Sprintf("cannot get block: no block with height %d", request.Height))
		return
	}
	if err := request.Send(r.BuildReply(request, &response)); err != nil {
		log.Warnf("error sending response: %s", err)
	}
}

func (r *Router) getBlockByHash(request RouterRequest) {
	var response api.MetaResponse
	response.Block = blockMetadataFromBlockModel(
		r.Scrutinizer.App.GetBlockByHash(request.Hash), true, false)
	if response.Block == nil {
		r.SendError(request, fmt.Sprintf("cannot get block: no block with hash %x", request.Hash))
		return
	}
	if err := request.Send(r.BuildReply(request, &response)); err != nil {
		log.Warnf("error sending response: %s", err)
	}
}

// TODO improve this function
func (r *Router) getBlockList(request RouterRequest) {
	var response api.MetaResponse
	for i := 0; i < request.ListSize; i++ {
		if uint32(request.From)+uint32(i) > r.vocapp.Height() {
			break
		}
		response.BlockList = append(response.BlockList,
			blockMetadataFromBlockModel(r.Scrutinizer.App.
				GetBlockByHeight(int64(request.From)+int64(i)), true, true))
	}
	if err := request.Send(r.BuildReply(request, &response)); err != nil {
		log.Warnf("error sending response: %s", err)
	}
}

func (r *Router) getTx(request RouterRequest) {
	var response api.MetaResponse
	tx, hash, err := r.Scrutinizer.App.GetTxHash(request.Height, request.TxIndex)
	if err != nil {
		r.SendError(request, fmt.Sprintf("cannot get tx: %v", err))
		return
	}
	response.Tx = &indexertypes.TxPackage{
		Tx:        tx.Tx,
		Index:     request.TxIndex,
		Hash:      hash,
		Signature: tx.Signature,
	}
	if err := request.Send(r.BuildReply(request, &response)); err != nil {
		log.Warnf("error sending response: %s", err)
	}
}

func (r *Router) getTxListForBlock(request RouterRequest) {
	var response api.MetaResponse
	block := r.vocapp.Node.BlockStore().LoadBlock(int64(request.Height))
	if block == nil {
		r.SendError(request, "cannot get tx list: block does not exist")
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
			r.SendError(request, fmt.Sprintf("cannot get signed tx: %v", err))
			return
		}
		if err = proto.Unmarshal(signedTx.Tx, tx); err != nil {
			r.SendError(request, fmt.Sprintf("cannot get tx: %v", err))
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
			Type:  txType,
			Index: int32(i),
			Hash:  tmtypes.Tx(block.Txs[i]).Hash(),
		})
	}
	if err := request.Send(r.BuildReply(request, &response)); err != nil {
		log.Warnf("error sending response: %s", err)
	}
}

func (r *Router) getProcessSummary(request RouterRequest) {
	var response api.MetaResponse
	if len(request.ProcessID) != types.ProcessIDsize {
		r.SendError(request, "cannot get envelope status: (malformed processId)")
		return
	}

	// Get process info
	procInfo, err := r.Scrutinizer.ProcessInfo(request.ProcessID)
	if err != nil {
		log.Warn(err)
		r.SendError(request, err.Error())
		return
	}

	// Get total number of votes (including invalid/null)
	eh, err := r.Scrutinizer.GetEnvelopeHeight(request.ProcessID)
	if err != nil {
		response.Message = fmt.Sprintf("cannot get envelope height: %v", err)
		if err := request.Send(r.BuildReply(request, &response)); err != nil {
			log.Warnf("error sending response: %s", err)
		}
		return
	}
	votes := uint32(eh)

	response.ProcessSummary = &api.ProcessSummary{
		BlockCount:      procInfo.EndBlock - procInfo.StartBlock,
		EntityID:        hex.EncodeToString(procInfo.EntityID),
		EntityIndex:     procInfo.EntityIndex,
		EnvelopeHeight:  &votes,
		Metadata:        procInfo.Metadata,
		SourceNetworkID: procInfo.SourceNetworkId,
		StartBlock:      procInfo.StartBlock,
		State:           models.ProcessStatus(procInfo.Status).String(),
		EnvelopeType:    procInfo.Envelope,
	}
	if err := request.Send(r.BuildReply(request, &response)); err != nil {
		log.Warnf("error sending response: %s", err)
	}
}

func blockMetadataFromBlockModel(
	block *tmtypes.Block, includeHeight, includeHash bool) *indexertypes.BlockMetadata {
	if block == nil {
		return nil
	}
	b := new(indexertypes.BlockMetadata)
	if includeHeight {
		b.Height = uint32(block.Height)
	}
	b.Timestamp = block.Time
	if includeHash {
		b.Hash = block.Hash().Bytes()
	}
	b.NumTxs = uint64(len(block.Txs))
	b.LastBlockHash = block.LastBlockID.Hash.Bytes()
	b.ProposerAddress = block.ProposerAddress.Bytes()
	return b
}

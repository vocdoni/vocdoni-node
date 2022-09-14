package rpcapi

import (
	"context"
	"fmt"

	tmtypes "github.com/tendermint/tendermint/types"
	"go.vocdoni.io/dvote/api"
	"go.vocdoni.io/dvote/vochain/scrutinizer/indexertypes"
	models "go.vocdoni.io/proto/build/go/models"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"
)

func (r *RPCAPI) getStats(request *api.APIrequest) (*api.APIresponse, error) {
	var err error
	stats := new(api.VochainStats)
	height := r.vocapp.Height()
	stats.BlockHeight = uint32(height)
	block := r.vocapp.GetBlockByHeight(int64(height))

	if block == nil {
		return nil, fmt.Errorf("could not get block at height: (%v)", height)
	}
	stats.BlockTimeStamp = int32(block.Header.Time.Unix())
	stats.EntityCount = int64(r.scrutinizer.EntityCount())
	if stats.EnvelopeCount, err = r.scrutinizer.GetEnvelopeHeight([]byte{}); err != nil {
		return nil, fmt.Errorf("could not count vote envelopes: %w", err)
	}
	stats.ProcessCount = int64(r.scrutinizer.ProcessCount([]byte{}))
	if stats.TransactionCount, err = r.scrutinizer.TransactionCount(); err != nil {
		return nil, fmt.Errorf("could not count transactions: %w", err)
	}
	vals, _ := r.vocapp.State.Validators(true)
	stats.ValidatorCount = len(vals)
	stats.BlockTime = *r.vocinfo.BlockTimes()
	stats.ChainID = r.vocapp.ChainID()
	if r.vocapp.Node != nil {
		gn, err := r.vocapp.Node.Genesis(context.Background())
		if err != nil {
			return nil, err
		}
		stats.GenesisTimeStamp = gn.Genesis.GenesisTime
	}
	stats.Syncing = r.vocapp.IsSynchronizing()

	var response api.APIresponse
	response.Stats = stats
	return &response, nil
}

func (r *RPCAPI) getEnvelopeList(request *api.APIrequest) (*api.APIresponse, error) {
	var response api.APIresponse
	max := request.ListSize
	if max > MaxListSize || max <= 0 {
		max = MaxListSize
	}
	var err error
	if response.Envelopes, err = r.scrutinizer.GetEnvelopes(
		request.ProcessID, max, request.From, request.SearchTerm); err != nil {
		return nil, fmt.Errorf("cannot get envelope list: %w", err)
	}
	return &response, nil
}

func (r *RPCAPI) getValidatorList(request *api.APIrequest) (*api.APIresponse, error) {
	var response api.APIresponse
	var err error
	if response.ValidatorList, err = r.vocapp.State.Validators(true); err != nil {
		return nil, fmt.Errorf("cannot get validator list: %w", err)
	}
	return &response, err
}

func (r *RPCAPI) getOracleList(request *api.APIrequest) (*api.APIresponse, error) {
	var response api.APIresponse
	var err error
	oracleList, err := r.vocapp.State.Oracles(true)
	if err != nil {
		return nil, fmt.Errorf("cannot get oracle list")
	}
	response.OracleList = new([]string)
	*response.OracleList = make([]string, len(oracleList))
	for _, oracle := range oracleList {
		*response.OracleList = append(*response.OracleList, oracle.String())
	}
	return &response, err
}

func (r *RPCAPI) getBlock(request *api.APIrequest) (*api.APIresponse, error) {
	var response api.APIresponse
	if request.Height > r.vocapp.Height() {
		return nil, fmt.Errorf("block height %d not valid for vochain with height %d", request.Height, r.vocapp.Height())
	}
	if response.Block = blockMetadataFromBlockModel(
		r.scrutinizer.App.GetBlockByHeight(int64(request.Height)), false, true); response.Block == nil {
		return nil, fmt.Errorf("cannot get block: no block with height %d", request.Height)
	}
	return &response, nil
}

func (r *RPCAPI) getBlockByHash(request *api.APIrequest) (*api.APIresponse, error) {
	var response api.APIresponse
	response.Block = blockMetadataFromBlockModel(
		r.scrutinizer.App.GetBlockByHash(request.Hash), true, false)
	if response.Block == nil {
		return nil, fmt.Errorf("cannot get block: no block with hash %x", request.Hash)
	}
	return &response, nil
}

// TODO improve this function
func (r *RPCAPI) getBlockList(request *api.APIrequest) (*api.APIresponse, error) {
	var response api.APIresponse
	for i := 0; i < request.ListSize; i++ {
		if uint32(request.From)+uint32(i) > r.vocapp.Height() {
			break
		}
		blkMeta := blockMetadataFromBlockModel(
			r.scrutinizer.App.GetBlockByHeight(
				int64(request.From)+int64(i),
			),
			true,
			true,
		)
		if blkMeta != nil {
			response.BlockList = append(response.BlockList, blkMeta)
		}
	}
	return &response, nil
}

func protoFormat(tx []byte) string {
	ptx := models.Tx{}
	if err := proto.Unmarshal(tx, &ptx); err != nil {
		return ""
	}
	pj := protojson.MarshalOptions{
		Multiline: false,
		Indent:    "",
	}
	return pj.Format(&ptx)
}

func (r *RPCAPI) getTx(request *api.APIrequest) (*api.APIresponse, error) {
	var response api.APIresponse
	tx, hash, err := r.scrutinizer.App.GetTxHash(request.Height, request.TxIndex)
	if err != nil {
		return nil, fmt.Errorf("cannot get tx: %w", err)
	}
	response.Payload = protoFormat(tx.Tx)
	ptx := models.Tx{}
	proto.Unmarshal(tx.Tx, &ptx)

	response.Tx = &indexertypes.TxPackage{
		Tx:        tx.Tx,
		Index:     new(int32),
		Hash:      hash,
		Signature: tx.Signature,
	}
	*response.Tx.Index = request.TxIndex
	return &response, nil
}

// TODO @pau: the transaction payload is returned twice, base64 and json
func (r *RPCAPI) getTxById(request *api.APIrequest) (*api.APIresponse, error) {
	var response api.APIresponse
	txRef, err := r.scrutinizer.GetTxReference(uint64(request.ID))
	if err != nil {
		return nil, fmt.Errorf("cannot get tx reference with ID %d: %w", request.ID, err)
	}
	tx, hash, err := r.scrutinizer.App.GetTxHash(txRef.BlockHeight, txRef.TxBlockIndex)
	if err != nil {
		return nil, fmt.Errorf("cannot get tx: %w", err)
	}
	response.Payload = protoFormat(tx.Tx)
	response.Tx = &indexertypes.TxPackage{
		Tx:          tx.Tx,
		ID:          uint32(txRef.Index),
		Index:       new(int32),
		BlockHeight: txRef.BlockHeight,
		Hash:        hash,
		Signature:   tx.Signature,
	}
	*response.Tx.Index = txRef.TxBlockIndex
	return &response, nil
}

func (r *RPCAPI) getTxByHash(request *api.APIrequest) (*api.APIresponse, error) {
	var response api.APIresponse
	txRef, err := r.scrutinizer.GetTxHashReference(request.Hash)
	if err != nil {
		return nil, fmt.Errorf("tx %x not found: %w", request.Hash, err)
	}
	tx, err := r.scrutinizer.App.GetTx(txRef.BlockHeight, txRef.TxBlockIndex)
	if err != nil {
		return nil, fmt.Errorf("cannot get tx: %w", err)
	}
	response.Payload = protoFormat(tx.Tx)
	response.Tx = &indexertypes.TxPackage{
		Tx:          tx.Tx,
		ID:          uint32(txRef.Index),
		Index:       new(int32),
		BlockHeight: txRef.BlockHeight,
		Hash:        request.Hash,
		Signature:   tx.Signature,
	}
	*response.Tx.Index = txRef.TxBlockIndex
	return &response, nil
}

func (r *RPCAPI) getTxListForBlock(request *api.APIrequest) (*api.APIresponse, error) {
	var response api.APIresponse
	block := r.vocapp.GetBlockByHeight(int64(request.Height))
	if block == nil {
		return nil, fmt.Errorf("cannot get tx list: (block does not exist)")
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
			return nil, fmt.Errorf("cannot get signed tx: %w", err)
		}
		if err = proto.Unmarshal(signedTx.Tx, tx); err != nil {
			return nil, fmt.Errorf("cannot get tx: %w", err)
		}
		txType := string(
			tx.ProtoReflect().WhichOneof(
				tx.ProtoReflect().Descriptor().Oneofs().Get(0)).Name())

		response.TxList = append(response.TxList, &indexertypes.TxMetadata{
			Type:  txType,
			Index: int32(i),
			Hash:  block.Txs[i].Hash(),
		})
	}
	return &response, nil
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

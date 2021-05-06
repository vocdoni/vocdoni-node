package types

import (
	"fmt"
	"math/big"
	"reflect"
	"strings"
	"time"

	tmtypes "github.com/tendermint/tendermint/types"
	"go.vocdoni.io/dvote/types"
)

// VotePackage represents the payload of a vote (usually base64 encoded)
type VotePackage struct {
	Nonce string `json:"nonce,omitempty"`
	Votes []int  `json:"votes"`
}

// VoteReference holds the db reference for a single vote
type VoteReference struct {
	Nullifier    types.HexBytes `badgerholdKey:"Nullifier"`
	ProcessID    types.HexBytes `badgerholdIndex:"ProcessID"`
	Height       uint32
	Weight       *big.Int
	TxIndex      int32
	CreationTime time.Time
}

// EnvelopeMetadata contains vote information for the EnvelopeList api
type EnvelopeMetadata struct {
	ProcessId types.HexBytes `json:"process_id"`
	Nullifier types.HexBytes `json:"nullifier"`
	TxIndex   int32          `json:"tx_index"`
	Height    uint32         `json:"height"`
	TxHash    types.HexBytes `json:"tx_hash"`
}

// EnvelopePackage contains a VoteEnvelope and auxiliary information for the Envelope api
type EnvelopePackage struct {
	EncryptionKeyIndexes []uint32         `json:"encryption_key_indexes"`
	Meta                 EnvelopeMetadata `json:"meta"`
	Nonce                types.HexBytes   `json:"nonce"`
	Signature            types.HexBytes   `json:"signature"`
	VotePackage          []byte           `json:"vote_package"`
	Weight               string           `json:"weight"`
}

// VochainStats contains information about the current Vochain statistics and state
type VochainStats struct {
	BlockHeight      uint32    `json:"block_height"`
	EntityCount      int64     `json:"entity_count"`
	EnvelopeCount    uint64    `json:"envelope_count"`
	ProcessCount     int64     `json:"process_count"`
	ValidatorCount   int       `json:"validator_count"`
	BlockTime        [5]int32  `json:"block_time"`
	BlockTimeStamp   int32     `json:"block_time_stamp"`
	ChainID          string    `json:"chain_id"`
	GenesisTimeStamp time.Time `json:"genesis_time_stamp"`
	Syncing          bool      `json:"syncing"`
}

// TxPackage contains a SignedTx and auxiliary information for the Transaction api
type TxPackage struct {
	Tx          []byte         `json:"tx"`
	BlockHeight uint32         `json:"block_height"`
	Index       int32          `json:"index"`
	Hash        types.HexBytes `json:"hash"`
	Signature   types.HexBytes `json:"signature"`
}

// TxMetadata contains tx information for the TransactionList api
type TxMetadata struct {
	Type        string         `json:"type"`
	BlockHeight uint32         `json:"block_height"`
	Index       int32          `json:"index"`
	Hash        types.HexBytes `json:"hash"`
}

// BlockMetadata contains the metadata for a single tendermint block
type BlockMetadata struct {
	ChainId         string         `json:"chain_id"`
	Height          uint32         `json:"height"`
	Timestamp       time.Time      `json:"timestamp"`
	Hash            types.HexBytes `json:"hash"`
	NumTxs          uint64         `json:"num_txs"`
	LastBlockHash   types.HexBytes `json:"last_block_hash"`
	ProposerAddress types.HexBytes `json:"proposer_address"`
}

func (b *BlockMetadata) String() string {
	v := reflect.ValueOf(b)
	t := v.Type()
	var builder strings.Builder
	builder.WriteString("{")
	for i := 0; i < t.NumField(); i++ {
		fv := v.Field(i)
		if fv.IsZero() {
			// omit zero values
			continue
		}
		if builder.Len() > 1 {
			builder.WriteString(" ")
		}
		ft := t.Field(i)
		builder.WriteString(ft.Name)
		builder.WriteString(":")
		if ft.Type.Kind() == reflect.Slice && ft.Type.Elem().Kind() == reflect.Uint8 {
			// print []byte as hexadecimal
			fmt.Fprintf(&builder, "%x", fv.Bytes())
		} else {
			fv = reflect.Indirect(fv) // print *T as T
			fmt.Fprintf(&builder, "%v", fv.Interface())
		}
	}
	builder.WriteString("}")
	return builder.String()
}

func BlockMetadataFromBlockModel(block *tmtypes.Block) *BlockMetadata {
	if block == nil {
		return nil
	}
	b := new(BlockMetadata)
	b.ChainId = block.ChainID
	b.Height = uint32(block.Height)
	b.Timestamp = block.Time
	b.Hash = block.Hash().Bytes()
	b.NumTxs = uint64(len(block.Txs))
	b.LastBlockHash = block.LastBlockID.Hash.Bytes()
	b.ProposerAddress = block.ProposerAddress.Bytes()
	return b
}

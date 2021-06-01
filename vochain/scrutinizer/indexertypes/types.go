package indexertypes

import (
	"encoding/json"
	"fmt"
	"math/big"
	"reflect"
	"strings"
	"time"

	"go.vocdoni.io/dvote/types"
	"go.vocdoni.io/proto/build/go/models"
)

const (
	Entities     = 0
	Processes    = 1
	Envelopes    = 2
	Transactions = 3
)

// CountStore holds the count of envelopes, processes, entities, or transactions
type CountStore struct {
	Type  int `badgerholdKey:"Type"`
	Count uint64
}

// Process represents an election process handled by the Vochain.
// The scrutinizer Process data type is different from the vochain state data type
// since it is optimized for querying purposes and not for keeping a shared consensus state.
type Process struct {
	ID                types.HexBytes             `badgerholdKey:"ID" json:"processId"`
	EntityID          types.HexBytes             `badgerholdIndex:"EntityID" json:"entityId"`
	EntityIndex       uint32                     `json:"entityIndex"`
	StartBlock        uint32                     `json:"startBlock"`
	EndBlock          uint32                     `badgerholdIndex:"EndBlock" json:"endBlock"`
	Rheight           uint32                     `badgerholdIndex:"Rheight" json:"-"`
	CensusRoot        types.HexBytes             `json:"censusRoot"`
	CensusURI         string                     `json:"censusURI"`
	Metadata          string                     `json:"metadata"`
	CensusOrigin      int32                      `json:"censusOrigin"`
	Status            int32                      `badgerholdIndex:"Status" json:"status"`
	Namespace         uint32                     `badgerholdIndex:"Namespace" json:"namespace"`
	Envelope          *models.EnvelopeType       `json:"envelopeType"`
	Mode              *models.ProcessMode        `json:"processMode"`
	VoteOpts          *models.ProcessVoteOptions `json:"voteOptions"`
	PrivateKeys       []string                   `json:"-"`
	PublicKeys        []string                   `json:"-"`
	QuestionIndex     uint32                     `json:"questionIndex"`
	CreationTime      time.Time                  `json:"creationTime"`
	HaveResults       bool                       `json:"haveResults"`
	FinalResults      bool                       `json:"finalResults"`
	SourceBlockHeight uint64                     `json:"sourceBlockHeight"`
	SourceNetworkId   string                     `badgerholdIndex:"SourceNetworkId" json:"sourceNetworkId"`
}

func (p Process) String() string {
	b, _ := json.Marshal(p)
	return string(b)
}

// Entity holds the db reference for an entity
type Entity struct {
	ID           types.HexBytes `badgerholdKey:"ID"`
	ProcessCount uint32
	CreationTime time.Time
}

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

// TxPackage contains a SignedTx and auxiliary information for the Transaction api
type TxPackage struct {
	Tx          []byte         `json:"tx"`
	Height      uint32         `json:"height,omitempty"`
	BlockHeight uint32         `json:"block_height,omitempty"`
	Index       int32          `json:"index,omitempty"`
	Hash        types.HexBytes `json:"hash"`
	Signature   types.HexBytes `json:"signature"`
}

// TxMetadata contains tx information for the TransactionList api
type TxMetadata struct {
	Type        string         `json:"type"`
	BlockHeight uint32         `json:"block_height,omitempty"`
	Index       int32          `json:"index"`
	Hash        types.HexBytes `json:"hash"`
}

// TxReference holds the db reference for a single transaction
type TxReference struct {
	Index        uint64 `badgerholdKey:"Index"`
	BlockHeight  uint32
	TxBlockIndex uint32
}

// BlockMetadata contains the metadata for a single tendermint block
type BlockMetadata struct {
	Height          uint32         `json:"height,omitempty"`
	Timestamp       time.Time      `json:"timestamp"`
	Hash            types.HexBytes `json:"hash,omitempty"`
	NumTxs          uint64         `json:"num_txs"`
	LastBlockHash   types.HexBytes `json:"last_block_hash"`
	ProposerAddress types.HexBytes `json:"proposer_address"`
}

// String prints the BlockMetadata in a human-readable format
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

// ________________________ CALLBACKS DATA STRUCTS ________________________

// ScrutinizerOnProcessData holds the required data for callbacks when
// a new process is added into the vochain.
type ScrutinizerOnProcessData struct {
	EntityID  []byte
	ProcessID []byte
}

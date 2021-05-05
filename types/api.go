package types

import (
	"encoding/json"
	"fmt"
	"math/big"
	"reflect"
	"strings"
	"time"

	tmtypes "github.com/tendermint/tendermint/types"
	"go.vocdoni.io/proto/build/go/models"
)

// MessageRequest holds a decoded request but does not decode the body
type RequestMessage struct {
	MetaRequest json.RawMessage `json:"request"`

	ID        string   `json:"id"`
	Signature HexBytes `json:"signature"`
}

// MetaRequest contains all of the possible request fields.
// Fields must be in alphabetical order
type MetaRequest struct {
	CensusID     string   `json:"censusId,omitempty"`
	CensusURI    string   `json:"censusUri,omitempty"`
	CensusKey    []byte   `json:"censusKey,omitempty"`
	CensusKeys   [][]byte `json:"censusKeys,omitempty"`
	CensusValue  []byte   `json:"censusValue,omitempty"`
	CensusValues [][]byte `json:"censusValues,omitempty"`
	CensusDump   []byte   `json:"censusDump,omitempty"`
	Content      []byte   `json:"content,omitempty"`
	Digested     bool     `json:"digested,omitempty"`
	EntityId     HexBytes `json:"entityId,omitempty"`
	Height       uint32   `json:"height,omitempty"`
	From         int      `json:"from,omitempty"`
	ListSize     int      `json:"listSize,omitempty"`
	Method       string   `json:"method"`
	Name         string   `json:"name,omitempty"`
	Namespace    uint32   `json:"namespace,omitempty"`
	Nullifier    HexBytes `json:"nullifier,omitempty"`
	Payload      []byte   `json:"payload,omitempty"`
	ProcessID    HexBytes `json:"processId,omitempty"`
	ProofData    HexBytes `json:"proofData,omitempty"`
	PubKeys      []string `json:"pubKeys,omitempty"`
	RootHash     HexBytes `json:"rootHash,omitempty"`
	SearchTerm   string   `json:"searchTerm,omitempty"`
	Signature    HexBytes `json:"signature,omitempty"`
	Status       string   `json:"status,omitempty"`
	Timestamp    int32    `json:"timestamp"`
	TxIndex      int32    `json:"txIndex,omitempty"`
	Type         string   `json:"type,omitempty"`
	URI          string   `json:"uri,omitempty"`
	WithResults  bool     `json:"withResults,omitempty"`
}

func (r MetaRequest) String() string {
	v := reflect.ValueOf(r)
	t := v.Type()
	var b strings.Builder
	b.WriteString(fmt.Sprintf("%s:{", r.Method))
	first := true
	for i := 0; i < t.NumField(); i++ {
		fv := v.Field(i)
		if fv.IsZero() {
			// omit zero values
			continue
		}
		ft := t.Field(i)
		if ft.Name == "Method" {
			continue
		}
		if first {
			first = false
		} else {
			b.WriteString(" ")
		}
		b.WriteString(ft.Name)
		b.WriteString(":")
		if ft.Type.Kind() == reflect.Slice && ft.Type.Elem().Kind() == reflect.Uint8 {
			// print []byte as hexadecimal
			fmt.Fprintf(&b, "%x", fv.Bytes())
		} else if ft.Type.Kind() == reflect.Slice && ft.Type.Elem().Kind() == reflect.Slice &&
			ft.Type.Elem().Elem().Kind() == reflect.Uint8 {
			for _, v := range fv.Interface().([][]byte) {
				fmt.Fprintf(&b, "%x ", v)
			}
		} else {
			fv = reflect.Indirect(fv) // print *T as T
			fmt.Fprintf(&b, "%v", fv.Interface())
		}
	}
	b.WriteString("}")
	return b.String()
}

// ResponseMessage wraps an api response
type ResponseMessage struct {
	MetaResponse json.RawMessage `json:"response"`

	ID        string   `json:"id"`
	Signature HexBytes `json:"signature"`
}

// MetaResponse contains all of the possible request fields.
// Fields must be in alphabetical order
// Those fields with valid zero-values (such as bool) must be pointers
type MetaResponse struct {
	APIList              []string            `json:"apiList,omitempty"`
	Block                *BlockMetadata      `json:"block,omitempty"`
	BlockList            []*BlockMetadata    `json:"blockList,omitempty"`
	BlockTime            *[5]int32           `json:"blockTime,omitempty"`
	BlockTimestamp       int32               `json:"blockTimestamp,omitempty"`
	CensusID             string              `json:"censusId,omitempty"`
	CensusList           []string            `json:"censusList,omitempty"`
	CensusKeys           [][]byte            `json:"censusKeys,omitempty"`
	CensusValues         []HexBytes          `json:"censusValues,omitempty"`
	CensusDump           []byte              `json:"censusDump,omitempty"`
	CommitmentKeys       []Key               `json:"commitmentKeys,omitempty"`
	Content              []byte              `json:"content,omitempty"`
	CreationTime         int64               `json:"creationTime,omitempty"`
	EncryptionPrivKeys   []Key               `json:"encryptionPrivKeys,omitempty"`
	EncryptionPublicKeys []Key               `json:"encryptionPubKeys,omitempty"`
	EntityID             string              `json:"entityId,omitempty"`
	EntityIDs            []string            `json:"entityIds,omitempty"`
	Envelope             *EnvelopePackage    `json:"envelope,omitempty"`
	Envelopes            []*EnvelopeMetadata `json:"envelopes,omitempty"`
	Files                []byte              `json:"files,omitempty"`
	Final                *bool               `json:"final,omitempty"`
	Finished             *bool               `json:"finished,omitempty"`
	Health               int32               `json:"health,omitempty"`
	Height               *uint32             `json:"height,omitempty"`
	InvalidClaims        []int               `json:"invalidClaims,omitempty"`
	Message              string              `json:"message,omitempty"`
	Nullifier            string              `json:"nullifier,omitempty"`
	Nullifiers           *[]string           `json:"nullifiers,omitempty"`
	Ok                   bool                `json:"ok"`
	Paused               *bool               `json:"paused,omitempty"`
	Payload              string              `json:"payload,omitempty"`
	ProcessID            HexBytes            `json:"processId,omitempty"`
	ProcessIDs           []string            `json:"processIds,omitempty"`
	Process              *Process            `json:"process,omitempty"`
	ProcessList          []string            `json:"processList,omitempty"`
	Registered           *bool               `json:"registered,omitempty"`
	Request              string              `json:"request"`
	Results              [][]string          `json:"results,omitempty"`
	RevealKeys           []Key               `json:"revealKeys,omitempty"`
	Root                 HexBytes            `json:"root,omitempty"`
	Siblings             HexBytes            `json:"siblings,omitempty"`
	Size                 *int64              `json:"size,omitempty"`
	State                string              `json:"state,omitempty"`
	Stats                *VochainStats       `json:"stats,omitempty"`
	Timestamp            int32               `json:"timestamp"`
	Type                 string              `json:"type,omitempty"`
	Tx                   *TxPackage          `json:"tx,omitempty"`
	TxList               []*TxMetadata       `json:"txList,omitempty"`
	URI                  string              `json:"uri,omitempty"`
	ValidatorList        []*models.Validator `json:"validatorlist,omitempty"`
	ValidProof           *bool               `json:"validProof,omitempty"`
	Weight               string              `json:"weight,omitempty"`
}

func (r MetaResponse) String() string {
	v := reflect.ValueOf(r)
	t := v.Type()
	var b strings.Builder
	b.WriteString("{")
	for i := 0; i < t.NumField(); i++ {
		fv := v.Field(i)
		if fv.IsZero() {
			// omit zero values
			continue
		}
		if b.Len() > 1 {
			b.WriteString(" ")
		}
		ft := t.Field(i)
		b.WriteString(ft.Name)
		b.WriteString(":")
		if ft.Type.Kind() == reflect.Slice && ft.Type.Elem().Kind() == reflect.Uint8 {
			// print []byte as hexadecimal
			fmt.Fprintf(&b, "%x", fv.Bytes())
		} else {
			fv = reflect.Indirect(fv) // print *T as T
			fmt.Fprintf(&b, "%v", fv.Interface())
		}
	}
	b.WriteString("}")
	return b.String()
}

// SetError sets the MetaResponse's Ok field to false, and Message to a string
// representation of v. Usually, v's type will be error or string.
func (r *MetaResponse) SetError(v interface{}) {
	r.Ok = false
	r.Message = fmt.Sprintf("%s", v)
}

type CensusDump struct {
	RootHash []byte `json:"rootHash"`
	Data     []byte `json:"data"`
}

// VotePackage represents the payload of a vote (usually base64 encoded)
type VotePackage struct {
	Nonce string `json:"nonce,omitempty"`
	Votes []int  `json:"votes"`
}

type Key struct {
	Idx int    `json:"idx"`
	Key string `json:"key"`
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

// EnvelopePackage contains a VoteEnvelope and auxiliary information for the Envelope api
type EnvelopePackage struct {
	EncryptionKeyIndexes []uint32         `json:"encryption_key_indexes"`
	Meta                 EnvelopeMetadata `json:"meta"`
	Nonce                HexBytes         `json:"nonce"`
	Signature            HexBytes         `json:"signature"`
	VotePackage          []byte           `json:"vote_package"`
	Weight               *big.Int         `json:"weight"`
}

// EnvelopeMetadata contains vote information for the EnvelopeList api
type EnvelopeMetadata struct {
	ProcessId HexBytes `json:"process_id"`
	Nullifier HexBytes `json:"nullifier"`
	TxIndex   int32    `json:"tx_index"`
	Height    uint32   `json:"height"`
	TxHash    HexBytes `json:"tx_hash"`
}

// TxPackage contains a SignedTx and auxiliary information for the Transaction api
type TxPackage struct {
	Tx          []byte   `json:"tx"`
	BlockHeight uint32   `json:"block_height"`
	Index       int32    `json:"index"`
	Hash        HexBytes `json:"hash"`
	Signature   HexBytes `json:"signature"`
}

// TxMetadata contains tx information for the TransactionList api
type TxMetadata struct {
	Type        string   `json:"type"`
	BlockHeight uint32   `json:"block_height"`
	Index       int32    `json:"index"`
	Hash        HexBytes `json:"hash"`
}

// BlockMetadata contains the metadata for a single tendermint block
type BlockMetadata struct {
	ChainId         string    `json:"chain_id"`
	Height          uint32    `json:"height"`
	Timestamp       time.Time `json:"timestamp"`
	Hash            HexBytes  `json:"hash"`
	NumTxs          uint64    `json:"num_txs"`
	LastBlockHash   HexBytes  `json:"last_block_hash"`
	ProposerAddress HexBytes  `json:"proposer_address"`
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

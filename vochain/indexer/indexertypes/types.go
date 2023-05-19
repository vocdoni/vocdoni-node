package indexertypes

import (
	"encoding/json"
	"fmt"
	"reflect"
	"strings"
	"time"

	"go.vocdoni.io/dvote/log"
	"go.vocdoni.io/dvote/types"
	indexerdb "go.vocdoni.io/dvote/vochain/indexer/db"
	"go.vocdoni.io/dvote/vochain/results"
	"go.vocdoni.io/dvote/vochain/state"
	"go.vocdoni.io/proto/build/go/models"
	"google.golang.org/protobuf/proto"
)

const (
	// CountStoreEntities is the key for the entity count db reference
	CountStoreEntities = uint8(iota)
	// CountStoreProcesses is the key for the process count db reference
	CountStoreProcesses
	// CountStoreEnvelopes is the key for the envelope count db reference
	CountStoreEnvelopes
	// CountStoreTransactions is the key for the transaction count db reference
	CountStoreTransactions
)

// CountStore holds the count of envelopes, processes, entities, or transactions
type CountStore struct {
	Type  uint8
	Count uint64
}

// Process represents an election process handled by the Vochain.
// The indexer Process data type is different from the vochain state data type
// since it is optimized for querying purposes and not for keeping a shared consensus state.
type Process struct {
	ID                types.HexBytes             `json:"processId"`
	EntityID          types.HexBytes             `json:"entityId"`
	StartBlock        uint32                     `json:"startBlock"`
	EndBlock          uint32                     `json:"endBlock"`
	CensusRoot        types.HexBytes             `json:"censusRoot"`
	RollingCensusRoot types.HexBytes             `json:"rollingCensusRoot"`
	CensusURI         string                     `json:"censusURI"`
	Metadata          string                     `json:"metadata"`
	CensusOrigin      int32                      `json:"censusOrigin"`
	Status            int32                      `json:"status"`
	Namespace         uint32                     `json:"namespace"`
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
	SourceNetworkId   string                     `json:"sourceNetworkId"` // string form of the enum to be user friendly
	MaxCensusSize     uint64                     `json:"maxCensusSize"`
	RollingCensusSize uint64                     `json:"rollingCensusSize"`
}

func ProcessFromDB(dbproc *indexerdb.Process) *Process {
	proc := &Process{
		ID:                dbproc.ID,
		EntityID:          nonEmptyBytes(dbproc.EntityID),
		StartBlock:        uint32(dbproc.StartBlock),
		EndBlock:          uint32(dbproc.EndBlock),
		HaveResults:       dbproc.HaveResults,
		FinalResults:      dbproc.FinalResults,
		CensusRoot:        nonEmptyBytes(dbproc.CensusRoot),
		RollingCensusRoot: nonEmptyBytes(dbproc.RollingCensusRoot),
		RollingCensusSize: uint64(dbproc.RollingCensusSize),
		MaxCensusSize:     uint64(dbproc.MaxCensusSize),
		CensusURI:         dbproc.CensusUri,
		CensusOrigin:      int32(dbproc.CensusOrigin),
		Status:            int32(dbproc.Status),
		Namespace:         uint32(dbproc.Namespace),
		PrivateKeys:       nonEmptySplit(dbproc.PrivateKeys, ","),
		PublicKeys:        nonEmptySplit(dbproc.PublicKeys, ","),
		CreationTime:      dbproc.CreationTime,
		SourceBlockHeight: uint64(dbproc.SourceBlockHeight),
		Metadata:          dbproc.Metadata,
	}

	if _, ok := models.SourceNetworkId_name[int32(dbproc.SourceNetworkID)]; !ok {
		log.Errorf("unknown SourceNetworkId: %d", dbproc.SourceNetworkID)
	} else {
		proc.SourceNetworkId = models.SourceNetworkId(dbproc.SourceNetworkID).String()
	}
	proc.Envelope = new(models.EnvelopeType)
	if err := proto.Unmarshal(dbproc.EnvelopePb, proc.Envelope); err != nil {
		log.Error(err)
	}
	proc.Mode = new(models.ProcessMode)
	if err := proto.Unmarshal(dbproc.ModePb, proc.Mode); err != nil {
		log.Error(err)
	}
	proc.VoteOpts = new(models.ProcessVoteOptions)
	if err := proto.Unmarshal(dbproc.VoteOptsPb, proc.VoteOpts); err != nil {
		log.Error(err)
	}
	return proc
}

func decodeVotes(input string) [][]*types.BigInt {
	// "a,b,c x,y,z ..."
	var votes [][]*types.BigInt
	for _, group := range strings.Split(input, " ") {
		var element []*types.BigInt
		for _, s := range strings.Split(group, ",") {
			n := new(types.BigInt)
			if err := n.UnmarshalText([]byte(s)); err != nil {
				log.Error(err) // TODO(mvdan): propagate errors via a database/sql interface?
				continue
			}
			element = append(element, n)
		}
		votes = append(votes, element)
	}
	return votes
}

func ResultsFromDB(dbproc *indexerdb.Process) *results.Results {
	results := &results.Results{
		ProcessID:      dbproc.ID,
		Votes:          decodeVotes(dbproc.ResultsVotes),
		Weight:         decodeBigint(dbproc.ResultsWeight),
		EnvelopeHeight: uint64(dbproc.ResultsEnvelopeHeight),
		Final:          dbproc.FinalResults,
		BlockHeight:    uint32(dbproc.ResultsBlockHeight),
	}
	results.EnvelopeType = new(models.EnvelopeType)
	if err := proto.Unmarshal(dbproc.EnvelopePb, results.EnvelopeType); err != nil {
		log.Error(err)
		return nil
	}
	results.VoteOpts = new(models.ProcessVoteOptions)
	if err := proto.Unmarshal(dbproc.VoteOptsPb, results.VoteOpts); err != nil {
		log.Error(err)
	}
	return results
}

func decodeBigint(s string) *types.BigInt {
	n := new(types.BigInt)
	if err := n.UnmarshalText([]byte(s)); err != nil {
		log.Error(err)
	}
	return n
}

func nonEmptyBytes(p []byte) []byte {
	if len(p) == 0 {
		return nil
	}
	return p
}

func nonEmptySplit(s, sep string) []string {
	list := strings.Split(s, sep)
	if len(list) == 1 && list[0] == "" {
		return nil // avoid []string{""} for s==""
	}
	return list
}

func (p Process) String() string {
	b, _ := json.Marshal(p)
	return string(b)
}

// Entity holds the db reference for an entity
type Entity struct {
	ID           types.HexBytes
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
	Nullifier      types.HexBytes
	ProcessID      types.HexBytes
	VoterID        state.VoterID
	Height         uint32
	Weight         *types.BigInt
	TxIndex        int32
	TxHash         types.HexBytes
	CreationTime   time.Time
	OverwriteCount uint32
}

func VoteReferenceFromDB(dbvote *indexerdb.GetVoteReferenceRow) *VoteReference {
	weightInt := new(types.BigInt)
	if err := weightInt.UnmarshalText([]byte(dbvote.Weight)); err != nil {
		panic(err) // should never happen
	}
	return &VoteReference{
		Nullifier:      dbvote.Nullifier,
		ProcessID:      dbvote.ProcessID,
		VoterID:        dbvote.VoterID,
		Height:         uint32(dbvote.Height),
		Weight:         weightInt,
		TxIndex:        int32(dbvote.TxIndex),
		TxHash:         dbvote.Hash,
		CreationTime:   dbvote.CreationTime,
		OverwriteCount: uint32(dbvote.OverwriteCount),
	}
}

// EnvelopeMetadata contains vote information for the EnvelopeList api
type EnvelopeMetadata struct {
	ProcessId types.HexBytes `json:"processId"`
	Nullifier types.HexBytes `json:"nullifier"`
	VoterID   types.HexBytes `json:"voterId"`
	TxIndex   int32          `json:"txIndex"`
	Height    uint32         `json:"height"`
	TxHash    types.HexBytes `json:"txHash"`
}

// EnvelopePackage contains a VoteEnvelope and auxiliary information for the Envelope api
type EnvelopePackage struct {
	EncryptionKeyIndexes []uint32         `json:"encryptionKeyIndexes"`
	Meta                 EnvelopeMetadata `json:"meta"`
	Nonce                types.HexBytes   `json:"nonce"`
	Signature            types.HexBytes   `json:"signature"`
	VotePackage          []byte           `json:"votePackage"`
	Weight               string           `json:"weight"`
	OverwriteCount       uint32           `json:"overwriteCount"`
	Date                 time.Time        `json:"date"`
}

// TxPackage contains a SignedTx and auxiliary information for the Transaction api
type TxPackage struct {
	Tx          []byte         `json:"tx"`
	ID          uint32         `json:"id,omitempty"`
	BlockHeight uint32         `json:"blockHeight,omitempty"`
	Index       *int32         `json:"index,omitempty"`
	Hash        types.HexBytes `json:"hash"`
	Signature   types.HexBytes `json:"signature"`
}

// TxMetadata contains tx information for the TransactionList api
type TxMetadata struct {
	Type        string         `json:"type"`
	BlockHeight uint32         `json:"blockHeight,omitempty"`
	Index       int32          `json:"index"`
	Hash        types.HexBytes `json:"hash"`
}

// TxReference holds the db reference for a single transaction
type TxReference struct {
	Index        uint64         `json:"transactionNumber"`
	Hash         types.HexBytes `json:"transactionHash"`
	BlockHeight  uint32         `json:"blockHeight"`
	TxBlockIndex int32          `json:"transactionIndex"`
	TxType       string         `json:"transactionType"`
}

func TxReferenceFromDB(dbtx *indexerdb.TxReference) *TxReference {
	return &TxReference{
		Index:        uint64(dbtx.ID),
		Hash:         dbtx.Hash,
		BlockHeight:  uint32(dbtx.BlockHeight),
		TxBlockIndex: int32(dbtx.TxBlockIndex),
		TxType:       dbtx.TxType,
	}
}

// BlockMetadata contains the metadata for a single tendermint block
type BlockMetadata struct {
	Height          uint32         `json:"height,omitempty"`
	Timestamp       time.Time      `json:"timestamp"`
	Hash            types.HexBytes `json:"hash,omitempty"`
	NumTxs          uint64         `json:"numTxs"`
	LastBlockHash   types.HexBytes `json:"lastBlockHash"`
	ProposerAddress types.HexBytes `json:"proposerAddress"`
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

// TokenTransferMeta contains the information of a token transfer and some extra useful information.
// The types are compatible with the SQL defined schema.
type TokenTransferMeta struct {
	Amount    uint64          `json:"amount"`
	From      types.AccountID `json:"from"`
	Height    uint64          `json:"height"`
	TxHash    types.Hash      `json:"txHash"`
	Timestamp time.Time       `json:"timestamp"`
	To        types.AccountID `json:"to"`
}

// ________________________ CALLBACKS DATA STRUCTS ________________________

// IndexerOnProcessData holds the required data for callbacks when
// a new process is added into the vochain.
type IndexerOnProcessData struct {
	EntityID  []byte
	ProcessID []byte
}

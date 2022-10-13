package api

import (
	"fmt"
	"time"

	"github.com/google/uuid"
	"go.vocdoni.io/dvote/types"
	"go.vocdoni.io/proto/build/go/models"
	"google.golang.org/protobuf/encoding/protojson"
)

const (
	ElectionCensusTypeCSP  = "csp"
	ElectionCensusTypeTree = "treeWeighted"
)

type Organization struct {
	OrganizationID types.HexBytes      `json:"organizationID,omitempty"`
	Elections      []*ElectionSummary  `json:"elections,omitempty"`
	Organizations  []*OrganizationList `json:"organizations,omitempty"`
	Count          *uint64             `json:"count,omitempty"`
}

type OrganizationList struct {
	OrganizationID types.HexBytes `json:"organizationID"`
	ElectionCount  uint64         `json:"electionCount"`
}

type ElectionSummary struct {
	ElectionID   types.HexBytes `json:"electionId"`
	Type         string         `json:"type"`
	Status       string         `json:"status"`
	StartDate    time.Time      `json:"startDate"`
	EndDate      time.Time      `json:"endDate"`
	VoteCount    uint64         `json:"voteCount"`
	FinalResults bool           `json:"finalResults"`
	Results      []Result       `json:"result,omitempty"`
}

type Election struct {
	ElectionSummary
	ElectionCount uint32          `json:"electionCount"`
	Census        *ElectionCensus `json:"census,omitempty"`
	MetadataURL   string          `json:"metadataURL"`
	CreationTime  time.Time       `json:"creationTime"`
	PublicKeys    []Key           `json:"publicKeys,omitempty"`
	PrivateKeys   []Key           `json:"privateKeys,omitempty"`
	VoteMode      VoteMode        `json:"voteMode,omitempty"`
	ElectionMode  ElectionMode    `json:"electionMode,omitempty"`
	TallyMode     TallyMode       `json:"tallyMode,omitempty"`
}

type ElectionCensus struct {
	CensusOrigin           string         `json:"censusOrigin"`
	CensusRoot             types.HexBytes `json:"censusRoot"`
	PostRegisterCensusRoot types.HexBytes `json:"postRegisterCensusRoot"`
	CensusURL              string         `json:"censusURL"`
}

type ElectionCreate struct {
	TxPayload   []byte         `json:"txPayload,omitempty"`
	Metadata    []byte         `json:"metadata,omitempty"`
	TxHash      types.HexBytes `json:"txHash"`
	ElectionID  types.HexBytes `json:"electionID"`
	MetadataURL string         `json:"metadataURL"`
}

type ElectionDescription struct {
	Title        LanguageString        `json:"title,omitempty"`
	Description  LanguageString        `json:"description,omitempty"`
	Header       string                `json:"header,omitempty"`
	StreamURI    string                `json:"streamUri,omitempty"`
	StartDate    time.Time             `json:"startDate,omitempty"`
	EndDate      time.Time             `json:"endDate,omitempty"`
	VoteType     VoteType              `json:"voteType,omitempty"`
	ElectionType ElectionType          `json:"electionType,omitempty"`
	Questions    []Question            `json:"questions,omitempty"`
	Census       CensusTypeDescription `json:"census,omitempty"`
}

type Result struct {
	Title []string `json:"title,omitempty"`
	Value []string `json:"value"`
}

type Key struct {
	Index int            `json:"index"`
	Key   types.HexBytes `json:"key"`
}

type Vote struct {
	TxPayload            []byte         `json:"txPayload,omitempty"`
	TxHash               types.HexBytes `json:"txHash,omitempty"`
	VoteID               types.HexBytes `json:"voteID,omitempty"`
	EncryptionKeyIndexes []uint32       `json:"encryptionKeys,omitempty"`
	VotePackage          string         `json:"package,omitempty"`
	VoteWeight           string         `json:"weight,omitempty"`
	VoteNumber           *uint32        `json:"number,omitempty"`
	ElectionID           types.HexBytes `json:"electionID,omitempty"`
	VoterID              types.HexBytes `json:"voterID,omitempty"`
	BlockHeight          uint32         `json:"blockHeight,omitempty"`
}

type CensusTypeDescription struct {
	Type      string         `json:"type,omitempty"`
	URL       string         `json:"url,omitempty"`
	PublicKey types.HexBytes `json:"publicKey,omitempty"`
	RootHash  types.HexBytes `json:"rootHash,omitempty"`
}

type VoteType struct {
	UniqueChoices     bool `json:"uniqueChoices,omitempty"`
	MaxVoteOverwrites int  `json:"maxVoteOverwrites,omitempty"`
	CostFromWeight    bool `json:"costFromWeight,omitempty"`
	CostExponent      int  `json:"costExponent,omitempty"`
	MaxCount          int  `json:"maxCount,omitempty"`
	MaxValue          int  `json:"maxValue,omitempty"`
}

type ElectionType struct {
	Autostart         bool `json:"autostart,omitempty"`
	Interruptible     bool `json:"interruptible,omitempty"`
	DynamicCensus     bool `json:"dynamicCensus,omitempty"`
	SecretUntilTheEnd bool `json:"secretUntilTheEnd,omitempty"`
	Anonymous         bool `json:"anonymous,omitempty"`
}

type Transaction struct {
	Payload   []byte            `json:"payload,omitempty"`
	Hash      types.HexBytes    `json:"hash,omitempty"`
	Response  []byte            `json:"response,omitempty"`
	Code      *uint32           `json:"code,omitempty"`
	Costs     map[string]uint64 `json:"costs,omitempty"`
	Address   types.HexBytes    `json:"address,omitempty"`
	ProcessID types.HexBytes    `json:"processId,omitempty"`
}

type ChainInfo struct {
	ID        string    `json:"chainId,omitempty"`
	BlockTime *[5]int32 `json:"blockTime,omitempty"`
	Height    *uint32   `json:"height,omitempty"`
	Timestamp *int64    `json:"blockTimestamp,omitempty"`
}

type Account struct {
	Address       types.HexBytes `json:"address"`
	Nonce         uint32         `json:"nonce"`
	Balance       uint64         `json:"balance"`
	ElectionIndex uint32         `json:"electionIndex"`
	InfoURL       string         `json:"infoURL,omitempty"`
	Token         *uuid.UUID     `json:"token,omitempty"`
}

type Census struct {
	CensusID types.HexBytes `json:"censusID,omitempty"`
	Root     types.HexBytes `json:"root,omitempty"`
	Weight   *types.BigInt  `json:"weight,omitempty"`
	Key      types.HexBytes `json:"key,omitempty"`
	Proof    types.HexBytes `json:"proof,omitempty"`
	Value    types.HexBytes `json:"value,omitempty"`
	Size     uint64         `json:"size,omitempty"`
	Valid    bool           `json:"valid,omitempty"`
	URI      string         `json:"uri,omitempty"`
}

type CensusDump struct {
	Type     models.Census_Type `json:"type"`
	RootHash []byte             `json:"rootHash"`
	Data     []byte             `json:"data"`
	Indexed  bool               `json:"indexed"`
}

// Protobuf wrappers

type VoteMode struct {
	*models.EnvelopeType
}

func (v VoteMode) MarshalJSON() ([]byte, error) {
	m := protojson.MarshalOptions{EmitUnpopulated: true, UseEnumNumbers: false}
	return m.Marshal(&v)
}

type ElectionMode struct {
	*models.ProcessMode
}

func (e ElectionMode) MarshalJSON() ([]byte, error) {
	m := protojson.MarshalOptions{EmitUnpopulated: true, UseEnumNumbers: false}
	return m.Marshal(&e)
}

type TallyMode struct {
	*models.ProcessVoteOptions
}

func (t TallyMode) MarshalJSON() ([]byte, error) {
	m := protojson.MarshalOptions{EmitUnpopulated: true, UseEnumNumbers: false}
	return m.Marshal(&t)
}

func CensusTypeToOrigin(ctype CensusTypeDescription) (models.CensusOrigin, []byte, error) {
	var origin models.CensusOrigin
	var root []byte
	switch ctype.Type {
	case ElectionCensusTypeCSP:
		origin = models.CensusOrigin_OFF_CHAIN_CA
		root = ctype.PublicKey
	case ElectionCensusTypeTree:
		origin = models.CensusOrigin_OFF_CHAIN_TREE_WEIGHTED
		root = ctype.RootHash
	default:
		return 0, nil, fmt.Errorf("census type %q is unknown", ctype)
	}
	if root == nil {
		return 0, nil, fmt.Errorf("census root is not correctyl specified")
	}
	return origin, root, nil
}

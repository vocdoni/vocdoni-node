package api

import (
	"fmt"
	"time"

	"github.com/google/uuid"
	"go.vocdoni.io/dvote/types"
	"go.vocdoni.io/proto/build/go/models"
	"google.golang.org/protobuf/encoding/protojson"
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
	ElectionID   types.HexBytes    `json:"electionId"`
	Status       string            `json:"status"`
	StartDate    time.Time         `json:"startDate"`
	EndDate      time.Time         `json:"endDate"`
	VoteCount    uint64            `json:"voteCount"`
	FinalResults bool              `json:"finalResults"`
	Results      [][]*types.BigInt `json:"result,omitempty"`
}

type Election struct {
	ElectionSummary
	ElectionCount uint32            `json:"electionCount"`
	Census        *ElectionCensus   `json:"census,omitempty"`
	MetadataURL   string            `json:"metadataURL"`
	CreationTime  time.Time         `json:"creationTime"`
	PublicKeys    []Key             `json:"publicKeys,omitempty"`
	PrivateKeys   []Key             `json:"privateKeys,omitempty"`
	VoteMode      VoteMode          `json:"voteMode,omitempty"`
	ElectionMode  ElectionMode      `json:"electionMode,omitempty"`
	TallyMode     TallyMode         `json:"tallyMode,omitempty"`
	Metadata      *ElectionMetadata `json:"metadata,omitempty"`
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
	Title        LanguageString        `json:"title"`
	Description  LanguageString        `json:"description"`
	Header       string                `json:"header"`
	StreamURI    string                `json:"streamUri"`
	StartDate    time.Time             `json:"startDate,omitempty"`
	EndDate      time.Time             `json:"endDate"`
	VoteType     VoteType              `json:"voteType"`
	ElectionType ElectionType          `json:"electionType"`
	Questions    []Question            `json:"questions"`
	Census       CensusTypeDescription `json:"census"`
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
	TransactionIndex     *int32         `json:"transactionIndex,omitempty"`
}

type CensusTypeDescription struct {
	Type      string         `json:"type"`
	URL       string         `json:"url,omitempty"`
	PublicKey types.HexBytes `json:"publicKey,omitempty"`
	RootHash  types.HexBytes `json:"rootHash,omitempty"`
}

type CensusParticipants struct {
	Participants []CensusParticipant `json:"participants"`
}

type CensusParticipant struct {
	Key    types.HexBytes `json:"key"`
	Weight *types.BigInt  `json:"weight"`
}

type VoteType struct {
	UniqueChoices     bool `json:"uniqueChoices"`
	MaxVoteOverwrites int  `json:"maxVoteOverwrites"`
	CostFromWeight    bool `json:"costFromWeight"`
	CostExponent      int  `json:"costExponent"`
	MaxCount          int  `json:"maxCount"`
	MaxValue          int  `json:"maxValue"`
}

type ElectionType struct {
	Autostart         bool `json:"autostart"`
	Interruptible     bool `json:"interruptible"`
	DynamicCensus     bool `json:"dynamicCensus"`
	SecretUntilTheEnd bool `json:"secretUntilTheEnd"`
	Anonymous         bool `json:"anonymous"`
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

type TransactionReference struct {
	Height uint32 `json:"blockHeight"`
	Index  uint32 `json:"transactionIndex"`
}

type ChainInfo struct {
	ID        string    `json:"chainId,omitempty"`
	BlockTime *[5]int32 `json:"blockTime,omitempty"`
	Height    *uint32   `json:"height,omitempty"`
	Timestamp *int64    `json:"blockTimestamp,omitempty"`
}

type Account struct {
	Address       types.HexBytes   `json:"address"`
	Nonce         uint32           `json:"nonce"`
	Balance       uint64           `json:"balance"`
	ElectionIndex uint32           `json:"electionIndex"`
	InfoURL       string           `json:"infoURL,omitempty"`
	Token         *uuid.UUID       `json:"token,omitempty"`
	Metadata      *AccountMetadata `json:"metadata,omitempty"`
}

type AccountSet struct {
	TxPayload   []byte         `json:"txPayload,omitempty"`
	Metadata    []byte         `json:"metadata,omitempty"`
	TxHash      types.HexBytes `json:"txHash"`
	MetadataURL string         `json:"metadataURL"`
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

type File struct {
	Payload []byte `json:"payload,omitempty"`
	CID     string `json:"cid,omitempty"`
}

type ValidatorList struct {
	Validators []Validator `json:"validators"`
}
type Validator struct {
	Power   uint64         `json:"power"`
	PubKey  types.HexBytes `json:"pubKey"`
	Address types.HexBytes `json:"address"`
	Name    string         `json:"name"`
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
	case CensusTypeCSP:
		origin = models.CensusOrigin_OFF_CHAIN_CA
		root = ctype.PublicKey
	case CensusTypeWeighted, CensusTypeZKWeighted:
		origin = models.CensusOrigin_OFF_CHAIN_TREE_WEIGHTED
		root = ctype.RootHash
	case CensusTypeZK:
		origin = models.CensusOrigin_OFF_CHAIN_TREE
		root = ctype.RootHash
	default:
		return 0, nil, fmt.Errorf("census type %q is unknown", ctype)
	}
	if root == nil {
		return 0, nil, fmt.Errorf("census root is not correctyl specified")
	}
	return origin, root, nil
}

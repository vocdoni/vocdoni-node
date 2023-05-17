package api

import (
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
	ElectionID     types.HexBytes    `json:"electionId"`
	OrganizationID types.HexBytes    `json:"organizationId"`
	Status         string            `json:"status"`
	StartDate      time.Time         `json:"startDate"`
	EndDate        time.Time         `json:"endDate"`
	VoteCount      uint64            `json:"voteCount"`
	FinalResults   bool              `json:"finalResults"`
	Results        [][]*types.BigInt `json:"result,omitempty"`
}

// ElectionResults is the struct used to wrap the results of an election
type ElectionResults struct {
	// ABIEncoded is the abi encoded election results
	ABIEncoded string `json:"abiEncoded"`
	// CensusRoot is the root of the census tree
	CensusRoot types.HexBytes `json:"censusRoot"`
	// ElectionID is the ID of the election
	ElectionID types.HexBytes `json:"electionId"`
	// OrganizationID is the ID of the organization that created the election
	OrganizationID types.HexBytes `json:"organizationId"`
	// Results is the list of votes
	Results [][]*types.BigInt `json:"results"`
	// SourceContractAddress is the address of the smart contract containing the census
	SourceContractAddress types.HexBytes `json:"sourceContractAddress,omitempty"`
}

type Election struct {
	ElectionSummary
	Census       *ElectionCensus   `json:"census,omitempty"`
	MetadataURL  string            `json:"metadataURL"`
	CreationTime time.Time         `json:"creationTime"`
	VoteMode     VoteMode          `json:"voteMode,omitempty"`
	ElectionMode ElectionMode      `json:"electionMode,omitempty"`
	TallyMode    TallyMode         `json:"tallyMode,omitempty"`
	Metadata     *ElectionMetadata `json:"metadata,omitempty"`
}

type ElectionKeys struct {
	PublicKeys  []Key `json:"publicKeys,omitempty"`
	PrivateKeys []Key `json:"privateKeys,omitempty"`
}

type ElectionCensus struct {
	CensusOrigin           string         `json:"censusOrigin"`
	CensusRoot             types.HexBytes `json:"censusRoot"`
	PostRegisterCensusRoot types.HexBytes `json:"postRegisterCensusRoot"`
	CensusURL              string         `json:"censusURL"`
	MaxCensusSize          uint64         `json:"maxCensusSize"`
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

type ElectionFilter struct {
	OrganizationID types.HexBytes `json:"organizationId,omitempty"`
	ElectionID     types.HexBytes `json:"electionId,omitempty"`
	WithResults    *bool          `json:"withResults,omitempty"`
	Status         string         `json:"status,omitempty"`
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
	OverwriteCount       *uint32        `json:"overwriteCount,omitempty"`
	Date                 *time.Time     `json:"date,omitempty"`
}

type CensusTypeDescription struct {
	Type      string         `json:"type"`
	Size      uint64         `json:"size"`
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

type TransactionMetadata struct {
	Type   string         `json:"type"`
	Height uint32         `json:"height"`
	Index  int32          `json:"index"`
	Hash   types.HexBytes `json:"hash"`
}

type BlockTransactionsInfo struct {
	BlockNumber       uint64                `json:"blockNumber"`
	TransactionsCount uint32                `json:"transactionCount"`
	Transactions      []TransactionMetadata `json:"transactions"`
}

type ChainInfo struct {
	ID                      string    `json:"chainId"`
	BlockTime               [5]int32  `json:"blockTime"`
	ElectionCount           uint64    `json:"electionCount"`
	OrganizationCount       uint64    `json:"organizationCount"`
	GenesisTime             time.Time `json:"genesisTime"`
	Height                  uint32    `json:"height"`
	Syncing                 bool      `json:"syncing"`
	Timestamp               int64     `json:"blockTimestamp"`
	TransactionCount        uint64    `json:"transactionCount"`
	ValidatorCount          uint32    `json:"validatorCount"`
	VoteCount               uint64    `json:"voteCount"`
	CircuitConfigurationTag string    `json:"cicuitConfigurationTag"`
	MaxCensusSize           uint64    `json:"maxCensusSize"`
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
	Type     string         `json:"type,omitempty"`
	Root     types.HexBytes `json:"root,omitempty"`
	Weight   *types.BigInt  `json:"weight,omitempty"`
	Key      types.HexBytes `json:"key,omitempty"`
	Proof    types.HexBytes `json:"proof,omitempty"`
	Value    types.HexBytes `json:"value,omitempty"`
	Size     uint64         `json:"size,omitempty"`
	Valid    bool           `json:"valid,omitempty"`
	URI      string         `json:"uri,omitempty"`
	Siblings []string       `json:"siblings,omitempty"`
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
	default:
		return 0, nil, ErrCensusTypeUnknown.Withf("%q", ctype)
	}
	if root == nil {
		return 0, nil, ErrCensusRootIsNil
	}
	return origin, root, nil
}

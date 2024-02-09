package api

import (
	"encoding/json"
	"time"

	comettypes "github.com/cometbft/cometbft/types"
	"github.com/google/uuid"
	"go.vocdoni.io/dvote/types"
	"go.vocdoni.io/dvote/vochain/indexer/indexertypes"
	"go.vocdoni.io/proto/build/go/models"
	"google.golang.org/protobuf/encoding/protojson"
)

type Organization struct {
	OrganizationID types.HexBytes      `json:"organizationID,omitempty" `
	Elections      []*ElectionSummary  `json:"elections,omitempty"`
	Organizations  []*OrganizationList `json:"organizations,omitempty"`
	Count          *uint64             `json:"count,omitempty" example:"1"`
}

type OrganizationList struct {
	OrganizationID types.HexBytes `json:"organizationID"  example:"0x370372b92514d81a0e3efb8eba9d036ae0877653"`
	ElectionCount  uint64         `json:"electionCount" example:"1"`
}

type ElectionSummary struct {
	ElectionID     types.HexBytes    `json:"electionId" `
	OrganizationID types.HexBytes    `json:"organizationId" `
	Status         string            `json:"status"`
	StartDate      time.Time         `json:"startDate"`
	EndDate        time.Time         `json:"endDate"`
	VoteCount      uint64            `json:"voteCount"`
	FinalResults   bool              `json:"finalResults"`
	Results        [][]*types.BigInt `json:"result,omitempty"`
	ManuallyEnded  bool              `json:"manuallyEnded"`
	FromArchive    bool              `json:"fromArchive"`
	ChainID        string            `json:"chainId"`
}

// ElectionResults is the struct used to wrap the results of an election
type ElectionResults struct {
	// ABIEncoded is the abi encoded election results
	ABIEncoded string `json:"abiEncoded" swaggerignore:"true"`
	// CensusRoot is the root of the census tree
	CensusRoot types.HexBytes `json:"censusRoot" `
	// ElectionID is the ID of the election
	ElectionID types.HexBytes `json:"electionId" `
	// OrganizationID is the ID of the organization that created the election
	OrganizationID types.HexBytes `json:"organizationId" `
	// Results is the list of votes
	Results [][]*types.BigInt `json:"results"`
	// SourceContractAddress is the address of the smart contract containing the census
	SourceContractAddress types.HexBytes `json:"sourceContractAddress,omitempty" `
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
	PublicKeys  []Key `json:"publicKeys,omitempty" swaggertype:"string"`
	PrivateKeys []Key `json:"privateKeys,omitempty" swaggertype:"string"`
}

type ElectionCensus struct {
	CensusOrigin           string         `json:"censusOrigin"`
	CensusRoot             types.HexBytes `json:"censusRoot" `
	PostRegisterCensusRoot types.HexBytes `json:"postRegisterCensusRoot" `
	CensusURL              string         `json:"censusURL"`
	MaxCensusSize          uint64         `json:"maxCensusSize"`
}

type ElectionCreate struct {
	TxPayload   []byte         `json:"txPayload,omitempty"`
	Metadata    []byte         `json:"metadata,omitempty"`
	TxHash      types.HexBytes `json:"txHash" `
	ElectionID  types.HexBytes `json:"electionID" `
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
	TempSIKs     bool                  `json:"tempSIKs"`
}

type ElectionFilter struct {
	OrganizationID types.HexBytes `json:"organizationId,omitempty" `
	ElectionID     types.HexBytes `json:"electionId,omitempty" `
	WithResults    *bool          `json:"withResults,omitempty"`
	Status         string         `json:"status,omitempty"`
}

type Key struct {
	Index int            `json:"index"`
	Key   types.HexBytes `json:"key" `
}

type Vote struct {
	TxPayload []byte         `json:"txPayload,omitempty"  extensions:"x-omitempty" swaggerignore:"true"`
	TxHash    types.HexBytes `json:"txHash,omitempty"  extensions:"x-omitempty" `
	VoteID    types.HexBytes `json:"voteID,omitempty"  extensions:"x-omitempty" `
	// Sent only for encrypted elections (no results until the end)
	EncryptionKeyIndexes []uint32 `json:"encryptionKeys,omitempty" extensions:"x-omitempty"`
	// For encrypted elections this will be codified
	VotePackage      json.RawMessage `json:"package,omitempty" extensions:"x-omitempty"`
	VoteWeight       string          `json:"weight,omitempty" extensions:"x-omitempty"`
	VoteNumber       *uint32         `json:"number,omitempty" extensions:"x-omitempty"`
	ElectionID       types.HexBytes  `json:"electionID,omitempty" extensions:"x-omitempty" `
	VoterID          types.HexBytes  `json:"voterID,omitempty" extensions:"x-omitempty" `
	BlockHeight      uint32          `json:"blockHeight,omitempty" extensions:"x-omitempty"`
	TransactionIndex *int32          `json:"transactionIndex,omitempty" extensions:"x-omitempty"`
	OverwriteCount   *uint32         `json:"overwriteCount,omitempty" extensions:"x-omitempty"`
	// Date when the vote was emitted
	Date *time.Time `json:"date,omitempty" extensions:"x-omitempty"`
}

type CensusTypeDescription struct {
	Type      string         `json:"type"`
	Size      uint64         `json:"size"`
	URL       string         `json:"url,omitempty"`
	PublicKey types.HexBytes `json:"publicKey,omitempty" `
	RootHash  types.HexBytes `json:"rootHash,omitempty" `
}

type CensusParticipants struct {
	Participants []CensusParticipant `json:"participants"`
}

type CensusParticipant struct {
	Key    types.HexBytes `json:"key" `
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
	Payload   []byte            `json:"payload,omitempty" extensions:"x-omitempty" swaggerignore:"true"`
	Hash      types.HexBytes    `json:"hash,omitempty" extensions:"x-omitempty" `
	Response  []byte            `json:"response,omitempty" extensions:"x-omitempty" swaggertype:"string" format:"base64"`
	Code      *uint32           `json:"code,omitempty" extensions:"x-omitempty"`
	Costs     map[string]uint64 `json:"costs,omitempty" extensions:"x-omitempty" swaggerignore:"true"`
	Address   types.HexBytes    `json:"address,omitempty" extensions:"x-omitempty" swaggerignore:"true" `
	ProcessID types.HexBytes    `json:"processId,omitempty" extensions:"x-omitempty" swaggerignore:"true" `
}

type TransactionReference struct {
	Height uint32 `json:"blockHeight"`
	Index  uint32 `json:"transactionIndex"`
}

type TransactionMetadata struct {
	Type   string         `json:"transactionType"`
	Number uint32         `json:"transactionNumber"`
	Index  int32          `json:"transactionIndex"`
	Hash   types.HexBytes `json:"transactionHash" `
}

type BlockTransactionsInfo struct {
	BlockNumber       uint64                `json:"blockNumber"`
	TransactionsCount uint32                `json:"transactionCount"`
	Transactions      []TransactionMetadata `json:"transactions"`
}

type GenericTransactionWithInfo struct {
	TxContent json.RawMessage          `json:"tx"`
	TxInfo    indexertypes.Transaction `json:"txInfo"`
	Signature types.HexBytes           `json:"signature"`
}

type ChainInfo struct {
	ID                string    `json:"chainId" example:"azeno"`
	BlockTime         [5]uint64 `json:"blockTime" example:"12000,11580,11000,11100,11100"`
	ElectionCount     uint64    `json:"electionCount" example:"120"`
	OrganizationCount uint64    `json:"organizationCount" example:"20"`
	GenesisTime       time.Time `json:"genesisTime"  format:"date-time" example:"2022-11-17T18:00:57.379551614Z"`
	Height            uint32    `json:"height" example:"5467"`
	Syncing           bool      `json:"syncing" example:"true"`
	Timestamp         int64     `json:"blockTimestamp" swaggertype:"string" format:"date-time" example:"2022-11-17T18:00:57.379551614Z"`
	TransactionCount  uint64    `json:"transactionCount" example:"554"`
	ValidatorCount    uint32    `json:"validatorCount" example:"5"`
	VoteCount         uint64    `json:"voteCount" example:"432"`
	CircuitVersion    string    `json:"circuitVersion" example:"v1.0.0"`
	MaxCensusSize     uint64    `json:"maxCensusSize" example:"50000"`
	NetworkCapacity   uint64    `json:"networkCapacity" example:"2000"`
}

type Account struct {
	Address       types.HexBytes   `json:"address" `
	Nonce         uint32           `json:"nonce"`
	Balance       uint64           `json:"balance"`
	ElectionIndex uint32           `json:"electionIndex"`
	InfoURL       string           `json:"infoURL,omitempty"`
	Token         *uuid.UUID       `json:"token,omitempty" swaggerignore:"true"`
	Metadata      *AccountMetadata `json:"metadata,omitempty"`
	SIK           types.HexBytes   `json:"sik"`
}

type AccountSet struct {
	TxPayload   []byte         `json:"txPayload,omitempty" swaggerignore:"true"`
	Metadata    []byte         `json:"metadata,omitempty" swaggerignore:"true"`
	TxHash      types.HexBytes `json:"txHash" `
	MetadataURL string         `json:"metadataURL" swaggertype:"string"`
}

type Census struct {
	CensusID types.HexBytes `json:"censusID,omitempty"`
	Type     string         `json:"type,omitempty"`
	Weight   *types.BigInt  `json:"weight,omitempty"`
	Size     uint64         `json:"size,omitempty"`
	Valid    bool           `json:"valid,omitempty"`
	URI      string         `json:"uri,omitempty"`
	// proof stuff
	CensusRoot     types.HexBytes `json:"censusRoot,omitempty"`
	CensusProof    types.HexBytes `json:"censusProof,omitempty"`
	Key            types.HexBytes `json:"key,omitempty"`
	Value          types.HexBytes `json:"value,omitempty"`
	CensusSiblings []string       `json:"censusSiblings,omitempty"`
}

type File struct {
	Payload []byte `json:"payload,omitempty" swaggerignore:"true"`
	CID     string `json:"cid,omitempty"`
}

type ValidatorList struct {
	Validators []Validator `json:"validators"`
}

type Validator struct {
	Power            uint64         `json:"power"`
	PubKey           types.HexBytes `json:"pubKey"`
	AccountAddress   types.HexBytes `json:"address"`
	Name             string         `json:"name"`
	ValidatorAddress types.HexBytes `json:"validatorAddress"`
	JoinHeight       uint64         `json:"joinHeight"`
	Votes            uint64         `json:"votes"`
	Proposals        uint64         `json:"proposals"`
	Score            uint32         `json:"score"`
}

type NextElectionID struct {
	OrganizationID types.HexBytes `json:"organizationId"`
	CensusOrigin   int32          `json:"censusOrigin"`
	EnvelopeType   struct {
		Serial         bool `json:"serial"`
		Anonymous      bool `json:"anonymous"`
		EncryptedVotes bool `json:"encryptedVotes"`
		UniqueValues   bool `json:"uniqueValues"`
		CostFromWeight bool `json:"costFromWeight"`
	} `json:"envelopeType"`
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
	case CensusTypeFarcaster:
		origin = models.CensusOrigin_FARCASTER_FRAME
		root = ctype.RootHash
	default:
		return 0, nil, ErrCensusTypeUnknown.Withf("%q", ctype)
	}
	if root == nil {
		return 0, nil, ErrCensusRootIsNil
	}
	return origin, root, nil
}

type Block struct {
	comettypes.Block `json:",inline"`
	Hash             types.HexBytes `json:"hash" `
}

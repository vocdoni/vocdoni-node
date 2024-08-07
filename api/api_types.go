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

// ### Params accepted ###

// PaginationParams allows the client to request a specific page, and how many items per page
type PaginationParams struct {
	Page  int `json:"page,omitempty"`
	Limit int `json:"limit,omitempty"`
}

// ElectionParams allows the client to filter elections
type ElectionParams struct {
	PaginationParams
	OrganizationID string `json:"organizationId,omitempty"`
	ElectionID     string `json:"electionId,omitempty"`
	Status         string `json:"status,omitempty"`
	WithResults    *bool  `json:"withResults,omitempty"`
	FinalResults   *bool  `json:"finalResults,omitempty"`
	ManuallyEnded  *bool  `json:"manuallyEnded,omitempty"`
}

// OrganizationParams allows the client to filter organizations
type OrganizationParams struct {
	PaginationParams
	OrganizationID string `json:"organizationId,omitempty"`
}

// AccountParams allows the client to filter accounts
type AccountParams struct {
	PaginationParams
	AccountID string `json:"accountId,omitempty"`
}

// TransactionParams allows the client to filter transactions
type TransactionParams struct {
	PaginationParams
	Height uint64 `json:"height,omitempty"`
	Type   string `json:"type,omitempty"`
}

// FeesParams allows the client to filter fees
type FeesParams struct {
	PaginationParams
	Reference string `json:"reference,omitempty"`
	Type      string `json:"type,omitempty"`
	AccountID string `json:"accountId,omitempty"`
}

// TransfersParams allows the client to filter transfers
type TransfersParams struct {
	PaginationParams
	AccountID     string `json:"accountId,omitempty"`
	AccountIDFrom string `json:"accountIdFrom,omitempty"`
	AccountIDTo   string `json:"accountIdTo,omitempty"`
}

// VoteParams allows the client to filter votes
type VoteParams struct {
	PaginationParams
	ElectionID string `json:"electionId,omitempty"`
}

// ### Objects returned ###

// CountResult wraps a count inside an object
type CountResult struct {
	Count uint64 `json:"count" example:"10"`
}

// Pagination contains all the values needed for the UI to easily organize the returned data
type Pagination struct {
	TotalItems   uint64  `json:"totalItems"`
	PreviousPage *uint64 `json:"previousPage"`
	CurrentPage  uint64  `json:"currentPage"`
	NextPage     *uint64 `json:"nextPage"`
	LastPage     uint64  `json:"lastPage"`
}

type OrganizationSummary struct {
	OrganizationID types.HexBytes `json:"organizationID"  example:"0x370372b92514d81a0e3efb8eba9d036ae0877653"`
	ElectionCount  uint64         `json:"electionCount" example:"1"`
}

// OrganizationsList is used to return a paginated list to the client
type OrganizationsList struct {
	Organizations []*OrganizationSummary `json:"organizations"`
	Pagination    *Pagination            `json:"pagination"`
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
	ChainID        string            `json:"chainId"`
}

// ElectionsList is used to return a paginated list to the client
type ElectionsList struct {
	Elections  []*ElectionSummary `json:"elections"`
	Pagination *Pagination        `json:"pagination"`
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
	Census       *ElectionCensus `json:"census,omitempty"`
	MetadataURL  string          `json:"metadataURL"`
	CreationTime time.Time       `json:"creationTime"`
	VoteMode     VoteMode        `json:"voteMode,omitempty"`
	ElectionMode ElectionMode    `json:"electionMode,omitempty"`
	TallyMode    TallyMode       `json:"tallyMode,omitempty"`
	Metadata     any             `json:"metadata,omitempty"`
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
	TxPayload                 []byte         `json:"txPayload,omitempty"`
	Metadata                  []byte         `json:"metadata,omitempty"`
	TxHash                    types.HexBytes `json:"txHash" `
	ElectionID                types.HexBytes `json:"electionID" `
	MetadataURL               string         `json:"metadataURL"`
	MetadataEncryptionPrivKey types.HexBytes `json:"metadataEncryptionPrivKey,omitempty"`
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

type Key struct {
	Index int            `json:"index"`
	Key   types.HexBytes `json:"key" `
}

type Vote struct {
	TxPayload []byte         `json:"txPayload,omitempty"  extensions:"x-omitempty" swaggerignore:"true"`
	TxHash    types.HexBytes `json:"txHash,omitempty"  extensions:"x-omitempty" `
	// VoteID here produces a `voteID` over JSON that differs in casing from the rest of params and JSONs
	// but is kept for backwards compatibility
	VoteID types.HexBytes `json:"voteID,omitempty"  extensions:"x-omitempty" `
	// Sent only for encrypted elections (no results until the end)
	EncryptionKeyIndexes []uint32 `json:"encryptionKeys,omitempty" extensions:"x-omitempty"`
	// For encrypted elections this will be codified
	VotePackage      json.RawMessage `json:"package,omitempty" extensions:"x-omitempty"`
	VoteWeight       string          `json:"weight,omitempty" extensions:"x-omitempty"` // [math/big.Int.String]
	VoteNumber       *uint32         `json:"number,omitempty" extensions:"x-omitempty"`
	ElectionID       types.HexBytes  `json:"electionID,omitempty" extensions:"x-omitempty" `
	VoterID          types.HexBytes  `json:"voterID,omitempty" extensions:"x-omitempty" `
	BlockHeight      uint32          `json:"blockHeight,omitempty" extensions:"x-omitempty"`
	TransactionIndex *int32          `json:"transactionIndex,omitempty" extensions:"x-omitempty"`
	OverwriteCount   *uint32         `json:"overwriteCount,omitempty" extensions:"x-omitempty"`
	// Date when the vote was emitted
	Date *time.Time `json:"date,omitempty" extensions:"x-omitempty"`
}

type VotesList struct {
	Votes      []*Vote     `json:"votes"`
	Pagination *Pagination `json:"pagination"`
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

// TransactionsList is used to return a paginated list to the client
type TransactionsList struct {
	Transactions []*indexertypes.Transaction `json:"transactions"`
	Pagination   *Pagination                 `json:"pagination"`
}

// FeesList is used to return a paginated list to the client
type FeesList struct {
	Fees       []*indexertypes.TokenFeeMeta `json:"fees"`
	Pagination *Pagination                  `json:"pagination"`
}

// TransfersList is used to return a paginated list to the client
type TransfersList struct {
	Transfers  []*indexertypes.TokenTransferMeta `json:"transfers"`
	Pagination *Pagination                       `json:"pagination"`
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
	InitialHeight     uint32    `json:"initialHeight"  example:"5467"`
	Height            uint32    `json:"height" example:"5467"`
	BlockStoreBase    uint32    `json:"blockStoreBase" example:"5467"`
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
	Address        types.HexBytes   `json:"address" `
	Nonce          uint32           `json:"nonce"`
	Balance        uint64           `json:"balance"`
	ElectionIndex  uint32           `json:"electionIndex"`
	TransfersCount uint64           `json:"transfersCount,omitempty"`
	FeesCount      uint64           `json:"feesCount,omitempty"`
	InfoURL        string           `json:"infoURL,omitempty"`
	Token          *uuid.UUID       `json:"token,omitempty" swaggerignore:"true"`
	Metadata       *AccountMetadata `json:"metadata,omitempty"`
	SIK            types.HexBytes   `json:"sik"`
}

type AccountsList struct {
	Accounts   []*indexertypes.Account `json:"accounts"`
	Pagination *Pagination             `json:"pagination"`
}

type AccountSet struct {
	TxPayload   []byte         `json:"txPayload,omitempty" swaggerignore:"true"`
	Metadata    []byte         `json:"metadata,omitempty" swaggerignore:"true"`
	TxHash      types.HexBytes `json:"txHash" `
	MetadataURL string         `json:"metadataURL" swaggertype:"string"`
}

type Census struct {
	// CensusID here produces a `censusID` over JSON that differs in casing from the rest of params and JSONs
	// but is kept for backwards compatibility
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

type BuildElectionID struct {
	Delta          int32          `json:"delta"` // 0 means build next ElectionID
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

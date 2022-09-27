package urlapi

import (
	"time"

	"github.com/google/uuid"
	"go.vocdoni.io/dvote/types"
	"go.vocdoni.io/dvote/vochain"
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
	ElectionID types.HexBytes `json:"electionId,omitempty"`
	Status     string         `json:"status,omitempty"`
	StartDate  time.Time      `json:"startDate,omitempty"`
	EndDate    time.Time      `json:"endDate,omitempty"`
}

type Election struct {
	Type        string   `json:"type,omitempty"`
	Status      string   `json:"status,omitempty"`
	VoteCount   *uint64  `json:"voteCount,omitempty"`
	Results     []Result `json:"result,omitempty"`
	Count       *uint64  `json:"count,omitempty"`
	PublicKeys  []Key    `json:"publicKeys,omitempty"`
	PrivateKeys []Key    `json:"privateKeys,omitempty"`
}

type Result struct {
	Title []string `json:"title"`
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
}

type ElectionDescription struct {
	Title        LanguageString `json:"title,omitempty"`
	Description  LanguageString `json:"description,omitempty"`
	Header       string         `json:"header,omitempty"`
	StreamURI    string         `json:"streamUri,omitempty"`
	StartDate    time.Time      `json:"startDate,omitempty"`
	EndDate      time.Time      `json:"endDate,omitempty"`
	VoteType     VoteType       `json:"voteType,omitempty"`
	ElectionMode ElectionMode   `json:"electionMode,omitempty"`
	Questions    []Question     `json:"questions,omitempty"`
	Census       CensusType     `json:"census,omitempty"`
}

type CensusType struct {
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

type ElectionMode struct {
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
	Address types.HexBytes   `json:"address,omitempty"`
	Account *vochain.Account `json:"account,omitempty"`
	Balance *uint64          `json:"balance,omitempty"`
	Token   *uuid.UUID       `json:"token,omitempty"`
}

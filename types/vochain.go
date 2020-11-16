package types

import (
	"encoding/json"
	"time"

	// Don't import tendermint/types, because that pulls in lots of indirect
	// dependencies which are too heavy for our low-level "types" package.
	// libs/bytes is okay, because it only pulls in std deps.
	tmbytes "github.com/tendermint/tendermint/libs/bytes"
	models "github.com/vocdoni/dvote-protobuf/build/go/models"
)

// ________________________ STATE ________________________
// Defined in ../../db/iavl.go for convenience

// ________________________ VOTE ________________________

// VotePackageStruct represents a vote package
type VotePackageStruct struct {
	// Type vote type
	Type string `json:"type" bare:"type"`
	// Nonce vote nonce
	Nonce string `json:"nonce" bare:"nonce"`
	// Votes directly mapped to the `questions` field of the process metadata
	Votes []int `json:"votes" bare:"votes"`
}

// Vote represents a single Vote
type Vote struct {
	EncryptionKeyIndexes []uint32 `json:"encryptionKeyIndexes,omitempty"`
	// Height the Terndemint block number where the vote is added
	Height int64 `json:"height,omitempty"`
	// Nullifier is the unique identifier of the vote
	Nullifier []byte `json:"nullifier,omitempty"`
	// ProcessID contains the unique voting process identifier
	ProcessID []byte `json:"processId,omitempty"`
	// VotePackage base64 encoded vote content
	VotePackage []byte `json:"votePackage,omitempty"`
}

// VoteProof contains the proof indicating that the user is in the census of the process
type VoteProof struct {
	Proof        string    `json:"proof,omitempty"`
	PubKey       string    `json:"pubKey,omitempty"`
	PubKeyDigest []byte    `json:"pubKeyDigest,omitempty"`
	Nullifier    []byte    `json:"nullifier,omitempty"`
	Created      time.Time `json:"timestamp"`
}

// ________________________ PROCESS ________________________

// Process represents a state per process
type Process struct {
	// Canceled if true process is canceled
	Canceled bool `json:"canceled,omitempty"`
	// CommitmentKeys are the reveal keys hashed
	CommitmentKeys []string `json:"commitmentKeys,omitempty"`
	// EncryptionPrivateKeys are the keys required to decrypt the votes
	EncryptionPrivateKeys []string `json:"encryptionPrivateKeys,omitempty"`
	// EncryptionPublicKeys are the keys required to encrypt the votes
	EncryptionPublicKeys []string `json:"encryptionPublicKeys,omitempty"`
	// EntityID identifies unequivocally a process
	EntityID []byte `json:"entityId,omitempty"`
	// KeyIndex
	KeyIndex int `json:"keyIndex,omitempty"`
	// MkRoot merkle root of all the census in the process
	MkRoot string `json:"mkRoot,omitempty"`
	// NumberOfBlocks represents the amount of tendermint blocks that the process will last
	NumberOfBlocks int64 `json:"numberOfBlocks,omitempty"`
	// Paused if true process is paused and cannot add or modify any vote
	Paused bool `json:"paused,omitempty"`
	// RevealKeys are the seed of the CommitmentKeys
	RevealKeys []string `json:"revealKeys,omitempty"`
	// StartBlock represents the tendermint block where the process goes from scheduled to active
	StartBlock int64 `json:"startBlock,omitempty"`
	// Type represents the process type
	Type string `json:"type,omitempty"`
}

// RequireKeys indicates wheter a process require Encryption or Commitment keys
func (p *Process) RequireKeys() bool {
	return ProcessRequireKeys[p.Type]
}

// IsEncrypted indicates wheter a process has an encrypted payload or not
func (p *Process) IsEncrypted() bool {
	return ProcessIsEncrypted[p.Type]
}

var ProcessRequireKeys = map[string]bool{
	PollVote:      false,
	PetitionSign:  false,
	EncryptedPoll: true,
	SnarkVote:     true,
}

var ProcessIsEncrypted = map[string]bool{
	PollVote:      false,
	PetitionSign:  false,
	EncryptedPoll: true,
	SnarkVote:     true,
}

// ________________________ TX ________________________

// UniqID returns a uniq identifier for the VoteTX. It depends on the Type.
func UniqID(tx *models.Tx, processType string) string {
	switch processType {
	case PollVote, PetitionSign, EncryptedPoll:
		if len(tx.Signature) > 32 {
			return string(tx.Signature[:32])
		}
	}
	return ""
}

// ________________________ VALIDATORS ________________________

// ________________________ QUERIES ________________________

// QueryData is an abstraction of any kind of data a query request could have
type QueryData struct {
	Method      string `json:"method"`
	ProcessID   string `json:"processId,omitempty"`
	Nullifier   string `json:"nullifier,omitempty"`
	From        int64  `json:"from,omitempty"`
	ListSize    int64  `json:"listSize,omitempty"`
	Timestamp   int64  `json:"timestamp,omitempty"`
	ProcessType string `json:"type,omitempty"`
}

// ________________________ GENESIS APP STATE ________________________

// GenesisAppState application state in genesis
type GenesisAppState struct {
	Validators []GenesisValidator `json:"validators"`
	Oracles    []string           `json:"oracles"`
}

// The rest of these genesis app state types are copied from
// github.com/tendermint/tendermint/types, for the sake of making this package
// lightweight and not have it import heavy indirect dependencies like grpc or
// crypto/*.

type GenesisDoc struct {
	GenesisTime     time.Time          `json:"genesis_time"`
	ChainID         string             `json:"chain_id"`
	ConsensusParams *ConsensusParams   `json:"consensus_params,omitempty"`
	Validators      []GenesisValidator `json:"validators,omitempty"`
	AppHash         tmbytes.HexBytes   `json:"app_hash"`
	AppState        json.RawMessage    `json:"app_state,omitempty"`
}

type ConsensusParams struct {
	Block     BlockParams     `json:"block"`
	Evidence  EvidenceParams  `json:"evidence"`
	Validator ValidatorParams `json:"validator"`
}

type BlockParams struct {
	MaxBytes int64 `json:"max_bytes"`
	MaxGas   int64 `json:"max_gas"`
	// Minimum time increment between consecutive blocks (in milliseconds)
	// Not exposed to the application.
	TimeIotaMs int64 `json:"time_iota_ms"`
}

type EvidenceParams struct {
	MaxAgeNumBlocks int64         `json:"max_age_num_blocks"` // only accept new evidence more recent than this
	MaxAgeDuration  time.Duration `json:"max_age_duration"`
}

type ValidatorParams struct {
	PubKeyTypes []string `json:"pub_key_types"`
}

type GenesisValidator struct {
	Address tmbytes.HexBytes `json:"address"`
	PubKey  PubKey           `json:"pub_key"`
	Power   int64            `json:"power"`
	Name    string           `json:"name"`
}

type PubKey interface {
	Address() tmbytes.HexBytes
	Bytes() []byte
	VerifyBytes(msg []byte, sig []byte) bool

	// Note that we can't keep Equals, because that would forcibly pull in
	// tmtypes.PubKey again. Two named interfaces can't be used
	// interchangeably, even if the underlying interface is identical.
	// Equals(PubKey) bool
}

// ________________________ CALLBACKS DATA STRUCTS ________________________

// ScrutinizerOnProcessData holds the required data for callbacks when
// a new process is added into the vochain.
type ScrutinizerOnProcessData struct {
	EntityID  []byte
	ProcessID []byte
}

package types

import (
	tmtypes "github.com/tendermint/tendermint/types"
)

// ________________________ STATE ________________________
// Defined in ../../db/iavl.go for convenience

// ________________________ VOTE ________________________

// VotePackageStruct represents a vote package
type VotePackageStruct struct {
	// Nonce vote nonce
	Nonce string `json:"nonce"`
	// Type vote type
	Type string `json:"type"`
	// Votes directly mapped to the `questions` field of the process metadata
	Votes []int `json:"votes"`
}

// Vote represents a single Vote
type Vote struct {
	EncryptionKeyIndexes []int `json:"encryptionKeyIndexes,omitempty"`
	// Height the Terndemint block number where the vote is added
	Height int64 `json:"height,omitempty"`
	// Nonce unique number per vote attempt, so that replay attacks can't reuse this payload
	Nonce string `json:"nonce,omitempty"`
	// Nullifier is the hash of the private key
	Nullifier string `json:"nullifier,omitempty"`
	// ProcessID contains the vote itself
	ProcessID string `json:"processId,omitempty"`
	// Proof contains the prove indicating that the user is in the census of the process
	Proof string `json:"proof,omitempty"`
	// Signature sign( JSON.stringify( { nonce, processId, proof, 'votePackage' } ), privateKey )
	Signature string `json:"signature,omitempty"`
	// VotePackage base64 encoded vote content
	VotePackage string `json:"votePackage,omitempty"`
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
	EntityID string `json:"entityId,omitempty"`
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

// ValidTypes represents an allowed specific tx type
var ValidTypes = map[string]string{
	TxVote:              "VoteTx",
	TxNewProcess:        "NewProcessTx",
	TxCancelProcess:     "CancelProcessTx",
	TxAddValidator:      "AdminTx",
	TxRemoveValidator:   "AdminTx",
	TxAddOracle:         "AdminTx",
	TxRemoveOracle:      "AdminTx",
	TxAddProcessKeys:    "AdminTx",
	TxRevealProcessKeys: "AdminTx",
}

// Tx is an abstraction for any specific tx which is primarly defined by its type
// For now we have 3 tx types {voteTx, newProcessTx, adminTx}
type Tx struct {
	Type string `json:"type"`
}

// VoteTx represents the info required for submmiting a vote
type VoteTx struct {
	EncryptionKeyIndexes []int  `json:"encryptionKeyIndexes,omitempty"`
	Nonce                string `json:"nonce,omitempty"`
	Nullifier            string `json:"nullifier,omitempty"`
	ProcessID            string `json:"processId"`
	Proof                string `json:"proof,omitempty"`
	Signature            string `json:"signature,omitempty"`
	Type                 string `json:"type,omitempty"`
	VotePackage          string `json:"votePackage,omitempty"`
}

func (tx *VoteTx) TxType() string {
	return "VoteTx"
}

// NewProcessTx represents the info required for starting a new process
type NewProcessTx struct {
	// EntityID the process belongs to
	EntityID string `json:"entityId"`
	// MkRoot merkle root of all the census in the process
	MkRoot string `json:"mkRoot,omitempty"`
	// MkURI merkle tree URI
	MkURI string `json:"mkURI,omitempty"`
	// NumberOfBlocks represents the tendermint block where the process goes from active to finished
	NumberOfBlocks int64  `json:"numberOfBlocks"`
	ProcessID      string `json:"processId"`
	ProcessType    string `json:"processType"`
	Signature      string `json:"signature,omitempty"`
	// StartBlock represents the tendermint block where the process goes from scheduled to active
	StartBlock int64  `json:"startBlock"`
	Type       string `json:"type,omitempty"`
}

func (tx *NewProcessTx) TxType() string {
	return "NewProcessTx"
}

// CancelProcessTx represents a tx for canceling a valid process
type CancelProcessTx struct {
	// EntityID the process belongs to
	ProcessID string `json:"processId"`
	Signature string `json:"signature,omitempty"`
	Type      string `json:"type,omitempty"`
}

func (tx *CancelProcessTx) TxType() string {
	return "CancelProcessTx"
}

// AdminTx represents a Tx that can be only executed by some authorized addresses
type AdminTx struct {
	Address              string `json:"address"`
	CommitmentKey        string `json:"commitmentKey,omitempty"`
	EncryptionPrivateKey string `json:"encryptionPrivateKey,omitempty"`
	EncryptionPublicKey  string `json:"encryptionPublicKey,omitempty"`
	KeyIndex             int    `json:"keyIndex,omitempty"`
	Nonce                string `json:"nonce"`
	Power                int64  `json:"power,omitempty"`
	ProcessID            string `json:"processId,omitempty"`
	PubKey               string `json:"publicKey,omitempty"`
	RevealKey            string `json:"revealKey,omitempty"`
	Signature            string `json:"signature,omitempty"`
	Type                 string `json:"type"` // addValidator, removeValidator, addOracle, removeOracle
}

func (tx *AdminTx) TxType() string {
	return "AdminTx"
}

// ValidateType a valid Tx type specified in ValidTypes. Returns empty string if invalid type.
func ValidateType(t string) string {
	val, ok := ValidTypes[t]
	if !ok {
		return ""
	}
	return val
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
	Validators []tmtypes.GenesisValidator `json:"validators"`
	Oracles    []string                   `json:"oracles"`
}

// ________________________ CALLBACKS DATA STRUCTS ________________________

// ScrutinizerOnProcessData holds the required data for callbacks when
// a new process is added into the vochain.
type ScrutinizerOnProcessData struct {
	EntityID  string
	ProcessID string
}

package types

import (
	"fmt"
	"strings"
)

// ________________________ STATE ________________________
// Defined in ../../db/iavl.go for convenience

// ________________________ VOTE ________________________

// Vote represents a signle Vote
type Vote struct {
	// Nonce unique number per vote attempt, so that replay attacks can't reuse this payload
	Nonce string
	// Nullifier is the hash of the private key
	Nullifier string
	// ProcessId contains the vote itself
	ProcessID string
	// Proof contains the prove indicating that the user is in the census of the process
	Proof string
	// Signature sign( JSON.stringify( { nonce, processId, proof, 'vote-package' } ), privateKey )
	Signature string
	// VotePackage base64 encoded vote content
	VotePackage string
}

// NewVote returns a new Vote instance
func NewVote() *Vote {
	return &Vote{}
}

// ________________________ PROCESS ________________________

// Process represents a state per process
type Process struct {
	// EntityID identifies unequivocally a process
	EntityID string
	// MkRoot merkle root of all the census in the process
	MkRoot string
	// NumberOfBlocks represents the amount of tendermint blocks that the process will last
	NumberOfBlocks int64
	// StartBlock represents the tendermint block where the process goes from scheduled to active
	StartBlock int64
	// CurrentState is the current process state
	CurrentState CurrentProcessState
	// EncryptionPublicKey are the keys required to encrypt the votes
	EncryptionPublicKeys []string
	// EncryptionPrivateKey are the keys required to decrypt the votes
	EncryptionPrivateKeys []string
}

// NewProcess returns a new Process instance
func NewProcess() *Process {
	return &Process{}
}

// CurrentProcessState represents the current phase of process state
type CurrentProcessState int8

const (
	// Scheduled process is scheduled to start at some point of time
	Scheduled CurrentProcessState = iota
	// Active process is in progress
	Active
	// Paused active process is paused
	Paused
	// Finished process is finished
	Finished
	// Canceled process is canceled and/or invalid
	Canceled
)

// String returns the CurrentProcessState as string
func (c *CurrentProcessState) String() string {
	switch *c {
	// scheduled
	case 0:
		return fmt.Sprint("scheduled")
	// active
	case 1:
		return fmt.Sprintf("active")
	// paused
	case 2:
		return fmt.Sprintf("paused")
	// finished
	case 3:
		return fmt.Sprintf("finished")
	// canceled
	case 4:
		return fmt.Sprintf("canceled")
	default:
		return ""
	}
}

// ________________________ TX ________________________

// ValidTypes represents an allowed specific tx type
var ValidTypes = map[string]string{
	"vote":            "VoteTx",
	"newProcess":      "NewProcessTx",
	"addValidator":    "AdminTx",
	"removeValidator": "AdminTx",
	"addOracle":       "AdminTx",
	"removeOracle":    "AdminTx"}

// Tx is an abstraction for any specific tx which is primarly defined by its type
// For now we have 3 tx types {voteTx, newProcessTx, adminTx}
type Tx struct {
	Type string `json:"type"`
}

// VoteTx represents the info required for submmiting a vote
type VoteTx struct {
	Nonce       string `json:"nonce,omitempty"`
	Nullifier   string `json:"nullifier,omitempty"`
	ProcessID   string `json:"processId"`
	Proof       string `json:"proof,omitempty"`
	Signature   string `json:"signature,omitempty"`
	Type        string `json:"type,omitempty"`
	VotePackage string `json:"vote-package,omitempty"`
}

// NewProcessTx represents the info required for starting a new process
type NewProcessTx struct {
	// EncryptionPublicKeys are the keys required to encrypt the votes
	EncryptionPublicKeys []string `json:"encryptionPublicKeys,omitempty"`
	//EntityID the process belongs to
	EntityID string `json:"entityId"`
	// MkRoot merkle root of all the census in the process
	MkRoot string `json:"mkRoot,omitempty"`
	// NumberOfBlocks represents the tendermint block where the process goes from active to finished
	NumberOfBlocks int64  `json:"numberOfBlocks"`
	ProcessID      string `json:"processId"`
	Signature      string `json:"signature,omitempty"`
	// StartBlock represents the tendermint block where the process goes from scheduled to active
	StartBlock int64  `json:"startBlock"`
	Type       string `json:"type"` // newProcess
}

func (p *NewProcessTx) ToJsonString() string {
	return fmt.Sprintf(`{
		"encryptionPublicKeys":[%s],
		"entityID":%s,
		"mkRoot":%s,
		"numberOfBlocks":%d,
		"processId":%s
		"startBlock":%d,
		"type":%s
	}`, strings.Join(p.EncryptionPublicKeys, ","),
		p.EntityID, p.MkRoot, p.NumberOfBlocks,
		p.ProcessID, p.StartBlock, p.Type)
}

// AdminTx represents a Tx that can be only executed by some authorized addresses
type AdminTx struct {
	Address   string `json:"address"`
	Nonce     string `json:"nonce"`
	Power     int64  `json:"power"`
	Signature string `json:"signature,omitempty"`
	Type      string `json:"type"` //addValidator, removeValidator, addOracle, removeOracle
}

// ValidateType a valid Tx type specified in ValidTypes
func ValidateType(t string) string {
	val, ok := ValidTypes[t]
	if !ok {
		return ""
	}
	return val
}

// ________________________ VALIDATORS ________________________

// PubKey represents a validator pubkey with a key scheme type and its pubkey value
type PubKey struct {
	Type  string `json:"type"`
	Value string `json:"value"`
}

// Validator represents a single tendermint validator
type Validator struct {
	Address string `json:"address"`
	PubKey  PubKey `json:"pub-key"`
	Power   int64  `json:"power"`
	Name    string `json:"name"`
}

// ________________________ QUERIES ________________________

// QueryData is an abstraction of any kind of data a query request could have
type QueryData struct {
	Method    string `json:"method"`
	ProcessID string `json:"processId,omitempty"`
	Nullifier string `json:"nullifier,omitempty"`
	From      int64  `json:"from,omitempty"`
	ListSize  int64  `json:"listSize,omitempty"`
	Timestamp int64  `json:"timestamp,omitempty"`
}

type AppState struct {
	Validators []struct {
		Address string `json:"address"`
		PubKey  struct {
			Type  string `json:"type"`
			Value string `json:"value"`
		} `json:"pubkey"`
		Power string `json:"power"`
		Name  string `json:"name"`
	} `json:"validators"`
	Oracles []string `json:"oracles"`
}

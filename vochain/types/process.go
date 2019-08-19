package vochain

import (
	"fmt"
)

// ________________________ PROCESS ________________________

// Process represents a state per process
type Process struct {
	EntityID string
	// Votes is a list containing all the processed and valid votes (here votes are final)
	Votes []Vote `json:"votes"`
	// MkRoot merkle root of all the census in the process
	MkRoot string `json:"mkroot"`
	// EndBlock represents the tendermint block where the process goes from active to finished
	NumberOfBlocks int64 `json:"endblock"`
	// InitBlock represents the tendermint block where the process goes from scheduled to active
	InitBlock int64 `json:"initblock"`
	// CurrentState is the current process state
	CurrentState CurrentProcessState `json:"currentstate"`
	// EncryptionKeys are the keys required to encrypt the votes
	EncryptionKeys string `json:"encryptionkeys"`
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
func (c CurrentProcessState) String() string {
	switch c {
	// scheduled
	case 0:
		return fmt.Sprintf("%s", "scheduled")
	// active
	case 1:
		return fmt.Sprintf("%s", "active")
	// paused
	case 2:
		return fmt.Sprintf("%s", "paused")
	// finished
	case 3:
		return fmt.Sprintf("%s", "finished")
	// canceled
	case 4:
		return fmt.Sprintf("%s", "canceled")
	default:
		return ""
	}
}

// ________________________ VOTE ________________________

// Vote represents a single vote
type Vote struct {
	// Payload contains the vote itself
	Payload string `json:"payload"`
	// Nullifier is a special hash that prevents double voting
	Nullifier string `json:"nullifier"`
	// CensusProof contains the prove indicating that the user is in the census of the process
	CensusProof string `json:"censusproof"`
}

// NewVote returns a new Vote instance
func NewVote() *Vote {
	return &Vote{}
}

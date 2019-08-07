package vochain

import (
	"fmt"

	tmtypes "github.com/tendermint/tendermint/types"
)

// GlobalState represents the Tendermint blockchain global state
type GlobalState struct {
	// ProcessList is a list containing all the processes in the blockchain
	ProcessList *ProcessState `json:"processList"`
	// ValidatorsPubk is a list containing all the Vochain allowed Validators public keys
	ValidatorsPubK []tmtypes.Address `json:"minerspubk"`
	// CensusManagerPubk is a list containing all the public keys allowed to init a new voting process
	CensusManagersPubK []tmtypes.Address `json:"censusmanagerpubk"`
}

// ProcessState represents a state per process
type ProcessState struct {
	// ID id of the process
	ID string `json:"id"`
	// MkRoot merkle root of all the census in the process
	MkRoot string `json:"mkroot"`
	// InitBlock represents the tendermint block where the process goes from scheduled to active
	InitBlock tmtypes.Block `json:"initblock"`
	// EndBlock represents the tendermint block where the process goes from active to finished
	EndBlock tmtypes.Block `json:"endblock"`
	// EncryptionKeys are the keys required to encrypt the votes
	EncryptionKeys []tmtypes.Address `json:"encryptionkeys"`
	// CurrentState is the current process state
	CurrentState CurrentProcessState `json:"currentstate"`
	// Votes is a list containing all the processed and valid votes (here votes are final)
	Votes []Vote `json:"votes"`
}

// Vote represents a single vote
type Vote struct {
	// Nullifier is a special hash that prevents double voting
	Nullifier string `json:"nullifier"`
	// Payload contains the vote itself
	Payload string `json:"payload"`
	// CensusProof contains the prove indicating that the user is in the census of the process
	CensusProof string `json:"censusproof"`
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
	}
	return ""
}

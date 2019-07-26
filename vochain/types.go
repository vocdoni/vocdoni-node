package vochain

import (
	"fmt"

	tmtypes "github.com/tendermint/tendermint/types"
)

// GlobalState represents the Tendermint blockchain global state
type GlobalState struct {
	ProcessList        *ProcessState     `json:"processList"`
	ValidatorsPubK     []tmtypes.Address `json:"minerspubk"`
	CensusManagersPubK []tmtypes.Address `json:"censusmanagerpubk"`
}

// ProcessState represents a state per process
type ProcessState struct {
	ID             string              `json:"id"`
	MkRoot         string              `json:"mkroot"`
	InitBlock      tmtypes.Block       `json:"initblock"`
	EndBlock       tmtypes.Block       `json:"endblock"`
	EncryptionKeys []tmtypes.Address   `json:"encryptionkeys"`
	CurrentState   CurrentProcessState `json:"currentstate"`
	Votes          []Vote              `json:"votes"`
}

// Vote represents a single vote
type Vote struct {
	Nullifier   string `json:"nullifier"`
	Payload     string `json:"payload"`
	CensusProof string `json:"censusproof"`
	VoteProof   string `json:"voteproof"`
}

// CurrentProcessState represents the current phase of process state
type CurrentProcessState int8

const (
	processScheduled CurrentProcessState = iota
	processInProgress
	processPaused
	processResumed
	rocessFinished
)

// String returns the CurrentProcessState integer as string
func (c CurrentProcessState) String() string {
	return fmt.Sprintf("%d", c)
}

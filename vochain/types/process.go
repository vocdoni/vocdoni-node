package vochain

import (
	"fmt"
)

// ________________________ PROCESS ________________________

// Process represents a state per process
type Process struct {
	// EntityID identifies unequivocally a process
	EntityID string `json:"entityId"`
	// Votes is a list containing all the processed and valid votes (here votes are final)
	Votes map[string]*Vote `json:"votes"`
	// MkRoot merkle root of all the census in the process
	MkRoot string `json:"mkroot"`
	// NumberOfBlocks represents the amount of tendermint blocks that the process will last
	NumberOfBlocks int64 `json:"numberOfBlocks"`
	// StartBlock represents the tendermint block where the process goes from scheduled to active
	StartBlock int64 `json:"startBlock"`
	// CurrentState is the current process state
	CurrentState CurrentProcessState `json:"currentState"`
	// EncryptionKeys are the keys required to encrypt the votes
	EncryptionKeys []string `json:"encryptionKeys"`
}

func (p *Process) String() string {
	return fmt.Sprintf(`{
		"entityId": %v,
		"votes": %v,
		"mkRoot": %v,
		"startBlock": %v,
		"numberOfBlocks": %v,
		"encryptionKeys": %v,
		"currentState": %v }`,
		p.EntityID,
		p.Votes,
		p.MkRoot,
		p.StartBlock,
		p.NumberOfBlocks,
		p.EncryptionKeys,
		p.CurrentState,
	)
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

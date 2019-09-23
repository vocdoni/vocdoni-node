package vochain

import (
	"fmt"
	"math/big"
)

// ________________________ PROCESS ________________________

// Process represents a state per process
type Process struct {
	// EntityAddress identifies unequivocally a process
	EntityAddress string `json:"entityAddress"`
	// Votes is a list containing all the processed and valid votes (here votes are final)
	Votes map[string]*Vote `json:"votes"`
	// MkRoot merkle root of all the census in the process
	MkRoot string `json:"mkroot"`
	// NumberOfBlocks represents the amount of tendermint blocks that the process will last
	NumberOfBlocks *big.Int `json:"numberOfBlocks"`
	// StartBlock represents the tendermint block where the process goes from scheduled to active
	StartBlock *big.Int `json:"startBlock"`
	// CurrentState is the current process state
	CurrentState CurrentProcessState `json:"currentState"`
	// EncryptionPrivateKey are the keys required to encrypt the votes
	EncryptionPrivateKey string `json:"encryptionPrivateKey"`
}

func (p *Process) String() string {
	return fmt.Sprintf(`{
		"entityAddress": %s,
		"votes": %v,
		"mkRoot": %s,
		"startBlock": %v,
		"numberOfBlocks": %v,
		"encryptionPrivateKey": %s,
		"currentState": %v }`,
		p.EntityAddress,
		p.Votes,
		p.MkRoot,
		p.StartBlock,
		p.NumberOfBlocks,
		p.EncryptionPrivateKey,
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

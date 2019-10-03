package vochain

import (
	tmtypes "github.com/tendermint/tendermint/types"
)

// ________________________ STATE ________________________

// State represents the state of the application
type State struct {
	// Validators is a list containing all the Vochain allowed Validators public keys
	Validators []tmtypes.GenesisValidator `json:"validators"`
	// Oracles is a list containing all the public keys allowed to do interchain comunication
	Oracles []string `json:"oracles"`
	// Processes is a map containing all processes
	Processes map[string]*Process `json:"processes"`
}

// NewState returns a new State instance
func NewState() *State {
	return &State{
		Validators: make([]tmtypes.GenesisValidator, 0),
		Oracles:    make([]string, 0),
		Processes:  make(map[string]*Process, 0),
	}
}

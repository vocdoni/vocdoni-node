package vochain

import (
	tmtypes "github.com/tendermint/tendermint/types"
)

// ________________________ STATE ________________________

// State represents the state of our application
type State struct {
	// ValidatorsPubk is a list containing all the Vochain allowed Validators public keys
	ValidatorsPubK []tmtypes.Address `json:"minerspubk"`
	// TrustedOraclesPubK is a list containing all the public keys allowed to do interchain comunication
	TrustedOraclesPubK []tmtypes.Address   `json:"trustedoraclespubk"`
	Processes          map[string]*Process `json:"entities"`
}

// NewState returns a new State instance
func NewState() *State {
	return &State{
		ValidatorsPubK:     make([]tmtypes.Address, 0),
		TrustedOraclesPubK: make([]tmtypes.Address, 0),
		Processes:          make(map[string]*Process, 0),
	}
}

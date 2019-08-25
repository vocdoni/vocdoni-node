package vochain

import (
	abcitypes "github.com/tendermint/tendermint/abci/types"
	eth "gitlab.com/vocdoni/go-dvote/crypto/signature"
)

// ________________________ STATE ________________________

// State represents the state of our application
type State struct {
	// ValidatorsPubk is a list containing all the Vochain allowed Validators public keys
	ValidatorsPubK abcitypes.ValidatorUpdates `json:"validatorsPubK"`
	// TrustedOraclesPubK is a list containing all the public keys allowed to do interchain comunication
	TrustedOraclesPubK []eth.Address `json:"trustedOraclesPubK"`
	// Processes is a map containing all processes
	Processes map[string]*Process `json:"entities"`
}

// NewState returns a new State instance
func NewState() *State {
	return &State{
		ValidatorsPubK:     make([]abcitypes.ValidatorUpdate, 0),
		TrustedOraclesPubK: make([]eth.Address, 0),
		Processes:          make(map[string]*Process, 0),
	}
}

package types

import (
	"bytes"
	"fmt"
)

// TX

// Tx represents a transaction that has a valid method and the args for that method are also valids
type Tx struct {
	Method TxMethod `json:"method"`
	Args   []string `json:"args"`
}

// String converets a Tx struct to a human easy readable string format
func (tx Tx) String() string {
	var buffer bytes.Buffer
	for _, i := range tx.Args {
		buffer.WriteString(i)
	}
	return fmt.Sprintf(`{ "method": %s, args: %s }`, tx.Method.String(), buffer.String())
}

// GetMethod gets the method from the Tx struct
func (tx Tx) GetMethod() TxMethod {
	return tx.Method
}

// GetArgs gets the arguments from the Tx struct
func (tx Tx) GetArgs() []string {
	return tx.Args
}

// TXMETHOD

// TxMethod is a string representing the allowed methods in the Vochain paradigm
type TxMethod string

const (
	// NewProcessTx is the method name for init a new process
	NewProcessTx TxMethod = "newProcessTx"
	// VoteTx is the method name for casting a vote
	VoteTx TxMethod = "voteTx"
	// AddCensusManagerTx is the method name for adding a new Census Manager
	AddCensusManagerTx TxMethod = "addCensusManagerTx"
	// RemoveCensusManagerTx is the method name for removing an existing Census Manager
	RemoveCensusManagerTx TxMethod = "removeCensusManagerTx"
	// AddValidatorTx is the method name for adding a new validator address in the consensusParams validator list
	AddValidatorTx TxMethod = "addValidatorTx"
	// RemoveValidatorTx is the method name for removing an existing validator address in the consensusParams validator list
	RemoveValidatorTx TxMethod = "removeValidatorTx"
	// GetProcessState is the method name for getting basic info about a process
	GetProcessState TxMethod = "getProcessState"
)

// String returns the CurrentProcessState as string
func (m TxMethod) String() string {
	return fmt.Sprintf("%s", string(m))
}

// ValidateMethod returns true if the method is defined in the TxMethod enum
func (tx Tx) ValidateMethod() bool {
	m := tx.GetMethod()
	if m == NewProcessTx || m == VoteTx || m == AddCensusManagerTx || m == RemoveCensusManagerTx || m == AddValidatorTx || m == RemoveValidatorTx || m == GetProcessState {
		return true
	}
	return false
}

// TXARGS

// ValidateArgs does a sanity check onto the arguments passed to a valid TxMethod
func (tx Tx) ValidateArgs() bool {
	// TODO (currently just returns true if the method exists, args are basically ignored and can be any random bytes)
	switch tx.GetMethod() {
	case NewProcessTx:
		return true
	case VoteTx:
		return true
	case AddCensusManagerTx:
		return true
	case RemoveCensusManagerTx:
		return true
	case AddValidatorTx:
		return true
	case RemoveValidatorTx:
		return true
	case GetProcessState:
		return true
	}
	return false
}

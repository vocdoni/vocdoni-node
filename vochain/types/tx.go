package vochain

import (
	"errors"
	"fmt"
	"math/big"

	tmtypes "github.com/tendermint/tendermint/types"
	eth "gitlab.com/vocdoni/go-dvote/crypto/signature"
)

// ________________________ TX ________________________

// Tx represents a raw Tx that has a method, args and the signature of the method plus data
type Tx struct {
	Method    string                 `json:"method"`
	Args      map[string]interface{} `json:"args"`
	Signature string                 `json:"signature"`
}

// ValidateMethod returns true if the method is defined in the TxMethod enum
func (tx *Tx) ValidateMethod() TxMethod {
	m := tx.Method
	switch m {
	case "newProcessTx":
		return NewProcessTx
	case "voteTx":
		return VoteTx
	case "addOracleTx":
		return AddOracleTx
	case "removeOracleTx":
		return RemoveOracleTx
	case "addValidatorTx":
		return AddValidatorTx
	case "removeValidatorTx":
		return RemoveValidatorTx
	default:
		return InvalidTx
	}
}

// ________________________ TX METHODS ________________________

// TxMethod is a string representing the allowed methods in the Vochain paradigm
type TxMethod string

const (
	// NewProcessTx is the method name for start a new process
	NewProcessTx TxMethod = "newProcessTx"
	// VoteTx is the method name for casting a vote
	VoteTx TxMethod = "voteTx"
	// AddOracleTx is the method name for adding a new Census Manager
	AddOracleTx TxMethod = "addOracleTx"
	// RemoveOracleTx is the method name for removing an existing Census Manager
	RemoveOracleTx TxMethod = "removeOracleTx"
	// AddValidatorTx is the method name for adding a new validator address
	// in the consensusParams validator list
	AddValidatorTx TxMethod = "addvalidatortx"
	// RemoveValidatorTx is the method name for removing an existing validator address
	// in the consensusParams validator list
	RemoveValidatorTx TxMethod = "removeValidatorTx"
	// InvalidTx represents any Tx which is not valid
	InvalidTx TxMethod = "invalidTx"
)

// String returns the CurrentProcessState as string
func (m *TxMethod) String() string {
	return string(*m)
}

// ________________________ TX ARGS ________________________

var (
	newProcessTxArgsKeys = []string{
		"entityAddress",
		"mkRoot",
		"numberOfBlocks",
		"processId",
		"startBlock",
		"encryptionPrivateKey",
	}
	voteTxArgsKeys = []string{
		"processId",
		"proof",
		"nullifier",
		"nonce",
		"votePackage",
		"signature",
		"timestamp",
	}
	listUpdatesArgsKeys = []string{
		"address",
		"timestamp",
	}
)

// TxArgs generic interface to address valid method args
type TxArgs interface {
	String() string
}

// NewProcessTxArgs represents the data required in order to start a new process
type NewProcessTxArgs struct {
	ProcessID string `json:"processId"`
	// EntityID the process belongs to
	EntityAddress string `json:"entityAddress"`
	// MkRoot merkle root of all the census in the process
	MkRoot string `json:"mkRoot"`
	// NumberOfBlocks represents the tendermint block where the process
	// goes from active to finished
	NumberOfBlocks *big.Int `json:"numberOfBlocks"`
	// StartBlock represents the tendermint block where the process goes
	// from scheduled to active
	StartBlock *big.Int `json:"startBlock"`
	// EncryptionPrivateKey are the keys required to encrypt the votes
	EncryptionPrivateKey string `json:"encryptionPrivateKey"`
}

func (n *NewProcessTxArgs) String() string {
	return fmt.Sprintf(`{
		"method": "newProcessTx",
		"args" : {
		"encryptionPrivateKey": "%s", 
		"entityAddress": "%s", 
		"startBlock": %d, 
		"mkRoot": "%s", 
		"numberOfBlocks": %d,
		"processId": "%s" }}`,
		n.EncryptionPrivateKey,
		n.EntityAddress,
		n.StartBlock,
		n.MkRoot,
		n.NumberOfBlocks,
		n.ProcessID,
	)
}

// VoteTxArgs represents the data required in order to cast a vote
const VOteTxArgsSize = 6

type VoteTxArgs struct {
	// Nonce for avoid replay attacks
	Nonce string `json:"nonce,omitempty"`
	// Nullifier for the vote, unique identifyer
	Nullifier string `json:"nullifier,omitempty"`
	// ProcessID the id of the process
	ProcessID string `json:"processId,omitempty"`
	// Proof proof inclusion into the census of the process
	Proof string `json:"proof,omitempty"`
	// Signature sign( JSON.stringify( { nonce, processId, proof, 'vote-package' } ), privateKey )
	Signature string `json:"signature,omitempty"`
	// VotePackage vote data
	VotePackage string `json:"vote-package,omitempty"`
}

type VoteTxArgsSigned struct {
	// Nonce for avoid replay attacks
	Nonce string `json:"nonce,omitempty"`
	// Nullifier for the vote, unique identifyer
	Nullifier string `json:"nullifier,omitempty"`
	// ProcessID the id of the process
	ProcessID string `json:"processId,omitempty"`
	// Proof proof inclusion into the census of the process
	Proof string `json:"proof,omitempty"`
	// VotePackage vote data
	VotePackage string `json:"vote-package,omitempty"`
}

func (n *VoteTxArgs) String() string {
	return fmt.Sprintf(`{
		"method": voteTx,
		"args" : {
		"proof": "%s",
		"nullifier": "%s",
		"votePackage": "%s",
		"processId": "%s",
		"nonce": "%s",
		"signature": "%s" }}`,
		n.Proof,
		n.Nullifier,
		n.VotePackage,
		n.ProcessID,
		n.Nonce,
		n.Signature,
	)
}

// AddOracleTxArgs represents the data required in
// order to add a new  oracle
type AddOracleTxArgs struct {
	Address   eth.Address `json:"address"`
	Timestamp int64       `json:"timestamp"`
}

func (n *AddOracleTxArgs) String() string {
	return fmt.Sprintf(`{ "method": "addOracleTx", address": %v, "timestamp": %v }`, n.Address, n.Timestamp)
}

// RemoveOracleTxArgs represents the data required in
// order to remove an existing  oracle
type RemoveOracleTxArgs struct {
	Address   eth.Address `json:"address"`
	Timestamp int64       `json:"timestamp"`
}

func (n *RemoveOracleTxArgs) String() string {
	return fmt.Sprintf(`{ "method": "removeOracleTx", "address": %v, "timestamp": %v }`, n.Address, n.Timestamp)
}

// AddValidatorTxArgs represents the data required in
// order to add a new validator node
type AddValidatorTxArgs struct {
	Address   tmtypes.Address `json:"address"`
	Power     int64           `json:"power"`
	Timestamp int64           `json:"timestamp"`
}

func (n *AddValidatorTxArgs) String() string {
	return fmt.Sprintf(`{
		"method": "addValidatorTx", 
		"address": %v,
		"power": %v,
		"timestamp": %v }`,
		n.Address,
		n.Power,
		n.Timestamp,
	)
}

// RemoveValidatorTxArgs represents the data required in
// order to remove an existing validator node
type RemoveValidatorTxArgs struct {
	Address   tmtypes.Address `json:"address"`
	Timestamp int64           `json:"timestamp"`
}

func (n *RemoveValidatorTxArgs) String() string {
	return fmt.Sprintf(`{ "method": "removeValidatorTx", "address": %v, "timestamp": %v }`, n.Address, n.Timestamp)
}

func (tx *Tx) validateNewProcessTxArgs() (TxArgs, error) {
	var t TxArgs

	// invalid length
	if len(tx.Args) != 6 {
		return t, errors.New("Invalid args number")
	}

	// check if all keys exist
	allOk := true
	var errMsg string
	for _, m := range newProcessTxArgsKeys {
		if _, ok := tx.Args[m]; !ok {
			allOk = false
			errMsg = m
		}
	}

	// create tx args specific struct
	if allOk {
		t = &NewProcessTxArgs{
			EntityAddress:  tx.Args["entityAddress"].(string),
			MkRoot:         tx.Args["mkRoot"].(string),
			NumberOfBlocks: big.NewInt(int64(tx.Args["numberOfBlocks"].(float64))),
			StartBlock:     big.NewInt(int64(tx.Args["startBlock"].(float64))),
			//encryptionPublicKey: strings.Split(tx.Args["encryptionPublicKey"].(string), ","),
			EncryptionPrivateKey: tx.Args["encryptionPrivateKey"].(string),
			ProcessID:            tx.Args["processId"].(string),
		}
		// sanity check done
		return t, nil
	}
	return nil, fmt.Errorf("cannot parse %v", errMsg)
}

func (tx *Tx) validateVoteTxArgs() (TxArgs, error) {
	var t TxArgs

	// invalid length
	if len(tx.Args) != VOteTxArgsSize {
		return nil, errors.New("Invalid args number")
	}

	// check if all keys exist
	allOk := true
	var errMsg string
	for _, m := range voteTxArgsKeys {
		if _, ok := tx.Args[m]; !ok {
			allOk = false
			errMsg = m
		}
	}

	// create tx args specific struct
	if allOk {
		t = &VoteTxArgs{
			ProcessID:   tx.Args["processId"].(string),
			Nullifier:   tx.Args["nullifier"].(string),
			Nonce:       tx.Args["nonce"].(string),
			VotePackage: tx.Args["votePackage"].(string),
			Proof:       tx.Args["proof"].(string),
			Signature:   tx.Args["signature"].(string),
		}
		// sanity check done
		return t, nil

	}
	return nil, fmt.Errorf("cannot parse %s", errMsg)
}

/*
func (tx *Tx) validateAddOracleTxArgs() (TxArgs, error) {
	var t TxArgs
	// invalid length
	if len(tx.Args) != 2 {
		return nil, errors.New("Invalid args number")
	}

	// check if all keys exist
	allOk := true
	var errMsg string
	for _, m := range listUpdatesArgsKeys {
		if _, ok := tx.Args[m]; !ok {
			allOk = false
			errMsg = m
		}
	}

	// create tx args specific struct
	if allOk {
		a := eth.AddressFromString(tx.Args["address"])
		timestamp, err := strconv.ParseInt(tx.Args["timestamp"], 10, 64)
		if err == nil {
			t = &AddOracleTxArgs{
				Address:   a,
				Timestamp: timestamp,
			}
			return t, nil
		}
	}
	return nil, fmt.Errorf("cannot parse %v", errMsg)
}

func (tx *Tx) validateRemoveOracleTxArgs() (TxArgs, error) {
	var t TxArgs

	// invalid length
	if len(tx.Args) != 2 {
		return nil, errors.New("Invalid args number")
	}

	// check if all keys exist
	allOk := true
	var errMsg string
	for _, m := range listUpdatesArgsKeys {
		if _, ok := tx.Args[m]; !ok {
			allOk = false
			errMsg = m
		}
	}

	// create tx args specific struct
	if allOk {
		a := eth.AddressFromString(tx.Args["address"])
		timestamp, err := strconv.ParseInt(tx.Args["timestamp"], 10, 64)
		if err == nil {
			t = &RemoveOracleTxArgs{
				Address:   a,
				Timestamp: timestamp,
			}
			return t, nil
		}
	}
	return nil, fmt.Errorf("cannot parse %v", errMsg)
}

func (tx *Tx) validateAddValidatorTxArgs() (TxArgs, error) {
	var t TxArgs

	// invalid length
	if len(tx.Args) != 3 {
		return nil, errors.New("Invalid args number")
	}

	// check if all keys exist
	allOk := true
	var errMsg string
	for _, m := range listUpdatesArgsKeys {
		if _, ok := tx.Args[m]; !ok {
			allOk = false
			errMsg = m
		}
	}

	// create tx args specific struct
	if allOk {
		if _, ok := tx.Args["power"]; ok {
			power, err := strconv.ParseInt(tx.Args["power"], 10, 64)
			if err == nil {
				timestamp, err := strconv.ParseInt(tx.Args["timestamp"], 10, 64)
				if err == nil {
					t = &AddValidatorTxArgs{
						Address:   []byte(tx.Args["address"]),
						Timestamp: timestamp,
						Power:     power,
					}
					return t, nil
				}
			}
		}
		return nil, errors.New("cannot parse power")
	}
	return nil, fmt.Errorf("cannot parse %v", errMsg)
}

func (tx *Tx) validateRemoveValidatorTxArgs() (TxArgs, error) {
	var t TxArgs
	// invalid length
	if len(tx.Args) != 2 {
		return nil, errors.New("Invalid args number")
	}

	// check if all keys exist
	allOk := true
	var errMsg string
	for _, m := range listUpdatesArgsKeys {
		if _, ok := tx.Args[m]; !ok {
			allOk = false
			errMsg = m
		}
	}

	// create tx args specific struct
	if allOk {
		timestamp, err := strconv.ParseInt(tx.Args["timestamp"], 10, 64)
		if err == nil {
			t = &RemoveValidatorTxArgs{
				Address:   []byte(tx.Args["address"]),
				Timestamp: timestamp,
			}
			return t, nil
		}
	}
	return nil, fmt.Errorf("cannot parse %v", errMsg)
}
*/
// ValidateArgs does a sanity check onto the arguments passed to a valid TxMethod
func (tx *Tx) ValidateArgs() (TxArgs, error) {
	switch tx.Method {
	case "newProcessTx":
		return tx.validateNewProcessTxArgs()
	case "voteTx":
		return tx.validateVoteTxArgs()
	case "addOracleTx":
		//return tx.validateAddOracleTxArgs()
		return nil, nil
	case "removeOracleTx":
		///return tx.validateRemoveOracleTxArgs()
		return nil, nil
	case "addValidatorTx":
		//return tx.validateAddValidatorTxArgs()
		return nil, nil
	case "removeValidatorTx":
		//return tx.validateRemoveValidatorTxArgs()
		return nil, nil
	default:
		return nil, errors.New("Cannot validate args")
	}
}

// ________________________ VALID TX ________________________

// ValidTx represents a Tx with a valid method and valid args for the method
type ValidTx struct {
	Method TxMethod `json:"method"`
	Args   TxArgs   `json:"args"`
}

// String converets a ValidTx struct to a human easy readable string format
func (vtx *ValidTx) String() string {
	return fmt.Sprintf(`{
		"method": %s,
		"args": %v}`,
		vtx.Method.String(),
		vtx.Args,
	)
}

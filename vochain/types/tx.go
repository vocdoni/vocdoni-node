package vochain

import (
	"errors"
	"fmt"

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
		"entityId",
		"entityResolver",
		"metadataHash",
		"mkRoot",
		"numberOfBlocks",
		"processId",
		"startBlock",
		"encryptionPublicKey",
		"timestamp",
	}
	voteTxArgsKeys = []string{
		"processId",
		"nullifier",
		"payload",
		"censusProof",
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
	EntityID string `json:"entityId"`
	// EntityResolver the resolver of the entity
	EntityResolver string `json:"entityResolver"`
	// MetadataHash hash of the entity metadata
	MetadataHash string `json:"metadataHash"`
	// MkRoot merkle root of all the census in the process
	MkRoot string `json:"mkRoot"`
	// NumberOfBlocks represents the tendermint block where the process
	// goes from active to finished
	NumberOfBlocks int64 `json:"numberOfBlocks"`
	// StartBlock represents the tendermint block where the process goes
	// from scheduled to active
	StartBlock int64 `json:"startBlock"`
	// encryptionPublicKey are the keys required to encrypt the votes
	EncryptionPublicKey string `json:"encryptionPublicKey"`
	// Timestamp for avoid flooding atacks
	Timestamp int64 `json:"timestamp"`
}

func (n *NewProcessTxArgs) String() string {
	return fmt.Sprintf(`{
		"method": newProcessTx,
		"encryptionPublicKey": %s 
		"entityId": "%s", 
		"entityResolver": "%s",
		"startBlock": %d, 
		"metadataHash": "%s", 
		"mkRoot": %s, 
		"numberOfBlocks": %d,
		"processId": "%s",
		"timestamp": %d}`,
		n.EncryptionPublicKey,
		n.EntityID,
		n.EntityResolver,
		n.StartBlock,
		n.MetadataHash,
		n.MkRoot,
		n.NumberOfBlocks,
		n.ProcessID,
		n.Timestamp,
	)
}

// VoteTxArgs represents the data required in order to cast a vote
type VoteTxArgs struct {
	// ProcessID the id of the process
	ProcessID string `json:"processId"`
	// Nullifier for the vote, unique identifyer
	Nullifier string `json:"nullifier"`
	// Payload vote data
	Payload string `json:"payload"`
	// CensusProof proof inclusion into the census of the process
	CensusProof string `json:"censusProof"`
	// Timestamp for avoid flooding atacks
	Timestamp int64 `json:"timestamp"`
}

func (n *VoteTxArgs) String() string {
	return fmt.Sprintf(`{
		"method": voteTx,
		"censusProof": "%s",
		"nullifier": "%s",
		"payload": "%s",
		"processId": "%s",
		"timestamp: %d }`,
		n.CensusProof,
		n.Nullifier,
		n.Payload,
		n.ProcessID,
		n.Timestamp,
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
	if len(tx.Args) != 9 {
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
			EntityID:       tx.Args["entityId"].(string),
			EntityResolver: tx.Args["entityResolver"].(string),
			MetadataHash:   tx.Args["metadataHash"].(string),
			MkRoot:         tx.Args["mkRoot"].(string),
			NumberOfBlocks: int64(tx.Args["numberOfBlocks"].(float64)),
			StartBlock:     int64(tx.Args["startBlock"].(float64)),
			//encryptionPublicKey: strings.Split(tx.Args["encryptionPublicKey"].(string), ","),
			EncryptionPublicKey: tx.Args["encryptionPublicKey"].(string),
			Timestamp:           int64(tx.Args["timestamp"].(float64)),
			ProcessID:           tx.Args["processId"].(string),
		}
		// sanity check done
		return t, nil

	}
	return nil, fmt.Errorf("cannot parse %v", errMsg)
}

func (tx *Tx) validateVoteTxArgs() (TxArgs, error) {
	var t TxArgs

	// invalid length
	if len(tx.Args) != 5 {
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
			Payload:     tx.Args["payload"].(string),
			CensusProof: tx.Args["censusProof"].(string),
			Timestamp:   int64(tx.Args["timestamp"].(float64)),
		}
		// sanity check done
		return t, nil

	}
	return nil, fmt.Errorf("cannot parse %v", errMsg)
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

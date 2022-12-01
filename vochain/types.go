package vochain

import (
	"encoding/json"
	"fmt"
	"reflect"
	"time"

	"go.vocdoni.io/dvote/types"
	"go.vocdoni.io/proto/build/go/models"
)

var (
	ErrInvalidURILength = fmt.Errorf("invalid URI length")
	ErrInvalidAddress   = fmt.Errorf("invalid address")
	ErrNilTx            = fmt.Errorf("nil transaction")
)

// PrefixDBCacheSize is the size of the cache for the MutableTree IAVL databases
var PrefixDBCacheSize = 0

// VotePackage represents the payload of a vote (usually base64 encoded)
type VotePackage struct {
	Nonce string `json:"nonce,omitempty"`
	Votes []int  `json:"votes"`
}

// ________________________ QUERIES ________________________

// QueryData is an abstraction of any kind of data a query request could have
type QueryData struct {
	Method      string `json:"method"`
	ProcessID   string `json:"processId,omitempty"`
	Nullifier   string `json:"nullifier,omitempty"`
	From        int64  `json:"from,omitempty"`
	ListSize    int64  `json:"listSize,omitempty"`
	Timestamp   int64  `json:"timestamp,omitempty"`
	ProcessType string `json:"type,omitempty"`
}

// ________________________ TRANSACTION COSTS __________________________
// TransactionCosts describes how much each operation should cost
type TransactionCosts struct {
	SetProcessStatus        uint32 `json:"Tx_SetProcessStatus"`
	SetProcessCensus        uint32 `json:"Tx_SetProcessCensus"`
	SetProcessResults       uint32 `json:"Tx_SetProcessResults"`
	SetProcessQuestionIndex uint32 `json:"Tx_SetProcessQuestionIndex"`
	RegisterKey             uint32 `json:"Tx_RegisterKey"`
	NewProcess              uint32 `json:"Tx_NewProcess"`
	SendTokens              uint32 `json:"Tx_SendTokens"`
	SetAccountInfoURI       uint32 `json:"Tx_SetAccountInfoURI"`
	CreateAccount           uint32 `json:"Tx_CreateAccount"`
	AddDelegateForAccount   uint32 `json:"Tx_AddDelegateForAccount"`
	DelDelegateForAccount   uint32 `json:"Tx_DelDelegateForAccount"`
	CollectFaucet           uint32 `json:"Tx_CollectFaucet"`
}

// AsMap returns the contents of TransactionCosts as a map. Its purpose
// is to keep knowledge of TransactionCosts' fields within itself, so the
// function using it only needs to iterate over the key-values.
func (t *TransactionCosts) AsMap() map[models.TxType]uint64 {
	b := make(map[models.TxType]uint64)

	tType := reflect.TypeOf(*t)
	tValue := reflect.ValueOf(*t)
	for i := 0; i < tType.NumField(); i++ {
		key := TxCostNameToTxType(tType.Field(i).Name)
		b[key] = tValue.Field(i).Uint()
	}
	return b
}

// TxCostNameToTxTypeMap maps a valid string to a txType
var TxCostNameToTxTypeMap = map[string]models.TxType{
	"SetProcessStatus":        models.TxType_SET_PROCESS_STATUS,
	"SetProcessCensus":        models.TxType_SET_PROCESS_CENSUS,
	"SetProcessResults":       models.TxType_SET_PROCESS_RESULTS,
	"SetProcessQuestionIndex": models.TxType_SET_PROCESS_QUESTION_INDEX,
	"SendTokens":              models.TxType_SEND_TOKENS,
	"SetAccountInfoURI":       models.TxType_SET_ACCOUNT_INFO_URI,
	"CreateAccount":           models.TxType_CREATE_ACCOUNT,
	"RegisterKey":             models.TxType_REGISTER_VOTER_KEY,
	"NewProcess":              models.TxType_NEW_PROCESS,
	"AddDelegateForAccount":   models.TxType_ADD_DELEGATE_FOR_ACCOUNT,
	"DelDelegateForAccount":   models.TxType_DEL_DELEGATE_FOR_ACCOUNT,
	"CollectFaucet":           models.TxType_COLLECT_FAUCET,
}

// TxCostNameToTxType converts a valid string to a txType
func TxCostNameToTxType(key string) models.TxType {
	if _, ok := TxCostNameToTxTypeMap[key]; ok {
		return TxCostNameToTxTypeMap[key]
	}
	return models.TxType_TX_UNKNOWN
}

// TxTypeToCostNameMap maps a valid txType to a string
var TxTypeToCostNameMap = map[models.TxType]string{
	models.TxType_SET_PROCESS_STATUS:         "SetProcessStatus",
	models.TxType_SET_PROCESS_CENSUS:         "SetProcessCensus",
	models.TxType_SET_PROCESS_RESULTS:        "SetProcessResults",
	models.TxType_SET_PROCESS_QUESTION_INDEX: "SetProcessQuestionIndex",
	models.TxType_SEND_TOKENS:                "SendTokens",
	models.TxType_SET_ACCOUNT_INFO_URI:       "SetAccountInfoURI",
	models.TxType_CREATE_ACCOUNT:             "CreateAccount",
	models.TxType_REGISTER_VOTER_KEY:         "RegisterKey",
	models.TxType_NEW_PROCESS:                "NewProcess",
	models.TxType_ADD_DELEGATE_FOR_ACCOUNT:   "AddDelegateForAccount",
	models.TxType_DEL_DELEGATE_FOR_ACCOUNT:   "DelDelegateForAccount",
	models.TxType_COLLECT_FAUCET:             "CollectFaucet",
}

// TxTypeToCostName converts a valid txType to a string
func TxTypeToCostName(txType models.TxType) string {
	if _, ok := TxTypeToCostNameMap[txType]; ok {
		return TxTypeToCostNameMap[txType]
	}
	return models.TxType_TX_UNKNOWN.String()
}

// ________________________ GENESIS APP STATE ________________________

// GenesisAccount represents an account in the genesis app state
type GenesisAccount struct {
	Address types.HexBytes `json:"address"`
	Balance uint64         `json:"balance"`
}

// GenesisAppState application state in genesis
type GenesisAppState struct {
	Validators []GenesisValidator `json:"validators"`
	Oracles    []types.HexBytes   `json:"oracles"`
	Accounts   []GenesisAccount   `json:"accounts"`
	Treasurer  types.HexBytes     `json:"treasurer"`
	TxCost     TransactionCosts   `json:"tx_cost"`
}

// The rest of these genesis app state types are copied from
// github.com/tendermint/tendermint/types, for the sake of making this package
// lightweight and not have it import heavy indirect dependencies like grpc or
// crypto/*.

type GenesisDoc struct {
	GenesisTime     time.Time          `json:"genesis_time"`
	ChainID         string             `json:"chain_id"`
	ConsensusParams *ConsensusParams   `json:"consensus_params,omitempty"`
	Validators      []GenesisValidator `json:"validators,omitempty"`
	AppHash         types.HexBytes     `json:"app_hash"`
	AppState        json.RawMessage    `json:"app_state,omitempty"`
}

type ConsensusParams struct {
	Block     BlockParams     `json:"block"`
	Evidence  EvidenceParams  `json:"evidence"`
	Validator ValidatorParams `json:"validator"`
	Version   VersionParams   `json:"version"`
}

// BlockParams define limits on the block size and gas plus minimum time
// between blocks.
type BlockParams struct {
	MaxBytes int64 `json:"max_bytes"`
	MaxGas   int64 `json:"max_gas"`
}

type EvidenceParams struct {
	MaxAgeNumBlocks int64 `json:"max_age_num_blocks"`
	// only accept new evidence more recent than this
	MaxAgeDuration time.Duration `json:"max_age_duration"`
}

type ValidatorParams struct {
	PubKeyTypes []string `json:"pub_key_types"`
}

type VersionParams struct {
	AppVersion uint64 `json:"app_version"`
}

type GenesisValidator struct {
	Address types.HexBytes   `json:"address"`
	PubKey  TendermintPubKey `json:"pub_key"`
	Power   string           `json:"power"`
	Name    string           `json:"name"`
}

type TendermintPubKey struct {
	Type  string `json:"type"`
	Value []byte `json:"value"`
}

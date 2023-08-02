package genesis

import (
	"reflect"

	"go.vocdoni.io/proto/build/go/models"
)

// TransactionCosts describes how much each operation should cost
type TransactionCosts struct {
	SetProcessStatus        uint32 `json:"Tx_SetProcessStatus"`
	SetProcessCensus        uint32 `json:"Tx_SetProcessCensus"`
	SetProcessQuestionIndex uint32 `json:"Tx_SetProcessQuestionIndex"`
	RegisterKey             uint32 `json:"Tx_RegisterKey"`
	NewProcess              uint32 `json:"Tx_NewProcess"`
	SendTokens              uint32 `json:"Tx_SendTokens"`
	SetAccountInfoURI       uint32 `json:"Tx_SetAccountInfoURI"`
	CreateAccount           uint32 `json:"Tx_CreateAccount"`
	AddDelegateForAccount   uint32 `json:"Tx_AddDelegateForAccount"`
	DelDelegateForAccount   uint32 `json:"Tx_DelDelegateForAccount"`
	CollectFaucet           uint32 `json:"Tx_CollectFaucet"`
	SetAccountSIK           uint32 `json:"Tx_SetSik"`
	DelAccountSIK           uint32 `json:"Tx_DelSik"`
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
	"SetAccountSIK":           models.TxType_SET_ACCOUNT_SIK,
	"DelAccountSIK":           models.TxType_DEL_ACCOUNT_SIK,
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
	models.TxType_SET_ACCOUNT_SIK:            "SetAccountSIK",
	models.TxType_DEL_ACCOUNT_SIK:            "DelAccountSIK",
}

// TxTypeToCostName converts a valid txType to a string
func TxTypeToCostName(txType models.TxType) string {
	if _, ok := TxTypeToCostNameMap[txType]; ok {
		return TxTypeToCostNameMap[txType]
	}
	return models.TxType_TX_UNKNOWN.String()
}

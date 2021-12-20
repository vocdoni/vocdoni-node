package vochain

import (
	"reflect"
	"testing"

	qt "github.com/frankban/quicktest"
	"go.vocdoni.io/proto/build/go/models"
)

func TestTransactionCostsAsMap(t *testing.T) {
	txCosts := TransactionCosts{
		SetProcessStatus:        100,
		SetProcessCensus:        100,
		SetProcessResults:       100,
		SetProcessQuestionIndex: 100,
		RegisterKey:             100,
		NewProcess:              100,
		SendTokens:              100,
		SetAccountInfo:          100,
		AddDelegateForAccount:   100,
		DelDelegateForAccount:   100,
		CollectFaucet:           100,
	}
	txCostsBytes := txCosts.AsMap()

	expected := map[models.TxType]uint64{
		models.TxType_SET_PROCESS_STATUS:         100,
		models.TxType_SET_PROCESS_CENSUS:         100,
		models.TxType_SET_PROCESS_RESULTS:        100,
		models.TxType_SET_PROCESS_QUESTION_INDEX: 100,
		models.TxType_REGISTER_VOTER_KEY:         100,
		models.TxType_NEW_PROCESS:                100,
		models.TxType_SEND_TOKENS:                100,
		models.TxType_SET_ACCOUNT_INFO:           100,
		models.TxType_ADD_DELEGATE_FOR_ACCOUNT:   100,
		models.TxType_DEL_DELEGATE_FOR_ACCOUNT:   100,
		models.TxType_COLLECT_FAUCET:             100,
	}
	qt.Assert(t, txCostsBytes, qt.DeepEquals, expected)
}
func TestTxCostNameToTxType(t *testing.T) {
	fields := map[string]models.TxType{
		"SetProcessStatus":        models.TxType_SET_PROCESS_STATUS,
		"SetProcessCensus":        models.TxType_SET_PROCESS_CENSUS,
		"SetProcessResults":       models.TxType_SET_PROCESS_RESULTS,
		"SetProcessQuestionIndex": models.TxType_SET_PROCESS_QUESTION_INDEX,
		"RegisterKey":             models.TxType_REGISTER_VOTER_KEY,
		"NewProcess":              models.TxType_NEW_PROCESS,
		"SendTokens":              models.TxType_SEND_TOKENS,
		"SetAccountInfo":          models.TxType_SET_ACCOUNT_INFO,
		"AddDelegateForAccount":   models.TxType_ADD_DELEGATE_FOR_ACCOUNT,
		"DelDelegateForAccount":   models.TxType_DEL_DELEGATE_FOR_ACCOUNT,
		"CollectFaucet":           models.TxType_COLLECT_FAUCET,
	}
	for k, v := range fields {
		qt.Assert(t, TxCostNameToTxType(k), qt.Equals, v)

		// check that TransactionCosts struct does have the fields that we
		// specify in this test
		_, found := reflect.TypeOf(TransactionCosts{}).FieldByName(k)
		qt.Assert(t, found, qt.IsTrue)
	}
}

package vochain

import (
	"reflect"
	"testing"

	qt "github.com/frankban/quicktest"
	"go.vocdoni.io/dvote/vochain/genesis"
	"go.vocdoni.io/proto/build/go/models"
)

func TestTransactionCostsAsMap(t *testing.T) {
	txCosts := genesis.TransactionCosts{
		SetProcessStatus:        100,
		SetProcessCensus:        200,
		SetProcessResults:       300,
		SetProcessQuestionIndex: 400,
		RegisterKey:             500,
		NewProcess:              600,
		SendTokens:              700,
		SetAccountInfoURI:       800,
		AddDelegateForAccount:   900,
		DelDelegateForAccount:   1000,
		CollectFaucet:           1100,
		CreateAccount:           1200,
		SetAccountSIK:           1300,
		DelAccountSIK:           1400,
	}
	txCostsBytes := txCosts.AsMap()

	expected := map[models.TxType]uint64{
		models.TxType_SET_PROCESS_STATUS:         100,
		models.TxType_SET_PROCESS_CENSUS:         200,
		models.TxType_SET_PROCESS_RESULTS:        300,
		models.TxType_SET_PROCESS_QUESTION_INDEX: 400,
		models.TxType_REGISTER_VOTER_KEY:         500,
		models.TxType_NEW_PROCESS:                600,
		models.TxType_SEND_TOKENS:                700,
		models.TxType_SET_ACCOUNT_INFO_URI:       800,
		models.TxType_ADD_DELEGATE_FOR_ACCOUNT:   900,
		models.TxType_DEL_DELEGATE_FOR_ACCOUNT:   1000,
		models.TxType_COLLECT_FAUCET:             1100,
		models.TxType_CREATE_ACCOUNT:             1200,
		models.TxType_SET_ACCOUNT_SIK:            1300,
		models.TxType_DEL_ACCOUNT_SIK:            1400,
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
		"SetAccountInfoURI":       models.TxType_SET_ACCOUNT_INFO_URI,
		"AddDelegateForAccount":   models.TxType_ADD_DELEGATE_FOR_ACCOUNT,
		"DelDelegateForAccount":   models.TxType_DEL_DELEGATE_FOR_ACCOUNT,
		"CollectFaucet":           models.TxType_COLLECT_FAUCET,
		"SetAccountSIK":           models.TxType_SET_ACCOUNT_SIK,
		"DelAccountSIK":           models.TxType_DEL_ACCOUNT_SIK,
	}
	for k, v := range fields {
		qt.Assert(t, genesis.TxCostNameToTxType(k), qt.Equals, v)

		// check that TransactionCosts struct does have the fields that we
		// specify in this test
		_, found := reflect.TypeOf(genesis.TransactionCosts{}).FieldByName(k)
		qt.Assert(t, found, qt.IsTrue)
	}
}

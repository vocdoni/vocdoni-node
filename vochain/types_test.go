package vochain

import (
	"reflect"
	"testing"

	qt "github.com/frankban/quicktest"
)

func TestTransactionCostsAsMap(t *testing.T) {
	txCosts := TransactionCosts{
		SetProcess:            0,
		RegisterKey:           1,
		NewProcess:            2,
		SendTokens:            3,
		SetAccountInfo:        4,
		AddDelegateForAccount: 5,
		DelDelegateForAccount: 6,
		CollectFaucet:         7,
	}
	txCostsBytes := txCosts.AsMap()

	expected := map[string]uint64{
		kSetProcess:            0,
		kRegisterKey:           1,
		kNewProcess:            2,
		kSendTokens:            3,
		kSetAccountInfo:        4,
		kAddDelegateForAccount: 5,
		kDelDelegateForAccount: 6,
		kCollectFaucet:         7,
	}
	qt.Assert(t, txCostsBytes, qt.DeepEquals, expected)
}
func TestTransactionCostsFieldUsesSameKeysAsState(t *testing.T) {
	fields := map[string]string{
		"SetProcess":            kSetProcess,
		"RegisterKey":           kRegisterKey,
		"NewProcess":            kNewProcess,
		"SendTokens":            kSendTokens,
		"SetAccountInfo":        kSetAccountInfo,
		"AddDelegateForAccount": kAddDelegateForAccount,
		"DelDelegateForAccount": kDelDelegateForAccount,
		"CollectFaucet":         kCollectFaucet,
	}
	for k, v := range fields {
		qt.Assert(t, TransactionCostsFieldToStateKey(k), qt.Equals, v)

		// check that TransactionCosts struct does have the fields that we
		// specify in this test
		_, found := reflect.TypeOf(TransactionCosts{}).FieldByName(k)
		qt.Assert(t, found, qt.IsTrue)
	}
}

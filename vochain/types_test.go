package vochain

import (
	"fmt"
	"reflect"
	"testing"

	qt "github.com/frankban/quicktest"
)

func TestTransactionCostsStructAsBytes(t *testing.T) {
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
	txCostsBytes, err := txCosts.StructAsBytes()
	qt.Assert(t, err, qt.IsNil)

	expected := map[string][]byte{
		kAddDelegateForAccount: {5, 0, 0, 0, 0, 0, 0, 0},
		kCollectFaucet:         {7, 0, 0, 0, 0, 0, 0, 0},
		kDelDelegateForAccount: {6, 0, 0, 0, 0, 0, 0, 0},
		kNewProcess:            {2, 0, 0, 0, 0, 0, 0, 0},
		kRegisterKey:           {1, 0, 0, 0, 0, 0, 0, 0},
		kSendTokens:            {3, 0, 0, 0, 0, 0, 0, 0},
		kSetAccountInfo:        {4, 0, 0, 0, 0, 0, 0, 0},
		kSetProcess:            {0, 0, 0, 0, 0, 0, 0, 0},
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

func TestTransactionCostsFieldFromStateKey(t *testing.T) {
	fieldName, err := TransactionCostsFieldFromStateKey("c_setProcess")
	qt.Assert(t, err, qt.IsNil)
	qt.Assert(t, fieldName, qt.Equals, "SetProcess")
	fmt.Println("Fieldname", fieldName)

	_, err = TransactionCostsFieldFromStateKey("c_fictionalField")
	qt.Assert(t, err, qt.IsNotNil)

	_, err = TransactionCostsFieldFromStateKey("c_")
	qt.Assert(t, err, qt.ErrorMatches, "state key must have a length greater than .*")
}

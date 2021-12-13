package vochain

import (
	"fmt"
	"testing"

	qt "github.com/frankban/quicktest"
)

func TestTransactionCostsStructAsBytes(t *testing.T) {
	txCosts := TransactionCosts{
		SetProcess:            10,
		RegisterKey:           10,
		NewProcess:            10,
		SendTokens:            10,
		SetAccountInfo:        10,
		AddDelegateForAccount: 10,
		DelDelegateForAccount: 10,
		CollectFaucet:         10,
	}
	txCostsBytes, err := txCosts.StructAsBytes()
	qt.Assert(t, err, qt.IsNil)

	expected := map[string][]byte{
		"c_addDelegateForAccount": {10, 0, 0, 0, 0, 0, 0, 0},
		"c_collectFaucet":         {10, 0, 0, 0, 0, 0, 0, 0},
		"c_delDelegateForAccount": {10, 0, 0, 0, 0, 0, 0, 0},
		"c_newProcess":            {10, 0, 0, 0, 0, 0, 0, 0},
		"c_registerKey":           {10, 0, 0, 0, 0, 0, 0, 0},
		"c_sendTokens":            {10, 0, 0, 0, 0, 0, 0, 0},
		"c_setAccountInfo":        {10, 0, 0, 0, 0, 0, 0, 0},
		"c_setProcess":            {10, 0, 0, 0, 0, 0, 0, 0},
	}
	qt.Assert(t, txCostsBytes, qt.DeepEquals, expected)
}

func TestTransactionCostsFieldFromStateKey(t *testing.T) {
	fieldName, err := TransactionCostsFieldFromStateKey("c_setProcess")
	qt.Assert(t, err, qt.IsNil)
	qt.Assert(t, fieldName, qt.Equals, "SetProcess")
	fmt.Println("Fieldname", fieldName)

	_, err = TransactionCostsFieldFromStateKey("c_fictionalField")
	qt.Assert(t, err, qt.IsNotNil)
}

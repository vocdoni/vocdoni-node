package vochain

import (
	"fmt"
	"testing"

	"github.com/ethereum/go-ethereum/common"
	abcitypes "github.com/tendermint/tendermint/abci/types"
	"go.vocdoni.io/dvote/crypto/ethereum"
	"go.vocdoni.io/proto/build/go/models"
	"google.golang.org/protobuf/proto"
)

func TestAddOracle(t *testing.T) {
	app := TestBaseApplication(t)
	oracle := ethereum.SignKeys{}
	if err := oracle.Generate(); err != nil {
		t.Fatal(err)
	}
	if err := app.State.AddOracle(common.HexToAddress(oracle.AddressString())); err != nil {
		t.Fatal(err)
	}

	// same addr should fail
	if err := testAddOracle(t, &oracle, app, oracle.Address()); err == nil {
		t.Fatal(err)
	}

	// new addr should pass
	newOracle := ethereum.SignKeys{}
	if err := newOracle.Generate(); err != nil {
		t.Fatal(err)
	}
	if err := testAddOracle(t, &oracle, app, newOracle.Address()); err != nil {
		t.Fatal(err)
	}

	// invalid oracle addr should fail
	if err := testAddOracle(t, &oracle, app, common.Address{}); err == nil {
		t.Fatal(err)
	}
}

func testAddOracle(t *testing.T,
	oracle *ethereum.SignKeys,
	app *BaseApplication,
	newOracle common.Address) error {
	var cktx abcitypes.RequestCheckTx
	var detx abcitypes.RequestDeliverTx
	var cktxresp abcitypes.ResponseCheckTx
	var detxresp abcitypes.ResponseDeliverTx
	var stx models.SignedTx
	var err error

	tx := &models.AdminTx{
		Txtype:  models.TxType_ADD_ORACLE,
		Address: newOracle.Bytes(),
	}

	if stx.Tx, err = proto.Marshal(&models.Tx{Payload: &models.Tx_Admin{Admin: tx}}); err != nil {
		t.Fatal(err)
	}

	if stx.Signature, err = oracle.SignVocdoni(stx.Tx); err != nil {
		t.Fatal(err)
	}

	if cktx.Tx, err = proto.Marshal(&stx); err != nil {
		t.Fatal(err)
	}
	cktxresp = app.CheckTx(cktx)
	if cktxresp.Code != 0 {
		return fmt.Errorf("checkTx failed: %s", cktxresp.Data)
	}
	if detx.Tx, err = proto.Marshal(&stx); err != nil {
		t.Fatal(err)
	}
	detxresp = app.DeliverTx(detx)
	if detxresp.Code != 0 {
		return fmt.Errorf("deliverTx failed: %s", detxresp.Data)
	}
	app.Commit()
	return nil
}

func TestRemoveOracle(t *testing.T) {
	app := TestBaseApplication(t)
	oracle := ethereum.SignKeys{}
	if err := oracle.Generate(); err != nil {
		t.Fatal(err)
	}

	if err := app.State.AddOracle(common.HexToAddress(oracle.AddressString())); err != nil {
		t.Fatal(err)
	}

	// not in list should fail
	newOracle := ethereum.SignKeys{}
	if err := newOracle.Generate(); err != nil {
		t.Fatal(err)
	}
	if err := testRemoveOracle(t, &oracle, app, newOracle.Address()); err == nil {
		t.Fatal(err)
	}

	// if in list should pass
	if err := testRemoveOracle(t, &oracle, app, oracle.Address()); err != nil {
		t.Fatal(err)
	}

	if err := app.State.AddOracle(common.HexToAddress(oracle.AddressString())); err != nil {
		t.Fatal(err)
	}

	// invalid oracle addr should fail
	if err := testRemoveOracle(t, &oracle, app, common.Address{}); err == nil {
		t.Fatal(err)
	}
}

func testRemoveOracle(t *testing.T, oracle *ethereum.SignKeys,
	app *BaseApplication, newOracle common.Address) error {
	var cktx abcitypes.RequestCheckTx
	var detx abcitypes.RequestDeliverTx
	var cktxresp abcitypes.ResponseCheckTx
	var detxresp abcitypes.ResponseDeliverTx
	var stx models.SignedTx
	var err error

	tx := &models.AdminTx{
		Txtype:  models.TxType_REMOVE_ORACLE,
		Address: newOracle.Bytes(),
	}

	if stx.Tx, err = proto.Marshal(&models.Tx{Payload: &models.Tx_Admin{Admin: tx}}); err != nil {
		t.Fatal(err)
	}

	if stx.Signature, err = oracle.SignVocdoni(stx.Tx); err != nil {
		t.Fatal(err)
	}

	if cktx.Tx, err = proto.Marshal(&stx); err != nil {
		t.Fatal(err)
	}
	cktxresp = app.CheckTx(cktx)
	if cktxresp.Code != 0 {
		return fmt.Errorf("checkTx failed: %s", cktxresp.Data)
	}
	if detx.Tx, err = proto.Marshal(&stx); err != nil {
		t.Fatal(err)
	}
	detxresp = app.DeliverTx(detx)
	if detxresp.Code != 0 {
		return fmt.Errorf("deliverTx failed: %s", detxresp.Data)
	}
	app.Commit()
	return nil
}

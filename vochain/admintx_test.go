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
	app, err := NewBaseApplication(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	oracle := ethereum.SignKeys{}
	if err := oracle.Generate(); err != nil {
		t.Fatal(err)
	}
	if err := app.State.AddOracle(common.HexToAddress(oracle.AddressString())); err != nil {
		t.Fatal(err)
	}

	// same addr should fail
	newOracle := oracle.Address()
	if err := testAddOracle(t, &oracle, app, newOracle); err == nil {
		t.Fatal(err)
	}

	// new addr should pass
	newOracle2 := ethereum.SignKeys{}
	if err := newOracle2.Generate(); err != nil {
		t.Fatal(err)
	}
	if err := testAddOracle(t, &oracle, app, newOracle2.Address()); err != nil {
		t.Fatal(err)
	}

	// invalid oracle addr should fail
	if err := testAddOracle(t, &oracle, app, common.Address{}); err == nil {
		t.Fatal(err)
	}
}

func testAddOracle(t *testing.T, oracle *ethereum.SignKeys, app *BaseApplication, newOracle common.Address) error {
	var cktx abcitypes.RequestCheckTx
	var detx abcitypes.RequestDeliverTx
	var cktxresp abcitypes.ResponseCheckTx
	var detxresp abcitypes.ResponseDeliverTx
	var vtx models.Tx

	tx := &models.AdminTx{
		Txtype:  models.TxType_ADD_ORACLE,
		Address: newOracle.Bytes(),
	}
	txBytes, err := proto.Marshal(tx)
	if err != nil {
		t.Fatal(err)
	}

	if vtx.Signature, err = oracle.Sign(txBytes); err != nil {
		t.Fatal(err)
	}
	vtx.Payload = &models.Tx_Admin{Admin: tx}

	if cktx.Tx, err = proto.Marshal(&vtx); err != nil {
		t.Fatal(err)
	}
	cktxresp = app.CheckTx(cktx)
	if cktxresp.Code != 0 {
		return fmt.Errorf("checkTx failed: %s", cktxresp.Data)
	}
	if detx.Tx, err = proto.Marshal(&vtx); err != nil {
		t.Fatal(err)
	}
	detxresp = app.DeliverTx(detx)
	if detxresp.Code != 0 {
		return fmt.Errorf("deliverTx failed: %s", detxresp.Data)
	}
	app.Commit()
	return nil
}

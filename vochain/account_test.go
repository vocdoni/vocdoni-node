package vochain

import (
	"fmt"
	"testing"

	qt "github.com/frankban/quicktest"
	abcitypes "github.com/tendermint/tendermint/abci/types"

	"go.vocdoni.io/dvote/crypto/ethereum"
	"go.vocdoni.io/proto/build/go/models"
	"google.golang.org/protobuf/proto"
)

// https://ipfs.io/ipfs/QmcRD4wkPPi6dig81r5sLj9Zm1gDCL4zgpEj9CfuRrGbzF
const infoURI string = "ipfs://QmcRD4wkPPi6dig81r5sLj9Zm1gDCL4zgpEj9CfuRrGbzF"

func TestSetAccountInfoTx(t *testing.T) {
	app := TestBaseApplication(t)
	signer := ethereum.SignKeys{}
	if err := signer.Generate(); err != nil {
		t.Fatal(err)
	}

	// should create an account
	if err := testSetAccountInfoTx(t, &signer, app, infoURI); err != nil {
		t.Fatal(err)
	}

	// should fail if infoURI is empty
	if err := testSetAccountInfoTx(t, &signer, app, ""); err == nil {
		t.Fatal(err)
	}

	// get account
	acc, err := app.State.GetAccount(signer.Address(), false)
	if err != nil {
		t.Fatal(err)
	}
	if acc.InfoURI != infoURI {
		t.Fatalf("infoURI missmatch, got %s expected %s", acc.InfoURI, infoURI)
	}
}

func testSetAccountInfoTx(t *testing.T,
	signer *ethereum.SignKeys,
	app *BaseApplication,
	infoURI string) error {
	var err error

	tx := &models.SetAccountInfoTx{
		Txtype:  models.TxType_SET_ACCOUNT_INFO,
		InfoURI: infoURI,
	}

	stx := &models.SignedTx{}
	if stx.Tx, err = proto.Marshal(&models.Tx{Payload: &models.Tx_SetAccountInfo{SetAccountInfo: tx}}); err != nil {
		t.Fatal(err)
	}

	if err := sendTx(app, signer, stx); err != nil {
		return err
	}
	app.Commit()
	return nil
}

func TestSetTransactionsCosts(t *testing.T) {
	app := TestBaseApplication(t)
	signer := ethereum.SignKeys{}
	if err := signer.Generate(); err != nil {
		t.Fatal(err)
	}

	// set tx cost for Tx
	if err := app.State.SetTxCost(models.TxType_COLLECT_FAUCET, 10); err != nil {
		t.Fatal(err)
	}
	// set treasurer account (same as signer for testing purposes)
	if err := app.State.SetTreasurer(signer.Address(), 0); err != nil {
		t.Fatal(err)
	}

	// should change tx cost if treasurer
	if err := testSetTransactionCostsTx(t, app, &signer, 0, 20); err != nil {
		t.Fatal(err)
	}
	// should not change tx costs if not treasurer
	if err := testSetTransactionCostsTx(t, app, &signer, 0, 20); err == nil {
		t.Fatal(err)
	}

	if cost, err := app.State.TxCost(models.TxType_COLLECT_FAUCET, false); err != nil {
		t.Fatal(err)
	} else {
		qt.Assert(t, cost, qt.Equals, uint64(20))
	}
}

func testSetTransactionCostsTx(t *testing.T,
	app *BaseApplication,
	signer *ethereum.SignKeys,
	nonce uint32,
	cost uint64) error {
	var err error

	// tx
	tx := &models.SetTransactionCostsTx{
		Txtype: models.TxType_COLLECT_FAUCET,
		Nonce:  nonce,
		Value:  cost,
	}

	stx := &models.SignedTx{}
	if stx.Tx, err = proto.Marshal(&models.Tx{Payload: &models.Tx_SetTransactionCosts{SetTransactionCosts: tx}}); err != nil {
		t.Fatal(err)
	}
	if err := sendTx(app, signer, stx); err != nil {
		return err
	}
	app.Commit()
	return nil
}

func TestMintTokensTx(t *testing.T) {
	app := TestBaseApplication(t)
	signer := ethereum.SignKeys{}
	if err := signer.Generate(); err != nil {
		t.Fatal(err)
	}

	app.State.SetTreasurer(signer.Address(), 0)
	err := app.State.CreateAccount(signer.Address(), "ipfs://", make([]common.Address, 0), 0)
	qt.Assert(t, err, qt.IsNil)
	toAccAddr := common.HexToAddress(randomEthAccount)
	err = app.State.CreateAccount(toAccAddr, "ipfs://", make([]common.Address, 0), 0)
	qt.Assert(t, err, qt.IsNil)
	app.Commit()

	// should mint
	if err := testMintTokensTx(t, &signer, app, randomEthAccount, 100, 0); err != nil {
		t.Fatal(err)
	}

	// should fail minting
	if err := testMintTokensTx(t, &signer, app, randomEthAccount, 0, 0); err == nil {
		t.Fatal(err)
	}

	// get account
	toAcc, err := app.State.GetAccount(toAccAddr, false)
	if err != nil {
		t.Fatal(err)
	}
	if toAcc == nil {
		t.Fatal(ErrAccountNotExist)
	}
	if toAcc.Balance != 100 {
		t.Fatal(fmt.Sprintf("infoURI missmatch, got %d expected %d", toAcc.Balance, 100))
	}
	// get treasurer
	treasurer, err := app.State.Treasurer(false)
	if err != nil {
		t.Fatal(err)
	}
	// check nonce incremented
	qt.Assert(t, treasurer.Nonce, qt.Equals, uint32(1))
}

func testMintTokensTx(t *testing.T,
	signer *ethereum.SignKeys,
	app *BaseApplication,
	to string,
	value uint64,
	nonce uint32) error {
	var err error

	toAddr := common.HexToAddress(to)
	// tx
	tx := &models.MintTokensTx{
		Txtype: models.TxType_MINT_TOKENS,
		To:     toAddr.Bytes(),
		Value:  value,
		Nonce:  nonce,
	}

	stx := &models.SignedTx{}
	if stx.Tx, err = proto.Marshal(&models.Tx{Payload: &models.Tx_MintTokens{MintTokens: tx}}); err != nil {
		t.Fatal(err)
	}

	if err := sendTx(app, signer, stx); err != nil {
		return err
	}
	app.Commit()
	return nil
}

// sendTx signs and sends a vochain transaction
func sendTx(app *BaseApplication, signer *ethereum.SignKeys, stx *models.SignedTx) error {
	var cktx abcitypes.RequestCheckTx
	var detx abcitypes.RequestDeliverTx
	var cktxresp abcitypes.ResponseCheckTx
	var detxresp abcitypes.ResponseDeliverTx
	var err error

	if stx.Signature, err = signer.SignVocdoniTx(stx.Tx); err != nil {
		return err
	}
	stxBytes, err := proto.Marshal(stx)
	if err != nil {
		return err
	}

	cktx.Tx = stxBytes
	cktxresp = app.CheckTx(cktx)
	if cktxresp.Code != 0 {
		return fmt.Errorf("checkTx failed: %s", cktxresp.Data)
	}
	detx.Tx = stxBytes
	detxresp = app.DeliverTx(detx)
	if detxresp.Code != 0 {
		return fmt.Errorf("deliverTx failed: %s", detxresp.Data)
	}
	return nil
}

package vochain

import (
	"fmt"
	"strings"
	"testing"

	"github.com/ethereum/go-ethereum/common"
	qt "github.com/frankban/quicktest"
	abcitypes "github.com/tendermint/tendermint/abci/types"
	"go.vocdoni.io/dvote/crypto/ethereum"
	"go.vocdoni.io/proto/build/go/models"
	"google.golang.org/protobuf/proto"
)

// https://ipfs.io/ipfs/QmcRD4wkPPi6dig81r5sLj9Zm1gDCL4zgpEj9CfuRrGbzF
const infoURI string = "ipfs://QmcRD4wkPPi6dig81r5sLj9Zm1gDCL4zgpEj9CfuRrGbzF"
const randomEthAccount = "0x52bc44d5378309ee2abf1539bf71de1b7d7be3b5"

func TestSetAccountInfoTx(t *testing.T) {
	app := TestBaseApplication(t)
	// set tx cost for Tx: SetAccountInfo
	if err := app.State.SetTxCost(models.TxType_SET_ACCOUNT_INFO, 10); err != nil {
		t.Fatal(err)
	}
	signer := ethereum.SignKeys{}
	if err := signer.Generate(); err != nil {
		t.Fatal(err)
	}
	// set treasurer account (same as signer for testing purposes)
	if err := app.State.setTreasurer(signer.Address()); err != nil {
		t.Fatal(err)
	}
	// should create an account if address does not exist
	if err := testSetAccountInfoTx(t, &signer, app, infoURI); err != nil {
		t.Fatal(err)
	}
	// should fail if account does not have enough balance
	if err := testSetAccountInfoTx(t, &signer, app, ipfsUrl); err == nil {
		t.Fatal(err)
	}
	// should pass if infoURI is not empty and acccount have enough balance
	// mint tokens for the newly created account
	if err := app.State.MintBalance(signer.Address(), 100); err != nil {
		t.Fatal(err)
	}
	if err := testSetAccountInfoTx(t, &signer, app, ipfsUrl); err != nil {
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
	qt.Assert(t, acc.InfoURI, qt.Equals, ipfsUrl)
	qt.Assert(t, acc.Balance, qt.Equals, uint64(90))
}

func testSetAccountInfoTx(t *testing.T,
	signer *ethereum.SignKeys,
	app *BaseApplication,
	infoURI string) error {
	var cktx abcitypes.RequestCheckTx
	var detx abcitypes.RequestDeliverTx
	var cktxresp abcitypes.ResponseCheckTx
	var detxresp abcitypes.ResponseDeliverTx
	var stx models.SignedTx
	var err error

	tx := &models.SetAccountInfoTx{
		Txtype:  models.TxType_SET_ACCOUNT_INFO,
		InfoURI: infoURI,
		Account: signer.Address().Bytes(),
	}

	if stx.Tx, err = proto.Marshal(&models.Tx{Payload: &models.Tx_SetAccountInfo{SetAccountInfo: tx}}); err != nil {
		t.Fatal(err)
	}

	if stx.Signature, err = signer.Sign(stx.Tx); err != nil {
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

func TestMintTokensTx(t *testing.T) {
	app := TestBaseApplication(t)
	signer := ethereum.SignKeys{}
	if err := signer.Generate(); err != nil {
		t.Fatal(err)
	}

	app.State.setTreasurer(signer.Address())
	app.Commit()

	// should mint
	if err := testMintTokensTx(t, &signer, app, randomEthAccount, 100); err != nil {
		t.Fatal(err)
	}

	// should fail minting
	if err := testMintTokensTx(t, &signer, app, randomEthAccount, 0); err == nil {
		t.Fatal(err)
	}

	// get account
	toAcc := common.HexToAddress(randomEthAccount)
	acc, err := app.State.GetAccount(toAcc, false)
	if err != nil {
		t.Fatal(err)
	}
	if acc.Balance != 100 {
		t.Fatal(fmt.Sprintf("infoURI missmatch, got %d expected %d", acc.Balance, 100))
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
	value uint64) error {
	var cktx abcitypes.RequestCheckTx
	var detx abcitypes.RequestDeliverTx
	var cktxresp abcitypes.ResponseCheckTx
	var detxresp abcitypes.ResponseDeliverTx
	var stx models.SignedTx
	var err error

	toAddr := common.HexToAddress(to)
	tx := &models.MintTokensTx{
		Txtype: models.TxType_MINT_TOKENS,
		To:     toAddr.Bytes(),
		Value:  value,
		Nonce:  0,
	}

	if stx.Tx, err = proto.Marshal(&models.Tx{Payload: &models.Tx_MintTokens{MintTokens: tx}}); err != nil {
		t.Fatal(err)
	}

	if stx.Signature, err = signer.Sign(stx.Tx); err != nil {
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

func TestSetDelegateTx(t *testing.T) {
	app := TestBaseApplication(t)
	signer := ethereum.SignKeys{}
	if err := signer.Generate(); err != nil {
		t.Fatal(err)
	}
	// set tx cost for Tx: AddDelegateForAccount
	if err := app.State.SetTxCost(models.TxType_ADD_DELEGATE_FOR_ACCOUNT, 10); err != nil {
		t.Fatal(err)
	}
	// set tx cost for Tx: DelDelegateForAccount
	if err := app.State.SetTxCost(models.TxType_DEL_DELEGATE_FOR_ACCOUNT, 10); err != nil {
		t.Fatal(err)
	}
	// set treasurer account (same as signer for testing purposes)
	if err := app.State.setTreasurer(signer.Address()); err != nil {
		t.Fatal(err)
	}
	// create account
	if err := app.State.SetAccount(
		signer.Address(),
		&Account{
			models.Account{
				Balance: 30,
				InfoURI: infoURI,
			},
		},
	); err != nil {
		t.Fatal(err)
	}
	// should add new delegate
	if err := testSetDelegateTx(t, &signer, app, randomEthAccount, 0, true); err != nil {
		t.Fatal(err)
	}
	// get account
	signerAcc, err := app.State.GetAccount(signer.Address(), false)
	if err != nil {
		t.Fatal(err)
	}
	qt.Assert(t, strings.ToLower(common.BytesToAddress(signerAcc.DelegateAddrs[0]).String()), qt.Equals, randomEthAccount)
	qt.Assert(t, signerAcc.Balance, qt.Equals, uint64(20))

	// should fail adding same delegate
	if err := testSetDelegateTx(t, &signer, app, randomEthAccount, 1, true); err == nil {
		t.Fatal(err)
	}
	// get account
	signerAcc, err = app.State.GetAccount(signer.Address(), false)
	if err != nil {
		t.Fatal(err)
	}
	qt.Assert(t, strings.ToLower(common.BytesToAddress(signerAcc.DelegateAddrs[0]).String()), qt.Equals, randomEthAccount)
	qt.Assert(t, signerAcc.Balance, qt.Equals, uint64(20))

	// should remove an existing delegate
	if err := testSetDelegateTx(t, &signer, app, randomEthAccount, 1, false); err != nil {
		t.Fatal(err)
	}
	// get account
	signerAcc, err = app.State.GetAccount(signer.Address(), false)
	if err != nil {
		t.Fatal(err)
	}
	qt.Assert(t, signerAcc.Balance, qt.Equals, uint64(10))
	// should fail removing a non existent delegate
	if err := testSetDelegateTx(t, &signer, app, randomEthAccount, 2, false); err == nil {
		t.Fatal(err)
	}
	// should fail using a wrong nonce
	if err := testSetDelegateTx(t, &signer, app, randomEthAccount, 4, true); err == nil {
		t.Fatal(err)
	}
	// add same delegate
	if err := testSetDelegateTx(t, &signer, app, randomEthAccount, 2, true); err != nil {
		t.Fatal(err)
	}
	// should fail if no balance
	if err := testSetDelegateTx(t, &signer, app, randomEthAccount, 3, false); err == nil {
		t.Fatal(err)
	}
	// get account
	signerAcc, err = app.State.GetAccount(signer.Address(), false)
	if err != nil {
		t.Fatal(err)
	}
	qt.Assert(t, strings.ToLower(common.BytesToAddress(signerAcc.DelegateAddrs[0]).String()), qt.Equals, randomEthAccount)
	qt.Assert(t, signerAcc.Balance, qt.Equals, uint64(0))
	qt.Assert(t, signerAcc.Nonce, qt.Equals, uint32(3))
}

func testSetDelegateTx(t *testing.T,
	signer *ethereum.SignKeys,
	app *BaseApplication,
	delegate string,
	nonce uint32,
	op bool) error { // op == true -> add, op == false -> del
	var cktx abcitypes.RequestCheckTx
	var detx abcitypes.RequestDeliverTx
	var cktxresp abcitypes.ResponseCheckTx
	var detxresp abcitypes.ResponseDeliverTx
	var stx models.SignedTx
	var err error

	delegateAddr := common.HexToAddress(delegate)
	tx := &models.SetAccountDelegateTx{}
	if op {
		tx.Txtype = models.TxType_ADD_DELEGATE_FOR_ACCOUNT
	} else {
		tx.Txtype = models.TxType_DEL_DELEGATE_FOR_ACCOUNT
	}
	tx.Delegate = delegateAddr.Bytes()
	tx.Nonce = nonce

	if stx.Tx, err = proto.Marshal(&models.Tx{Payload: &models.Tx_SetAccountDelegateTx{SetAccountDelegateTx: tx}}); err != nil {
		t.Fatal(err)
	}

	if stx.Signature, err = signer.Sign(stx.Tx); err != nil {
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

func TestSendTokensTx(t *testing.T) {
	app := TestBaseApplication(t)
	signer := ethereum.SignKeys{}
	if err := signer.Generate(); err != nil {
		t.Fatal(err)
	}
	signer2 := ethereum.SignKeys{}
	if err := signer2.Generate(); err != nil {
		t.Fatal(err)
	}
	// set tx cost for Tx: SendTokens
	if err := app.State.SetTxCost(models.TxType_SEND_TOKENS, 10); err != nil {
		t.Fatal(err)
	}
	// set treasurer account (same as signer for testing purposes)
	if err := app.State.setTreasurer(signer.Address()); err != nil {
		t.Fatal(err)
	}
	// create account
	if err := app.State.SetAccount(
		signer.Address(),
		&Account{
			models.Account{
				Balance: 30,
				InfoURI: infoURI,
			},
		},
	); err != nil {
		t.Fatal(err)
	}
	if err := app.State.SetAccount(
		signer2.Address(),
		&Account{
			models.Account{
				Balance: 10,
				InfoURI: infoURI,
			},
		},
	); err != nil {
		t.Fatal(err)
	}
	signerAcc, err := app.State.GetAccount(signer.Address(), false)
	if err != nil {
		t.Fatal(err)
	}
	// should transfer tokens
	if err := testSendTokensTx(t, &signer, app, signer2.Address().String(), 10, signerAcc.Nonce); err != nil {
		t.Fatal(err)
	}

	// should fail if sender does not have enough balance
	if err := testSendTokensTx(t, &signer, app, signer2.Address().String(), 100, signerAcc.Nonce+1); err == nil {
		t.Fatal(err)
	}

	// should fail if nonce is not correct
	if err := testSendTokensTx(t, &signer, app, signer2.Address().String(), 2, signerAcc.Nonce+5); err == nil {
		t.Fatal(err)
	}

	// should fail if account does not exist
	if err := testSendTokensTx(t, &signer, app, randomEthAccount, 50, signerAcc.Nonce+1); err == nil {
		t.Fatal(err)
	}

	// get accounts
	signerAcc, err = app.State.GetAccount(signer.Address(), false)
	if err != nil {
		t.Fatal(err)
	}
	toAcc, err := app.State.GetAccount(signer2.Address(), false)
	if err != nil {
		t.Fatal(err)
	}
	qt.Assert(t, signerAcc.Balance, qt.Equals, uint64(10))
	qt.Assert(t, signerAcc.Nonce, qt.Equals, uint32(1))
	qt.Assert(t, toAcc.Balance, qt.Equals, uint64(20))
}

func testSendTokensTx(t *testing.T,
	signer *ethereum.SignKeys,
	app *BaseApplication,
	to string,
	amount uint64,
	nonce uint32) error { // op == true -> add, op == false -> del
	var cktx abcitypes.RequestCheckTx
	var detx abcitypes.RequestDeliverTx
	var cktxresp abcitypes.ResponseCheckTx
	var detxresp abcitypes.ResponseDeliverTx
	var stx models.SignedTx
	var err error

	toAddr := common.HexToAddress(to)
	tx := &models.SendTokensTx{
		From:  signer.Address().Bytes(),
		To:    toAddr.Bytes(),
		Value: amount,
		Nonce: nonce,
	}
	if stx.Tx, err = proto.Marshal(&models.Tx{Payload: &models.Tx_SendTokens{SendTokens: tx}}); err != nil {
		t.Fatal(err)
	}

	if stx.Signature, err = signer.Sign(stx.Tx); err != nil {
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

func TestCollectFaucetTx(t *testing.T) {
	app := TestBaseApplication(t)
	signer := ethereum.SignKeys{}
	if err := signer.Generate(); err != nil {
		t.Fatal(err)
	}
	signer2 := ethereum.SignKeys{}
	if err := signer2.Generate(); err != nil {
		t.Fatal(err)
	}
	// set tx cost for Tx: SendTokens
	if err := app.State.SetTxCost(models.TxType_COLLECT_FAUCET, 10); err != nil {
		t.Fatal(err)
	}
	// set treasurer account (same as signer for testing purposes)
	if err := app.State.setTreasurer(signer.Address()); err != nil {
		t.Fatal(err)
	}
	// create account
	if err := app.State.SetAccount(
		signer.Address(),
		&Account{
			models.Account{
				Balance: 30,
				InfoURI: infoURI,
			},
		},
	); err != nil {
		t.Fatal(err)
	}
	if err := app.State.SetAccount(
		signer2.Address(),
		&Account{
			models.Account{
				Balance: 10,
				InfoURI: infoURI,
			},
		},
	); err != nil {
		t.Fatal(err)
	}
	// should transfer tokens
	if err := testCollectFaucetTx(t, app, &signer, &signer2, 10, 246728262361); err != nil {
		t.Fatal(err)
	}
	// should not transfer tokens if same payload identifier
	if err := testCollectFaucetTx(t, app, &signer, &signer2, 10, 246728262361); err == nil {
		t.Fatal(err)
	}
	signerAcc, err := app.State.GetAccount(signer.Address(), false)
	if err != nil {
		t.Fatal(err)
	}
	signerAcc2, err := app.State.GetAccount(signer2.Address(), false)
	if err != nil {
		t.Fatal(err)
	}
	qt.Assert(t, signerAcc.Balance, qt.Equals, uint64(10))
	qt.Assert(t, signerAcc2.Balance, qt.Equals, uint64(20))
}

func testCollectFaucetTx(t *testing.T,
	app *BaseApplication,
	from,
	to *ethereum.SignKeys,
	amount,
	nonce uint64) error { // op == true -> add, op == false -> del
	var cktx abcitypes.RequestCheckTx
	var detx abcitypes.RequestDeliverTx
	var cktxresp abcitypes.ResponseCheckTx
	var detxresp abcitypes.ResponseDeliverTx
	var stx models.SignedTx
	var err error

	// create faucet pkg
	faucetPayload := &models.FaucetPayload{
		Identifier: nonce,
		To:         to.Address().Bytes(),
		Amount:     amount,
	}
	faucetPayloadBytes, err := proto.Marshal(faucetPayload)
	if err != nil {
		t.Fatal(err)
	}
	faucetPayloadSignature, err := from.Sign(faucetPayloadBytes)
	if err != nil {
		t.Fatal(err)
	}
	faucetPackage := &models.FaucetPackage{
		Payload:   faucetPayload,
		Signature: faucetPayloadSignature,
	}

	tx := &models.CollectFaucetTx{
		FaucetPackage: faucetPackage,
	}
	if stx.Tx, err = proto.Marshal(&models.Tx{Payload: &models.Tx_CollectFaucet{CollectFaucet: tx}}); err != nil {
		t.Fatal(err)
	}

	if stx.Signature, err = to.Sign(stx.Tx); err != nil {
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

package vochain

import (
	"fmt"
	"math/rand"
	"testing"

	"github.com/ethereum/go-ethereum/common"
	qt "github.com/frankban/quicktest"
	abcitypes "github.com/tendermint/tendermint/abci/types"

	"go.vocdoni.io/dvote/crypto/ethereum"
	"go.vocdoni.io/dvote/util"
	"go.vocdoni.io/proto/build/go/models"
	"google.golang.org/protobuf/proto"
)

// https://ipfs.io/ipfs/QmcRD4wkPPi6dig81r5sLj9Zm1gDCL4zgpEj9CfuRrGbzF
const infoURI string = "ipfs://QmcRD4wkPPi6dig81r5sLj9Zm1gDCL4zgpEj9CfuRrGbzF"
const randomEthAccount = "0x52bc44d5378309ee2abf1539bf71de1b7d7be3b5"

func TestSetAccountInfoTx(t *testing.T) {
	app := TestBaseApplication(t)
	signer := ethereum.SignKeys{}
	if err := signer.Generate(); err != nil {
		t.Fatal(err)
	}
	signer2 := ethereum.SignKeys{}
	if err := signer2.Generate(); err != nil {
		t.Fatal(err)
	}
	signer3 := ethereum.SignKeys{}
	if err := signer3.Generate(); err != nil {
		t.Fatal(err)
	}

	// create burn account
	app.State.SetAccount(BurnAddress, &Account{})
	// set tx costs
	app.State.SetTxCost(models.TxType_SET_ACCOUNT_INFO, 100)
	app.State.SetTxCost(models.TxType_COLLECT_FAUCET, 100)

	// should create an account
	if err := testSetAccountInfoTx(t, &signer, common.Address{}, nil, app, infoURI); err != nil {
		t.Fatal(err)
	}

	// should fail if infoURI is empty
	if err := testSetAccountInfoTx(t, &signer, common.Address{}, nil, app, ""); err == nil {
		t.Fatal(err)
	}

	// should always create an account for the tx sender without taking into account the Account field
	if err := testSetAccountInfoTx(t, &signer2, signer.Address(), nil, app, infoURI); err != nil {
		t.Fatal(err)
	}

	// should create an account with tokens if faucet pkg is provided
	err := app.State.MintBalance(signer.Address(), 1000)
	if err != nil {
		t.Fatal(err)
	}
	faucetPkg, err := GenerateFaucetPackage(&signer, signer3.Address(), 100, rand.Uint64())
	if err != nil {
		t.Fatal(err)
	}
	if err := testSetAccountInfoTx(t, &signer3, common.Address{}, faucetPkg, app, infoURI); err != nil {
		t.Fatal(err)
	}

	// get account
	acc, err := app.State.GetAccount(signer.Address(), false)
	if err != nil {
		t.Fatal(err)
	}
	if acc == nil {
		t.Fatal(ErrAccountNotExist)
	}
	if acc.InfoURI != infoURI {
		t.Fatalf("infoURI missmatch, got %s expected %s", acc.InfoURI, infoURI)
	}
	if acc.Balance != 800 {
		t.Fatalf("expected balance to be %d got %d", 800, acc.Balance)
	}

	// get account2
	acc2, err := app.State.GetAccount(signer2.Address(), false)
	if err != nil {
		t.Fatal(err)
	}
	if acc2 == nil {
		t.Fatal(ErrAccountNotExist)
	}

	// get account3
	acc3, err := app.State.GetAccount(signer3.Address(), false)
	if err != nil {
		t.Fatal(err)
	}
	if acc3 == nil {
		t.Fatal(ErrAccountNotExist)
	}
	if acc3.Balance != 100 {
		t.Fatalf("expected balance to be %d got %d", 100, acc3.Balance)
	}

	// check burn address
	burnAcc, err := app.State.GetAccount(BurnAddress, false)
	if err != nil {
		t.Fatal(err)
	}
	if burnAcc == nil {
		t.Fatal(ErrAccountNotExist)
	}
	if burnAcc.Balance != 100 {
		t.Fatalf("expected balance to be %d got %d", 100, burnAcc.Balance)
	}
}

func testSetAccountInfoTx(t *testing.T,
	signer *ethereum.SignKeys,
	account common.Address,
	faucetPkg *models.FaucetPackage,
	app *BaseApplication,
	infoURI string) error {

	faucetPkgBytes, err := proto.Marshal(faucetPkg)
	if err != nil {
		return nil
	}

	tx := &models.SetAccountInfoTx{
		Txtype:        models.TxType_SET_ACCOUNT_INFO,
		InfoURI:       infoURI,
		Account:       account.Bytes(),
		FaucetPackage: faucetPkgBytes,
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
		t.Fatalf("infoURI missmatch, got %d expected %d", toAcc.Balance, 100)
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

func TestSendTokensTx(t *testing.T) {
	app := TestBaseApplication(t)

	signer := ethereum.SignKeys{}
	err := signer.Generate()
	qt.Assert(t, err, qt.IsNil)

	app.State.SetAccount(BurnAddress, &Account{})

	err = app.State.SetTreasurer(signer.Address(), 0)
	qt.Assert(t, err, qt.IsNil)

	err = app.State.SetTxCost(models.TxType_SEND_TOKENS, 10)
	qt.Assert(t, err, qt.IsNil)

	err = app.State.CreateAccount(signer.Address(), "ipfs://", make([]common.Address, 0), 0)
	qt.Assert(t, err, qt.IsNil)

	toAccAddr := common.HexToAddress(randomEthAccount)
	err = app.State.CreateAccount(toAccAddr, "ipfs://", make([]common.Address, 0), 0)
	qt.Assert(t, err, qt.IsNil)

	err = app.State.MintBalance(signer.Address(), 1000)
	qt.Assert(t, err, qt.IsNil)
	app.Commit()

	// should send
	err = testSendTokensTx(t, &signer, app, toAccAddr.String(), 100, 0)
	qt.Assert(t, err, qt.IsNil)

	// should fail sending if incorrect nonce
	err = testSendTokensTx(t, &signer, app, randomEthAccount, 100, 0)
	qt.Assert(t, err, qt.IsNotNil)
	// should fail sending if to acc does not exist
	noAccount := ethereum.SignKeys{}
	err = noAccount.Generate()
	qt.Assert(t, err, qt.IsNil)
	err = testSendTokensTx(t, &signer, app, noAccount.Address().String(), 100, 1)
	qt.Assert(t, err, qt.IsNotNil)
	// should fail sending if not enought balance
	err = testSendTokensTx(t, &signer, app, randomEthAccount, 1000, 1)
	qt.Assert(t, err, qt.IsNotNil)
	// get to account
	toAcc, err := app.State.GetAccount(toAccAddr, false)
	qt.Assert(t, err, qt.IsNil)
	qt.Assert(t, toAcc, qt.IsNotNil)
	qt.Assert(t, toAcc.Balance, qt.Equals, uint64(100))
	// get from acc
	fromAcc, err := app.State.GetAccount(signer.Address(), false)
	qt.Assert(t, err, qt.IsNil)
	qt.Assert(t, fromAcc, qt.IsNotNil)
	qt.Assert(t, fromAcc.Balance, qt.Equals, uint64(890))
}

func testSendTokensTx(t *testing.T,
	signer *ethereum.SignKeys,
	app *BaseApplication,
	to string,
	value uint64,
	nonce uint32) error {
	var err error

	toAddr := common.HexToAddress(to)
	// tx
	tx := &models.SendTokensTx{
		Txtype: models.TxType_SEND_TOKENS,
		From:   signer.Address().Bytes(),
		To:     toAddr.Bytes(),
		Value:  value,
		Nonce:  nonce,
	}

	stx := &models.SignedTx{}
	if stx.Tx, err = proto.Marshal(&models.Tx{Payload: &models.Tx_SendTokens{SendTokens: tx}}); err != nil {
		t.Fatal(err)
	}

	if err := sendTx(app, signer, stx); err != nil {
		return err
	}
	app.Commit()
	return nil
}

func TestSetAccountDelegateTx(t *testing.T) {
	app := TestBaseApplication(t)

	signer := ethereum.SignKeys{}
	err := signer.Generate()
	qt.Assert(t, err, qt.IsNil)

	app.State.SetAccount(BurnAddress, &Account{})

	err = app.State.SetTreasurer(signer.Address(), 0)
	qt.Assert(t, err, qt.IsNil)

	err = app.State.SetTxCost(models.TxType_ADD_DELEGATE_FOR_ACCOUNT, 10)
	qt.Assert(t, err, qt.IsNil)
	err = app.State.SetTxCost(models.TxType_DEL_DELEGATE_FOR_ACCOUNT, 10)
	qt.Assert(t, err, qt.IsNil)

	err = app.State.CreateAccount(signer.Address(), "ipfs://", make([]common.Address, 0), 0)
	qt.Assert(t, err, qt.IsNil)
	toAccAddr := common.HexToAddress(randomEthAccount)
	err = app.State.CreateAccount(toAccAddr, "ipfs://", make([]common.Address, 0), 0)
	qt.Assert(t, err, qt.IsNil)

	err = app.State.MintBalance(signer.Address(), 1000)
	qt.Assert(t, err, qt.IsNil)
	app.Commit()

	// should add delegate if owner
	err = testSetAccountDelegateTx(t, &signer, app, toAccAddr, true, 0)
	qt.Assert(t, err, qt.IsNil)

	// should fail if delegate already added
	err = testSetAccountDelegateTx(t, &signer, app, toAccAddr, true, 1)
	qt.Assert(t, err, qt.IsNotNil)

	// get from acc
	acc, err := app.State.GetAccount(signer.Address(), false)
	qt.Assert(t, err, qt.IsNil)
	qt.Assert(t, acc, qt.IsNotNil)
	qt.Assert(t, acc.DelegateAddrs[0], qt.DeepEquals, toAccAddr.Bytes())

	// should del an existing delegate
	err = testSetAccountDelegateTx(t, &signer, app, toAccAddr, false, 1)
	qt.Assert(t, err, qt.IsNil)

	// should fail deleting a non existent delegate
	err = testSetAccountDelegateTx(t, &signer, app, toAccAddr, false, 1)
	qt.Assert(t, err, qt.IsNotNil)

	// get from acc
	acc, err = app.State.GetAccount(signer.Address(), false)
	qt.Assert(t, err, qt.IsNil)
	qt.Assert(t, acc, qt.IsNotNil)
	qt.Assert(t, len(acc.DelegateAddrs), qt.Equals, 0)
}

func testSetAccountDelegateTx(t *testing.T,
	signer *ethereum.SignKeys,
	app *BaseApplication,
	delegate common.Address,
	op bool, // true == add, false == del
	nonce uint32) error {
	var err error

	// tx
	tx := &models.SetAccountDelegateTx{
		Delegate: delegate.Bytes(),
		Nonce:    nonce,
		Txtype:   models.TxType_ADD_DELEGATE_FOR_ACCOUNT,
	}
	if !op {
		tx.Txtype = models.TxType_DEL_DELEGATE_FOR_ACCOUNT
	}

	stx := &models.SignedTx{}
	if stx.Tx, err = proto.Marshal(&models.Tx{Payload: &models.Tx_SetAccountDelegateTx{SetAccountDelegateTx: tx}}); err != nil {
		t.Fatal(err)
	}

	if err := sendTx(app, signer, stx); err != nil {
		return err
	}
	app.Commit()
	return nil
}

func TestCollectFaucetTx(t *testing.T) {
	app := TestBaseApplication(t)

	signer := ethereum.SignKeys{}
	err := signer.Generate()
	qt.Assert(t, err, qt.IsNil)

	toSigner := ethereum.SignKeys{}
	err = toSigner.Generate()
	qt.Assert(t, err, qt.IsNil)

	app.State.SetAccount(BurnAddress, &Account{})

	err = app.State.SetTreasurer(signer.Address(), 0)
	qt.Assert(t, err, qt.IsNil)

	err = app.State.SetTxCost(models.TxType_COLLECT_FAUCET, 10)
	qt.Assert(t, err, qt.IsNil)

	err = app.State.CreateAccount(signer.Address(), "ipfs://", make([]common.Address, 0), 0)
	qt.Assert(t, err, qt.IsNil)
	err = app.State.CreateAccount(toSigner.Address(), "ipfs://", make([]common.Address, 0), 0)
	qt.Assert(t, err, qt.IsNil)

	err = app.State.MintBalance(signer.Address(), 1000)
	qt.Assert(t, err, qt.IsNil)
	app.Commit()

	randomIdentifier := uint64(util.RandomInt(0, 10000000))
	// should work if all data and tx are valid
	err = testCollectFaucetTx(t, &signer, &toSigner, app, 0, 900, randomIdentifier)
	qt.Assert(t, err, qt.IsNil)
	acc, err := app.State.GetAccount(toSigner.Address(), true)
	qt.Assert(t, err, qt.IsNil)
	qt.Assert(t, acc.Balance, qt.Equals, uint64(900))
	qt.Assert(t, acc.Nonce, qt.Equals, uint32(1))
	signerAcc, err := app.State.GetAccount(signer.Address(), true)
	qt.Assert(t, err, qt.IsNil)
	qt.Assert(t, signerAcc.Balance, qt.Equals, uint64(90))
	qt.Assert(t, signerAcc.Nonce, qt.Equals, uint32(0))
	burnAcc, err := app.State.GetAccount(BurnAddress, true)
	qt.Assert(t, err, qt.IsNil)
	qt.Assert(t, burnAcc.Balance, qt.Equals, uint64(10))

	// should fail if identifier is already used
	err = testCollectFaucetTx(t, &signer, &toSigner, app, 1, 1, randomIdentifier)
	qt.Assert(t, err, qt.IsNotNil)
	qt.Assert(t, err, qt.ErrorMatches, ".* nonce .* already used")

	// should fail if to acc does not exist
	nonExistentAccount := ethereum.SignKeys{}
	err = nonExistentAccount.Generate()
	qt.Assert(t, err, qt.IsNil)

	err = testCollectFaucetTx(t, &signer, &nonExistentAccount, app, 1, 1, randomIdentifier+1)
	qt.Assert(t, err, qt.IsNotNil)
	qt.Assert(t, err, qt.ErrorMatches, fmt.Sprintf(".* %s", ErrAccountNotExist))

	// should fail if from acc does not exist
	err = testCollectFaucetTx(t, &nonExistentAccount, &toSigner, app, 1, 1, randomIdentifier+1)
	qt.Assert(t, err, qt.IsNotNil)
	qt.Assert(t, err, qt.ErrorMatches, ".* the account signing the faucet payload does not exist")

	// should fail if amount is not valid
	err = testCollectFaucetTx(t, &signer, &toSigner, app, 2, 0, randomIdentifier+1)
	qt.Assert(t, err, qt.IsNotNil)
	qt.Assert(t, err, qt.ErrorMatches, ".* invalid faucet package payload amount")

	// should fail if tx sender nonce is not valid
	err = testCollectFaucetTx(t, &signer, &toSigner, app, 0, 1, randomIdentifier+1)
	qt.Assert(t, err, qt.IsNotNil)
	qt.Assert(t, err, qt.ErrorMatches, ".* invalid nonce")

	// should fail if from acc does not have enough balance
	err = testCollectFaucetTx(t, &signer, &toSigner, app, 1, 100000, randomIdentifier+1)
	qt.Assert(t, err, qt.IsNotNil)
	qt.Assert(t, err, qt.ErrorMatches, ".* faucet does not have enough balance .*")

	// check any value changed after tx failures
	acc, err = app.State.GetAccount(toSigner.Address(), true)
	qt.Assert(t, err, qt.IsNil)
	qt.Assert(t, acc.Balance, qt.Equals, uint64(900))
	qt.Assert(t, acc.Nonce, qt.Equals, uint32(1))
	signerAcc, err = app.State.GetAccount(signer.Address(), true)
	qt.Assert(t, err, qt.IsNil)
	qt.Assert(t, signerAcc.Balance, qt.Equals, uint64(90))
	qt.Assert(t, signerAcc.Nonce, qt.Equals, uint32(0))
	burnAcc, err = app.State.GetAccount(BurnAddress, true)
	qt.Assert(t, err, qt.IsNil)
	qt.Assert(t, burnAcc.Balance, qt.Equals, uint64(10))
}

func testCollectFaucetTx(t *testing.T,
	signer,
	to *ethereum.SignKeys,
	app *BaseApplication,
	nonce uint32,
	amount,
	identifier uint64) error {
	var err error

	// tx
	faucetPayload := &models.FaucetPayload{
		Identifier: identifier,
		To:         to.Address().Bytes(),
		Amount:     amount,
	}
	faucetPayloadBytes, err := proto.Marshal(faucetPayload)
	qt.Assert(t, err, qt.IsNil)
	faucetPayloadSignature, err := signer.SignEthereum(faucetPayloadBytes)
	qt.Assert(t, err, qt.IsNil)
	faucetPkg := &models.FaucetPackage{
		Payload:   faucetPayload,
		Signature: faucetPayloadSignature,
	}
	tx := &models.CollectFaucetTx{
		TxType:        models.TxType_COLLECT_FAUCET,
		FaucetPackage: faucetPkg,
		Nonce:         nonce,
	}
	stx := &models.SignedTx{}
	if stx.Tx, err = proto.Marshal(&models.Tx{Payload: &models.Tx_CollectFaucet{CollectFaucet: tx}}); err != nil {
		t.Fatal(err)
	}

	if err := sendTx(app, to, stx); err != nil {
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

	if stx.Signature, err = signer.SignVocdoniTx(stx.Tx, app.chainID); err != nil {
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

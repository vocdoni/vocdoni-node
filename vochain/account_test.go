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

const (
	infoURI,
	infoURI2,
	randomEthAccount string = "ipfs://QmcRD4wkPPi6dig81r5sLj9Zm1gDCL4zgpEj9CfuRrGbzF",
		"https://foo.bar",
		"0x52bc44d5378309ee2abf1539bf71de1b7d7be3b5"
)

func setupTestBaseApplicationAndSigners(t *testing.T,
	numberSigners int) (*BaseApplication, []*ethereum.SignKeys, error) {
	app := TestBaseApplication(t)
	signers := make([]*ethereum.SignKeys, numberSigners)
	for i := 0; i < numberSigners; i++ {
		signer := ethereum.SignKeys{}
		if err := signer.Generate(); err != nil {
			return nil, nil, err
		}
		signers[i] = &signer
	}
	// create burn account
	app.State.SetAccount(BurnAddress, &Account{})
	// set tx costs
	app.State.SetTxCost(models.TxType_SET_ACCOUNT_INFO_URI, 100)
	app.State.SetTxCost(models.TxType_COLLECT_FAUCET, 100)
	// save state
	app.Commit()
	return app, signers, nil
}

func TestSetAccountTx(t *testing.T) {
	app, signers, err := setupTestBaseApplicationAndSigners(t, 10)
	qt.Assert(t, err, qt.IsNil)
	// set account 0
	qt.Assert(t,
		app.State.SetAccount(signers[0].Address(), &Account{models.Account{Balance: 10000, InfoURI: infoURI}}),
		qt.IsNil,
	)
	app.Commit()

	// CREATE ACCOUNT

	// should work is a valid faucet payload and tx.Account is provided and no infoURI is set
	faucetPkg, err := GenerateFaucetPackage(signers[0], signers[1].Address(), 1000, rand.Uint64())
	qt.Assert(t, err, qt.IsNil)
	qt.Assert(t, testSetAccountTx(t,
		signers[1], signers[1].Address(), faucetPkg, app, "", 0),
		qt.IsNil,
	)
	signer0Account, err := app.State.GetAccount(signers[0].Address(), false)
	qt.Assert(t, err, qt.IsNil)
	qt.Assert(t, signer0Account, qt.IsNotNil)
	qt.Assert(t, signer0Account.Balance, qt.DeepEquals, uint64(8900))
	qt.Assert(t, signer0Account.Nonce, qt.DeepEquals, uint32(0))
	qt.Assert(t, signer0Account.InfoURI, qt.CmpEquals(), infoURI)
	signer1Account, err := app.State.GetAccount(signers[1].Address(), false)
	qt.Assert(t, err, qt.IsNil)
	qt.Assert(t, signer1Account, qt.IsNotNil)
	qt.Assert(t, signer1Account.Balance, qt.DeepEquals, uint64(1000))
	qt.Assert(t, signer1Account.Nonce, qt.DeepEquals, uint32(0))
	qt.Assert(t, signer1Account.InfoURI, qt.CmpEquals(), "")

	// should work is a valid faucet payload and tx.Account is not provided and no infoURI is set
	faucetPkg, err = GenerateFaucetPackage(signers[0], signers[2].Address(), 1000, rand.Uint64())
	qt.Assert(t, err, qt.IsNil)
	qt.Assert(t, testSetAccountTx(t,
		signers[2], common.Address{}, faucetPkg, app, "", 0),
		qt.IsNil,
	)
	signer0Account, err = app.State.GetAccount(signers[0].Address(), false)
	qt.Assert(t, err, qt.IsNil)
	qt.Assert(t, signer0Account, qt.IsNotNil)
	qt.Assert(t, signer0Account.Balance, qt.DeepEquals, uint64(7800))
	qt.Assert(t, signer0Account.Nonce, qt.DeepEquals, uint32(0))
	qt.Assert(t, signer0Account.InfoURI, qt.CmpEquals(), infoURI)
	signer2Account, err := app.State.GetAccount(signers[2].Address(), false)
	qt.Assert(t, err, qt.IsNil)
	qt.Assert(t, signer2Account, qt.IsNotNil)
	qt.Assert(t, signer2Account.Balance, qt.DeepEquals, uint64(1000))
	qt.Assert(t, signer2Account.Nonce, qt.DeepEquals, uint32(0))
	qt.Assert(t, signer2Account.InfoURI, qt.CmpEquals(), "")

	// should work is a valid faucet payload and tx.Account is provided and infoURI is set
	faucetPkg, err = GenerateFaucetPackage(signers[0], signers[3].Address(), 1000, rand.Uint64())
	qt.Assert(t, err, qt.IsNil)
	qt.Assert(t, testSetAccountTx(t,
		signers[3], signers[3].Address(), faucetPkg, app, infoURI, 0),
		qt.IsNil,
	)
	signer0Account, err = app.State.GetAccount(signers[0].Address(), false)
	qt.Assert(t, err, qt.IsNil)
	qt.Assert(t, signer0Account, qt.IsNotNil)
	qt.Assert(t, signer0Account.Balance, qt.DeepEquals, uint64(6700))
	qt.Assert(t, signer0Account.Nonce, qt.DeepEquals, uint32(0))
	qt.Assert(t, signer0Account.InfoURI, qt.CmpEquals(), infoURI)
	signer3Account, err := app.State.GetAccount(signers[3].Address(), false)
	qt.Assert(t, err, qt.IsNil)
	qt.Assert(t, signer3Account, qt.IsNotNil)
	qt.Assert(t, signer3Account.Balance, qt.DeepEquals, uint64(1000))
	qt.Assert(t, signer3Account.Nonce, qt.DeepEquals, uint32(0))
	qt.Assert(t, signer3Account.InfoURI, qt.CmpEquals(), infoURI)

	// should work is a valid faucet payload and tx.Account is not provided and infoURI is set
	faucetPkgIdentifierToReuse := rand.Uint64()
	faucetPkg, err = GenerateFaucetPackage(signers[0], signers[4].Address(), 1000, faucetPkgIdentifierToReuse)
	qt.Assert(t, err, qt.IsNil)
	qt.Assert(t, testSetAccountTx(t,
		signers[4], common.Address{}, faucetPkg, app, infoURI, 0),
		qt.IsNil,
	)
	signer0Account, err = app.State.GetAccount(signers[0].Address(), false)
	qt.Assert(t, err, qt.IsNil)
	qt.Assert(t, signer0Account, qt.IsNotNil)
	qt.Assert(t, signer0Account.Balance, qt.DeepEquals, uint64(5600))
	qt.Assert(t, signer0Account.Nonce, qt.DeepEquals, uint32(0))
	qt.Assert(t, signer0Account.InfoURI, qt.CmpEquals(), infoURI)
	signer4Account, err := app.State.GetAccount(signers[4].Address(), false)
	qt.Assert(t, err, qt.IsNil)
	qt.Assert(t, signer4Account, qt.IsNotNil)
	qt.Assert(t, signer4Account.Balance, qt.DeepEquals, uint64(1000))
	qt.Assert(t, signer4Account.Nonce, qt.DeepEquals, uint32(0))
	qt.Assert(t, signer4Account.InfoURI, qt.CmpEquals(), infoURI)

	// should not work if an invalid faucet package is provided
	// nil faucet package
	faucetPkg = &models.FaucetPackage{}
	qt.Assert(t, testSetAccountTx(t,
		signers[5], common.Address{}, faucetPkg, app, infoURI, 0),
		qt.IsNotNil,
	)
	signer0Account, err = app.State.GetAccount(signers[0].Address(), false)
	qt.Assert(t, err, qt.IsNil)
	qt.Assert(t, signer0Account, qt.IsNotNil)
	qt.Assert(t, signer0Account.Balance, qt.DeepEquals, uint64(5600))
	qt.Assert(t, signer0Account.Nonce, qt.DeepEquals, uint32(0))
	qt.Assert(t, signer0Account.InfoURI, qt.CmpEquals(), infoURI)
	signer5Account, err := app.State.GetAccount(signers[5].Address(), false)
	qt.Assert(t, err, qt.IsNil)
	qt.Assert(t, signer5Account, qt.IsNil)
	// nil faucet payload
	faucetPkg = &models.FaucetPackage{Payload: nil}
	qt.Assert(t, testSetAccountTx(t,
		signers[5], common.Address{}, faucetPkg, app, infoURI, 0),
		qt.IsNotNil,
	)
	signer0Account, err = app.State.GetAccount(signers[0].Address(), false)
	qt.Assert(t, err, qt.IsNil)
	qt.Assert(t, signer0Account, qt.IsNotNil)
	qt.Assert(t, signer0Account.Balance, qt.DeepEquals, uint64(5600))
	qt.Assert(t, signer0Account.Nonce, qt.DeepEquals, uint32(0))
	qt.Assert(t, signer0Account.InfoURI, qt.CmpEquals(), infoURI)
	signer5Account, err = app.State.GetAccount(signers[5].Address(), false)
	qt.Assert(t, err, qt.IsNil)
	qt.Assert(t, signer5Account, qt.IsNil)
	// tx sender does not match with faucet package to
	faucetPkg = &models.FaucetPackage{Payload: []byte{}}
	qt.Assert(t, testSetAccountTx(t,
		signers[5], common.Address{}, faucetPkg, app, infoURI, 0),
		qt.IsNotNil,
	)
	signer0Account, err = app.State.GetAccount(signers[0].Address(), false)
	qt.Assert(t, err, qt.IsNil)
	qt.Assert(t, signer0Account, qt.IsNotNil)
	qt.Assert(t, signer0Account.Balance, qt.DeepEquals, uint64(5600))
	qt.Assert(t, signer0Account.Nonce, qt.DeepEquals, uint32(0))
	qt.Assert(t, signer0Account.InfoURI, qt.CmpEquals(), infoURI)
	signer5Account, err = app.State.GetAccount(signers[5].Address(), false)
	qt.Assert(t, err, qt.IsNil)
	qt.Assert(t, signer5Account, qt.IsNil)

	// should not work if already used faucet package
	faucetPkg, err = GenerateFaucetPackage(signers[0], signers[5].Address(), 1000, faucetPkgIdentifierToReuse)
	qt.Assert(t, err, qt.IsNil)
	qt.Assert(t, testSetAccountTx(t,
		signers[5], common.Address{}, faucetPkg, app, infoURI, 0),
		qt.IsNotNil,
	)
	signer0Account, err = app.State.GetAccount(signers[0].Address(), false)
	qt.Assert(t, err, qt.IsNil)
	qt.Assert(t, signer0Account, qt.IsNotNil)
	qt.Assert(t, signer0Account.Balance, qt.DeepEquals, uint64(5600))
	qt.Assert(t, signer0Account.Nonce, qt.DeepEquals, uint32(0))
	qt.Assert(t, signer0Account.InfoURI, qt.CmpEquals(), infoURI)
	signer5Account, err = app.State.GetAccount(signers[5].Address(), false)
	qt.Assert(t, err, qt.IsNil)
	qt.Assert(t, signer5Account, qt.IsNil)

	// should not work if faucet package issuer does not have enough balance
	faucetPkg, err = GenerateFaucetPackage(signers[0], signers[5].Address(), 10000, rand.Uint64())
	qt.Assert(t, err, qt.IsNil)
	qt.Assert(t, testSetAccountTx(t,
		signers[5], common.Address{}, faucetPkg, app, infoURI, 0),
		qt.IsNotNil,
	)
	signer0Account, err = app.State.GetAccount(signers[0].Address(), false)
	qt.Assert(t, err, qt.IsNil)
	qt.Assert(t, signer0Account, qt.IsNotNil)
	qt.Assert(t, signer0Account.Balance, qt.DeepEquals, uint64(5600))
	qt.Assert(t, signer0Account.Nonce, qt.DeepEquals, uint32(0))
	qt.Assert(t, signer0Account.InfoURI, qt.CmpEquals(), infoURI)
	signer5Account, err = app.State.GetAccount(signers[5].Address(), false)
	qt.Assert(t, err, qt.IsNil)
	qt.Assert(t, signer5Account, qt.IsNil)

	// should not work if faucet package issuer account does not exist
	faucetPkg, err = GenerateFaucetPackage(signers[5], signers[6].Address(), 1000, rand.Uint64())
	qt.Assert(t, err, qt.IsNil)
	qt.Assert(t, testSetAccountTx(t,
		signers[6], common.Address{}, faucetPkg, app, infoURI, 0),
		qt.IsNotNil,
	)
	signer5Account, err = app.State.GetAccount(signers[5].Address(), false)
	qt.Assert(t, err, qt.IsNil)
	qt.Assert(t, signer5Account, qt.IsNil)
	signer6Account, err := app.State.GetAccount(signers[6].Address(), false)
	qt.Assert(t, err, qt.IsNil)
	qt.Assert(t, signer6Account, qt.IsNil)

	// should not work if faucet package issuer cannot cover the txs cost + faucet amount
	faucetPkg, err = GenerateFaucetPackage(signers[0], signers[5].Address(), 20000, rand.Uint64())
	qt.Assert(t, err, qt.IsNil)
	qt.Assert(t, testSetAccountTx(t,
		signers[5], common.Address{}, faucetPkg, app, infoURI, 0),
		qt.IsNotNil,
	)
	signer0Account, err = app.State.GetAccount(signers[0].Address(), false)
	qt.Assert(t, err, qt.IsNil)
	qt.Assert(t, signer0Account, qt.IsNotNil)
	qt.Assert(t, signer0Account.Balance, qt.DeepEquals, uint64(5600))
	qt.Assert(t, signer0Account.Nonce, qt.DeepEquals, uint32(0))
	qt.Assert(t, signer0Account.InfoURI, qt.CmpEquals(), infoURI)
	signer5Account, err = app.State.GetAccount(signers[5].Address(), false)
	qt.Assert(t, err, qt.IsNil)
	qt.Assert(t, signer5Account, qt.IsNil)
	// should not work if faucet package with an invalid amount
	faucetPkg, err = GenerateFaucetPackage(signers[0], signers[5].Address(), 0, rand.Uint64())
	qt.Assert(t, err, qt.IsNil)
	qt.Assert(t, testSetAccountTx(t,
		signers[5], common.Address{}, faucetPkg, app, infoURI, 0),
		qt.IsNotNil,
	)
	signer0Account, err = app.State.GetAccount(signers[0].Address(), false)
	qt.Assert(t, err, qt.IsNil)
	qt.Assert(t, signer0Account, qt.IsNotNil)
	qt.Assert(t, signer0Account.Balance, qt.DeepEquals, uint64(5600))
	qt.Assert(t, signer0Account.Nonce, qt.DeepEquals, uint32(0))
	qt.Assert(t, signer0Account.InfoURI, qt.CmpEquals(), infoURI)
	signer5Account, err = app.State.GetAccount(signers[5].Address(), false)
	qt.Assert(t, err, qt.IsNil)
	qt.Assert(t, signer5Account, qt.IsNil)

	// SET ACCOUNT INFO URI

	// should work for itself with faucet package (the faucet package is ignored)
	faucetPkg, err = GenerateFaucetPackage(signers[0], signers[0].Address(), 1000, rand.Uint64())
	qt.Assert(t, err, qt.IsNil)
	qt.Assert(t, testSetAccountTx(t,
		signers[0], common.Address{}, faucetPkg, app, infoURI2, 0),
		qt.IsNil,
	)
	signer0Account, err = app.State.GetAccount(signers[0].Address(), false)
	qt.Assert(t, err, qt.IsNil)
	qt.Assert(t, signer0Account, qt.IsNotNil)
	qt.Assert(t, signer0Account.Balance, qt.DeepEquals, uint64(5500))
	qt.Assert(t, signer0Account.Nonce, qt.DeepEquals, uint32(1))
	qt.Assert(t, signer0Account.InfoURI, qt.CmpEquals(), infoURI2)
	// should not work for itself with faucet package (the faucet package is ignored) and invalid InfoURI
	faucetPkg, err = GenerateFaucetPackage(signers[0], signers[0].Address(), 1000, rand.Uint64())
	qt.Assert(t, err, qt.IsNil)
	qt.Assert(t, testSetAccountTx(t,
		signers[0], common.Address{}, faucetPkg, app, "", 0),
		qt.IsNotNil,
	)
	signer0Account, err = app.State.GetAccount(signers[0].Address(), false)
	qt.Assert(t, err, qt.IsNil)
	qt.Assert(t, signer0Account, qt.IsNotNil)
	qt.Assert(t, signer0Account.Balance, qt.DeepEquals, uint64(5500))
	qt.Assert(t, signer0Account.Nonce, qt.DeepEquals, uint32(1))
	qt.Assert(t, signer0Account.InfoURI, qt.CmpEquals(), infoURI2)

	// should work for itself without a faucet package
	qt.Assert(t, testSetAccountTx(t,
		signers[0], common.Address{}, nil, app, infoURI, 1),
		qt.IsNil,
	)
	signer0Account, err = app.State.GetAccount(signers[0].Address(), false)
	qt.Assert(t, err, qt.IsNil)
	qt.Assert(t, signer0Account, qt.IsNotNil)
	qt.Assert(t, signer0Account.Balance, qt.DeepEquals, uint64(5400))
	qt.Assert(t, signer0Account.Nonce, qt.DeepEquals, uint32(2))
	qt.Assert(t, signer0Account.InfoURI, qt.CmpEquals(), infoURI)
	// should not work for itself without a faucet package and invalid InfoURI
	qt.Assert(t, testSetAccountTx(t,
		signers[0], common.Address{}, nil, app, "", 2),
		qt.IsNotNil,
	)
	signer0Account, err = app.State.GetAccount(signers[0].Address(), false)
	qt.Assert(t, err, qt.IsNil)
	qt.Assert(t, signer0Account, qt.IsNotNil)
	qt.Assert(t, signer0Account.Balance, qt.DeepEquals, uint64(5400))
	qt.Assert(t, signer0Account.Nonce, qt.DeepEquals, uint32(2))
	qt.Assert(t, signer0Account.InfoURI, qt.CmpEquals(), infoURI)

	// SET ACCOUNT INFO URI AS DELEGATE

	// should not work if not a delegate
	qt.Assert(t, testSetAccountTx(t,
		signers[1], signers[0].Address(), nil, app, infoURI, 0),
		qt.IsNotNil,
	)
	signer0Account, err = app.State.GetAccount(signers[0].Address(), false)
	qt.Assert(t, err, qt.IsNil)
	qt.Assert(t, signer0Account, qt.IsNotNil)
	qt.Assert(t, signer0Account.Balance, qt.DeepEquals, uint64(5400))
	qt.Assert(t, signer0Account.Nonce, qt.DeepEquals, uint32(2))
	qt.Assert(t, signer0Account.InfoURI, qt.CmpEquals(), infoURI)
	signer1Account, err = app.State.GetAccount(signers[1].Address(), false)
	qt.Assert(t, err, qt.IsNil)
	qt.Assert(t, signer1Account, qt.IsNotNil)
	qt.Assert(t, signer1Account.Balance, qt.DeepEquals, uint64(1000))
	qt.Assert(t, signer1Account.Nonce, qt.DeepEquals, uint32(0))
	qt.Assert(t, signer1Account.InfoURI, qt.CmpEquals(), "")
	// should work if delegate
	qt.Assert(t, app.State.SetAccountDelegate(
		signers[0].Address(),
		[]common.Address{signers[1].Address()},
		models.TxType_ADD_DELEGATE_FOR_ACCOUNT,
	), qt.IsNil)
	qt.Assert(t, testSetAccountTx(t,
		signers[1], signers[0].Address(), nil, app, infoURI2, 0),
		qt.IsNil,
	)
	signer0Account, err = app.State.GetAccount(signers[0].Address(), false)
	qt.Assert(t, err, qt.IsNil)
	qt.Assert(t, signer0Account, qt.IsNotNil)
	qt.Assert(t, signer0Account.Balance, qt.DeepEquals, uint64(5400))
	qt.Assert(t, signer0Account.Nonce, qt.DeepEquals, uint32(2))
	qt.Assert(t, signer0Account.InfoURI, qt.CmpEquals(), infoURI2)
	signer1Account, err = app.State.GetAccount(signers[1].Address(), false)
	qt.Assert(t, err, qt.IsNil)
	qt.Assert(t, signer1Account, qt.IsNotNil)
	qt.Assert(t, signer1Account.Balance, qt.DeepEquals, uint64(900))
	qt.Assert(t, signer1Account.Nonce, qt.DeepEquals, uint32(1))
	qt.Assert(t, signer1Account.InfoURI, qt.CmpEquals(), "")
}

func testSetAccountTx(t *testing.T,
	signer *ethereum.SignKeys,
	account common.Address,
	faucetPkg *models.FaucetPackage,
	app *BaseApplication,
	infoURI string,
	nonce uint32) error {
	var err error

	tx := &models.SetAccountTx{
		Nonce:         &nonce,
		Txtype:        models.TxType_SET_ACCOUNT_INFO_URI,
		InfoURI:       &infoURI,
		Account:       account.Bytes(),
		FaucetPackage: faucetPkg,
	}

	stx := &models.SignedTx{}
	if stx.Tx, err = proto.Marshal(&models.Tx{Payload: &models.Tx_SetAccount{SetAccount: tx}}); err != nil {
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
	tx := &models.SetAccountTx{
		Delegates: [][]byte{delegate.Bytes()},
		Nonce:     &nonce,
		Txtype:    models.TxType_ADD_DELEGATE_FOR_ACCOUNT,
	}
	if !op {
		tx.Txtype = models.TxType_DEL_DELEGATE_FOR_ACCOUNT
	}

	stx := &models.SignedTx{}
	if stx.Tx, err = proto.Marshal(&models.Tx{Payload: &models.Tx_SetAccount{SetAccount: tx}}); err != nil {
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
	qt.Assert(t, acc.Nonce, qt.Equals, uint32(0))
	signerAcc, err := app.State.GetAccount(signer.Address(), true)
	qt.Assert(t, err, qt.IsNil)
	qt.Assert(t, signerAcc.Balance, qt.Equals, uint64(90))
	qt.Assert(t, signerAcc.Nonce, qt.Equals, uint32(1))
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
	err = testCollectFaucetTx(t, &signer, &toSigner, app, 2, 5, randomIdentifier+1)
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
	qt.Assert(t, acc.Nonce, qt.Equals, uint32(0))
	signerAcc, err = app.State.GetAccount(signer.Address(), true)
	qt.Assert(t, err, qt.IsNil)
	qt.Assert(t, signerAcc.Balance, qt.Equals, uint64(90))
	qt.Assert(t, signerAcc.Nonce, qt.Equals, uint32(1))
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
		Payload:   faucetPayloadBytes,
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

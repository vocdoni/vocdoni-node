package vochain

import (
	"context"
	"fmt"
	"math/rand"
	"testing"

	abcitypes "github.com/cometbft/cometbft/abci/types"
	"github.com/ethereum/go-ethereum/common"
	qt "github.com/frankban/quicktest"

	"go.vocdoni.io/dvote/crypto/ethereum"
	"go.vocdoni.io/dvote/util"
	"go.vocdoni.io/dvote/vochain/state"
	"go.vocdoni.io/dvote/vochain/transaction/vochaintx"
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

func testCommitState(t *testing.T, app *BaseApplication) {
	if _, err := app.State.PrepareCommit(); err != nil {
		t.Fatal(err)
	}
	if _, err := app.CommitState(); err != nil {
		t.Fatal(err)
	}
}

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
	if err := app.State.SetAccount(state.BurnAddress, &state.Account{}); err != nil {
		return nil, nil, err
	}
	// set tx costs
	if err := app.State.SetTxBaseCost(models.TxType_SET_ACCOUNT_INFO_URI, 100); err != nil {
		return nil, nil, err
	}
	if err := app.State.SetTxBaseCost(models.TxType_ADD_DELEGATE_FOR_ACCOUNT, 100); err != nil {
		return nil, nil, err
	}
	if err := app.State.SetTxBaseCost(models.TxType_DEL_DELEGATE_FOR_ACCOUNT, 100); err != nil {
		return nil, nil, err
	}
	if err := app.State.SetTxBaseCost(models.TxType_CREATE_ACCOUNT, 100); err != nil {
		return nil, nil, err
	}
	if err := app.State.SetTxBaseCost(models.TxType_COLLECT_FAUCET, 100); err != nil {
		return nil, nil, err
	}
	// save state
	testCommitState(t, app)
	return app, signers, nil
}

func TestSetAccountTx(t *testing.T) {
	app, signers, err := setupTestBaseApplicationAndSigners(t, 10)
	qt.Assert(t, err, qt.IsNil)
	// set account 0
	qt.Assert(t,
		app.State.SetAccount(signers[0].Address(), &state.Account{
			Account: models.Account{Balance: 10000, InfoURI: infoURI}}),
		qt.IsNil,
	)
	testCommitState(t, app)

	// CREATE ACCOUNT

	// should work is a valid faucet payload and tx.Account is provided and no infoURI is set
	faucetPkg, err := GenerateFaucetPackage(signers[0], signers[1].Address(), 1000)
	qt.Assert(t, err, qt.IsNil)
	qt.Assert(t, testSetAccountTx(t,
		signers[1], signers[1].Address(), faucetPkg, app, "", 0, true),
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
	faucetPkg, err = GenerateFaucetPackage(signers[0], signers[2].Address(), 1000)
	qt.Assert(t, err, qt.IsNil)
	qt.Assert(t, testSetAccountTx(t,
		signers[2], common.Address{}, faucetPkg, app, "", 0, true),
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
	faucetPkg, err = GenerateFaucetPackage(signers[0], signers[3].Address(), 1000)
	qt.Assert(t, err, qt.IsNil)
	qt.Assert(t, testSetAccountTx(t,
		signers[3], signers[3].Address(), faucetPkg, app, infoURI, 0, true),
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
	faucetPkgToReuse, err := GenerateFaucetPackage(signers[0], signers[4].Address(), 1000)
	qt.Assert(t, err, qt.IsNil)
	qt.Assert(t, testSetAccountTx(t,
		signers[4], common.Address{}, faucetPkgToReuse, app, infoURI, 0, true),
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
		signers[5], common.Address{}, faucetPkg, app, infoURI, 0, true),
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
		signers[5], common.Address{}, faucetPkg, app, infoURI, 0, true),
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
		signers[5], common.Address{}, faucetPkg, app, infoURI, 0, true),
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
	qt.Assert(t, testSetAccountTx(t,
		signers[5], common.Address{}, faucetPkgToReuse, app, infoURI, 0, true),
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
	faucetPkg, err = GenerateFaucetPackage(signers[0], signers[5].Address(), 10000)
	qt.Assert(t, err, qt.IsNil)
	qt.Assert(t, testSetAccountTx(t,
		signers[5], common.Address{}, faucetPkg, app, infoURI, 0, true),
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
	faucetPkg, err = GenerateFaucetPackage(signers[5], signers[6].Address(), 1000)
	qt.Assert(t, err, qt.IsNil)
	qt.Assert(t, testSetAccountTx(t,
		signers[6], common.Address{}, faucetPkg, app, infoURI, 0, true),
		qt.IsNotNil,
	)
	signer5Account, err = app.State.GetAccount(signers[5].Address(), false)
	qt.Assert(t, err, qt.IsNil)
	qt.Assert(t, signer5Account, qt.IsNil)
	signer6Account, err := app.State.GetAccount(signers[6].Address(), false)
	qt.Assert(t, err, qt.IsNil)
	qt.Assert(t, signer6Account, qt.IsNil)

	// should not work if faucet package issuer cannot cover the txs cost + faucet amount
	faucetPkg, err = GenerateFaucetPackage(signers[0], signers[5].Address(), 20000)
	qt.Assert(t, err, qt.IsNil)
	qt.Assert(t, testSetAccountTx(t,
		signers[5], common.Address{}, faucetPkg, app, infoURI, 0, true),
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
	faucetPkg, err = GenerateFaucetPackage(signers[0], signers[5].Address(), 0)
	qt.Assert(t, err, qt.IsNil)
	qt.Assert(t, testSetAccountTx(t,
		signers[5], common.Address{}, faucetPkg, app, infoURI, 0, true),
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

	// should work if random nonce provided
	faucetPkg, err = GenerateFaucetPackage(signers[0], signers[5].Address(), 10)
	qt.Assert(t, err, qt.IsNil)
	qt.Assert(t, testSetAccountTx(t,
		signers[5], common.Address{}, faucetPkg, app, infoURI, rand.Uint32(), true),
		qt.IsNil,
	)
	signer0Account, err = app.State.GetAccount(signers[0].Address(), false)
	qt.Assert(t, err, qt.IsNil)
	qt.Assert(t, signer0Account, qt.IsNotNil)
	qt.Assert(t, signer0Account.Balance, qt.DeepEquals, uint64(5490))
	qt.Assert(t, signer0Account.Nonce, qt.DeepEquals, uint32(0))
	qt.Assert(t, signer0Account.InfoURI, qt.CmpEquals(), infoURI)
	signer5Account, err = app.State.GetAccount(signers[5].Address(), false)
	qt.Assert(t, err, qt.IsNil)
	qt.Assert(t, signer5Account, qt.IsNotNil)
	qt.Assert(t, signer5Account.Balance, qt.DeepEquals, uint64(10))
	qt.Assert(t, signer5Account.Nonce, qt.DeepEquals, uint32(0))
	qt.Assert(t, signer5Account.InfoURI, qt.CmpEquals(), infoURI)
	qt.Assert(t, signer5Account.DelegateAddrs, qt.DeepEquals, [][]byte(nil))

	// should work if random account provided, the account created is the signer one
	faucetPkg, err = GenerateFaucetPackage(signers[0], signers[6].Address(), 10)
	qt.Assert(t, err, qt.IsNil)
	qt.Assert(t, testSetAccountTx(t,
		signers[6], signers[7].Address(), faucetPkg, app, infoURI, rand.Uint32(), true),
		qt.IsNil,
	)
	signer0Account, err = app.State.GetAccount(signers[0].Address(), false)
	qt.Assert(t, err, qt.IsNil)
	qt.Assert(t, signer0Account, qt.IsNotNil)
	qt.Assert(t, signer0Account.Balance, qt.DeepEquals, uint64(5380))
	qt.Assert(t, signer0Account.Nonce, qt.DeepEquals, uint32(0))
	qt.Assert(t, signer0Account.InfoURI, qt.CmpEquals(), infoURI)
	signer6Account, err = app.State.GetAccount(signers[6].Address(), false)
	qt.Assert(t, err, qt.IsNil)
	qt.Assert(t, signer6Account, qt.IsNotNil)
	qt.Assert(t, signer6Account.Balance, qt.DeepEquals, uint64(10))
	qt.Assert(t, signer6Account.Nonce, qt.DeepEquals, uint32(0))
	qt.Assert(t, signer6Account.InfoURI, qt.CmpEquals(), infoURI)
	qt.Assert(t, signer6Account.DelegateAddrs, qt.DeepEquals, [][]byte(nil))
	signer7Account, err := app.State.GetAccount(signers[7].Address(), false)
	qt.Assert(t, err, qt.IsNil)
	qt.Assert(t, signer7Account, qt.IsNil)

	// should ignore tx cost if tx cost is set to 0
	qt.Assert(t, app.State.SetTxBaseCost(models.TxType_CREATE_ACCOUNT, 0), qt.IsNil)
	testCommitState(t, app)
	faucetPkg, err = GenerateFaucetPackage(signers[0], signers[7].Address(), 10)
	qt.Assert(t, err, qt.IsNil)
	qt.Assert(t, testSetAccountTx(t,
		signers[7], common.Address{}, faucetPkg, app, infoURI, 0, true),
		qt.IsNil,
	)
	signer0Account, err = app.State.GetAccount(signers[0].Address(), false)
	qt.Assert(t, err, qt.IsNil)
	qt.Assert(t, signer0Account, qt.IsNotNil)
	qt.Assert(t, signer0Account.Balance, qt.DeepEquals, uint64(5370))
	qt.Assert(t, signer0Account.Nonce, qt.DeepEquals, uint32(0))
	qt.Assert(t, signer0Account.InfoURI, qt.CmpEquals(), infoURI)
	signer7Account, err = app.State.GetAccount(signers[7].Address(), false)
	qt.Assert(t, err, qt.IsNil)
	qt.Assert(t, signer7Account, qt.IsNotNil)
	qt.Assert(t, signer7Account.Balance, qt.DeepEquals, uint64(10))
	qt.Assert(t, signer7Account.Nonce, qt.DeepEquals, uint32(0))
	qt.Assert(t, signer7Account.InfoURI, qt.CmpEquals(), infoURI)
	qt.Assert(t, signer7Account.DelegateAddrs, qt.DeepEquals, [][]byte(nil))

	// should work if tx cost is 0 and no faucet package is provided
	qt.Assert(t, testSetAccountTx(t,
		signers[8], common.Address{}, nil, app, infoURI, 0, true),
		qt.IsNil,
	)
	signer0Account, err = app.State.GetAccount(signers[0].Address(), false)
	qt.Assert(t, err, qt.IsNil)
	qt.Assert(t, signer0Account, qt.IsNotNil)
	qt.Assert(t, signer0Account.Balance, qt.DeepEquals, uint64(5370))
	qt.Assert(t, signer0Account.Nonce, qt.DeepEquals, uint32(0))
	qt.Assert(t, signer0Account.InfoURI, qt.CmpEquals(), infoURI)
	signer8Account, err := app.State.GetAccount(signers[8].Address(), false)
	qt.Assert(t, err, qt.IsNil)
	qt.Assert(t, signer8Account, qt.IsNotNil)
	qt.Assert(t, signer8Account.Balance, qt.DeepEquals, uint64(0))
	qt.Assert(t, signer8Account.Nonce, qt.DeepEquals, uint32(0))
	qt.Assert(t, signer8Account.InfoURI, qt.CmpEquals(), infoURI)
	qt.Assert(t, signer8Account.DelegateAddrs, qt.DeepEquals, [][]byte(nil))

	// should not work if tx cost is not 0 and no faucet package is provided
	qt.Assert(t, app.State.SetTxBaseCost(models.TxType_CREATE_ACCOUNT, 1), qt.IsNil)
	testCommitState(t, app)
	qt.Assert(t, testSetAccountTx(t,
		signers[9], common.Address{}, nil, app, infoURI, 0, true),
		qt.IsNotNil,
	)
	signer0Account, err = app.State.GetAccount(signers[0].Address(), false)
	qt.Assert(t, err, qt.IsNil)
	qt.Assert(t, signer0Account, qt.IsNotNil)
	qt.Assert(t, signer0Account.Balance, qt.DeepEquals, uint64(5370))
	qt.Assert(t, signer0Account.Nonce, qt.DeepEquals, uint32(0))
	qt.Assert(t, signer0Account.InfoURI, qt.CmpEquals(), infoURI)
	signer9Account, err := app.State.GetAccount(signers[9].Address(), false)
	qt.Assert(t, err, qt.IsNil)
	qt.Assert(t, signer9Account, qt.IsNil)

	// SET ACCOUNT INFO URI

	// should not work for itself with faucet package (the faucet package is ignored) and invalid InfoURI
	faucetPkg, err = GenerateFaucetPackage(signers[0], signers[0].Address(), 1000)
	qt.Assert(t, err, qt.IsNil)
	qt.Assert(t, testSetAccountTx(t,
		signers[0], common.Address{}, faucetPkg, app, "", 0, false),
		qt.IsNotNil,
	)
	signer0Account, err = app.State.GetAccount(signers[0].Address(), false)
	qt.Assert(t, err, qt.IsNil)
	qt.Assert(t, signer0Account, qt.IsNotNil)
	qt.Assert(t, signer0Account.Balance, qt.DeepEquals, uint64(5370))
	qt.Assert(t, signer0Account.Nonce, qt.DeepEquals, uint32(0))
	qt.Assert(t, signer0Account.InfoURI, qt.CmpEquals(), infoURI)

	// should not work for itself with faucet package (the faucet package is ignored) and invalid nonce
	qt.Assert(t, testSetAccountTx(t,
		signers[0], common.Address{}, faucetPkg, app, "ipfs://", rand.Uint32(), false),
		qt.IsNotNil,
	)
	signer0Account, err = app.State.GetAccount(signers[0].Address(), false)
	qt.Assert(t, err, qt.IsNil)
	qt.Assert(t, signer0Account, qt.IsNotNil)
	qt.Assert(t, signer0Account.Balance, qt.DeepEquals, uint64(5370))
	qt.Assert(t, signer0Account.Nonce, qt.DeepEquals, uint32(0))
	qt.Assert(t, signer0Account.InfoURI, qt.CmpEquals(), infoURI)

	// should not work for itself with an invalid infoURI
	qt.Assert(t, testSetAccountTx(t,
		signers[0], common.Address{}, nil, app, "", uint32(0), false),
		qt.IsNotNil,
	)
	qt.Assert(t, signer0Account, qt.IsNotNil)
	qt.Assert(t, signer0Account.Balance, qt.DeepEquals, uint64(5370))
	qt.Assert(t, signer0Account.Nonce, qt.DeepEquals, uint32(0))
	qt.Assert(t, signer0Account.InfoURI, qt.CmpEquals(), infoURI)

	// should not work for itself with an invalid nonce
	qt.Assert(t, testSetAccountTx(t,
		signers[0], common.Address{}, nil, app, infoURI2, uint32(1), false),
		qt.IsNotNil,
	)
	qt.Assert(t, signer0Account, qt.IsNotNil)
	qt.Assert(t, signer0Account.Balance, qt.DeepEquals, uint64(5370))
	qt.Assert(t, signer0Account.Nonce, qt.DeepEquals, uint32(0))
	qt.Assert(t, signer0Account.InfoURI, qt.CmpEquals(), infoURI)

	// should work for itself without a faucet package
	qt.Assert(t, testSetAccountTx(t,
		signers[0], common.Address{}, nil, app, infoURI2, 0, false),
		qt.IsNil,
	)
	signer0Account, err = app.State.GetAccount(signers[0].Address(), false)
	qt.Assert(t, err, qt.IsNil)
	qt.Assert(t, signer0Account, qt.IsNotNil)
	qt.Assert(t, signer0Account.Balance, qt.DeepEquals, uint64(5270))
	qt.Assert(t, signer0Account.Nonce, qt.DeepEquals, uint32(1))
	qt.Assert(t, signer0Account.InfoURI, qt.CmpEquals(), infoURI2)

	// SET ACCOUNT INFO URI AS DELEGATE

	// should not work if not a delegate
	qt.Assert(t, testSetAccountTx(t,
		signers[1], signers[0].Address(), nil, app, infoURI, 0, false),
		qt.IsNotNil,
	)
	signer0Account, err = app.State.GetAccount(signers[0].Address(), false)
	qt.Assert(t, err, qt.IsNil)
	qt.Assert(t, signer0Account, qt.IsNotNil)
	qt.Assert(t, signer0Account.Balance, qt.DeepEquals, uint64(5270))
	qt.Assert(t, signer0Account.Nonce, qt.DeepEquals, uint32(1))
	qt.Assert(t, signer0Account.InfoURI, qt.CmpEquals(), infoURI2)
	signer1Account, err = app.State.GetAccount(signers[1].Address(), false)
	qt.Assert(t, err, qt.IsNil)
	qt.Assert(t, signer1Account, qt.IsNotNil)
	qt.Assert(t, signer1Account.Balance, qt.DeepEquals, uint64(1000))
	qt.Assert(t, signer1Account.Nonce, qt.DeepEquals, uint32(0))
	qt.Assert(t, signer1Account.InfoURI, qt.CmpEquals(), "")
	// should work if delegate
	qt.Assert(t, app.State.SetAccountDelegate(
		signers[0].Address(),
		[][]byte{signers[1].Address().Bytes()},
		models.TxType_ADD_DELEGATE_FOR_ACCOUNT,
	), qt.IsNil)
	qt.Assert(t, testSetAccountTx(t,
		signers[1], signers[0].Address(), nil, app, infoURI, 0, false),
		qt.IsNil,
	)
	signer0Account, err = app.State.GetAccount(signers[0].Address(), false)
	qt.Assert(t, err, qt.IsNil)
	qt.Assert(t, signer0Account, qt.IsNotNil)
	qt.Assert(t, signer0Account.Balance, qt.DeepEquals, uint64(5270))
	qt.Assert(t, signer0Account.Nonce, qt.DeepEquals, uint32(1))
	qt.Assert(t, signer0Account.InfoURI, qt.CmpEquals(), infoURI)
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
	nonce uint32,
	create bool) error {
	var err error

	sik, err := signer.AccountSIK(nil)
	if err != nil {
		return err
	}

	tx := &models.SetAccountTx{
		Nonce:         &nonce,
		Txtype:        models.TxType_SET_ACCOUNT_INFO_URI,
		Account:       account.Bytes(),
		FaucetPackage: faucetPkg,
		SIK:           sik,
	}
	if create {
		tx.Txtype = models.TxType_CREATE_ACCOUNT
	}
	if infoURI != "" {
		tx.InfoURI = &infoURI
	}
	stx := &models.SignedTx{}
	if stx.Tx, err = proto.Marshal(&models.Tx{Payload: &models.Tx_SetAccount{SetAccount: tx}}); err != nil {
		t.Fatal(err)
	}

	if err := sendTx(app, signer, stx); err != nil {
		return err
	}
	testCommitState(t, app)
	return nil
}

func TestSendTokensTx(t *testing.T) {
	app := TestBaseApplication(t)

	signer := ethereum.SignKeys{}
	err := signer.Generate()
	qt.Assert(t, err, qt.IsNil)

	app.State.SetAccount(state.BurnAddress, &state.Account{})

	err = app.State.SetTxBaseCost(models.TxType_SEND_TOKENS, 10)
	qt.Assert(t, err, qt.IsNil)

	err = app.State.CreateAccount(signer.Address(), "ipfs://", [][]byte{}, 0)
	qt.Assert(t, err, qt.IsNil)

	toAccAddr := common.HexToAddress(randomEthAccount)
	err = app.State.CreateAccount(toAccAddr, "ipfs://", [][]byte{}, 0)
	qt.Assert(t, err, qt.IsNil)

	err = app.State.MintBalance(&vochaintx.TokenTransfer{
		ToAddress: signer.Address(),
		Amount:    1000,
	})
	qt.Assert(t, err, qt.IsNil)
	testCommitState(t, app)

	// should send
	err = testSendTokensTx(t, &signer, app, toAccAddr, 100, 0)
	qt.Assert(t, err, qt.IsNil)

	// should fail sending if incorrect nonce
	err = testSendTokensTx(t, &signer, app, toAccAddr, 100, 0)
	qt.Assert(t, err, qt.IsNotNil)
	// should fail sending if to acc does not exist
	noAccount := ethereum.SignKeys{}
	err = noAccount.Generate()
	qt.Assert(t, err, qt.IsNil)
	err = testSendTokensTx(t, &signer, app, noAccount.Address(), 100, 1)
	qt.Assert(t, err, qt.IsNotNil)
	// should fail sending if not enough balance
	err = testSendTokensTx(t, &signer, app, toAccAddr, 1000, 1)
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

func TestSendTokensTxToTheSameAccount(t *testing.T) {
	app := TestBaseApplication(t)

	signer := ethereum.SignKeys{}
	err := signer.Generate()
	qt.Assert(t, err, qt.IsNil)

	app.State.SetAccount(state.BurnAddress, &state.Account{})

	err = app.State.SetTxBaseCost(models.TxType_SEND_TOKENS, 10)
	qt.Assert(t, err, qt.IsNil)

	err = app.State.CreateAccount(signer.Address(), "ipfs://", [][]byte{}, 0)
	qt.Assert(t, err, qt.IsNil)

	err = app.State.MintBalance(&vochaintx.TokenTransfer{
		ToAddress: signer.Address(),
		Amount:    1000,
	})
	qt.Assert(t, err, qt.IsNil)
	testCommitState(t, app)

	err = testSendTokensTx(t, &signer, app, signer.Address(), 89, 0)
	qt.Assert(t, err, qt.IsNotNil)

}

func testSendTokensTx(t *testing.T,
	signer *ethereum.SignKeys,
	app *BaseApplication,
	toAddr common.Address,
	value uint64,
	nonce uint32) error {
	var err error

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
	testCommitState(t, app)
	return nil
}

func TestSetAccountDelegateTx(t *testing.T) {
	app := TestBaseApplication(t)

	signer := ethereum.SignKeys{}
	err := signer.Generate()
	qt.Assert(t, err, qt.IsNil)

	signer2 := ethereum.SignKeys{}
	err = signer2.Generate()
	qt.Assert(t, err, qt.IsNil)

	app.State.SetAccount(state.BurnAddress, &state.Account{})

	err = app.State.SetTxBaseCost(models.TxType_ADD_DELEGATE_FOR_ACCOUNT, 10)
	qt.Assert(t, err, qt.IsNil)
	err = app.State.SetTxBaseCost(models.TxType_DEL_DELEGATE_FOR_ACCOUNT, 10)
	qt.Assert(t, err, qt.IsNil)

	err = app.State.CreateAccount(signer.Address(), "ipfs://", [][]byte{}, 0)
	qt.Assert(t, err, qt.IsNil)
	toAccAddr := common.HexToAddress(randomEthAccount)
	err = app.State.CreateAccount(toAccAddr, "ipfs://", [][]byte{}, 0)
	qt.Assert(t, err, qt.IsNil)
	err = app.State.CreateAccount(signer2.Address(), "ipfs://", [][]byte{}, 0)
	qt.Assert(t, err, qt.IsNil)

	err = app.State.MintBalance(&vochaintx.TokenTransfer{
		ToAddress: signer.Address(),
		Amount:    1000,
	})
	qt.Assert(t, err, qt.IsNil)
	err = app.State.MintBalance(&vochaintx.TokenTransfer{
		ToAddress: signer2.Address(),
		Amount:    1000,
	})
	qt.Assert(t, err, qt.IsNil)

	testCommitState(t, app)

	// should add delegate if owner
	err = testSetAccountDelegateTx(t, &signer, app, toAccAddr, true, 0)
	qt.Assert(t, err, qt.IsNil)
	// get from acc
	acc, err := app.State.GetAccount(signer.Address(), false)
	qt.Assert(t, err, qt.IsNil)
	qt.Assert(t, acc, qt.IsNotNil)
	qt.Assert(t, acc.DelegateAddrs[0], qt.DeepEquals, toAccAddr.Bytes())

	// should fail adding himself as delegate
	err = testSetAccountDelegateTx(t, &signer2, app, signer2.Address(), true, 0)
	qt.Assert(t, err, qt.IsNotNil)
	acc2, err := app.State.GetAccount(signer2.Address(), false)
	qt.Assert(t, err, qt.IsNil)
	qt.Assert(t, acc2, qt.IsNotNil)
	qt.Assert(t, len(acc2.DelegateAddrs), qt.Equals, 0)

	// should fail if delegate already added
	err = testSetAccountDelegateTx(t, &signer, app, toAccAddr, true, 1)
	qt.Assert(t, err, qt.IsNotNil)

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
	testCommitState(t, app)
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

	app.State.SetAccount(state.BurnAddress, &state.Account{})

	err = app.State.SetTxBaseCost(models.TxType_COLLECT_FAUCET, 10)
	qt.Assert(t, err, qt.IsNil)

	err = app.State.CreateAccount(signer.Address(), "ipfs://", [][]byte{}, 0)
	qt.Assert(t, err, qt.IsNil)
	err = app.State.CreateAccount(toSigner.Address(), "ipfs://", [][]byte{}, 0)
	qt.Assert(t, err, qt.IsNil)

	err = app.State.MintBalance(&vochaintx.TokenTransfer{
		ToAddress: signer.Address(),
		Amount:    1000,
	})
	qt.Assert(t, err, qt.IsNil)

	testCommitState(t, app)

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
	burnAcc, err := app.State.GetAccount(state.BurnAddress, true)
	qt.Assert(t, err, qt.IsNil)
	qt.Assert(t, burnAcc.Balance, qt.Equals, uint64(10))

	// should fail if identifier is already used
	err = testCollectFaucetTx(t, &signer, &toSigner, app, 0, 1, randomIdentifier)
	qt.Assert(t, err, qt.IsNotNil)
	qt.Assert(t, err, qt.ErrorMatches, ".* faucet payload already used")

	// should fail if to acc does not exist
	nonExistentAccount := ethereum.SignKeys{}
	err = nonExistentAccount.Generate()
	qt.Assert(t, err, qt.IsNil)

	err = testCollectFaucetTx(t, &signer, &nonExistentAccount, app, 0, 1, randomIdentifier+1)
	qt.Assert(t, err, qt.IsNotNil)
	qt.Assert(t, err, qt.ErrorMatches, fmt.Sprintf(".* %s", state.ErrAccountNotExist))

	// should fail if from acc does not exist
	err = testCollectFaucetTx(t, &nonExistentAccount, &toSigner, app, 0, 1, randomIdentifier+1)
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
	err = testCollectFaucetTx(t, &signer, &toSigner, app, 0, 100000, randomIdentifier+1)
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
	burnAcc, err = app.State.GetAccount(state.BurnAddress, true)
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
	testCommitState(t, app)
	return nil
}

// sendTx signs and sends a vochain transaction
func sendTx(app *BaseApplication, signer *ethereum.SignKeys, stx *models.SignedTx) error {
	var err error

	if stx.Signature, err = signer.SignVocdoniTx(stx.Tx, app.chainID); err != nil {
		return err
	}
	stxBytes, err := proto.Marshal(stx)
	if err != nil {
		return err
	}

	cktx := new(abcitypes.RequestCheckTx)
	cktx.Tx = stxBytes
	cktxresp, _ := app.CheckTx(context.Background(), cktx)
	if cktxresp.Code != 0 {
		return fmt.Errorf("checkTx failed: %s", cktxresp.Data)
	}
	detxresp := app.deliverTx(stxBytes)
	if detxresp.Code != 0 {
		return fmt.Errorf("deliverTx failed: %s", detxresp.Data)
	}
	return nil
}

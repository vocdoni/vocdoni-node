package main

import (
	"context"
	"encoding/hex"
	"fmt"
	"net/url"
	"strings"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/google/uuid"
	"go.vocdoni.io/dvote/apiclient"
	"go.vocdoni.io/dvote/crypto/ethereum"
	"go.vocdoni.io/dvote/log"
	"go.vocdoni.io/dvote/vochain"
	"go.vocdoni.io/dvote/vochain/state"
	"go.vocdoni.io/proto/build/go/models"
)

func testTokenTransactions(c config) {
	treasurerSigner, err := privKeyToSigner(c.treasurerPrivKey)
	if err != nil {
		log.Fatal(err)
	}
	// create main signer
	mainSigner := &ethereum.SignKeys{}
	if err := mainSigner.Generate(); err != nil {
		log.Fatal(err)
	}

	// create other signer
	otherSigner := &ethereum.SignKeys{}
	if err := otherSigner.Generate(); err != nil {
		log.Fatal(err)
	}

	// Connect to the API host
	hostURL, err := url.Parse(c.host)
	if err != nil {
		log.Fatal(err)
	}
	log.Debugf("connecting to %s", hostURL.String())

	token := uuid.New()
	api, err := apiclient.NewHTTPclient(hostURL, &token)
	if err != nil {
		log.Fatal(err)
	}

	// Set the account in the API client, so we can sign transactions
	if err := api.SetAccount(hex.EncodeToString(c.accountKeys[0].PrivateKey())); err != nil {
		log.Fatal(err)
	}

	// check transaction cost
	if err := testGetTxCost(api, treasurerSigner); err != nil {
		log.Fatal(err)
	}

	// check create and set account
	if err := testCreateAndSetAccount(api, treasurerSigner, c.accountKeys[0], mainSigner, otherSigner); err != nil {
		log.Fatal(err)
	}

	// check send tokens
	if err := testSendTokens(api, treasurerSigner, mainSigner, otherSigner); err != nil {
		log.Fatal(err)
	}
}

func testGetTxCost(api *apiclient.HTTPclient, treasurerSigner *ethereum.SignKeys) error {
	// get treasurer nonce
	treasurer, err := api.Treasurer()
	if err != nil {
		return err
	}
	log.Infof("treasurer fetched %s with nonce %d", common.BytesToAddress(treasurer.Address), treasurer.Nonce)

	// get current tx cost
	txCost, err := api.TransactionCost(models.TxType_SET_ACCOUNT_INFO_URI)
	if err != nil {
		return err
	}
	log.Infof("tx cost of %s fetched successfully (%d)", models.TxType_SET_ACCOUNT_INFO_URI, txCost)

	// TODO?: set tx cost using SubmitTx
	return nil
}

func testCreateAndSetAccount(api *apiclient.HTTPclient, treasurer, keySigner, signer, signer2 *ethereum.SignKeys) error {
	// generate faucet package
	fp, err := vochain.GenerateFaucetPackage(keySigner, signer.Address(), 500)
	if err != nil {
		return fmt.Errorf("cannot generate faucet package: %v", err)
	}
	// create account with faucet package
	if err := ensureAccountExists(api.Clone(fmt.Sprintf("%x", signer.PrivateKey())), fp); err != nil {
		return err
	}

	// mint tokens to signer
	if err := ensureAccountHasTokens(api.Clone(fmt.Sprintf("%x", treasurer.PrivateKey())), signer.Address(), 1000000); err != nil {
		return fmt.Errorf("cannot mint tokens for account %s: %v", signer.Address(), err)
	}
	log.Infof("minted 1000000 tokens to %s", signer.Address())

	// check account created
	acc, err := api.Account(signer.Address().String())
	if err != nil {
		return err
	}
	if acc == nil {
		return state.ErrAccountNotExist
	}
	log.Infof("account %s successfully created: %+v", signer.Address(), acc)

	// // now try set own account info
	// err = c.ensureAccountInfoEquals(signer, "ipfs://XXX")
	// if err != nil {
	// 	return err
	// }

	// // check account info changed
	// acc, err = c.GetAccount(signer.Address())
	// if err != nil {
	// 	return err
	// }
	// if acc == nil {
	// 	return state.ErrAccountNotExist
	// }
	// if acc.InfoURI != "ipfs://XXX" {
	// 	return fmt.Errorf("expected account infoURI to be %s got %s", "ipfs://XXX", acc.InfoURI)
	// }
	// log.Infof("account %s infoURI successfully changed to %+v", signer.Address(), acc.InfoURI)

	// create account for signer2 with faucet package from signer
	faucetPkg, err := vochain.GenerateFaucetPackage(signer, signer2.Address(), 5000)
	if err != nil {
		return fmt.Errorf("cannot generate faucet package %v", err)
	}

	if err := ensureAccountExists(api.Clone(fmt.Sprintf("%x", signer2.PrivateKey())), faucetPkg); err != nil {
		return err
	}

	// check account created
	acc2, err := api.Account(signer2.Address().String())
	if err != nil {
		return err
	}
	if acc2 == nil {
		return state.ErrAccountNotExist
	}
	// check balance added from payload
	if acc2.Balance != 5000 {
		return fmt.Errorf("expected balance for account %s is %d but got %d", signer2.Address(), 5000, acc2.Balance)
	}
	log.Infof("account %s (%+v) successfully created with payload signed by %s", signer2.Address(), acc2, signer.Address())
	return nil
}

func testSendTokens(api *apiclient.HTTPclient, treasurerSigner, signer, signer2 *ethereum.SignKeys) error {
	txCost, err := api.TransactionCost(models.TxType_SEND_TOKENS)
	if err != nil {
		return err
	}
	log.Infof("tx cost of %s is %d", models.TxType_SEND_TOKENS, txCost)
	acc, err := api.Account(signer.Address().String())
	if err != nil {
		return err
	}
	if acc == nil {
		return state.ErrAccountNotExist
	}
	log.Infof("fetched from account %s with nonce %d and balance %d", signer.Address(), acc.Nonce, acc.Balance)
	// try send tokens
	txhash, err := api.Transfer(signer2.Address(), 100)
	if err != nil {
		return fmt.Errorf("cannot send tokens: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), time.Second*40)
	defer cancel()
	_, err = api.WaitUntilTxIsMined(ctx, txhash)
	if err != nil {
		return err
	}

	acc2, err := api.Account(signer2.Address().String())
	if err != nil {
		return err
	}
	if acc2 == nil {
		return state.ErrAccountNotExist
	}
	log.Infof("fetched to account %s with nonce %d and balance %d", signer2.Address(), acc2.Nonce, acc2.Balance)
	if acc2.Balance != 5100 {
		log.Fatalf("expected %s to have balance %d got %d", signer2.Address(), 5100, acc2.Balance)
	}
	acc3, err := api.Account(signer.Address().String())
	if err != nil {
		return err
	}
	if acc3 == nil {
		return state.ErrAccountNotExist
	}
	log.Infof("fetched from account %s with nonce %d and balance %d", signer.Address(), acc3.Nonce, acc3.Balance)
	if acc.Balance-(100+uint64(txCost)) != acc3.Balance {
		log.Fatalf("expected %s to have balance %d got %d", signer.Address(), acc.Balance-(100+uint64(txCost)), acc3.Balance)
	}
	if acc.Nonce+1 != acc3.Nonce {
		log.Fatalf("expected %s to have balance %d got %d", signer.Address(), acc.Nonce+1, acc3.Nonce)
	}
	return nil
}

func ensureAccountExists(api *apiclient.HTTPclient, faucetPkg *models.FaucetPackage) error {
	for i := 0; i < retries; i++ {
		acct, err := api.Account("")
		if err != nil {
			log.Debugf("GetAccount try %d: %v", i, err)
		}
		// if account exists, we're done
		if acct != nil {
			return nil
		}

		_, err = api.AccountBootstrap(faucetPkg, nil)
		if err != nil {
			if strings.Contains(err.Error(), "tx already exists in cache") {
				// don't worry then, someone else created it in a race, nevermind, job done.
			} else {
				return fmt.Errorf("cannot create account %s: %w", api.MyAddress(), err)
			}
		}

		ctx, cancel := context.WithTimeout(context.Background(), time.Second*10)
		defer cancel()
		api.WaitUntilNextBlock(ctx)
	}
	return fmt.Errorf("cannot create account %s after %d retries", api.MyAddress(), retries)
}

// ensureAccountHasTokens
//
// This tx needs to be signed by the treasurer private key
// so first use api.SetAccount(treasurer) or api.Clone(treasurer)
// before calling this method
func ensureAccountHasTokens(
	api *apiclient.HTTPclient,
	accountAddr common.Address,
	amount uint64,
) error {
	for i := 0; i < retries; i++ {
		acct, err := api.Account(accountAddr.String())
		if err != nil {
			log.Debugf("GetAccount try %d: %v", i, err)
		}
		// if balance is enough, we're done
		if acct != nil && acct.Balance >= amount {
			return nil
		}

		// MintTokens can return OK and then the tx be rejected anyway later.
		// So, don't panic on errors, we only care about acct.Balance in the end
		err = api.MintTokens(accountAddr, amount)
		if err != nil {
			log.Debugf("MintTokens try %d: %v", i, err)
		}

		ctx, cancel := context.WithTimeout(context.Background(), time.Second*10)
		defer cancel()
		api.WaitUntilNextBlock(ctx)
	}
	return fmt.Errorf("tried to mint %d times, yet balance still not enough", retries)
}

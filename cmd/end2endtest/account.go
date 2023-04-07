package main

import (
	"context"
	"encoding/hex"
	"fmt"
	"net/url"
	"strings"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/google/go-cmp/cmp"
	"github.com/google/uuid"
	apipkg "go.vocdoni.io/dvote/api"
	"go.vocdoni.io/dvote/apiclient"
	"go.vocdoni.io/dvote/crypto/ethereum"
	"go.vocdoni.io/dvote/log"
	"go.vocdoni.io/dvote/vochain"
	"go.vocdoni.io/dvote/vochain/state"
	"go.vocdoni.io/proto/build/go/models"
)

func testTokenTransactions(c config) {

	// create alice signer
	alice := &ethereum.SignKeys{}
	if err := alice.Generate(); err != nil {
		log.Fatal(err)
	}

	// create bob signer
	bob := &ethereum.SignKeys{}
	if err := bob.Generate(); err != nil {
		log.Fatal(err)
	}

	// Connect to the API host
	hostURL, err := url.Parse(c.host)
	if err != nil {
		log.Fatal(err)
	}
	log.Debugw("connecting to API", "host", hostURL.String())

	token := uuid.New()
	api, err := apiclient.NewHTTPclient(hostURL, &token)
	if err != nil {
		log.Fatal(err)
	}

	// check transaction cost
	if err := testGetTxCost(api); err != nil {
		log.Fatal(err)
	}

	fp, err := getFaucetPackage(c, alice.Address().Hex())
	if err != nil {
		log.Fatal(err)
	}
	// check create and set account
	if err := testCreateAndSetAccount(api, fp, alice, bob); err != nil {
		log.Fatal(err)
	}

	// check send tokens
	if err := testSendTokens(api, alice, bob); err != nil {
		log.Fatal(err)
	}
}

func testGetTxCost(api *apiclient.HTTPclient) error {
	// get treasurer nonce
	treasurer, err := api.Treasurer()
	if err != nil {
		return err
	}
	log.Infof("treasurer is %s", common.BytesToAddress(treasurer.Address))

	// get current tx cost
	txCost, err := api.TransactionCost(models.TxType_SET_ACCOUNT_INFO_URI)
	if err != nil {
		return err
	}
	log.Infow("fetched tx cost", "type", models.TxType_SET_ACCOUNT_INFO_URI.String(), "cost", txCost)
	return nil
}

func testCreateAndSetAccount(api *apiclient.HTTPclient, fp *models.FaucetPackage, alice, bob *ethereum.SignKeys) error {
	// create account with faucet package
	aliceAcc, err := ensureAccountExists(api.Clone(hex.EncodeToString(alice.PrivateKey())), fp)
	if err != nil {
		return err
	}
	if aliceAcc == nil {
		return state.ErrAccountNotExist
	}
	log.Infow("account successfully created", "account", aliceAcc)

	// now try set own account info
	_, err = ensureAccountMetadataEquals(api.Clone(hex.EncodeToString(alice.PrivateKey())),
		&apipkg.AccountMetadata{Version: "12345"})
	if err != nil {
		return err
	}

	// create account for bob with faucet package from alice
	afp, err := vochain.GenerateFaucetPackage(alice, bob.Address(), 50)
	if err != nil {
		return fmt.Errorf("cannot generate faucet package %v", err)
	}

	bobAcc, err := ensureAccountExists(api.Clone(hex.EncodeToString(bob.PrivateKey())), afp)
	if err != nil {
		return err
	}

	// check balance added from payload
	if bobAcc.Balance != 50 {
		return fmt.Errorf("expected balance for bob (%s) is %d but got %d", bob.Address(), 50, bobAcc.Balance)
	}
	log.Infow("account for bob successfully created with payload signed by alice",
		"alice", alice.Address(), "bob", bobAcc)
	return nil
}

func testSendTokens(api *apiclient.HTTPclient, aliceKeys, bobKeys *ethereum.SignKeys) error {
	// if both alice and bob start with 50 tokens each
	// alice sends 19 to bob
	// and bob sends 23 to alice
	// both pay 2 for each tx
	// resulting in balance 52 for alice
	// and 44 for bob

	txCost, err := api.TransactionCost(models.TxType_SEND_TOKENS)
	if err != nil {
		return err
	}
	log.Infow("fetched tx cost", "type", models.TxType_SEND_TOKENS.String(), "cost", txCost)

	alice := api.Clone(hex.EncodeToString(aliceKeys.PrivateKey()))
	bob := api.Clone(hex.EncodeToString(bobKeys.PrivateKey()))

	aliceAcc, err := alice.Account("")
	if err != nil {
		return err
	}
	if aliceAcc == nil {
		return state.ErrAccountNotExist
	}
	log.Infow("alice", "account", aliceKeys.Address(), "nonce", aliceAcc.Nonce, "balance", aliceAcc.Balance)

	bobAcc, err := bob.Account("")
	if err != nil {
		return err
	}
	if bobAcc == nil {
		return state.ErrAccountNotExist
	}
	log.Infow("bob", "account", bobKeys.Address(), "nonce", bobAcc.Nonce, "balance", bobAcc.Balance)

	// try to send tokens at the same time
	txhasha, err := alice.Transfer(bobKeys.Address(), 19)
	if err != nil {
		return fmt.Errorf("cannot send tokens: %v", err)
	}
	log.Infof("alice sent %d tokens to bob", 19)
	log.Debugf("tx hash is %x", txhasha)

	txhashb, err := bob.Transfer(aliceKeys.Address(), 23)
	if err != nil {
		return fmt.Errorf("cannot send tokens: %v", err)
	}
	log.Infof("bob sent %d tokens to alice", 23)
	log.Debugf("tx hash is %x", txhashb)

	ctx, cancel := context.WithTimeout(context.Background(), time.Second*40)
	defer cancel()
	txrefa, err := api.WaitUntilTxIsMined(ctx, txhasha)
	if err != nil {
		return err
	}
	txrefb, err := api.WaitUntilTxIsMined(ctx, txhashb)
	if err != nil {
		return err
	}
	log.Debugf("mined, tx refs are %+v and %+v", txrefa, txrefb)

	// now check the resulting state
	bobAccAfter, err := bob.Account("")
	if err != nil {
		return err
	}
	if bobAccAfter == nil {
		return state.ErrAccountNotExist
	}
	log.Infow("bob", "account", bobKeys.Address(), "nonce", bobAcc.Nonce, "balance", bobAcc.Balance)
	if bobAcc.Balance-23-txCost+19 != bobAccAfter.Balance {
		log.Fatalf("expected %s to have balance %d got %d", bobKeys.Address(), 300, bobAccAfter.Balance)
	}

	aliceAccAfter, err := alice.Account("")
	if err != nil {
		return err
	}
	if aliceAccAfter == nil {
		return state.ErrAccountNotExist
	}
	log.Infow("alice", "account", aliceKeys.Address(), "nonce", aliceAcc.Nonce, "balance", aliceAcc.Balance)
	if aliceAcc.Balance-19-txCost+23 != aliceAccAfter.Balance {
		log.Fatalf("expected %s to have balance %d got %d",
			aliceKeys.Address(), aliceAcc.Balance-(100+txCost), aliceAccAfter.Balance)
	}
	if aliceAcc.Nonce+1 != aliceAccAfter.Nonce {
		log.Fatalf("expected %s to have nonce %d got %d",
			aliceKeys.Address(), aliceAcc.Nonce+1, aliceAccAfter.Nonce)
	}
	return nil
}

func ensureAccountExists(api *apiclient.HTTPclient,
	faucetPkg *models.FaucetPackage) (*apipkg.Account, error) {
	for i := 0; i < retries; i++ {
		acct, err := api.Account("")
		if err != nil {
			log.Debugf("GetAccount try %d: %v", i, err)
		}
		// if account exists, we're done
		if acct != nil {
			return acct, nil
		}

		_, err = api.AccountBootstrap(faucetPkg, nil)
		if err != nil {
			if strings.Contains(err.Error(), "tx already exists in cache") {
				// don't worry then, someone else created it in a race, nevermind, job done.
			} else {
				return nil, fmt.Errorf("cannot create account %s: %w", api.MyAddress(), err)
			}
		}

		ctx, cancel := context.WithTimeout(context.Background(), time.Second*10)
		defer cancel()
		api.WaitUntilNextBlock(ctx)
	}
	return nil, fmt.Errorf("cannot create account %s after %d retries", api.MyAddress(), retries)
}

func ensureAccountMetadataEquals(api *apiclient.HTTPclient,
	metadata *apipkg.AccountMetadata) (*apipkg.Account, error) {
	for i := 0; i < retries; i++ {
		acct, err := api.Account("")
		if err != nil {
			log.Debugf("GetAccount try %d: %v", i, err)
		}
		// if account metadata is as expected, we're done
		if acct != nil && cmp.Equal(acct.Metadata, metadata) {
			return acct, nil
		}

		_, err = api.AccountSetMetadata(metadata)
		if err != nil {
			if strings.Contains(err.Error(), "tx already exists in cache") {
				// don't worry then, someone else created it in a race, or still pending, nevermind
			} else if strings.Contains(err.Error(), "invalid URI, must be different") {
				// the tx from previous loop JUST got mined, also nevermind
			} else {
				return nil, fmt.Errorf("cannot set account %s metadata: %w", api.MyAddress(), err)
			}
		}

		ctx, cancel := context.WithTimeout(context.Background(), time.Second*10)
		defer cancel()
		api.WaitUntilNextBlock(ctx)
	}
	return nil, fmt.Errorf("cannot set account %s metadata after %d retries", api.MyAddress(), retries)
}

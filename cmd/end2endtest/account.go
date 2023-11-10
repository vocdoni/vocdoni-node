package main

import (
	"context"
	"encoding/hex"
	"fmt"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/google/go-cmp/cmp"
	apipkg "go.vocdoni.io/dvote/api"
	"go.vocdoni.io/dvote/apiclient"
	"go.vocdoni.io/dvote/crypto/ethereum"
	"go.vocdoni.io/dvote/log"
	"go.vocdoni.io/dvote/vochain"
	"go.vocdoni.io/dvote/vochain/state"
	"go.vocdoni.io/proto/build/go/models"
)

func init() {
	ops["tokentxs"] = operation{
		test:        &E2ETokenTxs{},
		description: "Tests all token related transactions",
		example: os.Args[0] + " --operation=tokentxs " +
			"--host http://127.0.0.1:9090/v2",
	}
}

var _ VochainTest = (*E2ETokenTxs)(nil)

type E2ETokenTxs struct {
	api    *apiclient.HTTPclient
	config *config

	alice, bob *ethereum.SignKeys
	aliceFP    *models.FaucetPackage
}

func (t *E2ETokenTxs) Setup(api *apiclient.HTTPclient, config *config) error {
	t.api = api
	t.config = config

	// create alice signer
	t.alice = ethereum.NewSignKeys()
	if err := t.alice.Generate(); err != nil {
		return fmt.Errorf("error in alice.Generate: %w", err)
	}

	// create bob signer
	t.bob = ethereum.NewSignKeys()
	if err := t.bob.Generate(); err != nil {
		return fmt.Errorf("error in bob.Generate: %w", err)
	}

	// get faucet package for alice
	var err error
	t.aliceFP, err = faucetPackage(t.config.faucet, t.alice.Address().Hex())
	if err != nil {
		return err
	}

	// check create and set account
	if err := testCreateAndSetAccount(t.api, t.aliceFP, t.alice, t.bob); err != nil {
		return fmt.Errorf("error in testCreateAndSetAccount: %w", err)
	}

	return nil
}

func (*E2ETokenTxs) Teardown() error {
	// nothing to do here
	return nil
}

func (t *E2ETokenTxs) Run() error {
	// check send tokens
	if err := testSendTokens(t.api, t.alice, t.bob); err != nil {
		return fmt.Errorf("error in testSendTokens: %w", err)
	}

	return nil
}

func testCreateAndSetAccount(api *apiclient.HTTPclient, fp *models.FaucetPackage, alice, bob *ethereum.SignKeys) error {
	// create account with faucet package
	aliceAcc, err := ensureAccountExists(api.Clone(hex.EncodeToString(alice.PrivateKey())), fp)
	if err != nil {
		return fmt.Errorf("error in ensureAccountExists(alice) %w", err)
	}
	if aliceAcc == nil {
		return state.ErrAccountNotExist
	}
	log.Infow("account successfully created", "account", aliceAcc)

	// now try set own account info
	if _, err := ensureAccountMetadataEquals(api.Clone(hex.EncodeToString(alice.PrivateKey())),
		&apipkg.AccountMetadata{Version: "12345"}); err != nil {
		return err
	}

	// create account for bob with faucet package from alice
	afp, err := vochain.GenerateFaucetPackage(alice, bob.Address(), aliceAcc.Balance/2)
	if err != nil {
		return fmt.Errorf("error in GenerateFaucetPackage %v", err)
	}

	bobAcc, err := ensureAccountExists(api.Clone(hex.EncodeToString(bob.PrivateKey())), afp)
	if err != nil {
		return fmt.Errorf("error in ensureAccountExists(bob) %w", err)
	}

	// check balance added from payload
	if bobAcc.Balance != aliceAcc.Balance/2 {
		return fmt.Errorf("expected balance for bob (%s) is %d but got %d",
			bob.Address(), aliceAcc.Balance/2, bobAcc.Balance)
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
	// In addition, we send a couple of token txs to burn address to increase the nonce,
	// without waiting for them to be mined (this tests that the mempool transactions are properly ordered).

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
	log.Infow("alice before", "account", aliceKeys.Address(), "nonce", aliceAcc.Nonce, "balance", aliceAcc.Balance)

	bobAcc, err := bob.Account("")
	if err != nil {
		return err
	}
	if bobAcc == nil {
		return state.ErrAccountNotExist
	}
	log.Infow("bob before", "account", bobKeys.Address(), "nonce", bobAcc.Nonce, "balance", bobAcc.Balance)

	// try to send tokens at the same time:
	// alice sends 1/4 of her balance to bob
	// sends 1/3 of his balance to alice
	// Subtract 1 + txCost from each since we are sending an extra tx to increase the nonce to the burn address
	amountAtoB := (aliceAcc.Balance) / 4
	amountBtoA := (bobAcc.Balance) / 3

	// send a couple of token txs to increase the nonce, without waiting for them to be mined
	// this tests that the mempool transactions are properly ordered.
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		log.Warnf("send transactions with nonce+1, should not be mined before the others")
		// send 1 token to burn address with nonce + 1 (should be mined after the other txs)
		if _, err := alice.TransferWithNonce(state.BurnAddress, 1, aliceAcc.Nonce+1); err != nil {
			log.Fatalf("cannot burn tokens: %v", err)
		}
		if _, err := bob.TransferWithNonce(state.BurnAddress, 1, bobAcc.Nonce+1); err != nil {
			log.Fatalf("cannot burn tokens: %v", err)
		}
		wg.Done()
	}()
	log.Warnf("waiting 6 seconds to let the burn txs be sent")
	time.Sleep(6 * time.Second)
	var txhasha, txhashb []byte
	wg.Add(1)
	go func() {
		var err error
		txhasha, err = alice.TransferWithNonce(bobKeys.Address(), amountAtoB, aliceAcc.Nonce)
		if err != nil {
			log.Fatalf("cannot send tokens: %v", err)
		}
		log.Infof("alice sent %d tokens to bob", amountAtoB)
		log.Debugf("tx hash is %x", txhasha)

		txhashb, err = bob.TransferWithNonce(aliceKeys.Address(), amountBtoA, bobAcc.Nonce)
		if err != nil {
			log.Fatalf("cannot send tokens: %v", err)
		}
		log.Infof("bob sent %d tokens to alice", amountBtoA)
		log.Debugf("tx hash is %x", txhashb)
		wg.Done()
	}()

	wg.Wait()
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

	// after a tx is mined in a block, the indexer takes some time to update the balances
	// (i.e. seconds, if there are votes to be indexed)
	// give it one more block time
	_ = api.WaitUntilNextBlock()

	// now check the resulting state
	if err := checkAccountNonceAndBalance(alice, aliceAcc.Nonce+2,
		aliceAcc.Balance-amountAtoB-(2*txCost+1)+amountBtoA); err != nil {
		return err
	}
	if err := checkAccountNonceAndBalance(bob, bobAcc.Nonce+2,
		bobAcc.Balance-amountBtoA-(2*txCost+1)+amountAtoB); err != nil {
		return err
	}

	if err := checkTokenTransfersCount(alice, aliceKeys.Address()); err != nil {
		return err
	}
	if err := checkTokenTransfersCount(bob, bobKeys.Address()); err != nil {
		return err
	}

	return nil
}

func checkAccountNonceAndBalance(api *apiclient.HTTPclient, expNonce uint32, expBalance uint64) error {
	acc, err := api.Account("")
	if err != nil {
		return err
	}
	log.Infow("current state", "account", acc.Address.String(), "nonce", acc.Nonce, "balance", acc.Balance)

	if expBalance != acc.Balance {
		return fmt.Errorf("expected %s to have balance %d got %d", acc.Address.String(), expBalance, acc.Balance)
	}
	if expNonce != acc.Nonce {
		return fmt.Errorf("expected %s to have nonce %d got %d", acc.Address.String(), expNonce, acc.Nonce)
	}
	return nil
}

func checkTokenTransfersCount(api *apiclient.HTTPclient, address common.Address) error {
	tokenTxs, err := api.ListTokenTransfers(address, 0)
	if err != nil {
		return err
	}
	countTokenTxs := uint64(len(tokenTxs.Received) + len(tokenTxs.Sent))

	count, err := api.CountTokenTransfers(address)
	if err != nil {
		return err
	}
	if count != countTokenTxs {
		return fmt.Errorf("expected %s to match transfers count %d and %d", address, count, countTokenTxs)
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

		if _, err = api.AccountBootstrap(faucetPkg, nil, nil); err != nil {
			if strings.Contains(err.Error(), "tx already exists in cache") {
				// don't worry then, someone else created it in a race, nevermind, job done.
			} else {
				return nil, fmt.Errorf("cannot create account %s: %w", api.MyAddress(), err)
			}
		}

		_ = api.WaitUntilNextBlock()
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

		if _, err := api.AccountSetMetadata(metadata); err != nil {
			if strings.Contains(err.Error(), "tx already exists in cache") {
				// don't worry then, someone else created it in a race, or still pending, nevermind
			} else if strings.Contains(err.Error(), "invalid URI, must be different") {
				// the tx from previous loop JUST got mined, also nevermind
			} else {
				return nil, fmt.Errorf("cannot set account %s metadata: %w", api.MyAddress(), err)
			}
		}

		_ = api.WaitUntilNextBlock()
	}
	return nil, fmt.Errorf("cannot set account %s metadata after %d retries", api.MyAddress(), retries)
}

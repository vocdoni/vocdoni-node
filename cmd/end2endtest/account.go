package main

import (
	"context"
	"encoding/hex"
	"fmt"
	"net/url"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/google/uuid"
	"go.vocdoni.io/dvote/apiclient"
	"go.vocdoni.io/dvote/crypto/ethereum"
	"go.vocdoni.io/dvote/log"
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

	// check set transaction cost
	if err := testSetTxCost(api, treasurerSigner); err != nil {
		log.Fatal(err)
	}

}

func ensureTxCostEquals(api *apiclient.HTTPclient, signer *ethereum.SignKeys, txType models.TxType, cost uint32) error {
	for i := 0; i < retries; i++ {
		// if cost equals expected, we're done
		newTxCost, err := api.TransactionCost(models.TxType_SET_ACCOUNT_INFO_URI)
		if err != nil {
			log.Warn(err)
		}
		if newTxCost == cost {
			return nil
		}

		// else, try to set
		treasurerAccount, err := api.Treasurer()
		if err != nil {
			return fmt.Errorf("cannot get treasurer: %w", err)
		}

		log.Infof("will set %s txcost=%d (treasurer nonce: %d)", txType, cost, treasurerAccount.Nonce)
		txhash, err := api.SetTransactionCost(
			signer,
			txType,
			cost,
			treasurerAccount.Nonce)
		if err != nil {
			log.Warn(err)
			time.Sleep(time.Second)
			continue
		}
		ctx, cancel := context.WithTimeout(context.Background(), time.Second*10)
		defer cancel()
		_, err = api.WaitUntilTxIsMined(ctx, txhash)
		if err != nil {
			log.Warn(err)
		}
	}
	return fmt.Errorf("tried to set %s txcost=%d but failed %d times", txType, cost, retries)
}

func testSetTxCost(api *apiclient.HTTPclient, treasurerSigner *ethereum.SignKeys) error {
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

	// 	// check tx cost changes
	// 	err = c.ensureTxCostEquals(treasurerSigner, models.TxType_SET_ACCOUNT_INFO_URI, 1000)
	// 	if err != nil {
	// 		return err
	// 	}
	// 	log.Infof("tx cost of %s changed successfully from %d to %d", models.TxType_SET_ACCOUNT_INFO_URI, txCost, 1000)

	// 	// get new treasurer nonce
	// 	treasurer2, err := c.Treasurer()
	// 	if err != nil {
	// 		return err
	// 	}
	// 	log.Infof("treasurer nonce changed successfully from %d to %d", treasurer.Nonce, treasurer2.Nonce)

	// 	// set tx cost back to default
	// 	err = c.ensureTxCostEquals(treasurerSigner, models.TxType_SET_ACCOUNT_INFO_URI, 10)
	// 	if err != nil {
	// 		return fmt.Errorf("cannot set transaction cost: %v", err)
	// 	}

	return nil
}


package main

import (
	"encoding/json"
	"io/ioutil"
	"net/http"
	"net/url"
	"time"

	"go.vocdoni.io/dvote/apiclient"
	"go.vocdoni.io/dvote/crypto/ethereum"
	"go.vocdoni.io/dvote/log"
	"go.vocdoni.io/dvote/types"
	"go.vocdoni.io/proto/build/go/models"
	"google.golang.org/protobuf/proto"
)

func ensureTxIsMined(api *apiclient.HTTPclient, txHash types.HexBytes) {
	for startTime := time.Now(); time.Since(startTime) < 30*time.Second; {
		_, err := api.TransactionReference(txHash)
		if err == nil {
			return
		}
		time.Sleep(2 * time.Second)
	}
	log.Fatalf("tx %s not mined", txHash.String())
}

func getFaucetPkg(accountAddress string) ([]byte, error) {
	var c http.Client
	u, err := url.Parse(faucetURL + accountAddress)
	if err != nil {
		return nil, err
	}

	resp, err := c.Do(&http.Request{
		Method: "GET",
		URL:    u,
		Header: http.Header{
			"Authorization": []string{"Bearer " + faucetToken},
			"User-Agent":    []string{"Vocdoni API client / 1.0"},
		},
	})
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	data, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	log.Debugf("received response from faucet: %s", data)
	type faucetResp struct {
		Amount    uint32         `json:"amount"`
		Payload   []byte         `json:"faucetPayload"`
		Signature types.HexBytes `json:"signature"`
	}

	var fResp faucetResp
	json.Unmarshal(data, &fResp)
	if err != nil {
		return nil, err
	}
	return proto.Marshal(&models.FaucetPackage{
		Payload:   fResp.Payload,
		Signature: fResp.Signature})
}

func generateAccounts(number int) ([]*ethereum.SignKeys, error) {
	accounts := make([]*ethereum.SignKeys, number)
	for i := 0; i < number; i++ {
		accounts[i] = &ethereum.SignKeys{}
		if err := accounts[i].Generate(); err != nil {
			return nil, err
		}
	}
	return accounts, nil
}

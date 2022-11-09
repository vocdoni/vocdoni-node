package main

import (
	"encoding/json"
	"io/ioutil"
	"net/http"
	"net/url"
	"time"

	flag "github.com/spf13/pflag"

	"github.com/google/uuid"
	"go.vocdoni.io/dvote/apiclient"
	"go.vocdoni.io/dvote/crypto/ethereum"
	"go.vocdoni.io/dvote/log"
	"go.vocdoni.io/dvote/types"
	"go.vocdoni.io/dvote/util"
	"go.vocdoni.io/proto/build/go/models"
	"google.golang.org/protobuf/proto"
)

const (
	faucetURL   = "https://faucet-azeno.vocdoni.net/faucet/vocdoni/dev/"
	faucetToken = "158a58ba-bd3e-479e-b230-2814a34fae8f"
)

func main() {
	host := flag.String("host", "https://api-dev.vocdoni.net/v2", "API host to connect to")
	logLevel := flag.String("logLevel", "info", "log level")
	flag.Parse()
	log.Init(*logLevel, "stdout")

	// Connect to the API host
	hostURL, err := url.Parse(*host)
	if err != nil {
		log.Fatal(err)
	}
	log.Debugf("connecting to %s", hostURL.String())

	token := uuid.New()
	api, err := apiclient.NewHTTPclient(hostURL, &token)
	if err != nil {
		log.Fatal(err)
	}

	// Generate the organization account
	account := util.RandomHex(32)
	log.Infof("new account generated, private key is %s", account)
	if err := api.SetAccount(account); err != nil {
		log.Fatal(err)
	}

	// Get the faucet package of bootstrap tokens
	log.Infof("getting faucet package")
	faucetPkg, err := getFaucetPkg(api.MyAddress().Hex())
	if err != nil {
		log.Fatal(err)
	}

	// Create the organization account and bootstraping with the faucet package
	log.Infof("creating Vocdoni account %s", api.MyAddress().Hex())
	hash, err := api.AccountBootstrap(faucetPkg)
	if err != nil {
		log.Fatal(err)
	}
	ensureTxIsMined(api, hash)

	acc, err := api.Account("")
	if err != nil {
		log.Fatal(err)
	}
	if acc.Balance == 0 {
		log.Fatal("account balance is 0")
	}
	log.Infof("account %s balance is %d", api.MyAddress().Hex(), acc.Balance)

	// Create a new census
	censusID, err := api.NewCensus("weighted")
	if err != nil {
		log.Fatal(err)
	}
	log.Infof("new census created with id %s", censusID)

	// Genreate 10 participant accounts
	voterAccounts, err := generateAccounts(10)
	if err != nil {
		log.Fatal(err)
	}

	// Add the accounts to the census
	for _, voterAccount := range voterAccounts {
		log.Infof("adding voter %s", voterAccount.Address().Hex())
		if err := api.CensusAddVoter(censusID, voterAccount.Address().Bytes(), 10); err != nil {
			log.Fatal(err)
		}
	}

	// Publish the census
	root, err := api.CensusPublish(censusID)
	if err != nil {
		log.Fatal(err)
	}
	log.Infof("census published with root %s", root.String())

	// Generate the voting proofs
	proofs := []*apiclient.CensusProof{}
	for i, voterAccount := range voterAccounts {
		log.Infof("generating voting proof %d ", i)
		pr, err := api.CensusGenProof(root, voterAccount.Address().Bytes())
		if err != nil {
			log.Fatal(err)
		}
		proofs = append(proofs, pr)
	}
	log.Debugf(" %d voting proofs generated successfully", len(proofs))
}

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

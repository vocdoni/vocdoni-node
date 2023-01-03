package main

import (
	"flag"
	"math/big"
	"net/url"
	"sync"
	"testing"
	"time"

	qt "github.com/frankban/quicktest"
	"github.com/google/uuid"
	vapi "go.vocdoni.io/dvote/api"
	"go.vocdoni.io/dvote/apiclient"
	"go.vocdoni.io/dvote/crypto/ethereum"
	"go.vocdoni.io/dvote/log"
	"go.vocdoni.io/dvote/types"
	"go.vocdoni.io/dvote/util"
	"go.vocdoni.io/proto/build/go/models"
)

func TestAnonVoting(t *testing.T) {
	host := flag.String("host", "https://api-dev.vocdoni.net/v2", "API host to connect to")
	logLevel := flag.String("logLevel", "info", "log level (debug, info, warn, error, fatal)")
	accountPrivKey := flag.String("accountPrivKey", "0xf7FB77ee1F309D9468fB6DCB71aDD0f934a33c6B", "account private key (optional)")
	nvotes := flag.Int("votes", 10, "number of votes to cast")
	parallelCount := flag.Int("parallel", 4, "number of parallel requests")
	useDevFaucet := flag.Bool("devFaucet", true, "use the dev faucet for fetching tokens")

	flag.Parse()
	log.Init(*logLevel, "stdout")

	c := qt.New(t)
	hostURL, err := url.Parse(*host)
	c.Assert(err, qt.IsNil)
	log.Debugf("connecting to %s", hostURL.String())

	token := uuid.New()
	api, err := apiclient.NewHTTPclient(hostURL, &token)
	c.Assert(err, qt.IsNil)

	// Check if account is defined
	account := *accountPrivKey
	if account == "" {
		// Generate the organization account
		account = util.RandomHex(32)
		log.Infof("new account generated, private key is %s", account)
	}

	// Set the account in the API client, so we can sign transactions
	err = api.SetAccount(account)
	c.Assert(err, qt.IsNil)

	// If the account does not exist, create a new one
	// TODO: check if the account balance is low and use the faucet
	acc, err := api.Account("")
	if err != nil {
		var faucetPkg *models.FaucetPackage
		if *useDevFaucet {
			// Get the faucet package of bootstrap tokens
			log.Infof("getting faucet package")
			faucetPkg, err = apiclient.GetFaucetPackageFromRemoteService(
				apiclient.DefaultDevelopmentFaucetURL+api.MyAddress().Hex(),
				apiclient.DefaultDevelopmentFaucetToken,
			)
			c.Assert(err, qt.IsNil)
		}
		// Create the organization account and bootstraping with the faucet package
		log.Infof("creating Vocdoni account %s", api.MyAddress().Hex())
		log.Debugf("faucetPackage is %x", faucetPkg)
		hash, err := api.AccountBootstrap(faucetPkg, &vapi.AccountMetadata{
			Name:        map[string]string{"default": "test account " + api.MyAddress().Hex()},
			Description: map[string]string{"default": "test description"},
			Version:     "1.0",
		})
		c.Assert(err, qt.IsNil)

		ensureTxIsMined(api, hash)
		acc, err = api.Account("")
		c.Assert(err, qt.IsNil)

		if *useDevFaucet && acc.Balance == 0 {
			log.Fatal("account balance is 0")
		}
	}

	log.Infof("account %s balance is %d", api.MyAddress().Hex(), acc.Balance)

	// Create a new census
	censusID, err := api.NewCensus(vapi.CensusTypeZKWeighted)
	c.Assert(err, qt.IsNil)
	log.Infof("new census created with id %s", censusID.String())

	// Create voters keys
	var voterAccounts []*ethereum.SignKeys
	for i := 0; i < *nvotes; i++ {
		key := ethereum.NewSignKeys()
		err := key.Generate()
		c.Assert(err, qt.IsNil)
		voterAccounts = append(voterAccounts, key)
	}

	// Add voters to the census
	participants := &vapi.CensusParticipants{}
	for i, voterAccount := range voterAccounts {
		participants.Participants = append(participants.Participants,
			vapi.CensusParticipant{
				Key:    voterAccount.PublicKey(),
				Weight: (*types.BigInt)(new(big.Int).SetUint64(10)),
			})
		if i == len(voterAccounts)-1 || ((i+1)%vapi.MaxCensusAddBatchSize == 0) {
			err := api.CensusAddParticipants(censusID, participants)
			c.Assert(err, qt.IsNil)

			log.Infof("added %d participants to census %s", len(participants.Participants), censusID.String())
			participants = &vapi.CensusParticipants{}
		}
	}

	size, err := api.CensusSize(censusID)
	c.Assert(err, qt.IsNil)
	c.Assert(size, qt.Equals, *nvotes)

	// Publish the census
	root, _, err := api.CensusPublish(censusID)
	// root, censusURI, err := api.CensusPublish(censusID)
	if err != nil {
		log.Fatal(err)
	}
	log.Infof("census published with root %s", root.String())

	// Check census size (of the published census)
	size, err = api.CensusSize(root)
	if err != nil {
		log.Fatal(err)
	}
	if size != uint64(*nvotes) {
		log.Fatalf("published census size is %d, expected %d", size, *nvotes)
	}

	// Generate the voting proofs (parallelized)
	type voterProof struct {
		proof   *apiclient.CensusProof
		address string
	}
	proofs := make(map[string]*apiclient.CensusProof, *nvotes)
	proofCh := make(chan *voterProof)
	stopProofs := make(chan bool)
	go func() {
		for {
			select {
			case p := <-proofCh:
				proofs[p.address] = p.proof
			case <-stopProofs:
				return
			}
		}
	}()

	addNaccounts := func(accounts []*ethereum.SignKeys, wg *sync.WaitGroup) {
		defer wg.Done()
		log.Infof("generating %d voting proofs", len(accounts))
		for _, acc := range accounts {
			pr, err := api.CensusGenProof(root, acc.Address().Bytes())
			c.Assert(err, qt.IsNil)
			pr.KeyType = models.ProofArbo_ADDRESS
			proofCh <- &voterProof{
				proof:   pr,
				address: acc.Address().Hex(),
			}
		}
	}

	pcount := *nvotes / *parallelCount
	var wg sync.WaitGroup
	for i := 0; i < len(voterAccounts); i += pcount {
		end := i + pcount
		if end > len(voterAccounts) {
			end = len(voterAccounts)
		}
		wg.Add(1)
		go addNaccounts(voterAccounts[i:end], &wg)
	}

	wg.Wait()
	time.Sleep(time.Second) // wait a grace time for the last proof to be added
	log.Debugf("%d/%d voting proofs generated successfully", len(proofs), len(voterAccounts))
	stopProofs <- true
}

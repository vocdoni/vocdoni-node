package main

import (
	"context"
	"errors"
	"fmt"
	"math/big"
	"net/url"
	"os"
	"strings"
	"sync"
	"time"

	flag "github.com/spf13/pflag"
	vapi "go.vocdoni.io/dvote/api"
	"go.vocdoni.io/dvote/crypto/ethereum"
	"go.vocdoni.io/dvote/types"
	"go.vocdoni.io/proto/build/go/models"

	"github.com/google/uuid"
	"go.vocdoni.io/dvote/apiclient"
	"go.vocdoni.io/dvote/log"
	"go.vocdoni.io/dvote/util"
)

const (
	faucetURL   = "https://faucet-azeno.vocdoni.net/faucet/vocdoni/dev/"
	faucetToken = "158a58ba-bd3e-479e-b230-2814a34fae8f"
)

func main() {
	host := flag.String("host", "https://api-dev.vocdoni.net/v2", "API host to connect to")
	logLevel := flag.String("logLevel", "info", "log level")
	accountPrivKey := flag.String("accountPrivKey", "", "account private key (optional)")
	nvotes := flag.Int("votes", 10, "number of votes to cast")
	paralelCount := flag.Int("paralel", 4, "number of paralel requests")
	useDevFaucet := flag.Bool("devFaucet", true, "use the dev faucet for fetching tokens")
	timeout := flag.Int("timeout", 5, "timeout in minutes")
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

	// Check if account is defined
	account := *accountPrivKey
	if account == "" {
		// Generate the organization account
		account = util.RandomHex(32)
		log.Infof("new account generated, private key is %s", account)
	}

	// Set the account in the API client, so we can sign transactions
	if err := api.SetAccount(account); err != nil {
		log.Fatal(err)
	}

	// If the account does not exist, create a new one
	// TODO: check if the account balance is low and use the faucet
	acc, err := api.Account("")
	if err != nil {
		var faucetPkg []byte
		if *useDevFaucet {
			// Get the faucet package of bootstrap tokens
			log.Infof("getting faucet package")
			faucetPkg, err = getFaucetPkg(api.MyAddress().Hex())
			if err != nil {
				log.Fatal(err)
			}
		}
		// Create the organization account and bootstraping with the faucet package
		log.Infof("creating Vocdoni account %s", api.MyAddress().Hex())
		log.Debugf("faucetPackage is %x", faucetPkg)
		hash, err := api.AccountBootstrap(faucetPkg)
		if err != nil {
			log.Fatal(err)
		}
		ensureTxIsMined(api, hash)
		acc, err = api.Account("")
		if err != nil {
			log.Fatal(err)
		}
		if *useDevFaucet && acc.Balance == 0 {
			log.Fatal("account balance is 0")
		}
	}

	log.Infof("account %s balance is %d", api.MyAddress().Hex(), acc.Balance)

	// Create a new census
	censusID, err := api.NewCensus("weighted")
	if err != nil {
		log.Fatal(err)
	}
	log.Infof("new census created with id %s", censusID.String())

	// Genreate 10 participant accounts
	voterAccounts, err := generateAccounts(*nvotes)
	if err != nil {
		log.Fatal(err)
	}

	// Add the accounts to the census by batches
	participants := &vapi.CensusParticipants{}
	for i, voterAccount := range voterAccounts {
		participants.Participants = append(participants.Participants,
			vapi.CensusParticipant{
				Key:    voterAccount.Address().Bytes(),
				Weight: (*types.BigInt)(new(big.Int).SetUint64(10)),
			})
		if i == len(voterAccounts)-1 || ((i+1)%vapi.MaxCensusAddBatchSize == 0) {
			if err := api.CensusAddParticipants(censusID, participants); err != nil {
				log.Fatal(err)
			}
			log.Infof("added %d participants to census %s", len(participants.Participants), censusID.String())
			participants = &vapi.CensusParticipants{}
		}
	}

	// Check census size
	size, err := api.CensusSize(censusID)
	if err != nil {
		log.Fatal(err)
	}
	if size != uint64(*nvotes) {
		log.Fatalf("census size is %d, expected %d", size, *nvotes)
	}
	log.Infof("census %s size is %d", censusID.String(), size)

	// Publish the census
	root, censusURI, err := api.CensusPublish(censusID)
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

	// Generate the voting proofs (paralelized)
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
			if err != nil {
				log.Fatal(err)
			}
			pr.KeyType = models.ProofArbo_ADDRESS
			proofCh <- &voterProof{
				proof:   pr,
				address: acc.Address().Hex(),
			}
		}
	}

	pcount := *nvotes / *paralelCount
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

	// Create a new Election
	electionID, err := api.NewElection(&vapi.ElectionDescription{
		Title:       map[string]string{"default": fmt.Sprintf("Test election %s", util.RandomHex(8))},
		Description: map[string]string{"default": "Test election description"},
		EndDate:     time.Now().Add(time.Minute * 20),

		VoteType: vapi.VoteType{
			UniqueChoices:     false,
			MaxVoteOverwrites: 1,
		},

		ElectionType: vapi.ElectionType{
			Autostart:         true,
			Interruptible:     true,
			Anonymous:         false,
			SecretUntilTheEnd: false,
			DynamicCensus:     false,
		},

		Census: vapi.CensusTypeDescription{
			RootHash: root,
			URL:      censusURI,
			Type:     "weighted",
		},

		Questions: []vapi.Question{
			{
				Title:       map[string]string{"default": "Test question 1"},
				Description: map[string]string{"default": "Test question 1 description"},
				Choices: []vapi.ChoiceMetadata{
					{
						Title: map[string]string{"default": "Yes"},
						Value: 0,
					},
					{
						Title: map[string]string{"default": "No"},
						Value: 1,
					},
				},
			},
		},
	})
	if err != nil {
		log.Fatal(err)
	}

	election := ensureElectionCreated(api, electionID)
	log.Infof("created new election with id %s", electionID.String())
	log.Debugf("election details: %+v", *election)

	// Wait for the election to start
	waitUntilElectionStarts(api, electionID)

	// Send the votes (paralelized)
	startTime := time.Now()
	wg = sync.WaitGroup{}
	voteAccounts := func(accounts []*ethereum.SignKeys, wg *sync.WaitGroup) {
		defer wg.Done()
		log.Infof("sending %d votes", len(accounts))
		// We use maps instead of slices to have the capacity of resending votes
		// without repeating them.
		accountsMap := make(map[int]*ethereum.SignKeys, len(accounts))
		for i, acc := range accounts {
			accountsMap[i] = acc
		}
		// Send the votes
		votesSent := 0
		for {
			contextDeadlines := 0
			for i, voterAccount := range accountsMap {
				c := api.Clone(fmt.Sprintf("%x", voterAccount.PrivateKey()))
				_, err := c.Vote(&apiclient.VoteData{
					ElectionID:  electionID,
					ProofMkTree: proofs[voterAccount.Address().Hex()],
					Choices:     []int{i % 2},
				})
				// if the context deadline is reached, we don't need to print it (let's jus retry)
				if err != nil && errors.Is(err, context.DeadlineExceeded) || os.IsTimeout(err) {
					contextDeadlines++
					continue
				} else if err != nil && !strings.Contains(err.Error(), "already exists") {
					// if the error is not "vote already exists", we need to print it
					log.Warn(err)
					continue
				}
				// if the vote was sent successfully or already exists, we remove it from the accounts map
				votesSent++
				delete(accountsMap, i)

			}
			if len(accountsMap) == 0 {
				break
			}
			log.Infof("sent %d/%d votes... got %d HTTP errors", votesSent, len(accounts), contextDeadlines)
			time.Sleep(time.Second * 5)
		}
		log.Infof("successfully sent %d votes", votesSent)
		time.Sleep(time.Second * 2)
	}

	pcount = *nvotes / *paralelCount
	for i := 0; i < len(voterAccounts); i += pcount {
		end := i + pcount
		if end > len(voterAccounts) {
			end = len(voterAccounts)
		}
		wg.Add(1)
		go voteAccounts(voterAccounts[i:end], &wg)
	}

	wg.Wait()
	log.Infof("%d votes submitted successfully, took %s. At %d votes/second",
		*nvotes, time.Since(startTime), int(float64(*nvotes)/time.Since(startTime).Seconds()))

	// Wait for all the votes to be verified
	log.Infof("waiting for all the votes to be registered...")
	for {
		count, err := api.ElectionVoteCount(electionID)
		if err != nil {
			log.Warn(err)
		}
		if count == uint32(*nvotes) {
			break
		}
		time.Sleep(time.Second * 5)
		log.Infof("verified %d/%d votes", count, *nvotes)
		if time.Since(startTime) > time.Duration(*timeout)*time.Minute {
			log.Fatalf("timeout waiting for votes to be registered")
		}
	}

	log.Infof("%d votes registered successfully, took %s. At %d votes/second",
		*nvotes, time.Since(startTime), int(float64(*nvotes)/time.Since(startTime).Seconds()))

	// Set the account back to the organization account
	if err := api.SetAccount(account); err != nil {
		log.Fatal(err)
	}

	// End the election by seting the status to ENDED
	log.Infof("ending election...")
	hash, err := api.SetElectionStatus(electionID, "ENDED")
	if err != nil {
		log.Fatal(err)
	}

	// Check the election status is actually ENDED
	ensureTxIsMined(api, hash)
	election, err = api.Election(electionID)
	if err != nil {
		log.Fatal(err)
	}
	if election.Status != "ENDED" {
		log.Fatal("election status is not ENDED")
	}
	log.Infof("election %s status is ENDED", electionID.String())

	// Wait for the election to be in RESULTS state
	log.Infof("waiting for election to be in RESULTS state...")
	waitUntilElectionStatus(api, electionID, "RESULTS")
	log.Infof("election %s status is RESULTS", electionID.String())

	election, err = api.Election(electionID)
	if err != nil {
		log.Fatal(err)
	}
	log.Infof("election results: %v", election.Results)
}

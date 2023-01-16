package main

import (
	"encoding/hex"
	"flag"
	"fmt"
	"net/url"
	"time"

	"github.com/google/uuid"
	vapi "go.vocdoni.io/dvote/api"
	"go.vocdoni.io/dvote/apiclient"
	"go.vocdoni.io/dvote/crypto/ethereum"
	"go.vocdoni.io/dvote/log"
	"go.vocdoni.io/dvote/util"
	"go.vocdoni.io/proto/build/go/models"
)

func main() {
	host := flag.String("host", "https://api-dev.vocdoni.net/v2", "API host to connect to")
	logLevel := flag.String("logLevel", "info", "log level (debug, info, warn, error, fatal)")
	accountPrivKey := flag.String("accountPrivKey", "", "account private key (optional)")
	nvotes := flag.Int("votes", 10, "number of votes to cast")
	// parallelCount := flag.Int("parallel", 4, "number of parallel requests")
	useDevFaucet := flag.Bool("devFaucet", true, "use the dev faucet for fetching tokens")
	// timeout := flag.Int("timeout", 5, "timeout in minutes")
	// anonymous := flag.Bool("anonymous", false, "use for test anonymous weighted or only weighted voting")

	flag.Parse()
	log.Init(*logLevel, "stdout")

	hostURL, err := url.Parse(*host)
	if err != nil {
		log.Errorw(err, "error parsing the host URL")
		return
	}
	log.Debugf("connecting to %s", hostURL.String())

	token := uuid.New()
	api, err := apiclient.NewHTTPclient(hostURL, &token)
	if err != nil {
		log.Errorw(err, "error connecting to the host URL")
		return
	}

	// Check if account is defined
	account := *accountPrivKey
	if account == "" {
		// Generate the organization account
		key := ethereum.NewSignKeys()
		if err := key.AddHexKey(util.RandomHex(32)); err != nil {
			log.Errorw(err, "cannot create key")
		}

		account = hex.EncodeToString(key.PrivateKey())
		log.Infof("new account generated, private key is %s", account)
	}

	// Set the account in the API client, so we can sign transactions
	err = api.SetAccount(account)
	if err != nil {
		log.Errorw(err, "error setting up the account")
		return
	}

	// If the account does not exist, create a new one
	// TODO: check if the account balance is low and use the faucet
	acc, err := api.Account("")
	if err != nil {
		var faucetPkg *models.FaucetPackage
		if *useDevFaucet {
			// Get the faucet package of bootstrap tokens
			log.Infof("getting faucet package")
			faucetPkg, err = apiclient.GetFaucetPackageFromDevService(api.MyAddress().Hex())
			if err != nil {
				log.Errorw(err, "error setting up the faucet package")
				return
			}
		}
		// Create the organization account and bootstraping with the faucet package
		log.Infof("creating Vocdoni account %s", api.MyAddress().Hex())
		log.Debugf("faucetPackage is %x", faucetPkg)
		hash, err := api.AccountBootstrap(faucetPkg, &vapi.AccountMetadata{
			Name:        map[string]string{"default": "test account " + api.MyAddress().Hex()},
			Description: map[string]string{"default": "test description"},
			Version:     "1.0",
		})
		if err != nil {
			log.Errorw(err, "error setting up the account")
			return
		}

		ensureTxIsMined(api, hash)
		acc, err = api.Account("")
		if err != nil {
			log.Errorw(err, "error setting up the account")
			return
		}

		if *useDevFaucet && acc.Balance == 0 {
			log.Fatal("account balance is 0")
		}
	}

	log.Infof("account %s balance is %d", api.MyAddress().Hex(), acc.Balance)

	// Create a new census
	voterAccounts, err := generateAccounts(*nvotes)
	if err != nil {
		log.Errorw(err, "error generating the accounts to vote")
		return
	}

	_, censusRoot, censusURI, err := buildCensusZk(api, voterAccounts)
	if err != nil {
		log.Errorw(err, "error building the census")
		return
	}

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
			Anonymous:         true,
			SecretUntilTheEnd: false,
			DynamicCensus:     false,
		},

		Census: vapi.CensusTypeDescription{
			RootHash: censusRoot,
			URL:      censusURI,
			Type:     vapi.CensusTypeZKWeighted,
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
		log.Errorw(err, "error creating the election")
		return
	}

	election := ensureElectionCreated(api, electionID)
	log.Infof("created new election with id %s", electionID.String())
	log.Debugf("election details: %+v", *election)

	proofs := make(map[string]*apiclient.CensusProofZk, *nvotes)
	for _, acc := range voterAccounts {
		voterPrivKey, err := calcAnonPrivKey(acc)
		if err != nil {
			log.Errorw(err, "error calculating PublicKey")
			return
		}

		pr, err := api.CensusGenProofZk(censusRoot, electionID, voterPrivKey)
		if err != nil {
			log.Errorw(err, "error generating census proof")
			return
		}
		proofs[acc.Address().Hex()] = pr
	}

	time.Sleep(time.Second) // wait a grace time for the last proof to be added
	log.Debugf("%d/%d voting proofs generated successfully", len(proofs), len(voterAccounts))

	// Wait for the election to start
	waitUntilElectionStarts(api, electionID)

	// Send the votes (secuentially)
	for i, acc := range voterAccounts {
		_, err := api.Vote(&apiclient.VoteData{
			ElectionID:  electionID,
			ProofZkTree: proofs[acc.Address().Hex()],
			Choices:     []int{i % 2},
		})

		if err != nil {
			log.Warnw(err.Error(), map[string]interface{}{"election": electionID})
		}
	}

	// // Send the votes (parallelized)
	// startTime := time.Now()
	// wg := sync.WaitGroup{}
	// voteAccounts := func(accounts []*ethereum.SignKeys, wg *sync.WaitGroup) {
	// 	defer wg.Done()
	// 	log.Infof("sending %d votes", len(accounts))
	// 	// We use maps instead of slices to have the capacity of resending votes
	// 	// without repeating them.
	// 	accountsMap := make(map[int]*ethereum.SignKeys, len(accounts))
	// 	for i, acc := range accounts {
	// 		accountsMap[i] = acc
	// 	}
	// 	// Send the votes
	// 	votesSent := 0
	// 	for {
	// 		contextDeadlines := 0
	// 		for i, voterAccount := range accountsMap {
	// 			c := api.Clone(fmt.Sprintf("%x", voterAccount.PrivateKey()))
	// 			_, err := c.Vote(&apiclient.VoteData{
	// 				ElectionID:  electionID,
	// 				ProofZkTree: proofs[voterAccount.Address().Hex()],
	// 				Choices:     []int{i % 2},
	// 			})
	// 			// if the context deadline is reached, we don't need to print it (let's jus retry)
	// 			if err != nil && errors.Is(err, context.DeadlineExceeded) || os.IsTimeout(err) {
	// 				contextDeadlines++
	// 				continue
	// 			} else if err != nil && !strings.Contains(err.Error(), "already exists") {
	// 				// if the error is not "vote already exists", we need to print it
	// 				log.Warn(err)
	// 				continue
	// 			}
	// 			// if the vote was sent successfully or already exists, we remove it from the accounts map
	// 			votesSent++
	// 			delete(accountsMap, i)

	// 		}
	// 		if len(accountsMap) == 0 {
	// 			break
	// 		}
	// 		log.Infof("sent %d/%d votes... got %d HTTP errors", votesSent, len(accounts), contextDeadlines)
	// 		time.Sleep(time.Second * 5)
	// 	}
	// 	log.Infof("successfully sent %d votes", votesSent)
	// 	time.Sleep(time.Second * 2)
	// }

	// pcount := *nvotes / *parallelCount
	// for i := 0; i < len(voterAccounts); i += pcount {
	// 	end := i + pcount
	// 	if end > len(voterAccounts) {
	// 		end = len(voterAccounts)
	// 	}
	// 	wg.Add(1)
	// 	go voteAccounts(voterAccounts[i:end], &wg)
	// }

	// wg.Wait()
	// log.Infof("%d votes submitted successfully, took %s. At %d votes/second",
	// 	*nvotes, time.Since(startTime), int(float64(*nvotes)/time.Since(startTime).Seconds()))

	// Wait for all the votes to be verified
	// log.Infof("waiting for all the votes to be registered...")
	// for {
	// 	count, err := api.ElectionVoteCount(electionID)
	// 	if err != nil {
	// 		log.Errorw(err, "error verificating vote")
	// 	}
	// 	if count == uint32(*nvotes) {
	// 		break
	// 	}
	// 	time.Sleep(time.Second * 5)
	// 	log.Infof("verified %d/%d votes", count, *nvotes)
	// 	if time.Since(startTime) > time.Duration(*timeout)*time.Minute {
	// 		log.Fatalf("timeout waiting for votes to be registered")
	// 	}
	// }

	// log.Infof("%d votes registered successfully, took %s (%d votes/second)",
	// 	*nvotes, time.Since(startTime), int(float64(*nvotes)/time.Since(startTime).Seconds()))

	count, err := api.ElectionVoteCount(electionID)
	if err != nil {
		log.Errorw(err, "error verificating votes")
		return
	}
	if count == uint32(*nvotes) {
		log.Warn("error verificating vote")
		return
	}

	// Set the account back to the organization account
	err = api.SetAccount(account)
	if err != nil {
		log.Errorw(err, "error setting the account back to the organization account")
		return
	}

	// End the election by seting the status to ENDED
	log.Infof("ending election...")
	hash, err := api.SetElectionStatus(electionID, "ENDED")
	if err != nil {
		log.Errorw(err, "error ending the election")
		return
	}

	// Check the election status is actually ENDED
	ensureTxIsMined(api, hash)
	election, err = api.Election(electionID)
	if err != nil {
		log.Errorw(err, "error generating election")
		return
	} else if election.Status != "ENDED" {
		log.Fatal("the electing must be ended")
		return
	}
	log.Infof("election %s status is ENDED", electionID.String())

	// Wait for the election to be in RESULTS state
	log.Infof("waiting for election to be in RESULTS state...")
	waitUntilElectionStatus(api, electionID, "RESULTS")
	log.Infof("election %s status is RESULTS", electionID.String())

	election, err = api.Election(electionID)
	if err != nil {
		log.Errorw(err, "error getting the election results")
		return
	}
	log.Infof("election results: %v", election.Results)
}

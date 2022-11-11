package main

import (
	"fmt"
	"net/url"
	"time"

	flag "github.com/spf13/pflag"
	vapi "go.vocdoni.io/dvote/api"
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
		acc, err = api.Account("")
		if err != nil {
			log.Fatal(err)
		}
		if acc.Balance == 0 {
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

	// Add the accounts to the census
	for _, voterAccount := range voterAccounts {
		log.Infof("adding voter %s", voterAccount.Address().Hex())
		if err := api.CensusAddVoter(censusID, voterAccount.Address().Bytes(), 10); err != nil {
			log.Fatal(err)
		}
	}

	// Publish the census
	root, censusURI, err := api.CensusPublish(censusID)
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

	// Send the votes
	voteIDs := []types.HexBytes{}
	for i, voterAccount := range voterAccounts {
		log.Infof("voting %d", i)
		if err := api.SetAccount(fmt.Sprintf("%x", voterAccount.PrivateKey())); err != nil {
			log.Fatal(err)
		}
		vid, err := api.Vote(&apiclient.VoteData{
			ElectionID: electionID,
			ProofTree:  proofs[i],
			Choices:    []int{i % 2},
			KeyType:    models.ProofArbo_ADDRESS,
		})
		if err != nil {
			log.Fatal(err)
		}
		voteIDs = append(voteIDs, vid)
		log.Debugf("vote %d submit! with id %s", i, vid.String())
	}
	log.Debugf("%d votes submitted successfully", len(voteIDs))

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

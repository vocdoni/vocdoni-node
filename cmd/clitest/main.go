package main

import (
	"fmt"
	"net/url"
	"time"

	flag "github.com/spf13/pflag"
	vapi "go.vocdoni.io/dvote/api"

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

	// Create a new process
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
			SecretUntilTheEnd: true,
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
						Value: 0,
					},
				},
			},
		},
	})
	if err != nil {
		log.Fatal(err)
	}

	log.Infof("created new election with id %s", electionID.String())
	// next => check election ID exists
}

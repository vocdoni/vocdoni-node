package main

import (
	"context"
	"encoding/hex"
	"errors"
	"fmt"
	"math/big"
	"os"
	"strings"
	"sync"
	"time"

	vapi "go.vocdoni.io/dvote/api"
	"go.vocdoni.io/dvote/apiclient"
	"go.vocdoni.io/dvote/crypto/ethereum"
	"go.vocdoni.io/dvote/log"
	"go.vocdoni.io/dvote/types"
	"go.vocdoni.io/dvote/util"
	"go.vocdoni.io/proto/build/go/models"
)

func init() {
	ops["plaintextelection"] = operation{
		test:        &E2EPlaintextElection{},
		description: "Publishes a census and a non-anonymous, non-secret election, emits N votes and verifies the results",
		example:     os.Args[0] + " --operation=plaintextelection --votes=1000",
	}
}

var _ VochainTest = (*E2EPlaintextElection)(nil)

type E2EPlaintextElection struct {
	api    *apiclient.HTTPclient
	config *config
}

func (t *E2EPlaintextElection) Setup(api *apiclient.HTTPclient, config *config) error {
	t.api = api
	t.config = config
	return nil
}

func (t *E2EPlaintextElection) Teardown() error {
	// nothing to do here
	return nil
}

func (t *E2EPlaintextElection) Run() (duration time.Duration, err error) {
	start := time.Now()

	c := t.config
	api := t.api

	// Set the account in the API client, so we can sign transactions
	if err := api.SetAccount(hex.EncodeToString(c.accountKeys[0].PrivateKey())); err != nil {
		log.Fatal(err)
	}

	// If the account does not exist, create a new one
	// TODO: check if the account balance is low and use the faucet
	acc, err := api.Account("")
	if err != nil {
		log.Infof("getting faucet package")
		faucetPkg, err := getFaucetPackage(c, api.MyAddress().Hex())
		if err != nil {
			log.Fatal(err)
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
			log.Fatal(err)
		}
		ctx, cancel := context.WithTimeout(context.Background(), time.Second*40)
		defer cancel()
		if _, err := api.WaitUntilTxIsMined(ctx, hash); err != nil {
			log.Fatalf("gave up waiting for tx %x to be mined: %s", hash, err)
		}

		acc, err = api.Account("")
		if err != nil {
			log.Fatal(err)
		}
		if c.faucet != "" && acc.Balance == 0 {
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

	// Generate 10 participant accounts
	voterAccounts := ethereum.NewSignKeysBatch(c.nvotes)

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
			log.Infof("added %d participants to census %s",
				len(participants.Participants), censusID.String())
			participants = &vapi.CensusParticipants{}
		}
	}

	// Check census size
	size, err := api.CensusSize(censusID)
	if err != nil {
		log.Fatal(err)
	}
	if size != uint64(c.nvotes) {
		log.Fatalf("census size is %d, expected %d", size, c.nvotes)
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
	if size != uint64(c.nvotes) {
		log.Fatalf("published census size is %d, expected %d", size, c.nvotes)
	}

	// Generate the voting proofs (parallelized)
	type voterProof struct {
		proof   *apiclient.CensusProof
		address string
	}
	proofs := make(map[string]*apiclient.CensusProof, c.nvotes)
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

	pcount := c.nvotes / c.parallelCount
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
			Size:     uint64(len(voterAccounts)),
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
	log.Infof("created new election with id %s - now wait until it starts", electionID.String())

	// Wait for the election to start
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*40)
	defer cancel()
	election, err := api.WaitUntilElectionStarts(ctx, electionID)
	if err != nil {
		log.Fatal(err)
	}

	log.Debugf("election details: %+v", *election)

	// Send the votes (parallelized)
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

	pcount = c.nvotes / c.parallelCount
	for i := 0; i < len(voterAccounts); i += pcount {
		end := i + pcount
		if end > len(voterAccounts) {
			end = len(voterAccounts)
		}
		wg.Add(1)
		go voteAccounts(voterAccounts[i:end], &wg)
	}

	wg.Wait()
	log.Infof("%d votes submitted successfully, took %s (%d votes/second)",
		c.nvotes, time.Since(startTime), int(float64(c.nvotes)/time.Since(startTime).Seconds()))

	// Wait for all the votes to be verified
	log.Infof("waiting for all the votes to be registered...")
	for {
		count, err := api.ElectionVoteCount(electionID)
		if err != nil {
			log.Warn(err)
		}
		if count == uint32(c.nvotes) {
			break
		}
		time.Sleep(time.Second * 5)
		log.Infof("verified %d/%d votes", count, c.nvotes)
		if time.Since(startTime) > c.timeout {
			log.Fatalf("timeout waiting for votes to be registered")
		}
	}

	log.Infof("%d votes registered successfully, took %s (%d votes/second)",
		c.nvotes, time.Since(startTime), int(float64(c.nvotes)/time.Since(startTime).Seconds()))

	// Set the account back to the organization account
	if err := api.SetAccount(hex.EncodeToString(c.accountKeys[0].PrivateKey())); err != nil {
		log.Fatal(err)
	}

	// End the election by setting the status to ENDED
	log.Infof("ending election...")
	hash, err := api.SetElectionStatus(electionID, "ENDED")
	if err != nil {
		log.Fatal(err)
	}

	// Check the election status is actually ENDED
	ctx, cancel = context.WithTimeout(context.Background(), time.Second*40)
	defer cancel()
	if _, err := api.WaitUntilTxIsMined(ctx, hash); err != nil {
		log.Fatalf("gave up waiting for tx %s to be mined: %s", hash, err)
	}

	election, err = api.Election(electionID)
	if err != nil {
		log.Fatal(err)
	}
	if election.Status != "ENDED" {
		log.Fatal("election status is not ENDED")
	}
	log.Infof("election %s status is ENDED", electionID.String())

	// Wait for the election to be in RESULTS state
	ctx, cancel = context.WithTimeout(context.Background(), time.Second*300)
	defer cancel()
	election, err = api.WaitUntilElectionStatus(ctx, electionID, "RESULTS")
	if err != nil {
		log.Fatal(err)
	}
	log.Infof("election %s status is RESULTS", electionID.String())
	log.Infof("election results: %v", election.Results)

	return time.Since(start), nil
}
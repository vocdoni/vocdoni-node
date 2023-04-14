package main

import (
	"context"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"strings"
	"sync"
	"time"

	vapi "go.vocdoni.io/dvote/api"
	"go.vocdoni.io/dvote/apiclient"
	"go.vocdoni.io/dvote/crypto/ethereum"
	"go.vocdoni.io/dvote/log"
)

func init() {
	ops["encryptedelection"] = operation{
		test:        &E2EEncryptedElection{},
		description: "Publishes a census and a non-anonymous, secret-until-the-end election, emits N votes and verifies the results",
		example:     os.Args[0] + " --operation=encryptedelection --votes=1000",
	}
}

var _ VochainTest = (*E2EEncryptedElection)(nil)

type E2EEncryptedElection struct {
	electionBase
}

func (t *E2EEncryptedElection) Setup(api *apiclient.HTTPclient, c *config) error {
	t.api = api
	t.config = c

	// Set the account in the API client, so we can sign transactions
	if err := api.SetAccount(hex.EncodeToString(c.accountKeys[0].PrivateKey())); err != nil {
		log.Fatal(err)
	}

	// If the account does not exist, create a new one
	// TODO: check if the account balance is low and use the faucet
	acc, err := t.api.Account("")
	if err != nil {
		acc, err = t.createAccount(api.MyAddress().Hex())
		if err != nil {
			log.Fatal(err)
		}
	}

	log.Infof("account %s balance is %d", api.MyAddress().Hex(), acc.Balance)

	// Create a new census
	censusID, err := t.api.NewCensus(vapi.CensusTypeWeighted)
	if err != nil {
		log.Fatal(err)
	}
	log.Infof("new census created with id %s", censusID.String())

	// Generate 10 participant accounts
	t.voterAccounts = ethereum.NewSignKeysBatch(c.nvotes)

	// Add the accounts to the census by batches
	if err := t.addParticipantsCensus(vapi.CensusTypeWeighted, censusID); err != nil {
		log.Fatal(err)
	}

	// Check census size
	if err := t.isCensusSizeValid(censusID); err != nil {
		log.Fatal(err)
	}

	// Publish the census
	root, censusURI, err := t.api.CensusPublish(censusID)
	if err != nil {
		log.Fatal(err)
	}
	log.Infof("census published with root %s", root.String())

	// Check census size (of the published census)
	if err := t.isCensusSizeValid(root); err != nil {
		log.Fatal(err)
	}

	// Create a new Election
	description := newDefaultElectionDescription(root, censusURI, uint64(t.config.nvotes))
	_, err = t.createElection(description)
	if err != nil {
		log.Fatal(err)
	}

	//log.Debugf("created new election with id %s", electionID.String())

	t.proofs = t.generateProofs(root)

	return nil
}

func (t *E2EEncryptedElection) Teardown() error {
	// nothing to do here
	return nil
}

func (t *E2EEncryptedElection) Run() (duration time.Duration, err error) {
	c := t.config
	api := t.api

	// Send the votes (parallelized)
	startTime := time.Now()
	wg := sync.WaitGroup{}
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
					ElectionID:  t.election.ElectionID,
					ProofMkTree: t.proofs[voterAccount.Address().Hex()],
					Choices:     []int{i % 2},
				})
				// if the context deadline is reached, we don't need to print it (let's just retry)
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

	pcount := c.nvotes / c.parallelCount
	for i := 0; i < len(t.voterAccounts); i += pcount {
		end := i + pcount
		if end > len(t.voterAccounts) {
			end = len(t.voterAccounts)
		}
		wg.Add(1)
		go voteAccounts(t.voterAccounts[i:end], &wg)
	}

	wg.Wait()
	log.Infof("%d votes submitted successfully, took %s (%d votes/second)",
		c.nvotes, time.Since(startTime), int(float64(c.nvotes)/time.Since(startTime).Seconds()))

	// Wait for all the votes to be verified
	log.Infof("waiting for all the votes to be registered...")
	for {
		count, err := api.ElectionVoteCount(t.election.ElectionID)
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
	_, err = api.SetElectionStatus(t.election.ElectionID, "ENDED")
	if err != nil {
		log.Fatal(err)
	}

	// Wait for the election to be in RESULTS state
	ctx, cancel := context.WithTimeout(context.Background(), apiclient.WaitTimeout)
	defer cancel()
	election, err := api.WaitUntilElectionStatus(ctx, t.election.ElectionID, "RESULTS")
	if err != nil {
		log.Fatal(err)
	}
	log.Infof("election %s status is RESULTS", t.election.ElectionID.String())
	log.Infof("election results: %v", election.Results)

	return time.Since(startTime), nil
}

package main

import (
	"context"
	"fmt"
	"math/big"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	vapi "go.vocdoni.io/dvote/api"
	"go.vocdoni.io/dvote/apiclient"
	"go.vocdoni.io/dvote/crypto/ethereum"
	"go.vocdoni.io/dvote/crypto/zk"
	"go.vocdoni.io/dvote/log"
	"go.vocdoni.io/dvote/types"
	"go.vocdoni.io/dvote/util"
	"go.vocdoni.io/proto/build/go/models"
)

func mkTreeAnonVoteTest(c config) {
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
	if err := api.SetAccount(c.accountPrivKeys[0]); err != nil {
		log.Fatal(err)
	}

	// If the account does not exist, create a new one
	// TODO: check if the account balance is low and use the faucet
	acc, err := api.Account("")
	if err != nil {
		var faucetPkg *models.FaucetPackage
		if c.faucet != "" {
			// Get the faucet package of bootstrap tokens
			log.Infof("getting faucet package")
			if c.faucet == "dev" {
				faucetPkg, err = apiclient.GetFaucetPackageFromDevService(api.MyAddress().Hex())
			} else {
				faucetPkg, err = apiclient.GetFaucetPackageFromRemoteService(c.faucet+api.MyAddress().Hex(), c.faucetAuthToken)
			}

			if err != nil {
				log.Fatal(err)
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
	censusID, err := api.NewCensus(vapi.CensusTypeZKWeighted)
	if err != nil {
		log.Fatal(err)
	}
	log.Infof("new census created with id %s", censusID.String())

	// Generate n participant accounts
	voterAccounts := util.CreateEthRandomKeysBatch(c.nvotes)

	// Add the accounts to the census by batches
	participants := &vapi.CensusParticipants{}
	for i, voterAccount := range voterAccounts {
		zkAddr, err := zk.AddressFromSignKeys(voterAccount)
		if err != nil {
			log.Fatal(err)
		}

		participants.Participants = append(participants.Participants,
			vapi.CensusParticipant{
				Key:    zkAddr.Bytes(),
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
			RootHash: root,
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
		log.Fatal(err)
	}

	// Wait for the election creation
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*40)
	defer cancel()
	election, err := api.WaitUntilElectionCreated(ctx, electionID)
	if err != nil {
		log.Errorw(err, "error creating the election")
		return
	}
	log.Debugf("election details: %+v", *election)
	log.Infof("created new election with id %s - now wait until it starts", electionID.String())

	// Wait for the election to start
	ctx, cancel = context.WithTimeout(context.Background(), time.Second*40)
	defer cancel()
	election, err = api.WaitUntilElectionStarts(ctx, electionID)
	if err != nil {
		log.Fatal(err)
	}

	log.Debugf("election details: %+v", *election)

	startTime := time.Now()
	successProofs, successVotes := 0, 0
	proofCh, voteCh, stopCh := make(chan bool), make(chan bool), make(chan bool)
	go func() {
		for {
			select {
			case <-proofCh:
				successProofs++
			case <-voteCh:
				successVotes++
			case <-stopCh:
				return
			}
		}
	}()

	apiClientMtx := &sync.Mutex{}
	addNaccounts := func(accounts []*ethereum.SignKeys, wg *sync.WaitGroup) {
		defer wg.Done()
		log.Infof("generating %d voting proofs", len(accounts))
		for i, acc := range accounts {
			apiClientMtx.Lock()

			privKey := acc.PrivateKey()
			if err = api.SetAccount(privKey.String()); err != nil {
				apiClientMtx.Unlock()
				log.Fatal(err)
				return
			}

			zkAddr, err := zk.AddressFromSignKeys(acc)
			if err != nil {
				log.Fatal(err)
			}

			pr, err := api.CensusGenProof(root, zkAddr.Bytes())
			if err != nil {
				apiClientMtx.Unlock()
				log.Warnw(err.Error(),
					"current", i,
					"total", c.nvotes)
				continue
			}

			log.Debugw("census proof generated",
				"current", i,
				"total", len(accounts))
			proofCh <- true

			_, err = api.Vote(&apiclient.VoteData{
				ElectionID:   electionID,
				ProofMkTree:  pr,
				Choices:      []int{i % 2},
				VotingWeight: new(big.Int).SetUint64(8),
			})
			apiClientMtx.Unlock()
			if err != nil && !strings.Contains(err.Error(), "already exists") {
				// if the error is not "vote already exists", we need to print it
				log.Warn(err.Error())
				continue
			}
			log.Debugw("vote sent",
				"current", i,
				"total", len(accounts))
			voteCh <- true
			pr = nil
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
	log.Debugf("%d/%d voting proofs generated successfully", successProofs, len(voterAccounts))
	log.Debugf("%d/%d votes sent successfully", successVotes, len(voterAccounts))
	stopCh <- true

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

	log.Infof("%d votes registered successfully, took %s (%f votes/second)",
		c.nvotes, time.Since(startTime), float64(c.nvotes)/time.Since(startTime).Seconds())

	// Set the account back to the organization account
	if err := api.SetAccount(c.accountPrivKeys[0]); err != nil {
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
}

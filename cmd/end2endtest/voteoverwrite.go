package main

import (
	"context"
	"encoding/hex"
	"errors"
	"fmt"
	"math/big"
	"net/url"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	vapi "go.vocdoni.io/dvote/api"
	"go.vocdoni.io/dvote/apiclient"
	"go.vocdoni.io/dvote/crypto/ethereum"
	"go.vocdoni.io/dvote/log"
	"go.vocdoni.io/dvote/types"
	"go.vocdoni.io/dvote/util"
	"go.vocdoni.io/proto/build/go/models"
)

func init() {
	ops = append(ops,
		operation{
			fn:   voteOverwriteNotWaitNextBlockTest,
			name: "voteoverwritenotwaitnextblock",
			description: "Checks that the MaxVoteOverwrite feature is correctly implemented, even if a vote is consecutive " +
				"overwrite without wait the next block, that means the error in checkTx: overwrite count reached, it's not raised",
			example: os.Args[0] + " --operation=voteoverwritenotwaitnextblock # test against public dev API\n" +
				os.Args[0] + " --host http://127.0.0.1:9090/v2 --faucet=http://127.0.0.1:9090/v2/faucet/dev/" +
				" --operation=voteoverwritenotwaitnextblock # test against local testsuite",
		},
		operation{
			fn:          voteOverwriteWaitNextBlockTest,
			name:        "voteoverwritewaitnextblock",
			description: "Checks that the MaxVoteOverwrite feature is correctly implemented, one vote is consecutive overwrite using WaitUntilNextBlock",
			example: os.Args[0] + " --operation=voteoverwritenwaitnextblock # test against public dev API\n" +
				os.Args[0] + " --host http://127.0.0.1:9090/v2 --faucet=http://127.0.0.1:9090/v2/faucet/dev/" +
				" --operation=voteoverwritenwaitnextblock # test against local testsuite",
		},
	)
}

func voteOverwriteNotWaitNextBlockTest(c config) {
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
	censusID, err := api.NewCensus(vapi.CensusTypeWeighted)
	if err != nil {
		log.Fatal(err)
	}
	log.Infof("new census created with id %s", censusID.String())

	// Generate n participant accounts
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
			Type:     vapi.CensusTypeWeighted,
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
					Choices:     []int{0},
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
				log.Infof("sent %d/%d votes... got %d HTTP errors", votesSent, len(accounts), contextDeadlines)
				time.Sleep(time.Second * 2)
				break
			}
		}
		log.Infof("successfully sent %d votes", votesSent)
		time.Sleep(time.Second * 2)
	}

	pcount = c.nvotes / c.parallelCount
	for i := 1; i < len(voterAccounts); i += pcount {
		end := i + pcount
		if end > len(voterAccounts) {
			end = len(voterAccounts)
		}
		wg.Add(1)
		go voteAccounts(voterAccounts[i:end], &wg)
	}

	wg.Wait()
	log.Infof("%d votes submitted successfully, took %s (%d votes/second)",
		c.nvotes-1, time.Since(startTime), int(float64(c.nvotes)/time.Since(startTime).Seconds()))

	// TODO: remove repeated code
	// Send the only missing vote, should be fine
	cc := api.Clone(fmt.Sprintf("%x", voterAccounts[0].PrivateKey()))
	time.Sleep(time.Second)

	voteData := apiclient.VoteData{
		ElectionID:  electionID,
		ProofMkTree: proofs[voterAccounts[0].Address().Hex()],
		Choices:     []int{0},
	}

	contextDeadlines := 0

	if _, err := cc.Vote(&voteData); err != nil {
		if errors.Is(err, context.DeadlineExceeded) || os.IsTimeout(err) {
			contextDeadlines++
		} else if !strings.Contains(err.Error(), "already exists") {
			// if the error is not "vote already exists", we need to print it
			log.Warn(err)
		}
	}

	time.Sleep(time.Second * 5)
	log.Infof("got %d HTTP errors", contextDeadlines)

	// Second vote (firsts overwrite)
	voteData.Choices = []int{1}

	contextDeadlines = 0
	if _, err := cc.Vote(&voteData); err != nil {
		if errors.Is(err, context.DeadlineExceeded) || os.IsTimeout(err) {
			contextDeadlines++
		} else if !strings.Contains(err.Error(), "already exists") {
			// if the error is not "vote already exists", we need to print it
			log.Warn(err)
		}
	}

	time.Sleep(time.Second * 5)
	log.Infof("got %d HTTP errors", contextDeadlines)

	// Third vote (second overwrite, should fail)
	voteData.Choices = []int{0}

	contextDeadlines = 0

	// due the code does not wait until next block to do the consecutive overwrite vote, it
	// doesn't have enough time to get the checkTx error: count overwrite reached
	if _, err := cc.Vote(&voteData); err != nil {
		if errors.Is(err, context.DeadlineExceeded) || os.IsTimeout(err) {
			contextDeadlines++
		} else if !strings.Contains(err.Error(), "already exists") {
			// if the error is not "vote already exists", we need to print it
			log.Warn(err)
		}
	}

	log.Info("successfully sent the only missing vote")
	time.Sleep(time.Second * 5)

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
	_, err = api.SetElectionStatus(electionID, "ENDED")
	if err != nil {
		log.Fatal(err)
	}

	// Wait for the election to be in RESULTS state
	ctx, cancel = context.WithTimeout(context.Background(), time.Second*60)
	defer cancel()
	elres, err := api.WaitUntilElectionResults(ctx, electionID)
	if err != nil {
		log.Fatal(err)
	}

	firstChoice := fmt.Sprintf("%d", (c.nvotes-1)*10)
	// should count the firsts overwrite
	secondChoice := fmt.Sprintf("%d", 10)

	// according to the first overwrite
	resultExpected := [][]string{{firstChoice, secondChoice, "0"}}

	// only the first overwrite should be valid in the results and must math with the expected results
	if !matchResult(elres.Results, resultExpected) {
		log.Fatalf("election result must match, expected Results: %s but got Results: %v", resultExpected, elres.Results)
	}
	log.Infof("election %s status is RESULTS", electionID.String())
	log.Infof("election results: %v", elres.Results)

}

func voteOverwriteWaitNextBlockTest(c config) {
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
	censusID, err := api.NewCensus(vapi.CensusTypeWeighted)
	if err != nil {
		log.Fatal(err)
	}
	log.Infof("new census created with id %s", censusID.String())

	// Generate n participant accounts
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
			Type:     vapi.CensusTypeWeighted,
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
					Choices:     []int{0},
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
				log.Infof("sent %d/%d votes... got %d HTTP errors", votesSent, len(accounts), contextDeadlines)
				time.Sleep(time.Second * 2)
				break
			}
		}
		log.Infof("successfully sent %d votes", votesSent)
		time.Sleep(time.Second * 2)
	}

	pcount = c.nvotes / c.parallelCount
	for i := 1; i < len(voterAccounts); i += pcount {
		end := i + pcount
		if end > len(voterAccounts) {
			end = len(voterAccounts)
		}
		wg.Add(1)
		go voteAccounts(voterAccounts[i:end], &wg)
	}

	wg.Wait()
	log.Infof("%d votes submitted successfully, took %s (%d votes/second)",
		c.nvotes-1, time.Since(startTime), int(float64(c.nvotes)/time.Since(startTime).Seconds()))

	// TODO: remove repeated code
	// Send the only missing vote, should be fine
	cc := api.Clone(fmt.Sprintf("%x", voterAccounts[0].PrivateKey()))
	time.Sleep(time.Second)

	voteData := apiclient.VoteData{
		ElectionID:  electionID,
		ProofMkTree: proofs[voterAccounts[0].Address().Hex()],
		Choices:     []int{0},
	}

	contextDeadlines := 0

	if _, err := cc.Vote(&voteData); err != nil {
		if errors.Is(err, context.DeadlineExceeded) || os.IsTimeout(err) {
			contextDeadlines++
		} else if !strings.Contains(err.Error(), "already exists") {
			// if the error is not "vote already exists", we need to print it
			log.Warn(err)
		}
	}

	ctx, cancel = context.WithTimeout(context.Background(), time.Second*20)
	defer cancel()
	api.WaitUntilNextBlock(ctx)

	log.Infof("got %d HTTP errors", contextDeadlines)

	// Second vote (firsts overwrite)
	voteData.Choices = []int{1}

	contextDeadlines = 0
	if _, err := cc.Vote(&voteData); err != nil {
		if errors.Is(err, context.DeadlineExceeded) || os.IsTimeout(err) {
			contextDeadlines++
		} else if !strings.Contains(err.Error(), "already exists") {
			// if the error is not "vote already exists", we need to print it
			log.Warn(err)
		}
	}

	ctx, cancel = context.WithTimeout(context.Background(), time.Second*20)
	defer cancel()
	api.WaitUntilNextBlock(ctx)

	log.Infof("got %d HTTP errors", contextDeadlines)

	// Third vote (second overwrite, should fail)
	voteData.Choices = []int{0}

	contextDeadlines = 0

	// an error is expected by checkTx: count overwrite reached
	if _, err := cc.Vote(&voteData); err != nil {
		if errors.Is(err, context.DeadlineExceeded) || os.IsTimeout(err) {
			contextDeadlines++
		} else if !strings.Contains(err.Error(), "overwrite count reached") {
			log.Fatal("expected overwrite error, got: ", err)
		} else {
			// the log should be the error: count overwrite reached
			log.Infof("error expected: %s", err.Error())
		}
	}

	log.Info("successfully sent the only missing vote")
	time.Sleep(time.Second * 5)

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

	firstChoice := fmt.Sprintf("%d", (c.nvotes-1)*10)
	// should count the firsts overwrite
	secondChoice := fmt.Sprintf("%d", 10)

	// according to the first overwrite
	resultExpected := [][]string{{firstChoice, secondChoice, "0"}}

	if !matchResult(election.Results, resultExpected) {
		log.Fatalf("election result must match, expected Results: %s but got Results: %v", resultExpected, election.Results)
	}
	log.Infof("election %s status is RESULTS", electionID.String())
	log.Infof("election results: %v", election.Results)

}

// matchResult compare the expected vote results base in the overwrite applied, with the actual result returned by the API
func matchResult(results [][]*types.BigInt, expectedResult [][]string) bool {
	// only have one question
	for q := range results[0] {
		if !(expectedResult[0][q] == results[0][q].String()) {
			return false
		}
	}
	return true
}

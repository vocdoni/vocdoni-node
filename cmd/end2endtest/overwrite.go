package main

import (
	"context"
	"encoding/hex"
	"errors"
	"fmt"
	"go.vocdoni.io/dvote/crypto/ethereum"
	"os"
	"strings"
	"sync"
	"time"

	vapi "go.vocdoni.io/dvote/api"
	"go.vocdoni.io/dvote/apiclient"
	"go.vocdoni.io/dvote/log"
)

func init() {
	ops["overwritewaitnextblock"] = operation{
		test: &E2EAnonElection{},
		description: "Checks that the MaxVoteOverwrite feature is correctly implemented, even if a vote is consecutive " +
			"overwrite without wait the next block, that means the error in checkTx: overwrite count reached, it's not raised",
		example: os.Args[0] + " --operation=voteoverwritenotwaitnextblock --votes=1000",
	}
}

var _ VochainTest = (*E2EOverwriteElection)(nil)

type E2EOverwriteElection struct {
	e2eElection
}

func (t *E2EOverwriteElection) Setup(api *apiclient.HTTPclient, c *config) error {
	t.api = api
	t.config = c

	ed := newTestElectionDescription()
	ed.ElectionType = vapi.ElectionType{
		Autostart:     true,
		Interruptible: true,
	}
	ed.VoteType = vapi.VoteType{MaxVoteOverwrites: 1}
	ed.Census = vapi.CensusTypeDescription{Type: vapi.CensusTypeWeighted}

	if err := t.setupElection(ed); err != nil {
		return err
	}

	log.Debugf("election details: %+v", *t.election)
	return nil
}

func (t *E2EOverwriteElection) Teardown() error {
	// nothing to do here
	return nil
}

func (t *E2EOverwriteElection) Run() error {
	c := t.config
	api := t.api

	// TODO: remove repeated code
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

	pcount := c.nvotes / c.parallelCount
	for i := 1; i < len(t.voterAccounts); i += pcount {
		end := i + pcount
		if end > len(t.voterAccounts) {
			end = len(t.voterAccounts)
		}
		wg.Add(1)
		go voteAccounts(t.voterAccounts[i:end], &wg)
	}

	wg.Wait()
	log.Infof("%d votes submitted successfully, took %s (%d votes/second)",
		c.nvotes-1, time.Since(startTime), int(float64(c.nvotes)/time.Since(startTime).Seconds()))

	// TODO: remove repeated code
	// Send the only missing vote, should be fine
	cc := api.Clone(fmt.Sprintf("%x", t.voterAccounts[0].PrivateKey()))
	time.Sleep(time.Second)

	voteData := apiclient.VoteData{
		ElectionID:  t.election.ElectionID,
		ProofMkTree: t.proofs[t.voterAccounts[0].Address().Hex()],
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

	ctx, cancel := context.WithTimeout(context.Background(), time.Second*20)
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
	hash, err := api.SetElectionStatus(t.election.ElectionID, "ENDED")
	if err != nil {
		log.Fatal(err)
	}

	// Check the election status is actually ENDED
	ctx, cancel = context.WithTimeout(context.Background(), time.Second*40)
	defer cancel()
	if _, err := api.WaitUntilTxIsMined(ctx, hash); err != nil {
		log.Fatalf("gave up waiting for tx %s to be mined: %s", hash, err)
	}

	t.election, err = api.Election(t.election.ElectionID)
	if err != nil {
		log.Fatal(err)
	}
	if t.election.Status != "ENDED" {
		log.Fatal("election status is not ENDED")
	}
	log.Infof("election %s status is ENDED", t.election.ElectionID.String())

	// Wait for the election to be in RESULTS state
	ctx, cancel = context.WithTimeout(context.Background(), time.Second*300)
	defer cancel()
	election, err := api.WaitUntilElectionStatus(ctx, t.election.ElectionID, "RESULTS")
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
	log.Infof("election %s status is RESULTS", t.election.ElectionID.String())
	log.Infof("election results: %v", election.Results)

	return nil
}

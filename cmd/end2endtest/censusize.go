package main

import (
	"context"
	"encoding/hex"
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
	ops["censusizelection"] = operation{
		test:        &E2EMaxCensusSizeElection{},
		description: "Publishes a census with maxCensusSize smaller than the actual census size validate the maxCensusSize restriction feature",
		example:     os.Args[0] + " --operation=censusizelection --votes=1000",
	}
}

var _ VochainTest = (*E2EMaxCensusSizeElection)(nil)

type E2EMaxCensusSizeElection struct {
	e2eElection
}

func (t *E2EMaxCensusSizeElection) Setup(api *apiclient.HTTPclient, c *config) error {
	t.api = api
	t.config = c

	ed := newTestElectionDescription()
	ed.ElectionType = vapi.ElectionType{
		Autostart:     true,
		Interruptible: true,
	}
	ed.VoteType = vapi.VoteType{MaxVoteOverwrites: 1}
	ed.Census = vapi.CensusTypeDescription{
		Type: vapi.CensusTypeWeighted,
		Size: uint64(t.config.nvotes - 1),
	}

	if err := t.setupElection(ed); err != nil {
		return err
	}

	log.Debugf("election details: %+v", *t.election)
	return nil
}

func (t *E2EMaxCensusSizeElection) Teardown() error {
	// nothing to do here
	return nil
}

func (t *E2EMaxCensusSizeElection) Run() error {
	c := t.config
	api := t.api

	// Send the votes (parallelized)
	startTime := time.Now()
	wg := sync.WaitGroup{}
	voteAccounts := func(accounts []*ethereum.SignKeys, wg *sync.WaitGroup) {
		defer wg.Done()
		log.Infof("sending %d votes", len(accounts))

		votesSent := 0
		contextDeadlines := 0
		// send nvotes - 1 votes should be fine, due the censusSize was defined previously with nvotes - 1
		for _, acc := range accounts {
			ctxDeadline, err := t.sendVote(acc, []int{0}, nil)
			if err != nil {
				log.Error(err)
				break
			}
			contextDeadlines += ctxDeadline
			votesSent++
		}
		log.Infof("successfully sent %d votes... got %d HTTP errors", votesSent, contextDeadlines)
		time.Sleep(time.Second * 4)
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
		c.nvotes-1, time.Since(startTime), int(float64(c.nvotes-1)/time.Since(startTime).Seconds()))

	// the last vote should fail due maxCensusSize constrain
	log.Infof("sending the missing vote associated with the account %v", t.voterAccounts[0].Address())

	if _, err := t.sendVote(t.voterAccounts[0], []int{0}, nil); err != nil {
		// check the error expected for maxCensusSize
		if strings.Contains(err.Error(), "maxCensusSize reached") {
			log.Infof("error expected: %s", err.Error())
		} else {
			// any other error is not expected
			return err
		}
	}

	//  the last vote was not success and it not counts
	log.Infof("waiting for all the votes to be registered...")
	for {
		count, err := api.ElectionVoteCount(t.election.ElectionID)
		if err != nil {
			log.Warn(err)
		}
		if count == uint32(c.nvotes-1) {
			break
		}
		time.Sleep(time.Second * 5)
		log.Infof("verified %d/%d votes", count, c.nvotes)
		if time.Since(startTime) > c.timeout {
			log.Fatalf("timeout waiting for votes to be registered")
		}
	}

	log.Infof("%d votes registered successfully, took %s (%d votes/second)",
		c.nvotes-1, time.Since(startTime), int(float64(c.nvotes)/time.Since(startTime).Seconds()))

	// Set the account back to the organization account
	if err := api.SetAccount(hex.EncodeToString(c.accountKeys[0].PrivateKey())); err != nil {
		return err
	}

	// End the election by setting the status to ENDED
	log.Infof("ending election...")
	if _, err := api.SetElectionStatus(t.election.ElectionID, "ENDED"); err != nil {
		return err
	}

	// Wait for the election to be in RESULTS state
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*100)
	defer cancel()
	elres, err := api.WaitUntilElectionResults(ctx, t.election.ElectionID)
	if err != nil {
		return err
	}

	// should not count the last vote
	firstChoice := fmt.Sprintf("%d", (c.nvotes-1)*10)
	secondChoice := fmt.Sprintf("%d", 0)
	resultExpected := [][]string{{firstChoice, secondChoice, "0"}}

	if !matchResult(elres.Results, resultExpected) {
		log.Fatalf("election result must match, expected Results: %s but got Results: %v", resultExpected, elres.Results)
	}
	log.Infof("election %s status is RESULTS", t.election.ElectionID.String())
	log.Infof("election results: %v", elres.Results)

	return nil
}

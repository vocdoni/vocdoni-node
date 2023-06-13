package main

import (
	"context"
	"encoding/hex"
	"fmt"
	"os"
	"sync"
	"time"

	vapi "go.vocdoni.io/dvote/api"
	"go.vocdoni.io/dvote/apiclient"
	"go.vocdoni.io/dvote/crypto/ethereum"
	"go.vocdoni.io/dvote/log"
	"go.vocdoni.io/dvote/types"
)

func init() {
	ops["overwritelection"] = operation{
		test: &E2EOverwriteElection{},
		description: "Checks that the MaxVoteOverwrite feature is correctly implemented, even if a vote is consecutive " +
			"overwrite without wait the next block, that means the error in checkTx: overwrite count reached, it's not raised",
		example: os.Args[0] + " --operation=overwritelection --votes=1000",
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
	ed.VoteType = vapi.VoteType{MaxVoteOverwrites: 2}
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

	// Send the votes (parallelized)
	startTime := time.Now()
	wg := sync.WaitGroup{}
	voteAccounts := func(accounts []*ethereum.SignKeys, wg *sync.WaitGroup) {
		defer wg.Done()
		log.Infof("sending %d votes", len(accounts))

		votesSent := 0
		contextDeadlines := 0
		for _, acc := range accounts {
			ctxDeadline, err := t.sendVote(voteInfo{voterAccount: acc, choice: []int{0}}, nil)
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

	// overwrite the previous vote (choice 0) associated with account of index 0, using enough time to do it in the nextBlock
	// try to make 3 overwrites (number of choices passed to the method). The last overwrite should fail due the maxVoteOverwrite constrain
	ctxDeadlines, err := t.overwriteVote([]int{0, 1, 0}, 0, nextBlock)
	if err != nil {
		return err
	}
	log.Infof("the account %v send an overwrite vote, got %d HTTP errors", t.voterAccounts[0].Address(), ctxDeadlines)
	time.Sleep(time.Second * 5)

	// now the overwrite vote is done in the sameBlock using account of index 1
	if ctxDeadlines, err = t.overwriteVote([]int{1, 1, 0}, 1, sameBlock); err != nil {
		return err
	}
	log.Infof("the account %v send an overwrite vote, got %d HTTP errors", t.voterAccounts[1].Address(), ctxDeadlines)
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

	// should count the first overwrite
	expectedResults := [][]*types.BigInt{votesToBigInt(uint64(c.nvotes-2)*10, 20, 0)}

	// only the first overwrite should be valid in the results and must math with the expected results
	if !matchResults(elres.Results, expectedResults) {
		return fmt.Errorf("election result must match, expected Results: %s but got Results: %v", expectedResults, elres.Results)
	}
	log.Infof("election %s status is RESULTS", t.election.ElectionID.String())
	log.Infof("election results: %v", elres.Results)

	return nil
}

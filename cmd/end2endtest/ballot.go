package main

import (
	"context"
	"encoding/hex"
	"fmt"
	"os"
	"sync"
	"time"

	"go.vocdoni.io/dvote/apiclient"
	"go.vocdoni.io/dvote/crypto/ethereum"
	"go.vocdoni.io/dvote/log"
	"go.vocdoni.io/proto/build/go/models"
)

func init() {
	ops["ballotelection"] = operation{
		test:        &E2EBallotElection{},
		description: "ballot election with unique values, maxCount, maxValue, maxTotalCost to test different ballotProtocol configurations",
		example:     os.Args[0] + " --operation=ballotelection --votes=1000",
	}
}

var _ VochainTest = (*E2EBallotElection)(nil)

type E2EBallotElection struct {
	e2eElection
}

func (t *E2EBallotElection) Setup(api *apiclient.HTTPclient, c *config) error {
	t.api = api
	t.config = c

	p := &models.Process{
		StartBlock: 0,
		BlockCount: 100,
		Status:     models.ProcessStatus_READY,
		EnvelopeType: &models.EnvelopeType{
			EncryptedVotes: false,
			UniqueValues:   true},
		CensusOrigin: models.CensusOrigin_OFF_CHAIN_TREE_WEIGHTED,
		VoteOptions: &models.ProcessVoteOptions{
			MaxCount:     2,
			MaxValue:     6,
			MaxTotalCost: 10,
			CostExponent: 1,
		},
		Mode: &models.ProcessMode{
			AutoStart:     true,
			Interruptible: true,
		},
		MaxCensusSize: uint64(t.config.nvotes),
	}

	if err := t.setupElectionRaw(p); err != nil {
		return err
	}

	log.Debugf("election details: %+v", *t.election)
	return nil
}

func (t *E2EBallotElection) Teardown() error {
	// nothing to do here
	return nil
}

func (t *E2EBallotElection) Run() error {
	c := t.config
	api := t.api

	bdata := ballotData{
		maxValue:     t.election.TallyMode.MaxValue,
		maxCount:     t.election.TallyMode.MaxCount,
		maxTotalCost: t.election.TallyMode.MaxTotalCost,
		costExponent: t.election.TallyMode.CostExponent,
	}

	votes, expectedResults := ballotVotes(bdata, t.config.nvotes)

	// Send the votes (parallelized)
	startTime := time.Now()

	wg := sync.WaitGroup{}
	apiClientMtx := &sync.Mutex{}
	indexv := 0
	voteAccounts := func(accounts []*ethereum.SignKeys, wg *sync.WaitGroup) {
		defer wg.Done()
		log.Infof("sending %d votes", len(accounts))

		votesSent := 0
		contextDeadlines := 0
		for _, acc := range accounts {
			apiClientMtx.Lock()
			ctxDeadline, err := t.sendVote(voteInfo{voterAccount: acc, choice: votes[indexv]}, nil)
			if err != nil {
				apiClientMtx.Unlock()
				log.Error(err)
				break
			}
			indexv++
			contextDeadlines += ctxDeadline
			votesSent++

			apiClientMtx.Unlock()
		}
		log.Infof("successfully sent %d votes... got %d HTTP errors", votesSent, contextDeadlines)
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

	if !matchResults(elres.Results, expectedResults) {
		return fmt.Errorf("election result must match, expected Results: %v but got Results: %v", expectedResults, elres.Results)
	}

	log.Infof("election %s status is RESULTS", t.election.ElectionID.String())
	log.Infof("election results: %v", elres.Results)

	return nil
}

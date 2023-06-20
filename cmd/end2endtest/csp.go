package main

import (
	"context"
	"encoding/hex"
	"os"
	"time"

	"go.vocdoni.io/dvote/apiclient"
	"go.vocdoni.io/dvote/log"
	"go.vocdoni.io/proto/build/go/models"
)

func init() {
	ops["cspelection"] = operation{
		test:        &E2ECSPElection{},
		description: "csp election",
		example:     os.Args[0] + " --operation=cspelection --votes=1000",
	}
}

var _ VochainTest = (*E2ECSPElection)(nil)

type E2ECSPElection struct {
	e2eElection
}

func (t *E2ECSPElection) Setup(api *apiclient.HTTPclient, c *config) error {
	t.api = api
	t.config = c

	p := &models.Process{
		StartBlock: 0,
		BlockCount: 100,
		Status:     models.ProcessStatus_READY,
		EnvelopeType: &models.EnvelopeType{
			EncryptedVotes: false,
			UniqueValues:   true},
		CensusOrigin: models.CensusOrigin_OFF_CHAIN_CA,
		VoteOptions: &models.ProcessVoteOptions{
			MaxCount: 1,
			MaxValue: 1,
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

func (t *E2ECSPElection) Teardown() error {
	// nothing to do here
	return nil
}

func (t *E2ECSPElection) Run() error {
	c := t.config
	api := t.api

	// Send the votes (parallelized)
	startTime := time.Now()

	log.Infow("enqueuing votes", "n", len(t.voterAccounts), "election", t.election.ElectionID)
	votes := []*apiclient.VoteData{}
	for _, acct := range t.voterAccounts {
		votes = append(votes, &apiclient.VoteData{
			ElectionID:   t.election.ElectionID,
			ProofCSP:     t.proofs[acct.Address().Hex()].Proof,
			Choices:      []int{0},
			VoterAccount: acct,
		})
	}
	t.sendVotes(votes)

	log.Infow("votes submitted successfully",
		"n", c.nvotes, "time", time.Since(startTime),
		"vps", int(float64(c.nvotes)/time.Since(startTime).Seconds()))

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

	log.Infof("election %s status is RESULTS", t.election.ElectionID.String())
	log.Infof("election results: %v", elres.Results)

	return nil
}

package main

import (
	"context"
	"fmt"
	"os"
	"time"

	vapi "go.vocdoni.io/dvote/api"
	"go.vocdoni.io/dvote/apiclient"
	"go.vocdoni.io/dvote/crypto/ethereum"
	"go.vocdoni.io/dvote/log"
	"go.vocdoni.io/dvote/vochain/state"
)

func init() {
	ops["hysteresis"] = operation{
		test:        &E2EHysteresis{},
		description: "Performs a complete test of anonymous election, from creating a census to voting and validating votes",
		example:     os.Args[0] + " --operation=hysteresis --votes=1000",
	}
}

var _ VochainTest = (*E2EHysteresis)(nil)

type E2EHysteresis struct {
	e2eElection
}

func (t *E2EHysteresis) Setup(api *apiclient.HTTPclient, c *config) error {
	t.api = api
	t.config = c

	ed := newTestElectionDescription()
	ed.ElectionType = vapi.ElectionType{
		Autostart:     true,
		Interruptible: true,
		Anonymous:     true,
	}
	ed.VoteType = vapi.VoteType{MaxVoteOverwrites: 1}
	ed.Census = vapi.CensusTypeDescription{Type: vapi.CensusTypeZKWeighted}

	if err := t.setupElection(ed); err != nil {
		return err
	}
	log.Debugf("election details: %+v", *t.election)
	return nil
}

func (t *E2EHysteresis) Teardown() error {
	// nothing to do here
	return nil
}

func (t *E2EHysteresis) Run() error {
	// Send the votes (parallelized)
	startTime := time.Now()

	votes := []*apiclient.VoteData{}
	for i, acct := range t.voterAccounts {
		votes = append(votes, &apiclient.VoteData{
			ElectionID:   t.election.ElectionID,
			ProofMkTree:  t.proofs[acct.Address().Hex()],
			Choices:      []int{i % 2},
			VoterAccount: acct,
		})
	}
	validVotes := len(t.voterAccounts) / 2
	log.Infow("enqueuing votes", "n", validVotes, "election", t.election.ElectionID)
	t.sendVotes(votes[:validVotes])

	log.Infow("votes submitted successfully",
		"n", validVotes, "time", time.Since(startTime),
		"vps", int(float64(validVotes)/time.Since(startTime).Seconds()))

	if err := t.verifyVoteCount(validVotes); err != nil {
		return err
	}

	log.Info("wating to reach hysteresis to create new account and force to delete old sikroots")
	ctx, cancel := context.WithTimeout(context.Background(), t.config.timeout)
	defer cancel()
	if err := t.api.WaitUntilNBlocks(ctx, state.SIKROOT_HYSTERESIS_BLOCKS); err != nil {
		return err
	}

	log.Info("create first account to force to update sik roots")
	testAccount := ethereum.NewSignKeys()
	if err := testAccount.Generate(); err != nil {
		return err
	}
	testAccountPrivKey := testAccount.PrivateKey()
	if _, _, err := t.createAccount(testAccountPrivKey.String()); err != nil {
		return err
	}
	log.Info("wating to reach hysteresis to create new account and force to delete the last old sik root")
	ctx, cancel = context.WithTimeout(context.Background(), t.config.timeout)
	defer cancel()
	if err := t.api.WaitUntilNBlocks(ctx, state.SIKROOT_HYSTERESIS_BLOCKS); err != nil {
		return err
	}

	log.Info("create second account to force to update sik roots")
	testAccount = ethereum.NewSignKeys()
	if err := testAccount.Generate(); err != nil {
		return err
	}
	testAccountPrivKey = testAccount.PrivateKey()
	if _, _, err := t.createAccount(testAccountPrivKey.String()); err != nil {
		return err
	}

	invalidVotes := len(votes) - validVotes
	log.Infow("enqueuing invalid votes", "n", invalidVotes, "election", t.election.ElectionID)
	errors := t.sendVotes(votes[validVotes:])
	if got := len(errors); got != invalidVotes {
		return fmt.Errorf("%d errors expected about hysteresis expected, got %d", invalidVotes, got)
	}

	elres, err := t.endElectionAndFetchResults()
	if err != nil {
		return err
	}

	log.Infof("election %s status is RESULTS", t.election.ElectionID.String())
	log.Infof("election results: %v", elres.Results)

	return nil
}

package main

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	vapi "go.vocdoni.io/dvote/api"
	"go.vocdoni.io/dvote/apiclient"
	"go.vocdoni.io/dvote/log"
)

func init() {
	ops["raceDuringCommit"] = operation{
		test:        &E2ERaceDuringCommit{},
		description: "Creates an election, votes, ends it, and while waiting for the results, spams the API to check for unexpected errors during block commit",
		example:     os.Args[0] + " --operation=raceDuringCommit",
	}
}

var _ VochainTest = (*E2ERaceDuringCommit)(nil)

type E2ERaceDuringCommit struct {
	e2eElection
}

func (t *E2ERaceDuringCommit) Setup(api *apiclient.HTTPclient, c *config) error {
	t.api = api
	t.config = c

	ed := newTestElectionDescription(2)
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

func (*E2ERaceDuringCommit) Teardown() error {
	// nothing to do here
	return nil
}

func (t *E2ERaceDuringCommit) Run() error {
	c := t.config

	// Send the votes (parallelized)
	startTime := time.Now()

	log.Infof("enqueuing %d votes", len(t.voterAccounts))
	votes := []*apiclient.VoteData{}
	for i, acct := range t.voterAccounts {
		votes = append(votes, &apiclient.VoteData{
			ElectionID:   t.election.ElectionID,
			ProofMkTree:  t.proofs[acct.Address().Hex()],
			Choices:      []int{i % 2},
			VoterAccount: acct,
		})
	}
	errs := t.sendVotes(votes)
	if len(errs) > 0 {
		return fmt.Errorf("error in sendVotes %+v", errs)
	}

	log.Infow("votes submitted successfully",
		"n", c.nvotes, "time", time.Since(startTime),
		"vps", int(float64(c.nvotes)/time.Since(startTime).Seconds()))

	if err := t.verifyVoteCount(t.config.nvotes); err != nil {
		return err
	}

	// Set the account back to the organization account
	api := t.api.Clone(t.config.accountPrivKeys[0])

	// End the election by setting the status to ENDED
	log.Infof("ending election...")
	if _, err := api.SetElectionStatus(t.election.ElectionID, "ENDED"); err != nil {
		return fmt.Errorf("cannot set election status to ENDED %w", err)
	}

	// Wait for the election to be in RESULTS state
	ctx, cancel := context.WithTimeout(context.Background(), t.config.timeout*3)
	defer cancel()

	for {
		election, err := t.api.ElectionResults(t.election.ElectionID)
		if err != nil && !strings.Contains(err.Error(), "5024") { // "election results are not yet available" (TODO: proper code matching)
			return fmt.Errorf("found an unexpected error %w", err)
		}
		if err == nil && election != nil {
			log.Infow("election published results", "election",
				t.election.ElectionID.String(), "duration", time.Since(startTime).String())
			return nil
		}
		select {
		case <-time.After(5 * time.Millisecond): // very short interval to spam the API and hit the window where the block commit happens
			continue
		case <-ctx.Done():
			return fmt.Errorf("election %s never published results after %s: %w",
				t.election.ElectionID.String(), time.Since(startTime).String(), ctx.Err())
		}
	}
}

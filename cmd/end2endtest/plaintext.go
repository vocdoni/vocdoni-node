package main

import (
	"fmt"
	"os"
	"time"

	vapi "go.vocdoni.io/dvote/api"
	"go.vocdoni.io/dvote/apiclient"
	"go.vocdoni.io/dvote/log"
)

func init() {
	ops["plaintextelection"] = operation{
		test:        &E2EPlaintextElection{},
		description: "Publishes a census and a non-anonymous, non-secret election, emits N votes and verifies the results",
		example:     os.Args[0] + " --operation=plaintextelection --votes=1000",
	}
}

var _ VochainTest = (*E2EPlaintextElection)(nil)

type E2EPlaintextElection struct {
	e2eElection
}

func (t *E2EPlaintextElection) Setup(api *apiclient.HTTPclient, c *config) error {
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

func (*E2EPlaintextElection) Teardown() error {
	// nothing to do here
	return nil
}

func (t *E2EPlaintextElection) Run() error {
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

	elres, err := t.endElectionAndFetchResults()
	if err != nil {
		return err
	}

	log.Infof("election %s status is RESULTS", t.election.ElectionID.String())
	log.Infof("election results: %v", elres.Results)

	return nil
}

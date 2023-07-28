package main

import (
	"math/big"
	"os"
	"time"

	vapi "go.vocdoni.io/dvote/api"
	"go.vocdoni.io/dvote/apiclient"
	"go.vocdoni.io/dvote/log"
)

func init() {
	ops["anonelection"] = operation{
		test:        &E2EAnonElection{},
		description: "Performs a complete test of anonymous election, from creating a census to voting and validating votes",
		example:     os.Args[0] + " --operation=anonelection --votes=1000",
	}
}

var _ VochainTest = (*E2EAnonElection)(nil)

type E2EAnonElection struct {
	e2eElection
}

func (t *E2EAnonElection) Setup(api *apiclient.HTTPclient, c *config) error {
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

func (t *E2EAnonElection) Teardown() error {
	// nothing to do here
	return nil
}

func (t *E2EAnonElection) Run() error {
	// Send the votes (parallelized)
	startTime := time.Now()

	log.Infow("enqueuing votes", "n", len(t.voterAccounts), "election", t.election.ElectionID)
	votes := []*apiclient.VoteData{}
	for i, acct := range t.voterAccounts {
		votes = append(votes, &apiclient.VoteData{
			ElectionID:   t.election.ElectionID,
			ProofMkTree:  t.proofs[acct.AddressString()],
			ProofSIKTree: t.sikproofs[acct.AddressString()],
			Choices:      []int{i % 2},
			VoterAccount: acct,
			VoteWeight:   big.NewInt(defaultWeight / 2),
		})
	}
	t.sendVotes(votes)

	log.Infow("votes submitted successfully",
		"n", len(t.voterAccounts), "time", time.Since(startTime),
		"vps", int(float64(len(t.voterAccounts))/time.Since(startTime).Seconds()))

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

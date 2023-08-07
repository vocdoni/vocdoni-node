package main

import (
	"fmt"
	"os"
	"time"

	"go.vocdoni.io/dvote/apiclient"
	"go.vocdoni.io/dvote/log"
	"go.vocdoni.io/proto/build/go/models"
)

func init() {
	ops["ballotRanked"] = operation{
		test:        &E2EBallotRanked{},
		description: "ballot election to test ranked voting",
		example:     os.Args[0] + " --operation=ballotRanked --votes=1000",
	}
	ops["ballotQuadratic"] = operation{
		test:        &E2EBallotQuadratic{},
		description: "ballot election to test quadratic voting",
		example:     os.Args[0] + " --operation=ballotQuadratic --votes=1000",
	}
	ops["ballotRange"] = operation{
		test:        &E2EBallotRange{},
		description: "ballot election to test range voting",
		example:     os.Args[0] + " --operation=ballotRange --votes=1000",
	}
	ops["ballotApproval"] = operation{
		test:        &E2EBallotApproval{},
		description: "ballot election to test approval voting",
		example:     os.Args[0] + " --operation=ballotApproval --votes=1000",
	}
}

var _ VochainTest = (*E2EBallotRanked)(nil)
var _ VochainTest = (*E2EBallotQuadratic)(nil)
var _ VochainTest = (*E2EBallotRange)(nil)
var _ VochainTest = (*E2EBallotApproval)(nil)

type E2EBallotRanked struct{ e2eElection }
type E2EBallotQuadratic struct{ e2eElection }
type E2EBallotRange struct{ e2eElection }
type E2EBallotApproval struct{ e2eElection }

func (t *E2EBallotRanked) Setup(api *apiclient.HTTPclient, c *config) error {
	t.api = api
	t.config = c

	//setup for ranked voting
	p := newTestProcess()
	p.VoteOptions = &models.ProcessVoteOptions{
		MaxCount:     4,
		MaxValue:     3,
		MaxTotalCost: 6,
		CostExponent: 1,
	}

	if err := t.setupElectionRaw(p); err != nil {
		return err
	}

	log.Debugf("election details: %+v", *t.election)

	return nil
}

func (*E2EBallotRanked) Teardown() error { return nil }

func (t *E2EBallotRanked) Run() error {
	nvotes := t.config.nvotes

	choices, expectedResults := ballotVotes(t.election.TallyMode, nvotes, "ranked", true)

	// Send the votes (parallelized)
	startTime := time.Now()

	log.Infow("enqueuing votes", "n", len(t.voterAccounts), "election", t.election.ElectionID)
	votes := []*apiclient.VoteData{}
	for i, acct := range t.voterAccounts {
		votes = append(votes, &apiclient.VoteData{
			ElectionID:   t.election.ElectionID,
			ProofMkTree:  t.proofs[acct.Address().Hex()],
			Choices:      choices[i],
			VoterAccount: acct,
		})
	}
	t.sendVotes(votes)

	log.Infow("votes submitted successfully",
		"n", len(t.voterAccounts), "time", time.Since(startTime),
		"vps", int(float64(len(t.voterAccounts))/time.Since(startTime).Seconds()))

	if err := t.verifyVoteCount(t.config.nvotes); err != nil {
		return fmt.Errorf("error in verifyVoteCount: %s", err)
	}

	elres, err := t.endElectionAndFetchResults()
	if err != nil {
		return fmt.Errorf("error in electionAndFetchResults: %s", err)
	}
	if !matchResults(elres.Results, expectedResults) {
		return fmt.Errorf("election result must match, expected Results: %s but got Results: %v", expectedResults, elres.Results)
	}

	log.Infof("election %s status is RESULTS", t.election.ElectionID.String())
	log.Infof("election results: %v", elres.Results)

	return nil
}

// TODO: cut and paste the other tests
func (t *E2EBallotQuadratic) Setup(api *apiclient.HTTPclient, c *config) error {
	t.api = api
	return nil
}
func (*E2EBallotQuadratic) Teardown() error { return nil }
func (*E2EBallotQuadratic) Run() error      { return nil }

func (t *E2EBallotRange) Setup(api *apiclient.HTTPclient, c *config) error { t.api = api; return nil }
func (*E2EBallotRange) Teardown() error                                    { return nil }
func (*E2EBallotRange) Run() error                                         { return nil }

func (t *E2EBallotApproval) Setup(api *apiclient.HTTPclient, c *config) error {
	t.api = api
	return nil
}
func (*E2EBallotApproval) Teardown() error { return nil }
func (*E2EBallotApproval) Run() error      { return nil }

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

	//setup for ranked voting
	p := newTestProcess()
	// update to use csp origin
	p.CensusOrigin = models.CensusOrigin_OFF_CHAIN_CA

	if err := t.setupElectionRaw(p); err != nil {
		return err
	}

	log.Debugf("election details: %+v", *t.election)
	return nil
}

func (*E2ECSPElection) Teardown() error {
	// nothing to do here
	return nil
}

func (t *E2ECSPElection) Run() error {
	c := t.config

	// Send the votes (parallelized)
	startTime := time.Now()

	log.Infow("enqueuing votes", "n", t.config.nvotes, "election", t.election.ElectionID)
	votes := []*apiclient.VoteData{}

	t.voters.Range(func(key, value any) bool {
		if acctp, ok := value.(acctProof); ok {
			votes = append(votes, &apiclient.VoteData{
				Election:     t.election,
				ProofCSP:     acctp.proof.Proof,
				Choices:      []int{0},
				VoterAccount: acctp.account,
			})
		}
		return true
	})
	errs := t.sendVotes(votes)
	if len(errs) > 0 {
		return fmt.Errorf("error in sendVotes %+v", errs)
	}

	log.Infow("votes submitted successfully",
		"n", c.nvotes, "time", time.Since(startTime),
		"vps", int(float64(c.nvotes)/time.Since(startTime).Seconds()))

	elres, err := t.verifyAndEndElection(t.config.nvotes)
	if err != nil {
		return err
	}

	log.Infof("election %s status is RESULTS", t.election.ElectionID.String())
	log.Infof("election results: %v", elres.Results)

	return nil
}

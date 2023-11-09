package main

import (
	"fmt"
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
	ops["anonelectionTempSIKs"] = operation{
		test:        &E2EAnonElectionTempSIKs{},
		description: "Performs a complete test of anonymous election with TempSIKs flag to vote with half of the accounts that are not registered, and the remaining half with registered accounts",
		example:     os.Args[0] + " --operation=anonelectionTempSIKS --votes=1000",
	}
}

var _ VochainTest = (*E2EAnonElection)(nil)
var _ VochainTest = (*E2EAnonElectionTempSIKs)(nil)

type E2EAnonElection struct{ e2eElection }
type E2EAnonElectionTempSIKs struct{ e2eElection }

func (t *E2EAnonElection) Setup(api *apiclient.HTTPclient, c *config) error {
	t.api = api
	t.config = c

	ed := newTestElectionDescription(2)
	ed.ElectionType = vapi.ElectionType{
		Autostart:     true,
		Interruptible: true,
		Anonymous:     true,
	}
	ed.VoteType = vapi.VoteType{MaxVoteOverwrites: 1}
	ed.Census = vapi.CensusTypeDescription{Type: vapi.CensusTypeZKWeighted}

	if err := t.setupElection(ed, t.config.nvotes); err != nil {
		return err
	}
	log.Debugf("election details: %+v", *t.election)
	return nil
}

func (*E2EAnonElection) Teardown() error {
	// nothing to do here
	return nil
}

func (t *E2EAnonElection) Run() error {
	var vcount int

	startTime := time.Now()
	votes := []*apiclient.VoteData{}

	log.Infow("enqueuing votes", "n", t.config.nvotes, "election", t.election.ElectionID)
	t.voters.Range(func(key, value any) bool {
		if acctp, ok := value.(acctProof); ok {
			votes = append(votes, &apiclient.VoteData{
				Election:     t.election,
				ProofMkTree:  acctp.proof,
				ProofSIKTree: acctp.proofSIK,
				Choices:      []int{vcount % 2},
				VoterAccount: acctp.account,
				VoteWeight:   big.NewInt(defaultWeight / 2),
			})
			vcount += 1
		}
		return true
	})

	errs := t.sendVotes(votes)
	if len(errs) > 0 {
		return fmt.Errorf("error in sendVotes %+v", errs)
	}

	log.Infow("votes submitted successfully",
		"n", t.config.nvotes, "time", time.Since(startTime),
		"vps", int(float64(t.config.nvotes)/time.Since(startTime).Seconds()))

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

func (t *E2EAnonElectionTempSIKs) Setup(api *apiclient.HTTPclient, c *config) error {
	t.api = api
	t.config = c

	ed := newTestElectionDescription(2)
	ed.ElectionType = vapi.ElectionType{
		Autostart:     true,
		Interruptible: true,
		Anonymous:     true,
	}
	ed.VoteType = vapi.VoteType{MaxVoteOverwrites: 1}
	ed.Census = vapi.CensusTypeDescription{Type: vapi.CensusTypeZKWeighted}
	ed.TempSIKs = true

	if err := t.setupElection(ed, t.config.nvotes); err != nil {
		return err
	}
	log.Debugf("election details: %+v", *t.election)
	return nil
}

func (*E2EAnonElectionTempSIKs) Teardown() error {
	// nothing to do here
	return nil
}

func (t *E2EAnonElectionTempSIKs) Run() error {
	var vcount int

	startTime := time.Now()
	votes := []*apiclient.VoteData{}

	log.Infow("enqueuing votes", "n", t.config.nvotes, "election", t.election.ElectionID)
	t.voters.Range(func(key, value any) bool {
		if acctp, ok := value.(acctProof); ok {
			votes = append(votes, &apiclient.VoteData{
				Election:     t.election,
				ProofMkTree:  acctp.proof,
				ProofSIKTree: acctp.proofSIK,
				Choices:      []int{vcount % 2},
				VoterAccount: acctp.account,
				VoteWeight:   big.NewInt(defaultWeight / 2),
			})
			vcount += 1
		}
		return true
	})

	errs := t.sendVotes(votes)
	if len(errs) > 0 {
		return fmt.Errorf("error in sendVotes %+v", errs)
	}

	log.Infow("votes submitted successfully",
		"n", t.config.nvotes, "time", time.Since(startTime),
		"vps", int(float64(t.config.nvotes)/time.Since(startTime).Seconds()))

	elres, err := t.verifyAndEndElection(t.config.nvotes)
	if err != nil {
		return err
	}

	log.Infof("election %s status is RESULTS", t.election.ElectionID.String())
	log.Infof("election results: %v", elres.Results)

	return nil
}

package main

import (
	"fmt"
	"os"
	"time"

	vapi "go.vocdoni.io/dvote/api"
	"go.vocdoni.io/dvote/apiclient"
	"go.vocdoni.io/dvote/log"
	"go.vocdoni.io/dvote/types"
)

func init() {
	ops["censusizelection"] = operation{
		testFunc: func() VochainTest {
			return &E2EMaxCensusSizeElection{}
		},
		description: "Publishes a census with maxCensusSize smaller than the actual census size validate the maxCensusSize restriction feature",
		example:     os.Args[0] + " --operation=censusizelection --votes=1000",
	}
}

var _ VochainTest = (*E2EMaxCensusSizeElection)(nil)

type E2EMaxCensusSizeElection struct {
	e2eElection
}

func (t *E2EMaxCensusSizeElection) Setup(api *apiclient.HTTPclient, c *config) error {
	t.api = api
	t.config = c

	ed := newTestElectionDescription(2)
	ed.ElectionType = vapi.ElectionType{
		Autostart:     true,
		Interruptible: true,
	}
	ed.VoteType = vapi.VoteType{MaxVoteOverwrites: 1}
	ed.Census = vapi.CensusTypeDescription{
		Type: vapi.CensusTypeWeighted,
		Size: uint64(t.config.nvotes - 1),
	}

	if err := t.setupElection(ed, t.config.nvotes, true); err != nil {
		return err
	}

	logElection(t.election)
	return nil
}

func (*E2EMaxCensusSizeElection) Teardown() error {
	// nothing to do here
	return nil
}

func (t *E2EMaxCensusSizeElection) Run() error {
	c := t.config

	// Send the votes (parallelized)
	startTime := time.Now()

	// send nvotes - 1 votes should be fine, due the censusSize was defined previously with nvotes - 1
	log.Infow("enqueuing votes", "n", t.config.nvotes-1, "election", t.election.ElectionID)
	votes := []*apiclient.VoteData{}

	t.voters.Range(func(key, value any) bool {
		if acctp, ok := value.(acctProof); ok {
			votes = append(votes, &apiclient.VoteData{
				Election:     t.election,
				ProofMkTree:  acctp.proof,
				Choices:      []int{0},
				VoterAccount: acctp.account,
			})
		}
		return true
	})
	errs := t.sendVotes(votes[1:], 5)
	if len(errs) > 0 {
		return fmt.Errorf("error in sendVotes %+v", errs)
	}

	log.Infow("votes submitted successfully",
		"n", len(votes[1:]), "time", time.Since(startTime),
		"vps", int(float64(len(votes[1:]))/time.Since(startTime).Seconds()))

	// the missing vote should fail due maxCensusSize constrain
	_ = t.api.WaitUntilNextBlock()
	log.Infof("sending the missing vote associated with the account %v", votes[0].VoterAccount.Address())

	if _, err := t.api.Vote(votes[0]); err != nil {
		// check the error expected for maxCensusSize
		log.Infof("error expected: %s", err.Error())
	} else {
		return fmt.Errorf("expected maxCensusSize limit error")
	}

	// one vote is not valid
	elres, err := t.verifyAndEndElection(t.config.nvotes - 1)
	if err != nil {
		return err
	}

	// should not count the last vote
	expectedResults := [][]*types.BigInt{votesToBigInt(uint64(c.nvotes-1)*10, 0, 0)}

	if !matchResults(elres.Results, expectedResults) {
		return fmt.Errorf("election result must match, expected Results: %s but got Results: %v", expectedResults, elres.Results)
	}
	log.Infof("election %s status is RESULTS", t.election.ElectionID.String())
	log.Infof("election results: %v", elres.Results)

	return nil
}

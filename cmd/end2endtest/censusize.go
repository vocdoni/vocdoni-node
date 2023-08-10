package main

import (
	"fmt"
	"os"
	"strings"
	"time"

	vapi "go.vocdoni.io/dvote/api"
	"go.vocdoni.io/dvote/apiclient"
	"go.vocdoni.io/dvote/log"
	"go.vocdoni.io/dvote/types"
)

func init() {
	ops["censusizelection"] = operation{
		test:        &E2EMaxCensusSizeElection{},
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

	ed := newTestElectionDescription()
	ed.ElectionType = vapi.ElectionType{
		Autostart:     true,
		Interruptible: true,
	}
	ed.VoteType = vapi.VoteType{MaxVoteOverwrites: 1}
	ed.Census = vapi.CensusTypeDescription{
		Type: vapi.CensusTypeWeighted,
		Size: uint64(t.config.nvotes - 1),
	}

	if err := t.setupElection(ed, false); err != nil {
		return err
	}

	log.Debugf("election details: %+v", *t.election)
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
	log.Infow("enqueuing votes", "n", len(t.voterAccounts[1:]), "election", t.election.ElectionID)
	votes := []*apiclient.VoteData{}
	for _, acct := range t.voterAccounts[1:] {
		votes = append(votes, &apiclient.VoteData{
			ElectionID:   t.election.ElectionID,
			ProofMkTree:  t.proofs[acct.Address().Hex()],
			Choices:      []int{0},
			VoterAccount: acct,
		})
	}
	t.sendVotes(votes)

	log.Infow("votes submitted successfully",
		"n", len(t.voterAccounts[1:]), "time", time.Since(startTime),
		"vps", int(float64(len(t.voterAccounts[1:]))/time.Since(startTime).Seconds()))

	// the missing vote should fail due maxCensusSize constrain
	time.Sleep(time.Second * 4)
	log.Infof("sending the missing vote associated with the account %v", t.voterAccounts[0].Address())

	v := apiclient.VoteData{
		ElectionID:   t.election.ElectionID,
		ProofMkTree:  t.proofs[t.voterAccounts[0].Address().Hex()],
		VoterAccount: t.voterAccounts[0],
		Choices:      []int{0},
	}
	if _, err := t.api.Vote(&v); err != nil {
		// check the error expected for maxCensusSize
		if strings.Contains(err.Error(), "maxCensusSize reached") {
			log.Infof("error expected: %s", err.Error())
		} else {
			// any other error is not expected
			return err
		}
	}

	// one vote is not valid
	if err := t.verifyVoteCount(t.config.nvotes - 1); err != nil {
		return err
	}

	elres, err := t.endElectionAndFetchResults()
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

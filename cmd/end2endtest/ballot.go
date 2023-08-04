package main

import (
	"fmt"
	"os"
	"sync"
	"time"

	"go.vocdoni.io/dvote/apiclient"
	"go.vocdoni.io/dvote/log"
	"go.vocdoni.io/proto/build/go/models"
)

func init() {
	ops["ballotelection"] = operation{
		test:        &E2EBallotElection{},
		description: "ballot election to test different ballotProtocol voting: ranked, quadratic, range and approval",
		example:     os.Args[0] + " --operation=ballotelection --votes=1000",
	}
}

var _ VochainTest = (*E2EBallotElection)(nil)

type E2EBallotElection struct {
	elections []e2eElection
}

func (t *E2EBallotElection) Setup(api *apiclient.HTTPclient, c *config) error {
	var err error
	if t.elections, err = newE2EElections(api, c, 4); err != nil {
		return err
	}

	//setup for ranked voting
	p := newTestProcess()
	p.VoteOptions = &models.ProcessVoteOptions{
		MaxCount:     4,
		MaxValue:     3,
		MaxTotalCost: 6,
		CostExponent: 1,
	}

	if err := t.elections[0].setupElectionRaw(p); err != nil {
		return err
	}

	log.Debugf("election details: %+v", *t.elections[0].election)
	// setup for quadratic voting
	p2 := newTestProcess()
	// update uniqueValues to false
	p2.EnvelopeType.UniqueValues = false
	p2.VoteOptions = &models.ProcessVoteOptions{
		MaxCount:     4,
		MaxValue:     3,
		MaxTotalCost: 12,
		CostExponent: 2,
	}

	if err := t.elections[1].setupElectionRaw(p2); err != nil {
		return err
	}

	log.Debugf("election details: %+v", *t.elections[1].election)

	// setup for range voting
	p3 := newTestProcess()
	p3.VoteOptions = &models.ProcessVoteOptions{
		MaxCount:     4,
		MaxValue:     3,
		MaxTotalCost: 10,
		CostExponent: 1,
	}

	if err := t.elections[2].setupElectionRaw(p3); err != nil {
		return err
	}

	log.Debugf("election details: %+v", *t.elections[2].election)

	//setup for approval voting
	p4 := newTestProcess()
	// update uniqueValues to false
	p4.EnvelopeType.UniqueValues = false
	p4.VoteOptions = &models.ProcessVoteOptions{
		MaxCount:     4,
		MaxValue:     1,
		MaxTotalCost: 4,
		CostExponent: 1,
	}

	if err := t.elections[3].setupElectionRaw(p4); err != nil {
		return err
	}

	log.Debugf("election details: %+v", *t.elections[3].election)
	return nil

}

func (*E2EBallotElection) Teardown() error {
	// nothing to do here
	return nil
}

func (t *E2EBallotElection) Run() error {
	nvotes := t.elections[0].config.nvotes
	var wg sync.WaitGroup
	errCh := make(chan error, 4)

	choicesE1, expectedResults := ballotVotes(t.elections[0].election.TallyMode, nvotes, "ranked", true)
	choicesE2, expectedResults2 := ballotVotes(t.elections[1].election.TallyMode, nvotes, "quadratic", false)
	choicesE3, expectedResults3 := ballotVotes(t.elections[2].election.TallyMode, nvotes, "range", true)
	choicesE4, expectedResults4 := ballotVotes(t.elections[3].election.TallyMode, nvotes, "approval", false)

	// Send the votes (parallelized)
	startTime := time.Now()

	wg.Add(1)
	go func() {
		// ranked voting
		defer wg.Done()

		log.Infow("enqueuing votes", "n", len(t.elections[0].voterAccounts), "election", t.elections[0].election.ElectionID)
		votes := []*apiclient.VoteData{}
		for i, acct := range t.elections[0].voterAccounts {
			votes = append(votes, &apiclient.VoteData{
				ElectionID:   t.elections[0].election.ElectionID,
				ProofMkTree:  t.elections[0].proofs[acct.Address().Hex()],
				Choices:      choicesE1[i],
				VoterAccount: acct,
			})
		}
		t.elections[0].sendVotes(votes)

		log.Infow("votes submitted successfully",
			"n", len(t.elections[0].voterAccounts), "time", time.Since(startTime),
			"vps", int(float64(len(t.elections[0].voterAccounts))/time.Since(startTime).Seconds()))

		if err := t.elections[0].verifyVoteCount(t.elections[0].config.nvotes); err != nil {
			errCh <- fmt.Errorf("error in verifyVoteCount: %s", err)
			return
		}

		elres, err := t.elections[0].endElectionAndFetchResults()
		if err != nil {
			errCh <- fmt.Errorf("error in electionAndFetchResults: %s", err)
			return
		}
		if !matchResults(elres.Results, expectedResults) {
			errCh <- fmt.Errorf("election result must match, expected Results: %s but got Results: %v", expectedResults, elres.Results)
			return
		}

		log.Infof("election %s status is RESULTS", t.elections[0].election.ElectionID.String())
		log.Infof("election results: %v", elres.Results)
	}()

	wg.Add(1)
	go func() {
		// quadratic voting
		defer wg.Done()

		log.Infow("enqueuing votes", "n", len(t.elections[1].voterAccounts), "election", t.elections[1].election.ElectionID)
		votes := []*apiclient.VoteData{}
		for i, acct := range t.elections[1].voterAccounts {
			votes = append(votes, &apiclient.VoteData{
				ElectionID:   t.elections[1].election.ElectionID,
				ProofMkTree:  t.elections[1].proofs[acct.Address().Hex()],
				Choices:      choicesE2[i],
				VoterAccount: acct,
			})
		}
		t.elections[1].sendVotes(votes)

		log.Infow("votes submitted successfully",
			"n", len(t.elections[1].voterAccounts), "time", time.Since(startTime),
			"vps", int(float64(len(t.elections[1].voterAccounts))/time.Since(startTime).Seconds()))

		if err := t.elections[1].verifyVoteCount(t.elections[1].config.nvotes); err != nil {
			errCh <- fmt.Errorf("error in verifyVoteCount: %s", err)
			return
		}

		elres, err := t.elections[1].endElectionAndFetchResults()
		if err != nil {
			errCh <- fmt.Errorf("error in electionAndFetchResults: %s", err)
			return
		}
		if !matchResults(elres.Results, expectedResults2) {
			errCh <- fmt.Errorf("election result must match, expected Results: %s but got Results: %v", expectedResults2, elres.Results)
			return
		}

		log.Infof("election %s status is RESULTS", t.elections[1].election.ElectionID.String())
		log.Infof("election results: %v", elres.Results)
	}()

	wg.Add(1)
	go func() {
		// range voting
		defer wg.Done()

		log.Infow("enqueuing votes", "n", len(t.elections[2].voterAccounts), "election", t.elections[2].election.ElectionID)
		votes := []*apiclient.VoteData{}
		for i, acct := range t.elections[2].voterAccounts {
			votes = append(votes, &apiclient.VoteData{
				ElectionID:   t.elections[2].election.ElectionID,
				ProofMkTree:  t.elections[2].proofs[acct.Address().Hex()],
				Choices:      choicesE3[i],
				VoterAccount: acct,
			})
		}
		t.elections[2].sendVotes(votes)

		log.Infow("votes submitted successfully",
			"n", len(t.elections[2].voterAccounts), "time", time.Since(startTime),
			"vps", int(float64(len(t.elections[2].voterAccounts))/time.Since(startTime).Seconds()))

		if err := t.elections[2].verifyVoteCount(t.elections[2].config.nvotes); err != nil {
			errCh <- fmt.Errorf("error in verifyVoteCount: %s", err)
			return
		}

		elres, err := t.elections[2].endElectionAndFetchResults()
		if err != nil {
			errCh <- fmt.Errorf("error in electionAndFetchResults: %s", err)
			return
		}
		if !matchResults(elres.Results, expectedResults3) {
			errCh <- fmt.Errorf("election result must match, expected Results: %s but got Results: %v", expectedResults3, elres.Results)
			return
		}

		log.Infof("election %s status is RESULTS", t.elections[2].election.ElectionID.String())
		log.Infof("election results: %v", elres.Results)
	}()

	wg.Add(1)
	go func() {
		// approval voting
		defer wg.Done()

		log.Infow("enqueuing votes", "n", len(t.elections[3].voterAccounts), "election", t.elections[3].election.ElectionID)
		votes := []*apiclient.VoteData{}
		for i, acct := range t.elections[3].voterAccounts {
			votes = append(votes, &apiclient.VoteData{
				ElectionID:   t.elections[3].election.ElectionID,
				ProofMkTree:  t.elections[3].proofs[acct.Address().Hex()],
				Choices:      choicesE4[i],
				VoterAccount: acct,
			})
		}
		t.elections[3].sendVotes(votes)

		log.Infow("votes submitted successfully",
			"n", len(t.elections[3].voterAccounts), "time", time.Since(startTime),
			"vps", int(float64(len(t.elections[3].voterAccounts))/time.Since(startTime).Seconds()))

		if err := t.elections[3].verifyVoteCount(t.elections[3].config.nvotes); err != nil {
			errCh <- fmt.Errorf("error in verifyVoteCount: %s", err)
			return
		}

		elres, err := t.elections[3].endElectionAndFetchResults()
		if err != nil {
			errCh <- fmt.Errorf("error in electionAndFetchResults: %s", err)
			return
		}
		if !matchResults(elres.Results, expectedResults4) {
			errCh <- fmt.Errorf("election result must match, expected Results: %s but got Results: %v", expectedResults4, elres.Results)
			return
		}

		log.Infof("election %s status is RESULTS", t.elections[3].election.ElectionID.String())
		log.Infof("election results: %v", elres.Results)
	}()

	wg.Wait()
	close(errCh)

	for err := range errCh {
		if err != nil {
			return err
		}
	}

	return nil
}

package main

import (
	"context"
	"fmt"
	"os"
	"time"

	vapi "go.vocdoni.io/dvote/api"
	"go.vocdoni.io/dvote/apiclient"
	"go.vocdoni.io/dvote/log"
)

func init() {
	ops["electionTimeBounds"] = operation{
		testFunc: func() VochainTest {
			return &E2EElectionTimeBounds{}
		},
		description: "Creates an election, tests voting before, during, and after the election period. Tests also increase of election time bounds.",
		example:     os.Args[0] + " --operation=electionTimeBounds",
	}
}

var _ VochainTest = (*E2EElectionTimeBounds)(nil)

type E2EElectionTimeBounds struct {
	e2eElection
}

func (t *E2EElectionTimeBounds) Setup(api *apiclient.HTTPclient, c *config) error {
	log.Infow("setting up election time bounds test", "votes", c.nvotes)
	t.api = api
	t.config = c

	ed := newTestElectionDescription(2)
	ed.ElectionType = vapi.ElectionType{
		Autostart:     true,
		Interruptible: true,
	}
	ed.VoteType = vapi.VoteType{
		MaxCount:          2,
		MaxValue:          1,
		MaxVoteOverwrites: 1,
	}

	// Set start time to 20 seconds from now
	ed.StartDate = time.Now().Add(20 * time.Second)

	// Set end time to 40 seconds from now (20 seconds after start)
	ed.EndDate = ed.StartDate.Add(20 * time.Second)

	ed.Census = vapi.CensusTypeDescription{Type: vapi.CensusTypeWeighted}

	if err := t.setupElection(ed, 10, false); err != nil {
		return err
	}

	logElection(t.election)
	return nil
}

func (*E2EElectionTimeBounds) Teardown() error {
	// Implement teardown logic if necessary
	return nil
}

func (t *E2EElectionTimeBounds) Run() error {
	// Attempt to vote before the election starts
	if err := t.attemptVotes("before"); err != nil {
		return err
	}

	// Wait until the election starts
	if _, err := t.waitUntilElectionStarts(t.election.ElectionID); err != nil {
		return err
	}

	// Vote during the election
	if err := t.attemptVotes("during"); err != nil {
		return err
	}

	// Increase the election duration
	const newDuration = 40 // seconds

	ctx, cancel := context.WithTimeout(context.Background(), time.Second*newDuration*2)
	defer cancel()

	hash, err := t.api.SetElectionDuration(t.election.ElectionID, newDuration)
	if err != nil {
		return fmt.Errorf("error setting election duration: %v", err)
	}
	if _, err := t.api.WaitUntilTxIsMined(ctx, hash); err != nil {
		return fmt.Errorf("error waiting for transaction to be mined: %v", err)
	}

	// Check if the election duration was updated correctly
	election, err := t.api.Election(t.election.ElectionID)
	if err != nil {
		return fmt.Errorf("error getting election: %v", err)
	}
	if election.EndDate.Unix() != election.StartDate.Add(newDuration*time.Second).Unix() {
		return fmt.Errorf("election end date not updated correctly, expected %v, got %v", election.StartDate.Add(newDuration*time.Second), election.EndDate)
	}
	log.Infow("election duration increased", "electionID", t.election.ElectionID, "newDuration", newDuration)

	// Send new votes
	if err := t.attemptVotes("during"); err != nil {
		return err
	}

	// Wait until the election ends
	if _, err := t.api.WaitUntilElectionResults(ctx, t.election.ElectionID); err != nil {
		return fmt.Errorf("error waiting for election results: %v", err)
	}

	// Check the current time is actually after the new election end date
	if time.Now().Before(election.EndDate) {
		return fmt.Errorf("current time is before the new election end date, expected after %v, got %v", election.EndDate, time.Now())
	}

	// Attempt to vote after the election has ended
	if err := t.attemptVotes("after"); err != nil {
		return err
	}

	return nil
}

// Helper function to attempt voting
func (t *E2EElectionTimeBounds) attemptVotes(phase string) error {
	log.Infof("attempting to vote %s the election period", phase)
	votes := []*apiclient.VoteData{}

	// Set the number of votes to send
	voteCount := 2

	// Iterate over the voters and create vote data
	t.voters.Range(func(key, value any) bool {
		if acctp, ok := value.(acctProof); ok {
			votes = append(votes, &apiclient.VoteData{
				Election:     t.election,
				ProofMkTree:  acctp.proof,
				Choices:      []int{0, 1},
				VoterAccount: acctp.account,
			})
		}
		if len(votes) >= voteCount {
			return false
		}
		return true
	})

	errs := t.sendVotes(votes, 0)
	switch phase {
	case "before":
		if len(errs) != len(votes) {
			return fmt.Errorf("expected error voting before election starts, got %d errors, expected %d errors", len(errs), len(votes))
		}
	case "during":
		if len(errs) > 0 {
			return fmt.Errorf("unexpected error voting during election: %+v", errs)
		}
	case "after":
		if len(errs) != len(votes) {
			return fmt.Errorf("expected error voting after election ends")
		}
	default:
		return fmt.Errorf("invalid phase: %s", phase)
	}

	return nil
}

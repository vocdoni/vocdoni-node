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
		description: "Creates an election, tests voting before, during, and after the election period",
		example:     os.Args[0] + " --operation=electionTimeBounds --votes=10",
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

	// Set start time to 30 seconds from now
	ed.StartDate = time.Now().Add(30 * time.Second)

	// Set end time to 60 seconds from now (30 seconds after start)
	ed.EndDate = ed.StartDate.Add(30 * time.Second)

	ed.Census = vapi.CensusTypeDescription{Type: vapi.CensusTypeWeighted}

	if err := t.setupElection(ed, t.config.nvotes, false); err != nil {
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

	// Wait until the election ends
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*60)
	defer cancel()
	if _, err := t.api.WaitUntilElectionResults(ctx, t.election.ElectionID); err != nil {
		return fmt.Errorf("error waiting for election results: %v", err)
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

	t.voters.Range(func(key, value any) bool {
		if acctp, ok := value.(acctProof); ok {
			votes = append(votes, &apiclient.VoteData{
				Election:     t.election,
				ProofMkTree:  acctp.proof,
				Choices:      []int{0, 1},
				VoterAccount: acctp.account,
			})
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

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
	ops["overwritelection"] = operation{
		testFunc: func() VochainTest {
			return &E2EOverwriteElection{}
		},
		description: "Checks that the MaxVoteOverwrite feature is correctly implemented, even if a vote is consecutive " +
			"overwrite without wait the next block, that means the error in checkTx: overwrite count reached, it's not raised",
		example: os.Args[0] + " --operation=overwritelection --votes=1000",
	}
}

var _ VochainTest = (*E2EOverwriteElection)(nil)

type E2EOverwriteElection struct {
	e2eElection
}

func (t *E2EOverwriteElection) Setup(api *apiclient.HTTPclient, c *config) error {
	t.api = api
	t.config = c

	ed := newTestElectionDescription(7)
	ed.ElectionType = vapi.ElectionType{
		Autostart:     true,
		Interruptible: true,
	}
	ed.VoteType = vapi.VoteType{MaxVoteOverwrites: 2}
	ed.Census = vapi.CensusTypeDescription{Type: vapi.CensusTypeWeighted}

	if err := t.setupElection(ed, t.config.nvotes, true); err != nil {
		return err
	}

	logElection(t.election)
	return nil
}

func (*E2EOverwriteElection) Teardown() error {
	// nothing to do here
	return nil
}

func (t *E2EOverwriteElection) Run() error {
	c := t.config

	// Send the votes (parallelized)
	startTime := time.Now()

	log.Infow("enqueuing votes", "n", t.config.nvotes, "election", t.election.ElectionID)
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
	errs := t.sendVotes(votes, 5)
	if len(errs) > 0 {
		return fmt.Errorf("error in sendVotes %+v", errs)
	}

	log.Infow("votes submitted successfully",
		"n", c.nvotes, "time", time.Since(startTime),
		"vps", int(float64(c.nvotes)/time.Since(startTime).Seconds()))

	// overwrite the previous vote (choice 0) associated with account of index 0, using enough time to do it in the nextBlock
	// try to make 3 overwrites (number of choices passed to the method). The last overwrite should fail due the maxVoteOverwrite constrain
	err := t.overwriteVote([]int{1, 2, 3}, votes[0])
	if err != nil {
		return err
	}
	log.Infof("the account %v send an overwrite vote", votes[0].VoterAccount.Address())

	elres, err := t.verifyAndEndElection(t.config.nvotes)
	if err != nil {
		return err
	}

	// should count only the first overwrite
	expectedResults := [][]*types.BigInt{votesToBigInt(uint64(c.nvotes-1)*10, 0, 10, 0, 0, 0, 0, 0)}

	// only the first overwrite should be valid in the results and must math with the expected results
	if !matchResults(elres.Results, expectedResults) {
		return fmt.Errorf("election result must match, expected Results: %s but got Results: %v", expectedResults, elres.Results)
	}
	log.Infof("election %s status is RESULTS", t.election.ElectionID.String())
	log.Infof("election results: %v", elres.Results)

	return nil
}

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
		test: &E2EOverwriteElection{},
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

	ed := newTestElectionDescription()
	ed.ElectionType = vapi.ElectionType{
		Autostart:     true,
		Interruptible: true,
	}
	ed.VoteType = vapi.VoteType{MaxVoteOverwrites: 2}
	ed.Census = vapi.CensusTypeDescription{Type: vapi.CensusTypeWeighted}

	if err := t.setupElection(ed); err != nil {
		return err
	}

	log.Debugf("election details: %+v", *t.election)
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

	log.Infow("enqueuing votes", "n", len(t.voterAccounts), "election", t.election.ElectionID)
	votes := []*apiclient.VoteData{}
	for _, acct := range t.voterAccounts {
		votes = append(votes, &apiclient.VoteData{
			ElectionID:   t.election.ElectionID,
			ProofMkTree:  t.proofs[acct.Address().Hex()],
			Choices:      []int{0},
			VoterAccount: acct,
		})
	}
	t.sendVotes(votes)

	log.Infow("votes submitted successfully",
		"n", c.nvotes, "time", time.Since(startTime),
		"vps", int(float64(c.nvotes)/time.Since(startTime).Seconds()))

	// overwrite the previous vote (choice 0) associated with account of index 0, using enough time to do it in the nextBlock
	// try to make 3 overwrites (number of choices passed to the method). The last overwrite should fail due the maxVoteOverwrite constrain
	err := t.overwriteVote([]int{0, 1, 0}, 0, nextBlock)
	if err != nil {
		return err
	}
	log.Infof("the account %v send an overwrite vote", t.voterAccounts[0].Address())
	time.Sleep(time.Second * 5)

	// now the overwrite vote is done in the sameBlock using account of index 1
	if err = t.overwriteVote([]int{1, 1, 0}, 1, sameBlock); err != nil {
		return err
	}
	log.Infof("the account %v send an overwrite vote", t.voterAccounts[1].Address())
	time.Sleep(time.Second * 5)

	if err := t.verifyVoteCount(t.config.nvotes); err != nil {
		return err
	}

	elres, err := t.endElectionAndFetchResults()
	if err != nil {
		return err
	}

	// should count only the first overwrite
	expectedResults := [][]*types.BigInt{votesToBigInt(uint64(c.nvotes-2)*10, 20, 0)}

	// only the first overwrite should be valid in the results and must math with the expected results
	if !matchResults(elres.Results, expectedResults) {
		return fmt.Errorf("election result must match, expected Results: %s but got Results: %v", expectedResults, elres.Results)
	}
	log.Infof("election %s status is RESULTS", t.election.ElectionID.String())
	log.Infof("election results: %v", elres.Results)

	return nil
}

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
	ops["ballotelection"] = operation{
		test:        &E2EBallotElection{},
		description: "ballot election with unique values, maxCount, maxValue, maxTotalCost to test different ballotProtocol configurations",
		example:     os.Args[0] + " --operation=ballotelection --votes=1000",
	}
}

var _ VochainTest = (*E2EBallotElection)(nil)

type E2EBallotElection struct {
	e2eElection
}

func (t *E2EBallotElection) Setup(api *apiclient.HTTPclient, c *config) error {
	t.api = api
	t.config = c

	p := &models.Process{
		StartBlock: 0,
		BlockCount: 100,
		Status:     models.ProcessStatus_READY,
		EnvelopeType: &models.EnvelopeType{
			EncryptedVotes: false,
			UniqueValues:   true},
		CensusOrigin: models.CensusOrigin_OFF_CHAIN_TREE_WEIGHTED,
		VoteOptions: &models.ProcessVoteOptions{
			MaxCount:     2,
			MaxValue:     6,
			MaxTotalCost: 10,
			CostExponent: 1,
		},
		Mode: &models.ProcessMode{
			AutoStart:     true,
			Interruptible: true,
		},
		MaxCensusSize: uint64(t.config.nvotes),
	}

	if err := t.setupElectionRaw(p); err != nil {
		return err
	}

	log.Debugf("election details: %+v", *t.election)
	return nil
}

func (*E2EBallotElection) Teardown() error {
	// nothing to do here
	return nil
}

func (t *E2EBallotElection) Run() error {
	bdata := ballotData{
		maxValue:     t.election.TallyMode.MaxValue,
		maxCount:     t.election.TallyMode.MaxCount,
		maxTotalCost: t.election.TallyMode.MaxTotalCost,
		costExponent: t.election.TallyMode.CostExponent,
	}

	choices, expectedResults := ballotVotes(bdata, len(t.voterAccounts))

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
		return err
	}

	elres, err := t.endElectionAndFetchResults()
	if err != nil {
		return err
	}
	if !matchResults(elres.Results, expectedResults) {
		return fmt.Errorf("election result must match, expected Results: %v but got Results: %v", expectedResults, elres.Results)
	}

	log.Infof("election %s status is RESULTS", t.election.ElectionID.String())
	log.Infof("election results: %v", elres.Results)

	return nil
}

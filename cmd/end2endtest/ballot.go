package main

import (
	"fmt"
	"math"
	"os"
	"time"

	vapi "go.vocdoni.io/dvote/api"
	"go.vocdoni.io/dvote/apiclient"
	"go.vocdoni.io/dvote/log"
	"go.vocdoni.io/dvote/types"
	"go.vocdoni.io/proto/build/go/models"
)

const (
	rankedVote    = "ranked"
	quadraticVote = "quadratic"
	rangeVote     = "range"
	approvalVote  = "approval"
)

func init() {
	ops["ballotRanked"] = operation{
		test:        &E2EBallotRanked{},
		description: "ballot election to test ranked voting",
		example:     os.Args[0] + " --operation=ballotRanked --votes=1000",
	}
	ops["ballotQuadratic"] = operation{
		test:        &E2EBallotQuadratic{},
		description: "ballot election to test quadratic voting",
		example:     os.Args[0] + " --operation=ballotQuadratic --votes=1000",
	}
	ops["ballotRange"] = operation{
		test:        &E2EBallotRange{},
		description: "ballot election to test range voting",
		example:     os.Args[0] + " --operation=ballotRange --votes=1000",
	}
	ops["ballotApproval"] = operation{
		test:        &E2EBallotApproval{},
		description: "ballot election to test approval voting",
		example:     os.Args[0] + " --operation=ballotApproval --votes=1000",
	}
}

var _ VochainTest = (*E2EBallotRanked)(nil)
var _ VochainTest = (*E2EBallotQuadratic)(nil)
var _ VochainTest = (*E2EBallotRange)(nil)
var _ VochainTest = (*E2EBallotApproval)(nil)

type E2EBallotRanked struct{ e2eElection }
type E2EBallotQuadratic struct{ e2eElection }
type E2EBallotRange struct{ e2eElection }
type E2EBallotApproval struct{ e2eElection }

func (t *E2EBallotRanked) Setup(api *apiclient.HTTPclient, c *config) error {
	t.api = api
	t.config = c

	//setup for ranked voting
	p := newTestProcess()
	p.VoteOptions = &models.ProcessVoteOptions{
		MaxCount:     4,
		MaxValue:     3,
		MaxTotalCost: 6,
		CostExponent: 1,
	}

	if err := t.setupElectionRaw(p); err != nil {
		return fmt.Errorf("error in setupElectionRaw for ranked voting: %s", err)
	}

	log.Debugf("election details: %+v", *t.election)
	return nil
}

func (*E2EBallotRanked) Teardown() error {
	// nothing to do here
	return nil
}

func (t *E2EBallotRanked) Run() error {
	nvotes := t.config.nvotes

	choices, expectedResults := ballotVotes(
		t.election.TallyMode, nvotes,
		rankedVote, t.election.VoteMode.UniqueValues)

	if err := sendAndValidateVotes(t.e2eElection, choices, expectedResults); err != nil {
		return err
	}

	return nil
}

func (t *E2EBallotQuadratic) Setup(api *apiclient.HTTPclient, c *config) error {
	t.api = api
	t.config = c

	// setup for quadratic voting
	p := newTestProcess()
	// update uniqueValues to false
	p.EnvelopeType.UniqueValues = false
	p.VoteOptions = &models.ProcessVoteOptions{
		MaxCount:     4,
		MaxValue:     3,
		MaxTotalCost: 12,
		CostExponent: 2,
	}

	if err := t.setupElectionRaw(p); err != nil {
		return fmt.Errorf("error in setupElectionRaw for quadratic voting: %s", err)
	}

	log.Debugf("election details: %+v", *t.election)
	return nil
}

func (*E2EBallotQuadratic) Teardown() error {
	// nothing to do here
	return nil
}

func (t *E2EBallotQuadratic) Run() error {
	nvotes := t.config.nvotes

	choices, expectedResults := ballotVotes(
		t.election.TallyMode, nvotes,
		quadraticVote, t.election.VoteMode.UniqueValues)

	if err := sendAndValidateVotes(t.e2eElection, choices, expectedResults); err != nil {
		return err
	}

	return nil
}

func (t *E2EBallotRange) Setup(api *apiclient.HTTPclient, c *config) error {
	t.api = api
	t.config = c

	// setup for range voting
	p := newTestProcess()
	p.VoteOptions = &models.ProcessVoteOptions{
		MaxCount:     4,
		MaxValue:     3,
		MaxTotalCost: 10,
		CostExponent: 1,
	}

	if err := t.setupElectionRaw(p); err != nil {
		return fmt.Errorf("error in setupElectionRaw for range voting: %s", err)
	}

	log.Debugf("election details: %+v", *t.election)
	return nil
}

func (*E2EBallotRange) Teardown() error {
	// nothing to do here
	return nil
}

func (t *E2EBallotRange) Run() error {
	nvotes := t.config.nvotes

	choices, expectedResults := ballotVotes(
		t.election.TallyMode, nvotes,
		rangeVote, t.election.VoteMode.UniqueValues)

	if err := sendAndValidateVotes(t.e2eElection, choices, expectedResults); err != nil {
		return err
	}

	return nil
}

func (t *E2EBallotApproval) Setup(api *apiclient.HTTPclient, c *config) error {
	t.api = api
	t.config = c

	//setup for approval voting
	p := newTestProcess()
	// update uniqueValues to false
	p.EnvelopeType.UniqueValues = false
	p.VoteOptions = &models.ProcessVoteOptions{
		MaxCount:     4,
		MaxValue:     1,
		MaxTotalCost: 4,
		CostExponent: 1,
	}

	if err := t.setupElectionRaw(p); err != nil {
		return fmt.Errorf("error in setupElectionRaw for approval voting: %s", err)
	}

	log.Debugf("election details: %+v", *t.election)
	return nil
}

func (*E2EBallotApproval) Teardown() error {
	// nothing to do here
	return nil
}

func (t *E2EBallotApproval) Run() error {
	nvotes := t.config.nvotes

	choices, expectedResults := ballotVotes(
		t.election.TallyMode, nvotes,
		approvalVote, t.election.VoteMode.UniqueValues)

	if err := sendAndValidateVotes(t.e2eElection, choices, expectedResults); err != nil {
		return err
	}

	return nil
}

func sendAndValidateVotes(e e2eElection, choices [][]int, expectedResults [][]*types.BigInt) error {
	nvotes := e.config.nvotes
	startTime := time.Now()

	log.Infow("enqueuing votes", "n", len(e.voterAccounts), "election", e.election.ElectionID)
	votes := []*apiclient.VoteData{}
	for i, acct := range e.voterAccounts {
		votes = append(votes, &apiclient.VoteData{
			ElectionID:   e.election.ElectionID,
			ProofMkTree:  e.proofs[acct.Address().Hex()],
			Choices:      choices[i],
			VoterAccount: acct,
		})
	}
	errs := e.sendVotes(votes)
	if len(errs) > 0 {
		return fmt.Errorf("error in sendVotes %+v", errs)
	}

	log.Infow("votes submitted successfully",
		"n", len(e.voterAccounts), "time", time.Since(startTime),
		"vps", int(float64(len(e.voterAccounts))/time.Since(startTime).Seconds()))

	if err := e.verifyVoteCount(nvotes); err != nil {
		return fmt.Errorf("error in verifyVoteCount: %s", err)
	}

	elres, err := e.endElectionAndFetchResults()
	if err != nil {
		return fmt.Errorf("error in electionAndFetchResults: %s", err)
	}

	if !matchResults(elres.Results, expectedResults) {
		return fmt.Errorf("election result must match, expected Results: %s but got Results: %v", expectedResults, elres.Results)
	}

	log.Infof("election %s status is RESULTS", e.election.ElectionID.String())
	log.Infof("election results: %v", elres.Results)
	return nil
}

// ballotVotes from a default list of 10 vote values that exceed the max value, max total cost and is not unique will
func ballotVotes(vop vapi.TallyMode, nvotes int, ballotType string, expectUniqueV bool) ([][]int, [][]*types.BigInt) {
	votes := make([][]int, 0, nvotes)
	var resultsFields [][]*types.BigInt
	var v [][]int

	switch ballotType {
	// fill initial 10 votes to be sent by the ballot test
	case rankedVote:
		v = [][]int{
			{0, 5, 0, 2}, {3, 1, 0, 2}, {3, 2, 0, 1}, {1, 0, 3, 2},
			{2, 3, 1, 1}, {4, 1, 1, 0}, {0, 0, 2, 1}, {0, 2, 1, 3},
			{1, 1, 1, 4}, {0, 3, 2, 1},
		}
		// default results on field 1,2,3,4 for 10 votes
		resultsFields = append(resultsFields, votesToBigInt(20, 10, 0, 20),
			votesToBigInt(10, 10, 20, 10), votesToBigInt(20, 10, 10, 10),
			votesToBigInt(0, 20, 20, 10))

	case quadraticVote:
		v = [][]int{
			{2, 4, 2, 0}, {3, 3, 2, 2}, {2, 3, 0, 1}, {0, 2, 3, 3},
			{1, 2, 1, 2}, {5, 0, 1, 0}, {0, 2, 2, 2}, {2, 1, 1, 2},
			{1, 3, 0, 1}, {1, 3, 1, 1},
		}
		resultsFields = append(resultsFields, votesToBigInt(10, 30, 10, 0),
			votesToBigInt(0, 10, 20, 20), votesToBigInt(10, 30, 10, 0),
			votesToBigInt(0, 20, 30, 0))

	case rangeVote:
		v = [][]int{
			{5, 0, 4, 2}, {3, 1, 2, 4}, {6, 0, 1, 3}, {0, 3, 2, 1},
			{2, 1, 3, 0}, {3, 2, 1, 3}, {1, 3, 2, 0}, {2, 1, 1, 2},
			{1, 2, 0, 3}, {4, 3, 1, 1},
		}
		resultsFields = append(resultsFields, votesToBigInt(10, 20, 10, 0),
			votesToBigInt(0, 10, 10, 20), votesToBigInt(10, 0, 20, 10),
			votesToBigInt(20, 10, 0, 10))

	case approvalVote:
		v = [][]int{
			{1, 0, 0, 0}, {2, 1, 0, 1}, {0, 1, 2, 3}, {0, 0, 0, 1},
			{0, 0, 1, 1}, {2, 2, 1, 0}, {1, 0, 1, 1}, {0, 0, 0, 0},
			{1, 1, 0, 3}, {1, 1, 1, 1},
		}
		resultsFields = append(resultsFields, votesToBigInt(30, 30, 0, 0),
			votesToBigInt(50, 10, 0, 0), votesToBigInt(30, 30, 0, 0),
			votesToBigInt(20, 40, 0, 0))
	}

	// less than 10 votes
	if nvotes < 10 {
		for i := 0; i < int(vop.MaxCount); i++ {
			// initialize default results with zero values on field 1,2,3,4 for less than 10 votes
			resultsFields[i] = votesToBigInt(make([]uint64, vop.MaxCount)...)
		}
	} else {
		// greater o equal than 10 votes
		for i := 0; i < nvotes/10; i++ {
			votes = append(votes, v...)
		}
		// nvotes split 10, for example for 44 nvotes, nvoteDid10 will be 4
		// and that number will be multiplied by each default result to obtain the results for 40 votes
		nvotesDiv10 := new(types.BigInt).SetUint64(uint64(nvotes / 10))

		for _, field := range resultsFields {
			field[0] = new(types.BigInt).Mul(field[0], nvotesDiv10)
			field[1] = new(types.BigInt).Mul(field[1], nvotesDiv10)
			field[2] = new(types.BigInt).Mul(field[2], nvotesDiv10)
			field[3] = new(types.BigInt).Mul(field[3], nvotesDiv10)
		}
	}

	remainVotes := nvotes % 10
	if remainVotes != 0 {
		votes = append(votes, v[:remainVotes]...)

		for _, vote := range v[:remainVotes] {
			isValidTotalCost := math.Pow(float64(vote[0]), float64(vop.CostExponent))+
				math.Pow(float64(vote[1]), float64(vop.CostExponent))+
				math.Pow(float64(vote[2]), float64(vop.CostExponent))+
				math.Pow(float64(vote[3]), float64(vop.CostExponent)) <= float64(vop.MaxTotalCost)
			isValidValues := vote[0] <= int(vop.MaxValue) &&
				vote[1] <= int(vop.MaxValue) &&
				vote[2] <= int(vop.MaxValue) &&
				vote[3] <= int(vop.MaxValue)

			repeatValues := vote[0] == vote[1] || vote[2] == vote[1] ||
				vote[2] == vote[0] || vote[0] == vote[3] ||
				vote[1] == vote[3] || vote[2] == vote[3]

			if expectUniqueV && repeatValues {
				continue
			}

			if isValidTotalCost && isValidValues {
				resultsFields[0][vote[0]].Add(resultsFields[0][vote[0]], new(types.BigInt).SetUint64(10))
				resultsFields[1][vote[1]].Add(resultsFields[1][vote[1]], new(types.BigInt).SetUint64(10))
				resultsFields[2][vote[2]].Add(resultsFields[2][vote[2]], new(types.BigInt).SetUint64(10))
				resultsFields[3][vote[3]].Add(resultsFields[3][vote[3]], new(types.BigInt).SetUint64(10))
			}
		}
	}

	log.Debug("vote values generated", votes)
	log.Debug("results expected", resultsFields)
	return votes, resultsFields
}

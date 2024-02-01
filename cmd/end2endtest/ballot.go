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
		testFunc: func() VochainTest {
			return &E2EBallotRanked{}
		},
		description: "ballot election to test ranked voting",
		example:     os.Args[0] + " --operation=ballotRanked --votes=1000",
	}
	ops["ballotQuadratic"] = operation{
		testFunc: func() VochainTest {
			return &E2EBallotQuadratic{}
		},
		description: "ballot election to test quadratic voting",
		example:     os.Args[0] + " --operation=ballotQuadratic --votes=1000",
	}
	ops["ballotRange"] = operation{
		testFunc: func() VochainTest {
			return &E2EBallotRange{}
		},
		description: "ballot election to test range voting",
		example:     os.Args[0] + " --operation=ballotRange --votes=1000",
	}
	ops["ballotApproval"] = operation{
		testFunc: func() VochainTest {
			return &E2EBallotApproval{}
		},
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
	var vcount int
	nvotes := e.config.nvotes
	startTime := time.Now()

	log.Infow("enqueuing votes", "n", e.config.nvotes, "election", e.election.ElectionID)
	votes := []*apiclient.VoteData{}

	e.voters.Range(func(key, value any) bool {
		if acctp, ok := value.(acctProof); ok {
			votes = append(votes, &apiclient.VoteData{
				Election:     e.election,
				ProofMkTree:  acctp.proof,
				Choices:      choices[vcount],
				VoterAccount: acctp.account,
			})
			vcount += 1
		}
		return true
	})
	errs := e.sendVotes(votes, 5)
	if len(errs) > 0 {
		return fmt.Errorf("error in sendVotes %+v", errs)
	}

	log.Infow("votes submitted successfully",
		"n", e.config.nvotes, "time", time.Since(startTime),
		"vps", float64(e.config.nvotes)/time.Since(startTime).Seconds())

	elres, err := e.verifyAndEndElection(nvotes)
	if err != nil {
		return err
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
		resultsFields = append(resultsFields, votesToBigInt(30, 30),
			votesToBigInt(50, 10), votesToBigInt(30, 30),
			votesToBigInt(20, 40))
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
		// nvotes split 10, for example for 44 nvotes, nvotesDiv10 will be 4
		// and that number will be multiplied by each default result to obtain the results for 40 votes
		nvotesDiv10 := new(types.BigInt).SetUint64(uint64(nvotes / 10))

		for _, field := range resultsFields {
			for i := 0; i < len(field); i++ {
				field[i] = new(types.BigInt).Mul(field[i], nvotesDiv10)
			}
		}
	}

	remainVotes := nvotes % 10
	if remainVotes != 0 {
		votes = append(votes, v[:remainVotes]...)

		for _, vote := range v[:remainVotes] {
			if expectUniqueV && containsRepeatedValues(vote) {
				continue
			}

			if totalCost(vote, int(vop.CostExponent)) > float64(vop.MaxTotalCost) {
				continue
			}

			if valuesExceedMax(vote, int(vop.MaxValue)) {
				continue
			}

			for i, resultField := range resultsFields {
				resultField[vote[i]].Add(resultField[vote[i]], new(types.BigInt).SetUint64(10))
			}
		}
	}

	log.Debug("vote values generated", votes)
	log.Debug("results expected", resultsFields)
	return votes, resultsFields
}

func valuesExceedMax(vote []int, max int) bool {
	for _, v := range vote {
		if v > max {
			return true
		}
	}
	return false
}

func totalCost(vote []int, exp int) float64 {
	var total float64
	for _, v := range vote {
		total += math.Pow(float64(v), float64(exp))
	}
	return total
}

func containsRepeatedValues(vote []int) bool {
	dups := make(map[int]bool)
	for _, v := range vote {
		if dups[v] {
			return true
		}
		dups[v] = true
	}
	return false
}

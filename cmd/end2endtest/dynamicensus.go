package main

import (
	"context"
	"fmt"
	"os"
	"strings"
	"sync"
	"time"

	vapi "go.vocdoni.io/dvote/api"
	"go.vocdoni.io/dvote/apiclient"
	"go.vocdoni.io/dvote/log"
	"go.vocdoni.io/dvote/types"
)

func init() {
	ops["dynamicensuselection"] = operation{
		test:        &E2EDynamicensusElection{},
		description: "Publish an election with the dynamic census flag set to true and another election with dynamic census flag set to false",
		example:     os.Args[0] + " --operation=dynamicensuselection --votes=1000",
	}
}

var _ VochainTest = (*E2EDynamicensusElection)(nil)

type E2EDynamicensusElection struct {
	elections []e2eElection
}

func (t *E2EDynamicensusElection) Setup(api *apiclient.HTTPclient, c *config) error {
	t.elections = append(t.elections, e2eElection{api: api, config: c})
	t.elections = append(t.elections, e2eElection{api: api, config: c})

	eDescriptions := []struct {
		d             *vapi.ElectionDescription
		dynamicCensus bool
	}{
		{d: newTestElectionDescription(), dynamicCensus: true},
		{d: newTestElectionDescription(), dynamicCensus: false},
	}

	for i, ed := range eDescriptions {
		ed.d.ElectionType.Autostart = true
		ed.d.ElectionType.Interruptible = true
		ed.d.ElectionType.DynamicCensus = ed.dynamicCensus
		ed.d.Census = vapi.CensusTypeDescription{Type: vapi.CensusTypeWeighted}

		// create a census with 2 voterAccounts less than the nvotes passed, that will allow to create another
		// census with the missing nvotes
		censusRoot, censusURI, err := t.elections[i].setupCensus(vapi.CensusTypeWeighted, t.elections[i].config.nvotes-2)
		if err != nil {
			return err
		}

		// add the censusRoot and censusURI to the election description
		ed.d.Census = vapi.CensusTypeDescription{
			Type:     vapi.CensusTypeWeighted,
			RootHash: censusRoot,
			URL:      censusURI,
			Size:     uint64(t.elections[1].config.nvotes),
		}

		// set up the election with the custom census created
		if err := t.elections[i].setupElection(ed.d); err != nil {
			return err
		}
		log.Debugf("election detail: %+v", *t.elections[i].election)
	}

	return nil
}

func (t *E2EDynamicensusElection) Teardown() error {
	// nothing to do here
	return nil
}

func (t *E2EDynamicensusElection) Run() error {
	var wg sync.WaitGroup
	errCh := make(chan error, 2)

	setupNewCensusAndVote := func(election e2eElection) (types.HexBytes, apiclient.VoteData, error) {
		api := election.api
		nvotes := election.config.nvotes

		censusRoot2, censusURI2, err := election.setupCensus(vapi.CensusTypeWeighted, 2)
		if err != nil {
			return nil, apiclient.VoteData{}, err
		}
		log.Infof("root : %x URI: %s  err: %s", censusRoot2, censusURI2, err)

		voterKey := election.voterAccounts[nvotes-1].Address().Bytes()
		proof, err := api.CensusGenProof(censusRoot2, voterKey)
		if err != nil {
			return nil, apiclient.VoteData{}, err
		}

		v := apiclient.VoteData{
			ElectionID:   election.election.ElectionID,
			ProofMkTree:  proof,
			VoterAccount: election.voterAccounts[nvotes-1],
			Choices:      []int{1},
		}
		if _, err := api.Vote(&v); err != nil {
			// check for an error different from expected when try to vote in an election associated with other census
			if !strings.Contains(err.Error(), "merkle proof verification failed") {
				return nil, apiclient.VoteData{}, fmt.Errorf("unexpected error when voting %s", err)
			}
			log.Debugw("error expected when try to vote,", "error:", err)
		}
		return censusRoot2, v, nil
	}

	wg.Add(1)
	// election with dynamic census enabled
	go func() {
		defer wg.Done()

		api := t.elections[0].api
		electionID := t.elections[0].election.ElectionID
		nvotes := t.elections[0].config.nvotes

		// Send the votes (parallelized)
		startTime := time.Now()

		log.Infof("enqueuing %d votes", len(t.elections[0].voterAccounts[1:]))
		votes := []*apiclient.VoteData{}
		for _, acct := range t.elections[0].voterAccounts[1:] {
			votes = append(votes, &apiclient.VoteData{
				ElectionID:   electionID,
				ProofMkTree:  t.elections[0].proofs[acct.Address().Hex()],
				Choices:      []int{0},
				VoterAccount: acct,
			})
		}
		t.elections[0].sendVotes(votes)

		log.Infow("votes submitted successfully",
			"n", len(t.elections[0].voterAccounts[1:]), "time", time.Since(startTime),
			"vps", int(float64(len(t.elections[0].voterAccounts[1:]))/time.Since(startTime).Seconds()))

		var (
			censusRoot2 types.HexBytes
			v           apiclient.VoteData
			err         error
		)

		if censusRoot2, v, err = setupNewCensusAndVote(t.elections[0]); err != nil {
			errCh <- fmt.Errorf("unexpected error from electionID: %s erro: %s", electionID, err.Error())
			return
		}

		log.Debugf("election details before set a new census: %s %s %x", t.elections[0].election.Census.CensusOrigin,
			t.elections[0].election.Census.CensusURL, t.elections[0].election.Census.CensusRoot)

		hash, err := api.TransactionSetCensus(electionID, vapi.ElectionCensus{
			CensusOrigin: "OFF_CHAIN_TREE_WEIGHTED",
			CensusRoot:   censusRoot2,
			CensusURL:    "http://test/census",
		})

		if err != nil {
			errCh <- fmt.Errorf("unexpected error from set process census %s", err)
			return
		}
		log.Debugw("process census set", "tx hash:", hash)

		ctx, cancel := context.WithTimeout(context.Background(), apiclient.WaitTimeout)
		defer cancel()
		if _, err := api.WaitUntilTxIsMined(ctx, hash); err != nil {
			errCh <- fmt.Errorf("gave up waiting for tx %x to be mined: %s", hash, err)
			return
		}

		t.elections[0].election, err = api.Election(electionID)
		if err != nil {
			errCh <- fmt.Errorf("unexpected error when retrieve election details, %s", err)
			return
		}
		log.Debugf("election details after: %s %s %x", t.elections[0].election.Census.CensusOrigin, t.elections[0].election.Census.CensusURL, t.elections[0].election.Census.CensusRoot)

		// try again to vote with the vote data from the last census created
		if _, err := api.Vote(&v); err != nil {
			errCh <- fmt.Errorf("unexpected error when voting with the new census %s", err)
			return
		}

		// try again to vote after the census update with the missing vote from the first census created
		v = apiclient.VoteData{
			ElectionID:   electionID,
			ProofMkTree:  t.elections[0].proofs[t.elections[0].voterAccounts[0].Address().Hex()],
			VoterAccount: t.elections[0].voterAccounts[0],
			Choices:      []int{0},
		}
		if _, err := api.Vote(&v); err != nil {
			// check if the error is not the expected
			if !strings.Contains(err.Error(), "merkle proof verification failed") {
				errCh <- fmt.Errorf("unexpected error when voting with the first census %s", err)
				return
			}
			log.Debugw("error expected when try to vote,", "error:", err)
		}

		if err := t.elections[0].verifyVoteCount(nvotes - 2); err != nil {
			errCh <- fmt.Errorf("error in verifyVoteCount: %s", err)
			return
		}

		elres, err := t.elections[0].endElectionAndFetchResults()
		if err != nil {
			errCh <- fmt.Errorf("error in electionAndFetchResults: %s", err)
			return
		}

		expectedResults := [][]*types.BigInt{votesToBigInt(uint64(nvotes-3)*10, 10, 0)}

		if !matchResults(elres.Results, expectedResults) {
			errCh <- fmt.Errorf("election result must match, expected Results: %s but got Results: %v", expectedResults, elres.Results)
			return
		}

		t.elections[0].election, err = api.Election(electionID)
		if err != nil {
			errCh <- fmt.Errorf("unexpected err when retrieve election details, %s", err)
			return
		}

		log.Infof("election %s status is RESULTS", electionID.String())
		log.Infof("election results: %v %x %s", elres.Results, elres.CensusRoot, t.elections[0].election.Census.CensusURL)

	}()

	// election with dynamic census disabled
	wg.Add(1)
	go func() {
		defer wg.Done()

		api := t.elections[1].api
		electionID := t.elections[1].election.ElectionID

		// Send the votes (parallelized)
		startTime := time.Now()

		log.Infof("enqueuing %d votes", len(t.elections[1].voterAccounts[1:]))
		votes := []*apiclient.VoteData{}
		for _, acct := range t.elections[1].voterAccounts[1:] {
			votes = append(votes, &apiclient.VoteData{
				ElectionID:   electionID,
				ProofMkTree:  t.elections[1].proofs[acct.Address().Hex()],
				Choices:      []int{0},
				VoterAccount: acct,
			})
		}
		t.elections[1].sendVotes(votes)

		log.Infow("votes submitted successfully",
			"n", len(t.elections[1].voterAccounts[1:]), "time", time.Since(startTime),
			"vps", int(float64(len(t.elections[1].voterAccounts[1:]))/time.Since(startTime).Seconds()))

		var censusRoot2 types.HexBytes
		var err error

		if censusRoot2, _, err = setupNewCensusAndVote(t.elections[1]); err != nil {
			errCh <- fmt.Errorf("unexpected error from electionID: %s, error: %s", electionID, err)
			return
		}

		log.Debugf("election details before: %s %s %x", t.elections[1].election.Census.CensusOrigin, t.elections[1].election.Census.CensusURL, t.elections[1].election.Census.CensusRoot)

		if _, err := api.TransactionSetCensus(electionID, vapi.ElectionCensus{
			CensusOrigin: "OFF_CHAIN_TREE_WEIGHTED",
			CensusRoot:   censusRoot2,
			CensusURL:    "http://test/census",
		}); err != nil {
			// check if the error is not expected
			if !strings.Contains(err.Error(), "only processes with dynamic census can update their census") {
				errCh <- fmt.Errorf("unexpected error when update the census %s", err)
				return
			}
			log.Debugw("error expected on update census", "error:", err)
		}
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

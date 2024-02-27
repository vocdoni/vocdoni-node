package main

import (
	"fmt"
	"math/big"
	"os"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	vapi "go.vocdoni.io/dvote/api"
	"go.vocdoni.io/dvote/apiclient"
	"go.vocdoni.io/dvote/log"
)

func init() {
	ops["anonelection"] = operation{
		testFunc: func() VochainTest {
			return &E2EAnonElection{}
		},
		description: "Performs a complete test of anonymous election, from creating a census to voting and validating votes",
		example:     os.Args[0] + " --operation=anonelection --votes=1000",
	}
	ops["anonelectionTempSIKs"] = operation{
		testFunc: func() VochainTest {
			return &E2EAnonElectionTempSIKs{}
		},
		description: "Performs a complete test of anonymous election with TempSIKs flag to vote with half of the accounts that are not registered, and the remaining half with registered accounts",
		example:     os.Args[0] + " --operation=anonelectionTempSIKS --votes=1000",
	}
	ops["anonelectionEncrypted"] = operation{
		testFunc: func() VochainTest {
			return &E2EAnonElectionEncrypted{}
		},
		description: "Performs a complete test of encrypted anonymous election, from creating a census to voting and validating votes",
		example:     os.Args[0] + " --operation=anonelectionEncrypted --votes=1000",
	}
}

var _ VochainTest = (*E2EAnonElection)(nil)
var _ VochainTest = (*E2EAnonElectionTempSIKs)(nil)
var _ VochainTest = (*E2EAnonElectionEncrypted)(nil)

type E2EAnonElection struct{ e2eElection }
type E2EAnonElectionTempSIKs struct{ e2eElection }
type E2EAnonElectionEncrypted struct{ e2eElection }

func (t *E2EAnonElection) Setup(api *apiclient.HTTPclient, c *config) error {
	t.api = api
	t.config = c

	ed := newTestElectionDescription(2)
	ed.ElectionType = vapi.ElectionType{
		Autostart:     true,
		Interruptible: true,
		Anonymous:     true,
	}
	ed.VoteType = vapi.VoteType{MaxVoteOverwrites: 1}
	ed.Census = vapi.CensusTypeDescription{Type: vapi.CensusTypeZKWeighted}

	if err := t.setupElection(ed, t.config.nvotes, true); err != nil {
		return err
	}
	log.Debugf("election details: %+v", *t.election)
	return nil
}

func (*E2EAnonElection) Teardown() error {
	// nothing to do here
	return nil
}

func (t *E2EAnonElection) Run() error {
	var vcount int

	startTime := time.Now()
	votes := []*apiclient.VoteData{}

	log.Infow("enqueuing votes", "n", t.config.nvotes, "election", t.election.ElectionID)
	t.voters.Range(func(key, value any) bool {
		if acctp, ok := value.(acctProof); ok {
			votes = append(votes, &apiclient.VoteData{
				Election:     t.election,
				ProofMkTree:  acctp.proof,
				ProofSIKTree: acctp.proofSIK,
				Choices:      []int{vcount % 2},
				VoterAccount: acctp.account,
				VoteWeight:   big.NewInt(defaultWeight / 2),
			})
			vcount += 1
		}
		return true
	})

	errs := t.sendVotes(votes, 5)
	if len(errs) > 0 {
		return fmt.Errorf("error in sendVotes %+v", errs)
	}

	log.Infow("votes submitted successfully",
		"n", t.config.nvotes, "time", time.Since(startTime),
		"vps", int(float64(t.config.nvotes)/time.Since(startTime).Seconds()))

	if err := t.verifyVoteCount(t.config.nvotes); err != nil {
		return err
	}

	elres, err := t.endElectionAndFetchResults()
	if err != nil {
		return err
	}

	log.Infof("election %s status is RESULTS", t.election.ElectionID.String())
	log.Infof("election results: %v", elres.Results)

	return nil
}

func (t *E2EAnonElectionTempSIKs) Setup(api *apiclient.HTTPclient, c *config) error {
	t.api = api
	t.config = c

	ed := newTestElectionDescription(2)
	ed.ElectionType = vapi.ElectionType{
		Autostart:     true,
		Interruptible: true,
		Anonymous:     true,
	}
	ed.VoteType = vapi.VoteType{MaxVoteOverwrites: 1}
	ed.Census = vapi.CensusTypeDescription{Type: vapi.CensusTypeZKWeighted}
	// use temporal siks
	ed.TempSIKs = true

	if err := t.setupElection(ed, t.config.nvotes, true); err != nil {
		return err
	}
	logElection(t.election)
	return nil
}

func (*E2EAnonElectionTempSIKs) Teardown() error {
	// nothing to do here
	return nil
}

func (t *E2EAnonElectionTempSIKs) Run() error {
	var vcount int

	startTime := time.Now()
	votes := []*apiclient.VoteData{}

	log.Infow("enqueuing votes", "n", t.config.nvotes, "election", t.election.ElectionID)
	t.voters.Range(func(key, value any) bool {
		if acctp, ok := value.(acctProof); ok {
			votes = append(votes, &apiclient.VoteData{
				Election:     t.election,
				ProofMkTree:  acctp.proof,
				ProofSIKTree: acctp.proofSIK,
				Choices:      []int{vcount % 2},
				VoterAccount: acctp.account,
				VoteWeight:   big.NewInt(defaultWeight / 2),
			})
			vcount += 1
		}
		return true
	})

	errs := t.sendVotes(votes, 3)
	if len(errs) > 0 {
		return fmt.Errorf("error in sendVotes %+v", errs)
	}

	log.Infow("votes submitted successfully",
		"n", t.config.nvotes, "time", time.Since(startTime),
		"vps", int(float64(t.config.nvotes)/time.Since(startTime).Seconds()))

	elres, err := t.verifyAndEndElection(t.config.nvotes)
	if err != nil {
		return err
	}

	removedCount, err := removedSiks(t.api, votes, t.config.parallelCount)
	if err != nil {
		return err
	}

	// the half of accounts should not have valid sik, due they are using temporal sik
	if int(removedCount) != len(votes)/2 {
		return fmt.Errorf("unexpected number of accounts without valid SIK, got %d want %d", removedCount, len(votes)/2)
	}

	log.Infof("election %s status is RESULTS", t.election.ElectionID.String())
	log.Infof("election results: %v", elres.Results)

	return nil
}

// removedSiks get the number of accounts without a valid SIK
func removedSiks(api *apiclient.HTTPclient, votes []*apiclient.VoteData, parallelCount int) (accts int32, err error) {
	var removedCount int32
	var wg sync.WaitGroup
	errorChan := make(chan error, len(votes))

	// worker pool channel with a maximum of parallelCount workers
	workerPool := make(chan struct{}, parallelCount)

	for _, v := range votes {
		wg.Add(1)
		workerPool <- struct{}{} // reserves a slot in the worker pool
		go func(v *apiclient.VoteData) {
			defer wg.Done()
			defer func() { <-workerPool }() // make available for another goroutine to use
			privKey := v.VoterAccount.PrivateKey()
			accountApi := api.Clone(privKey.String())
			valid, err := accountApi.ValidSIK()

			if err != nil {
				if !strings.Contains(err.Error(), "SIK not found") {
					errorChan <- fmt.Errorf("unexpected SIK validation error for account: %x, %s", v.VoterAccount.Address(), err)
				}
				atomic.AddInt32(&removedCount, 1)
				log.Infof("SIK removed for account %x", v.VoterAccount.Address())

			} else if valid {
				log.Infof("valid SIK for account: %x", v.VoterAccount.Address())
			}
		}(v)
	}

	go func() {
		wg.Wait()
		close(errorChan)
	}()

	for err := range errorChan {
		if err != nil {
			return 0, err
		}
	}
	return removedCount, nil
}

func (t *E2EAnonElectionEncrypted) Setup(api *apiclient.HTTPclient, c *config) error {
	t.api = api
	t.config = c

	ed := newTestElectionDescription(2)
	ed.ElectionType = vapi.ElectionType{
		Autostart:         true,
		Interruptible:     true,
		Anonymous:         true,
		SecretUntilTheEnd: true,
	}
	ed.VoteType = vapi.VoteType{MaxVoteOverwrites: 1}
	ed.Census = vapi.CensusTypeDescription{Type: vapi.CensusTypeZKWeighted}

	if err := t.setupElection(ed, t.config.nvotes, true); err != nil {
		return err
	}
	log.Debugf("election details: %+v", *t.election)
	return nil
}

func (*E2EAnonElectionEncrypted) Teardown() error {
	// nothing to do here
	return nil
}

func (t *E2EAnonElectionEncrypted) Run() error {
	var vcount int

	startTime := time.Now()
	votes := []*apiclient.VoteData{}

	log.Infow("enqueuing votes", "n", t.config.nvotes, "election", t.election.ElectionID)
	t.voters.Range(func(key, value any) bool {
		if acctp, ok := value.(acctProof); ok {
			votes = append(votes, &apiclient.VoteData{
				Election:     t.election,
				ProofMkTree:  acctp.proof,
				ProofSIKTree: acctp.proofSIK,
				Choices:      []int{vcount % 2},
				VoterAccount: acctp.account,
				VoteWeight:   big.NewInt(defaultWeight / 2),
			})
			vcount += 1
		}
		return true
	})

	errs := t.sendVotes(votes, 5)
	if len(errs) > 0 {
		return fmt.Errorf("error in sendVotes %+v", errs)
	}

	log.Infow("votes submitted successfully",
		"n", t.config.nvotes, "time", time.Since(startTime),
		"vps", int(float64(t.config.nvotes)/time.Since(startTime).Seconds()))

	if err := t.verifyVoteCount(t.config.nvotes); err != nil {
		return err
	}

	elres, err := t.endElectionAndFetchResults()
	if err != nil {
		return err
	}

	log.Infof("election %s status is RESULTS", t.election.ElectionID.String())
	log.Infof("election results: %v", elres.Results)

	return nil
}

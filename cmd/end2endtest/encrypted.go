package main

import (
	"fmt"
	"os"
	"time"

	vapi "go.vocdoni.io/dvote/api"
	"go.vocdoni.io/dvote/apiclient"
	"go.vocdoni.io/dvote/log"
)

func init() {
	ops["encryptedelection"] = operation{
		test:        &E2EEncryptedElection{},
		description: "Publishes a census and a non-anonymous, secret-until-the-end election, emits N votes and verifies the results",
		example:     os.Args[0] + " --operation=encryptedelection --votes=1000",
	}
}

var _ VochainTest = (*E2EEncryptedElection)(nil)

type E2EEncryptedElection struct {
	e2eElection
}

func (t *E2EEncryptedElection) Setup(api *apiclient.HTTPclient, c *config) error {
	t.api = api
	t.config = c

	ed := newTestElectionDescription(2)
	ed.ElectionType = vapi.ElectionType{
		Autostart:         true,
		Interruptible:     true,
		SecretUntilTheEnd: true,
	}
	ed.VoteType = vapi.VoteType{MaxVoteOverwrites: 1}
	ed.Census = vapi.CensusTypeDescription{Type: vapi.CensusTypeWeighted}

	if err := t.setupElection(ed, t.config.nvotes); err != nil {
		return err
	}

	log.Debugf("election details: %+v", *t.election)
	return nil
}

func (*E2EEncryptedElection) Teardown() error {
	// nothing to do here
	return nil
}

func (t *E2EEncryptedElection) Run() error {
	var vcount int
	api := t.api

	// get the encryption key, that will be use each voter
	keys, err := api.EncryptionKeys(t.election.ElectionID)
	if err != nil {
		return err
	}

	// Send the votes (parallelized)
	startTime := time.Now()

	log.Infow("enqueuing votes", "n", t.config.nvotes, "election", t.election.ElectionID)
	votes := []*apiclient.VoteData{}

	t.voters.Range(func(key, value any) bool {
		if acctp, ok := value.(acctProof); ok {
			votes = append(votes, &apiclient.VoteData{
				Election:     t.election,
				ProofMkTree:  acctp.proof,
				Choices:      []int{vcount % 2},
				VoterAccount: acctp.account,
				Keys:         keys,
			})
			vcount += 1
		}
		return true
	})
	errs := t.sendVotes(votes)
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

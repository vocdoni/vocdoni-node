package main

import (
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

	ed := newTestElectionDescription()
	ed.ElectionType = vapi.ElectionType{
		Autostart:         true,
		Interruptible:     true,
		SecretUntilTheEnd: true,
	}
	ed.VoteType = vapi.VoteType{MaxVoteOverwrites: 1}
	ed.Census = vapi.CensusTypeDescription{Type: vapi.CensusTypeWeighted}

	if err := t.setupElection(ed); err != nil {
		return err
	}

	log.Debugf("election details: %+v", *t.election)
	return nil
}

func (t *E2EEncryptedElection) Teardown() error {
	// nothing to do here
	return nil
}

func (t *E2EEncryptedElection) Run() error {
	api := t.api

	// get the encryption key, that will be use each voter
	keys, err := api.EncryptionKeys(t.election.ElectionID)
	if err != nil {
		return err
	}

	// Send the votes (parallelized)
	startTime := time.Now()

	log.Infow("enqueuing votes", "n", len(t.voterAccounts), "election", t.election.ElectionID)
	votes := []*apiclient.VoteData{}
	for i, acct := range t.voterAccounts {
		votes = append(votes, &apiclient.VoteData{
			ElectionID:   t.election.ElectionID,
			ProofMkTree:  t.proofs[acct.Address().Hex()],
			Choices:      []int{i % 2},
			VoterAccount: acct,
			Keys:         keys,
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

	log.Infof("election %s status is RESULTS", t.election.ElectionID.String())
	log.Infof("election results: %v", elres.Results)

	return nil
}

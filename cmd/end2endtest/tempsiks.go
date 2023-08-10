package main

import (
	"context"
	"fmt"
	"math/big"
	"os"
	"time"

	vapi "go.vocdoni.io/dvote/api"
	"go.vocdoni.io/dvote/apiclient"
	"go.vocdoni.io/dvote/log"
)

func init() {
	ops["tempsiks"] = operation{
		test:        &E2ETempSIKs{},
		description: "Performs a test of anonymous election with temporal SIKS",
		example:     os.Args[0] + " --operation=tempsiks --votes=1000",
	}
}

var _ VochainTest = (*E2ETempSIKs)(nil)

type E2ETempSIKs struct {
	e2eElection
}

func (t *E2ETempSIKs) Setup(api *apiclient.HTTPclient, c *config) error {
	t.api = api
	t.config = c

	ed := newTestElectionDescription()
	ed.ElectionType = vapi.ElectionType{
		Autostart:     true,
		Interruptible: true,
		Anonymous:     true,
	}
	ed.VoteType = vapi.VoteType{MaxVoteOverwrites: 1}
	ed.Census = vapi.CensusTypeDescription{Type: vapi.CensusTypeZKWeighted}
	ed.TempSIKs = true

	if err := t.setupElection(ed, false); err != nil {
		return err
	}
	log.Debugf("election details: %+v", *t.election)
	return nil
}

func (*E2ETempSIKs) Teardown() error {
	// nothing to do here
	return nil
}

func (t *E2ETempSIKs) Run() error {
	// Send the votes (parallelized)
	startTime := time.Now()

	// register temporal SIKs
	for i, acc := range t.voterAccounts {
		log.Infow(fmt.Sprintf("register temporal sik %d/%d", i, t.config.nvotes),
			"address", acc.AddressString())
		pKey := acc.PrivateKey()
		client := t.api.Clone(pKey.String())
		hash, err := client.RegisterSIKForVote(t.election.ElectionID, t.proofs[acc.AddressString()], nil)
		if err != nil {
			return err
		}
		ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
		defer cancel()
		if _, err := client.WaitUntilTxIsMined(ctx, hash); err != nil {
			return err
		}
		t.sikproofs[acc.AddressString()], err = client.GenSIKProof()
		if err != nil {
			return err
		}
	}

	log.Infow("enqueuing votes", "n", len(t.voterAccounts), "election", t.election.ElectionID)
	votes := []*apiclient.VoteData{}
	for i, acct := range t.voterAccounts {
		votes = append(votes, &apiclient.VoteData{
			ElectionID:   t.election.ElectionID,
			ProofMkTree:  t.proofs[acct.AddressString()],
			ProofSIKTree: t.sikproofs[acct.AddressString()],
			Choices:      []int{i % 2},
			VoterAccount: acct,
			VoteWeight:   big.NewInt(defaultWeight / 2),
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

	log.Info("check if temporal SIKs are purged")
	for _, acc := range t.voterAccounts {
		pKey := acc.PrivateKey()
		client := t.api.Clone(pKey.String())
		if valid, err := client.ValidSIK(); err == nil || valid {
			return fmt.Errorf("sik expected to be removed")
		}
	}

	log.Infof("election %s status is RESULTS", t.election.ElectionID.String())
	log.Infof("election results: %v", elres.Results)

	return nil
}

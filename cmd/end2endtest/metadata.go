package main

import (
	"bytes"
	"context"
	"fmt"
	"os"

	vapi "go.vocdoni.io/dvote/api"
	"go.vocdoni.io/dvote/apiclient"
	"go.vocdoni.io/dvote/log"
	"go.vocdoni.io/dvote/types"
	"go.vocdoni.io/dvote/util"
)

func init() {
	ops["metadataelection"] = operation{
		testFunc: func() VochainTest {
			return &E2EMetadataElection{}
		},
		description: "Updates the metadata of an election, first to a new IPFS document and then to a URL hosted elsewhere with an arbitrary hash, and checks that votes attesting replaced metadata are rejected while votes attesting the current one are accepted.",
		example:     os.Args[0] + " --operation=metadataelection --votes=4",
	}
}

var _ VochainTest = (*E2EMetadataElection)(nil)

type E2EMetadataElection struct {
	e2eElection
}

func (t *E2EMetadataElection) Setup(api *apiclient.HTTPclient, c *config) error {
	t.api = api
	t.config = c
	if t.config.nvotes < 3 {
		return fmt.Errorf("metadataelection needs at least 3 votes")
	}

	ed := newTestElectionDescription(2)
	ed.ElectionType = vapi.ElectionType{
		Autostart:     true,
		Interruptible: true,
	}
	ed.Census = vapi.CensusTypeDescription{Type: vapi.CensusTypeWeighted}

	if err := t.setupElection(ed, t.config.nvotes, true); err != nil {
		return err
	}
	if len(t.election.MetadataHash) == 0 {
		return fmt.Errorf("election %s was created without a metadata hash", t.election.ElectionID.String())
	}

	logElection(t.election)
	return nil
}

func (*E2EMetadataElection) Teardown() error {
	// nothing to do here
	return nil
}

func (t *E2EMetadataElection) Run() error {
	var voters []acctProof
	t.voters.Range(func(_, value any) bool {
		if acctp, ok := value.(acctProof); ok {
			voters = append(voters, acctp)
		}
		return true
	})
	waitMined := func(txHash types.HexBytes) error {
		ctx, cancel := context.WithTimeout(context.Background(), t.config.timeout)
		defer cancel()
		if _, err := t.api.WaitUntilTxIsMined(ctx, txHash); err != nil {
			return fmt.Errorf("metadata update tx %s not mined: %w", txHash, err)
		}
		return nil
	}
	vote := func(election *vapi.Election, voter acctProof) error {
		_, err := t.api.Vote(&apiclient.VoteData{
			Election:     election,
			ProofMkTree:  voter.proof,
			Choices:      []int{1},
			VoterAccount: voter.account,
		})
		return err
	}

	// a vote attesting the current metadata is accepted
	original := t.election
	if err := vote(original, voters[0]); err != nil {
		return fmt.Errorf("vote attesting the current metadata was rejected: %w", err)
	}

	// update the metadata to a new IPFS document
	metadata := &vapi.ElectionMetadata{
		Title:       map[string]string{"default": "e2e updated election"},
		Description: map[string]string{"default": "metadata replaced by the metadataelection e2e test"},
		Version:     "1.0",
	}
	txHash, err := t.api.SetElectionMetadata(original.ElectionID, metadata)
	if err != nil {
		return fmt.Errorf("cannot update the election metadata: %w", err)
	}
	if err := waitMined(txHash); err != nil {
		return err
	}
	updated, err := t.api.Election(original.ElectionID)
	if err != nil {
		return err
	}
	if bytes.Equal(updated.MetadataHash, original.MetadataHash) {
		return fmt.Errorf("election metadata hash did not change after the update")
	}
	log.Infow("election metadata updated", "txHash", txHash.String(),
		"oldHash", original.MetadataHash.String(), "newHash", updated.MetadataHash.String())

	// a vote attesting the replaced metadata is rejected
	if err := vote(original, voters[1]); err == nil {
		return fmt.Errorf("vote attesting the replaced metadata was accepted")
	} else {
		log.Infow("vote attesting the replaced metadata rejected as expected", "error", err.Error())
	}

	// the same voter, now shown the new metadata, can vote
	if err := vote(updated, voters[1]); err != nil {
		return fmt.Errorf("vote attesting the new metadata was rejected: %w", err)
	}

	// point the metadata to a URL hosted elsewhere, with a hash which is not a
	// SHA-256: the chain stores both without interpreting them
	externalURI := "https://example.com/e2e/metadata.json"
	externalHash := util.RandomBytes(20)
	if txHash, err = t.api.SetElectionMetadataURI(original.ElectionID, externalURI, externalHash); err != nil {
		return fmt.Errorf("cannot point the election metadata to %s: %w", externalURI, err)
	}
	if err := waitMined(txHash); err != nil {
		return err
	}
	external, err := t.api.Election(original.ElectionID)
	if err != nil {
		return err
	}
	if external.MetadataURL != externalURI || !bytes.Equal(external.MetadataHash, externalHash) {
		return fmt.Errorf("election metadata is %s %s, want %s %x",
			external.MetadataURL, external.MetadataHash, externalURI, externalHash)
	}
	log.Infow("election metadata pointed to an external URL", "txHash", txHash.String(),
		"url", externalURI, "hash", external.MetadataHash.String())

	if err := vote(updated, voters[2]); err == nil {
		return fmt.Errorf("vote attesting the replaced metadata was accepted")
	}
	for _, voter := range voters[2:] {
		if err := vote(external, voter); err != nil {
			return fmt.Errorf("vote attesting the external metadata was rejected: %w", err)
		}
	}
	if err := t.verifyVoteCount(len(voters)); err != nil {
		return err
	}

	// the history lists the creation version and both updates
	history, err := t.api.ElectionMetadataHistory(original.ElectionID)
	if err != nil {
		return err
	}
	if len(history.Versions) != 3 {
		return fmt.Errorf("expected 3 metadata versions, got %d", len(history.Versions))
	}
	if !bytes.Equal(history.Versions[0].MetadataHash, original.MetadataHash) ||
		!bytes.Equal(history.Versions[1].MetadataHash, updated.MetadataHash) ||
		!bytes.Equal(history.Versions[2].MetadataHash, externalHash) ||
		history.Versions[2].MetadataURL != externalURI {
		return fmt.Errorf("metadata history does not match: %+v", history.Versions)
	}
	log.Infow("metadata history verified", "versions", len(history.Versions))

	if _, err := t.endElectionAndFetchResults(); err != nil {
		return err
	}
	log.Infof("election %s status is RESULTS", original.ElectionID.String())
	return nil
}

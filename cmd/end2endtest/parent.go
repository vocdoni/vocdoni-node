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
)

func init() {
	ops["parentelection"] = operation{
		testFunc: func() VochainTest {
			return &E2EParentElection{}
		},
		description: "Creates a metadata-only parent election and an election linked to it, updates the parent metadata, and checks that votes attesting the replaced parent metadata are rejected while votes attesting the current one are accepted, and that the parent ends without results.",
		example:     os.Args[0] + " --operation=parentelection --votes=4",
	}
}

var _ VochainTest = (*E2EParentElection)(nil)

type E2EParentElection struct {
	e2eElection
	parent *vapi.Election
}

func (t *E2EParentElection) Setup(api *apiclient.HTTPclient, c *config) error {
	t.api = api
	t.config = c
	if t.config.nvotes < 3 {
		return fmt.Errorf("parentelection needs at least 3 votes")
	}
	if err := t.setupAccount(); err != nil {
		return err
	}

	electionType := vapi.ElectionType{
		Autostart:     true,
		Interruptible: true,
	}
	pd := newTestElectionDescription(2)
	pd.ElectionType = electionType
	parentID, err := t.api.NewMetadataOnlyElection(pd, true)
	if err != nil {
		return fmt.Errorf("cannot create the parent election: %w", err)
	}
	if t.parent, err = t.api.Election(parentID); err != nil {
		return err
	}
	if !t.parent.MetadataOnly || len(t.parent.MetadataHash) == 0 || t.parent.Census != nil {
		return fmt.Errorf("parent election %s is not metadata-only: %+v", parentID.String(), t.parent)
	}
	log.Infow("created metadata-only parent election", "electionId", parentID.String())

	ed := newTestElectionDescription(2)
	ed.ElectionType = electionType
	ed.Census = vapi.CensusTypeDescription{Type: vapi.CensusTypeWeighted}
	ed.ParentElectionID = parentID
	if err := t.setupElection(ed, t.config.nvotes, true); err != nil {
		return err
	}
	if !bytes.Equal(t.election.ParentElectionID, parentID) || t.election.MetadataOnly {
		return fmt.Errorf("election %s is not linked to parent %s", t.election.ElectionID.String(), parentID.String())
	}

	logElection(t.election)
	return nil
}

func (*E2EParentElection) Teardown() error {
	// nothing to do here
	return nil
}

func (t *E2EParentElection) Run() error {
	var voters []acctProof
	t.voters.Range(func(_, value any) bool {
		if acctp, ok := value.(acctProof); ok {
			voters = append(voters, acctp)
		}
		return true
	})
	vote := func(voter acctProof, parentMetadataHash types.HexBytes) error {
		_, err := t.api.Vote(&apiclient.VoteData{
			Election:           t.election,
			ProofMkTree:        voter.proof,
			Choices:            []int{1},
			VoterAccount:       voter.account,
			ParentMetadataHash: parentMetadataHash,
		})
		return err
	}

	// the parent lists the election as its child
	children, err := t.api.ElectionChildren(t.parent.ElectionID)
	if err != nil {
		return err
	}
	if len(children.Elections) != 1 || !bytes.Equal(children.Elections[0].ElectionID, t.election.ElectionID) {
		return fmt.Errorf("parent children do not match: %+v", children.Elections)
	}

	// a vote attesting the current parent metadata is accepted
	if err := vote(voters[0], nil); err != nil {
		return fmt.Errorf("vote attesting the current parent metadata was rejected: %w", err)
	}

	// update the parent metadata
	metadata := &vapi.ElectionMetadata{
		Title:       map[string]string{"default": "e2e updated parent election"},
		Description: map[string]string{"default": "parent metadata replaced by the parentelection e2e test"},
		Questions:   []vapi.Question{},
		Version:     "1.0",
	}
	txHash, err := t.api.SetElectionMetadata(t.parent.ElectionID, metadata)
	if err != nil {
		return fmt.Errorf("cannot update the parent election metadata: %w", err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), t.config.timeout)
	defer cancel()
	if _, err := t.api.WaitUntilTxIsMined(ctx, txHash); err != nil {
		return fmt.Errorf("parent metadata update tx %s not mined: %w", txHash, err)
	}
	updated, err := t.api.Election(t.parent.ElectionID)
	if err != nil {
		return err
	}
	if bytes.Equal(updated.MetadataHash, t.parent.MetadataHash) {
		return fmt.Errorf("parent election metadata hash did not change after the update")
	}
	log.Infow("parent election metadata updated", "txHash", txHash.String(),
		"oldHash", t.parent.MetadataHash.String(), "newHash", updated.MetadataHash.String())

	// a vote attesting the replaced parent metadata is rejected
	err = vote(voters[1], t.parent.MetadataHash)
	if err == nil {
		return fmt.Errorf("vote attesting the replaced parent metadata was accepted")
	}
	log.Infow("vote attesting the replaced parent metadata rejected as expected", "error", err.Error())

	// votes attesting the new parent metadata, fetched by the client, are accepted
	for _, voter := range voters[1:] {
		if err := vote(voter, nil); err != nil {
			return fmt.Errorf("vote attesting the new parent metadata was rejected: %w", err)
		}
	}
	if err := t.verifyVoteCount(len(voters)); err != nil {
		return err
	}

	// the parent ends, but has no results to compute
	if _, err := t.api.SetElectionStatus(t.parent.ElectionID, "ENDED"); err != nil {
		return fmt.Errorf("cannot end the parent election: %w", err)
	}
	if _, err := t.api.WaitUntilElectionStatus(ctx, t.parent.ElectionID, "ENDED"); err != nil {
		return err
	}
	for range 3 {
		if err := t.api.WaitUntilNextBlock(); err != nil {
			return err
		}
	}
	ended, err := t.api.Election(t.parent.ElectionID)
	if err != nil {
		return err
	}
	if ended.Status != "ENDED" {
		return fmt.Errorf("parent election status is %s, want ENDED", ended.Status)
	}
	log.Infof("parent election %s status is ENDED", t.parent.ElectionID.String())

	if _, err := t.endElectionAndFetchResults(); err != nil {
		return err
	}
	log.Infof("election %s status is RESULTS", t.election.ElectionID.String())
	return nil
}

package main

import (
	"fmt"
	"os"

	vapi "go.vocdoni.io/dvote/api"
	"go.vocdoni.io/dvote/apiclient"
	"go.vocdoni.io/dvote/log"
	"go.vocdoni.io/dvote/types"
)

func init() {
	ops["memoelection"] = operation{
		testFunc: func() VochainTest {
			return &E2EMemoElection{}
		},
		description: "Casts votes carrying an optional free-text memo and verifies each memo round-trips through the API.",
		example:     os.Args[0] + " --operation=memoelection --votes=5",
	}
}

var _ VochainTest = (*E2EMemoElection)(nil)

type E2EMemoElection struct {
	e2eElection
}

func (t *E2EMemoElection) Setup(api *apiclient.HTTPclient, c *config) error {
	t.api = api
	t.config = c

	ed := newTestElectionDescription(2)
	ed.ElectionType = vapi.ElectionType{
		Autostart:     true,
		Interruptible: true,
	}
	ed.VoteType = vapi.VoteType{MaxVoteOverwrites: 1}
	ed.Census = vapi.CensusTypeDescription{Type: vapi.CensusTypeWeighted}

	if err := t.setupElection(ed, t.config.nvotes, true); err != nil {
		return err
	}

	logElection(t.election)
	return nil
}

func (*E2EMemoElection) Teardown() error {
	// nothing to do here
	return nil
}

func (t *E2EMemoElection) Run() error {
	// Give each voter a distinct memo, with one voter left empty to cover the
	// "empty memo is not stored -> reads back empty" path. Cast votes directly
	// (not via sendVotes, which discards the voteID) so we can read each back.
	type castVote struct {
		voteID types.HexBytes
		memo   string
	}
	var cast []castVote
	var idx int
	var castErr error
	t.voters.Range(func(_, value any) bool {
		acctp, ok := value.(acctProof)
		if !ok {
			return true
		}
		memo := fmt.Sprintf("e2e memo ✓ #%d", idx)
		if idx == 0 {
			memo = "" // one voter without a memo
		}
		voteID, err := t.api.Vote(&apiclient.VoteData{
			Election:     t.election,
			ProofMkTree:  acctp.proof,
			Choices:      []int{idx % 2},
			VoterAccount: acctp.account,
			Memo:         memo,
		})
		if err != nil {
			castErr = fmt.Errorf("could not cast vote %d: %w", idx, err)
			return false
		}
		cast = append(cast, castVote{voteID: voteID, memo: memo})
		idx++
		return true
	})
	if castErr != nil {
		return castErr
	}

	// Wait until every vote is committed and indexed before reading them back.
	if err := t.verifyVoteCount(t.config.nvotes); err != nil {
		return err
	}

	// Read each vote back and assert the memo round-tripped verbatim.
	for _, cv := range cast {
		got, err := t.api.GetVote(cv.voteID)
		if err != nil {
			return fmt.Errorf("could not fetch vote %s: %w", cv.voteID.String(), err)
		}
		if got.Memo != cv.memo {
			return fmt.Errorf("memo mismatch for vote %s: got %q, want %q",
				cv.voteID.String(), got.Memo, cv.memo)
		}
	}
	log.Infow("all vote memos verified", "n", len(cast))

	if _, err := t.endElectionAndFetchResults(); err != nil {
		return err
	}
	log.Infof("election %s status is RESULTS", t.election.ElectionID.String())
	return nil
}

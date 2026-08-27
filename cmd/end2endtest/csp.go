package main

import (
	"fmt"
	"math/big"
	"os"
	"time"

	"go.vocdoni.io/dvote/apiclient"
	"go.vocdoni.io/dvote/log"
	"go.vocdoni.io/dvote/test/testcommon/testcsp"
	"go.vocdoni.io/proto/build/go/models"
)

func init() {
	ops["cspelection"] = operation{
		testFunc: func() VochainTest {
			return &E2ECSPElection{}
		},
		description: "csp election",
		example:     os.Args[0] + " --operation=cspelection --votes=1000",
	}
	ops["cspelectionv2"] = operation{
		testFunc: func() VochainTest {
			return &E2ECSPElectionV2{}
		},
		description: "csp election with census origin OFF_CHAIN_CA_V2 and weighted blind pid-salted proofs (issue #1424 derivation)",
		example:     os.Args[0] + " --operation=cspelectionv2 --votes=1000",
	}
}

var _ VochainTest = (*E2ECSPElection)(nil)

type E2ECSPElection struct {
	e2eElection
}

func (t *E2ECSPElection) Setup(api *apiclient.HTTPclient, c *config) error {
	t.api = api
	t.config = c

	// setup for ranked voting
	p := newTestProcess()
	// update to use csp origin
	p.CensusOrigin = models.CensusOrigin_OFF_CHAIN_CA

	if err := t.setupElectionRaw(p); err != nil {
		return err
	}

	logElection(t.election)
	return nil
}

func (*E2ECSPElection) Teardown() error {
	// nothing to do here
	return nil
}

func (t *E2ECSPElection) Run() error {
	c := t.config

	// Send the votes (parallelized)
	startTime := time.Now()

	log.Infow("enqueuing votes", "n", t.config.nvotes, "election", t.election.ElectionID)
	votes := []*apiclient.VoteData{}

	t.voters.Range(func(key, value any) bool {
		if acctp, ok := value.(acctProof); ok {
			votes = append(votes, &apiclient.VoteData{
				Election:     t.election,
				ProofCSP:     acctp.proof.Proof,
				Choices:      []int{0},
				VoterAccount: acctp.account,
			})
		}
		return true
	})
	errs := t.sendVotes(votes, 5)
	if len(errs) > 0 {
		return fmt.Errorf("error in sendVotes %+v", errs)
	}

	log.Infow("votes submitted successfully",
		"n", c.nvotes, "time", time.Since(startTime),
		"vps", int(float64(c.nvotes)/time.Since(startTime).Seconds()))

	elres, err := t.verifyAndEndElection(t.config.nvotes)
	if err != nil {
		return err
	}

	log.Infof("election %s status is RESULTS", t.election.ElectionID.String())
	log.Infof("election results: %v", elres.Results)

	return nil
}

var _ VochainTest = (*E2ECSPElectionV2)(nil)

// E2ECSPElectionV2 runs a CSP election with census origin OFF_CHAIN_CA_V2,
// where every proof is ECDSA_BLIND_PIDSALTED with a CSP-authorized weight,
// exercising the fixed salt derivation of issue #1424 end to end.
type E2ECSPElectionV2 struct {
	E2ECSPElection
}

func (t *E2ECSPElectionV2) Setup(api *apiclient.HTTPclient, c *config) error {
	signer, err := testcsp.NewSigner()
	if err != nil {
		return err
	}
	t.cspSigner = signer
	t.cspProofType = models.ProofCA_ECDSA_BLIND_PIDSALTED
	t.cspVoteWeight = big.NewInt(10)

	t.api = api
	t.config = c

	p := newTestProcess()
	p.CensusOrigin = models.CensusOrigin_OFF_CHAIN_CA_V2

	if err := t.setupElectionRaw(p); err != nil {
		return err
	}

	logElection(t.election)
	return nil
}

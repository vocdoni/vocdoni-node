package ist

import (
	"fmt"

	"go.vocdoni.io/dvote/log"
	"go.vocdoni.io/dvote/vochain/results"
)

const (
	// BlocksToWaitForResults is the factor used to compute the
	// height at which the results will be committed to the state.
	// Note that changing this factor might break consensus and backwards
	// compatibility.
	BlocksToWaitForResults = 2
)

// commitResults commits the results to the state
func (c *Controller) commitResults(electionID []byte, r *results.Results) error {
	log.Infow("committing results", "electionID", fmt.Sprintf("%x", electionID))
	return c.state.SetProcessResults(electionID, results.ResultsToProto(r))
}

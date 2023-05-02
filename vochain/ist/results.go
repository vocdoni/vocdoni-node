package ist

import (
	"errors"
	"fmt"
	"time"

	"go.vocdoni.io/dvote/db"
	"go.vocdoni.io/dvote/log"
	"go.vocdoni.io/dvote/vochain/results"
)

const (
	// BlocksToWaitForResultsFactor is the factor used to compute the
	// height at which the results will be committed to the state.
	// The factor represents the number of votes required to compute
	// in the time window of one blockchain block.
	// For example, if the factor is 500, and there are 1000 votes,
	// the results will be committed two blocks after.
	// Note that changing this factor might break consensus and backwards
	// compatibility.
	BlocksToWaitForResultsFactor = 5000

	// ExtraWaitSecondsForResults is the number of seconds extra to wait for the results
	ExtraWaitSecondsForResults = time.Second * 20
)

func (c *Controller) scheduleCommitResults(electionID []byte) error {
	commitHeight, err := c.computeResultsCommitHeight(electionID)
	if err != nil {
		return fmt.Errorf("cannot compute results commit height: %w", err)
	}
	if err := c.Schedule(commitHeight, electionID, Action{
		ID:         ActionCommitResults,
		ElectionID: electionID,
	}); err != nil {
		return fmt.Errorf("cannot schedule results commit: %w", err)
	}
	return nil
}

// computeAndStoreResults computes the results for an election.
// The results will be stored in the no-state once available.
func (c *Controller) computeAndStoreResults(electionID []byte) error {
	r, err := results.ComputeResults(electionID, c.state)
	if err != nil {
		return fmt.Errorf("cannot compute results: %w", err)
	}
	rEncoded, err := r.Encode()
	if err != nil {
		return fmt.Errorf("cannot encode results: %w", err)
	}
	if err := c.storeToNoState(dbResultsIndex(electionID), rEncoded); err != nil {
		return fmt.Errorf("cannot store results: %w", err)
	}
	return nil
}

// commitResults commits the results to the state. If the reults parameter is nil, it will
// try to get the results from the no-state. If the results are not available, it will
// compute them again.
func (c *Controller) commitResults(electionID []byte, r *results.Results) error {
	if r == nil {
		var rEncoded []byte
		startTime := time.Now()
		var err error
		for {
			// For safety we compute the results again if the time is up.
			if time.Since(startTime) > ExtraWaitSecondsForResults {
				log.Warn("results not available on commit, recomputing")
				r, err = results.ComputeResults(electionID, c.state)
				if err != nil {
					return err
				}
				break
			}
			rEncoded, err = c.retrieveFromNoState(dbResultsIndex(electionID))
			if err != nil {
				if errors.Is(err, db.ErrKeyNotFound) {
					// Wait until the results are available
					log.Debugf("results for electionID %x not found, remaining extra waiting time %s",
						electionID, ExtraWaitSecondsForResults-time.Since(startTime))
					// Wait before retrying
					time.Sleep(time.Millisecond * 1000)
					continue
				}
				return fmt.Errorf("cannot get results from no-state: %w", err)
			}
			r = new(results.Results)
			if err := r.Decode(rEncoded); err != nil {
				return fmt.Errorf("cannot decode results: %w", err)
			}
			break
		}
	}

	// Delete the stored results
	_ = c.deleteFromNoState(dbResultsIndex(electionID))

	log.Infow("committing results", "electionID", fmt.Sprintf("%x", electionID))
	return c.state.SetProcessResults(electionID, results.ResultsToProto(r))
}

// computeResultsCommitHeight computes the height at which the results will be
// committed to the state. The formula is:
// currentHeight + 2 + (number of votes / BlocksToWaitForResultsFactor)
func (c *Controller) computeResultsCommitHeight(electionID []byte) (uint32, error) {
	nvotes, err := c.state.CountVotes(electionID, true)
	if err != nil {
		return 0, err
	}
	return c.state.CurrentHeight() + 2 + uint32(nvotes/BlocksToWaitForResultsFactor), nil
}

package ist

import (
	"bytes"
	"fmt"
)

/*
This mechanism manages validator power based on how often each validator participates in block
production over a fixed rolling window of `updatePowerPeriod` blocks.

As a general idea, when a validator does not participate in block production, its power decays over
time. When a validator does participate, its power increases until it reaches the maximum. When a new
validator joins the set, it starts with `newValidatorPower`.

Parameters:

 1. `maxPower`: maximum power any validator can achieve.
 2. `minPower`: floor at which a validator with a persistently negative signal is pinned before eviction.
    A validator kept at this floor still signs blocks but contributes only `minPower/maxPower` (≈0.1%) to
    consensus weight; effectively a "shadow" seat.
 3. `updatePowerPeriod`: frequency (in blocks) at which power and score are recomputed. Also the length
    of the rolling window used to compute the score.
 4. `positiveScoreThreshold`: score at or above which a validator's power increases.
 5. `powerIncrement`: amount added per period on a positive score signal.
 6. `powerDecayRate`: exponential decay per period on a negative signal.
 7. `inactiveGraceBlocks`: distance a validator must remain at the floor before being removed from the
    set. Re-adding a validator is a manual, coordinated operation; a several-day tolerance is cheap and
    preferred over premature eviction.

Workflow (only at heights that are a multiple of `updatePowerPeriod`):

  - `models.Validator.Height` is the join height and `models.Validator.Votes` is the lifetime count
    of blocks the validator signed. Both are canonical, never reset, and exposed as `joinHeight` and
    `votes` on `GET /chain/validators`. The score-window boundary — the height and lifetime-Votes
    value seen at the last score computation — lives in TreeExtra under `vldSW/`.
  - The score is `(Votes - votesAtWindowStart) / (currentHeight - windowStartHeight) * 100`, i.e.
    the percentage of blocks the validator attended during the just-closed window. On the first
    period after a validator joins the set, the fallback boundary is `(v.Height, 0)`, so the ratio
    still reflects that partial first window. This keeps the score fully reactive to recent
    behaviour and prevents a long-lived validator's lifetime average from masking a fresh outage.
  - If the score is above or equal to `positiveScoreThreshold` or has improved, the power is incremented
    by `powerIncrement`, capped at `maxPower`.
  - If the score dropped or is zero, the power decays at `powerDecayRate`, floored at `minPower`.
  - The inactive-since marker is set to the current height the first period the validator reaches the
    floor, and cleared the first period it climbs back above.
  - A validator whose marker is at least `inactiveGraceBlocks` behind the current height is removed
    from the set, unless doing so would leave fewer than three validators.

Simulations (12s block time):

  - Silent validator (regardless of prior lifetime): score drops to 0 the first period after silence
    begins, and every subsequent period; power drops by 10% per period. With the uint64 truncation
    applied at each step, a validator starting at `maxPower` (1000) reaches `minPower` (1) in 49
    periods ≈ 490 blocks ≈ 1h 38min (pinned by `TestFloorReachedInExpectedPeriodCount`).
  - Once at the floor, the validator stays in the set for `inactiveGraceBlocks` before being evicted,
    about 21 days at the current parameters.
  - A recovering validator reaches `maxPower` from `newValidatorPower` in ~95 periods ≈ 950 blocks ≈ 3h 10min.
*/

const (
	maxPower               = 1000 // maximum power of a validator
	minPower               = 1    // minimum power floor for a validator inside the grace period
	updatePowerPeriod      = 10   // number of blocks to wait before updating validators power
	positiveScoreThreshold = 80   // if this minimum score is kept, the validator power will be increased
	powerIncrement         = 10   // amount added per period on a positive score signal (1% of maxPower)
	powerDecayRate         = 0.10 // 10% decay rate per period on a negative score signal
)

// inactiveGraceBlocks is the block distance a validator must remain at
// minPower before being removed from the set (~21d at 12s/block). Exposed as
// a var so tests can shorten it; production code never mutates it.
var inactiveGraceBlocks uint32 = 150_000

func (c *Controller) updateValidatorScore(voteAddresses [][]byte, proposer []byte) error {
	// Clear any pubkeys buffered by a prior run in this block (handles the
	// rare FinalizeBlock re-execution path when ProcessProposal's cached
	// results are invalidated by a hash mismatch).
	c.removedPubKeys = nil

	validators, err := c.state.Validators(true)
	if err != nil {
		return fmt.Errorf("cannot update validator score: %w", err)
	}
	// Attribute this block's votes and proposer.
	for _, voteAddr := range voteAddresses {
		for k, v := range validators {
			if bytes.Equal(voteAddr, v.ValidatorAddress) {
				validators[k].Votes++
				if bytes.Equal(proposer, v.ValidatorAddress) {
					validators[k].Proposals++
				}
				break
			}
		}
	}

	height := c.state.CurrentHeight()
	updatePeriod := height%updatePowerPeriod == 0

	for idx, v := range validators {
		if updatePeriod {
			// Score is the participation rate over the just-closed window. The
			// window boundary (height and lifetime-Votes count at the last
			// score computation) is stored in TreeExtra under vldSW/; the
			// canonical models.Validator.Height (join height) and .Votes
			// (lifetime signature count) are never mutated by scoring.
			windowStart, votesAtStart, hasWindow, err := c.state.ValidatorScoreWindow(v.Address, true)
			if err != nil {
				return fmt.Errorf("cannot read validator score window: %w", err)
			}
			if !hasWindow {
				// First period after joining the set: use the join height and
				// zero lifetime-votes as the boundary.
				windowStart = uint32(v.Height)
				votesAtStart = 0
			}
			gap := height - windowStart
			if gap == 0 {
				gap = updatePowerPeriod
			}
			windowVotes := v.Votes - votesAtStart
			newScore := uint32(float64(windowVotes) / float64(gap) * 100)
			switch {
			case newScore > v.Score ||
				(newScore >= positiveScoreThreshold && v.Score == newScore):
				v.Power = min(v.Power+powerIncrement, maxPower)
			case newScore < v.Score || newScore == 0:
				v.Power = max(uint64(float64(v.Power)*(1-powerDecayRate)), minPower)
			}
			v.Score = newScore
			// Advance the window so the next score reflects only the next
			// updatePowerPeriod blocks. The canonical Height/Votes on
			// models.Validator are intentionally left untouched here so the
			// public /chain/validators response keeps returning the join height
			// and lifetime vote count.
			if err := c.state.SetValidatorScoreWindow(v.Address, height, v.Votes); err != nil {
				return fmt.Errorf("cannot set validator score window: %w", err)
			}

			since, marked, err := c.state.ValidatorInactiveSince(v.Address, true)
			if err != nil {
				return fmt.Errorf("cannot read validator inactive-since: %w", err)
			}
			switch {
			case v.Power <= minPower && !marked:
				if err := c.state.SetValidatorInactiveSince(v.Address, height); err != nil {
					return fmt.Errorf("cannot set validator inactive-since: %w", err)
				}
			case v.Power > minPower && marked:
				if err := c.state.ClearValidatorInactiveSince(v.Address); err != nil {
					return fmt.Errorf("cannot clear validator inactive-since: %w", err)
				}
			case v.Power <= minPower && marked && height-since >= inactiveGraceBlocks:
				if len(validators) > 3 {
					if err := c.state.RemoveValidator(v); err != nil {
						return fmt.Errorf("cannot remove validator: %w", err)
					}
					if err := c.state.ClearValidatorInactiveSince(v.Address); err != nil {
						return fmt.Errorf("cannot clear validator inactive-since: %w", err)
					}
					if err := c.state.ClearValidatorScoreWindow(v.Address); err != nil {
						return fmt.Errorf("cannot clear validator score window: %w", err)
					}
					// Record the pubkey so FinalizeBlock can emit the ABCI
					// Power=0 ValidatorUpdate required to actually remove this
					// validator from CometBFT's internal ValidatorSet.
					c.removedPubKeys = append(c.removedPubKeys, bytes.Clone(v.PubKey))
					delete(validators, idx)
					continue
				}
				// Never leave the set below three validators, even past the grace period.
			}
		}
		if err := c.state.AddValidator(v); err != nil {
			return fmt.Errorf("cannot update validator score: %w", err)
		}
	}
	return nil
}

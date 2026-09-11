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

  - The score is `Votes / (currentHeight - Height) * 100`, i.e. the percentage of blocks the validator
    attended during the just-closed window. After scoring, `Votes` is reset to 0 and `Height` is set
    to the current height, so the next window starts fresh. This keeps the score fully reactive to
    recent behaviour and prevents a long-lived validator's lifetime average from masking a fresh
    outage.
  - If the score is above or equal to `positiveScoreThreshold` or has improved, the power is incremented
    by `powerIncrement`, capped at `maxPower`.
  - If the score dropped or is zero, the power decays at `powerDecayRate`, floored at `minPower`.
  - The inactive-since marker is set to the current height the first period the validator reaches the
    floor, and cleared the first period it climbs back above.
  - A validator whose marker is at least `inactiveGraceBlocks` behind the current height is removed
    from the set, unless doing so would leave fewer than three validators.

Simulations (12s block time):

  - Silent validator (regardless of prior lifetime): score drops to 0 the first period after silence
    begins, and every subsequent period; power halves-and-a-bit per period until it reaches `minPower`
    in ~66 periods ≈ 660 blocks ≈ 2h 12min.
  - Once at the floor, the validator stays in the set for `inactiveGraceBlocks` before being evicted,
    about 21 days at the current parameters.
  - A recovering validator reaches `maxPower` from `newValidatorPower` in ~100 periods ≈ 1000 blocks ≈ 3h 20min.
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
var inactiveGraceBlocks uint32 = 180

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
			// Score is the participation rate over the just-closed window.
			// `Height` and `Votes` are reset at the end of this branch so the
			// denominator here is always `updatePowerPeriod` after the first
			// period, and equals the actual gap during any partial first
			// window (e.g. a validator joined mid-period).
			gap := height - uint32(v.Height)
			if gap == 0 {
				gap = updatePowerPeriod
			}
			newScore := uint32(float64(v.Votes) / float64(gap) * 100)
			switch {
			case newScore > v.Score ||
				(newScore >= positiveScoreThreshold && v.Score == newScore):
				v.Power = min(v.Power+powerIncrement, maxPower)
			case newScore < v.Score || newScore == 0:
				v.Power = max(uint64(float64(v.Power)*(1-powerDecayRate)), minPower)
			}
			v.Score = newScore
			// Reset the window so the next score reflects only the next
			// updatePowerPeriod blocks. Proposals is left cumulative on purpose
			// (metric-only, not consensus-critical).
			v.Height = uint64(height)
			v.Votes = 0

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

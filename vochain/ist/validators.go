package ist

import (
	"bytes"
	"encoding/hex"
	"fmt"
	"slices"

	"go.vocdoni.io/proto/build/go/models"
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

Workflow:

Every block runs a cleanup pass that reaps validators tombstoned by a previous tick's eviction
(state leaves carrying `Power == 0`). CometBFT already ejected them on the previous FinalizeBlock —
validatorUpdate emitted their Power=0 leaf naturally — so the cleanup pass deletes the state leaf
and clears the per-validator IST markers. Routing eviction through this two-tick tombstone keeps
state and CometBFT's ValidatorSet consistent without any side-channel: state IS the source of
truth for what validatorUpdate emits.

Every `updatePowerPeriod`-boundary block, in addition, runs the score/power update:

  - `models.Validator.Height` is the join height and `models.Validator.Votes` is the lifetime count
    of blocks the validator signed. Both are canonical, never reset, and exposed as `joinHeight` and
    `votes` on `GET /chain/validators`. The score-window boundary — the height and lifetime-Votes
    value seen at the last score computation — lives in TreeExtra under `vldSW/`.
  - The score is `(Votes - votesAtWindowStart) / (currentHeight - windowStartHeight) * 100`, i.e.
    the percentage of blocks the validator attended during the just-closed window. On the first
    period a validator ever sees under this scheme (no vldSW/ entry yet — fresh add or first
    boundary after a chain-restart activation), the boundary is seeded at `(height, v.Votes)` and
    the score/power branches are skipped. The next period produces the first real score against a
    full fresh window. Without this skip, the fallback would reconstruct a lifetime-average score
    from the canonical `v.Votes` and `v.Height`, spuriously rewarding validators whose accumulated
    Votes reflect activity from a pre-activation rule set.
  - If the score is above or equal to `positiveScoreThreshold` or has improved, the power is incremented
    by `powerIncrement`, capped at `maxPower`.
  - If the score dropped or is zero, the power decays at `powerDecayRate`, floored at `minPower`.
  - The inactive-since marker is set to the current height the first period the validator reaches the
    floor, and cleared the first period it climbs back above.
  - Validators past `inactiveGraceBlocks` at the floor become eviction candidates. Up to
    `len(validators) - 3` of them are tombstoned per tick, in sorted-by-hex(Address) order — the
    deterministic order is critical for consensus when several validators pass grace in the same
    tick. Tombstoning a validator is `v.Power = 0` persisted to state; the next FinalizeBlock's
    validatorUpdate emits the ABCI Power=0 signal and the next block's cleanup pass reaps the leaf.

Simulations (12s block time):

  - Silent validator (regardless of prior lifetime): score drops to 0 the first scored period after
    silence begins, and every subsequent period; power drops by 10% per period. With the uint64
    truncation applied at each step, a validator starting at `maxPower` (1000) reaches `minPower`
    (1) after 49 scored decay periods; adding the initial seed-only period, 50 total periods ≈ 500
    blocks ≈ 1h 40min (pinned by `TestFloorReachedInExpectedPeriodCount`).
  - Once at the floor, the validator stays in the set for `inactiveGraceBlocks` before being tombstoned,
    about 21 days at the current parameters. State cleanup follows on the very next block.
  - A recovering validator reaches `maxPower` from `newValidatorPower` in ~95 scored periods (plus
    the seed period) ≈ 960 blocks ≈ 3h 12min.
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
	// Read uncommitted state so a validator re-added earlier in the same
	// block via SET_ACCOUNT_VALIDATOR is visible here — otherwise Pass 1
	// would silently reap the fresh leaf on top of a stale Power=0 seen
	// only in the committed tree.
	validators, err := c.state.Validators(false)
	if err != nil {
		return fmt.Errorf("cannot update validator score: %w", err)
	}

	// Attribute this block's votes and proposer. Build a byValAddr lookup so
	// the loop is O(votes + validators) instead of O(votes × validators).
	byValAddr := make(map[string]*models.Validator, len(validators))
	for _, v := range validators {
		byValAddr[string(v.ValidatorAddress)] = v
	}
	// modified tracks which validator entries actually changed this block, so
	// non-update-period ticks only rewrite the leaves that received a Votes or
	// Proposals bump instead of re-marshaling every validator every block.
	modified := make(map[string]bool, len(voteAddresses))
	for _, voteAddr := range voteAddresses {
		v, ok := byValAddr[string(voteAddr)]
		if !ok {
			continue
		}
		v.Votes++
		if bytes.Equal(proposer, v.ValidatorAddress) {
			v.Proposals++
		}
		modified[hex.EncodeToString(v.Address)] = true
	}

	height := c.state.CurrentHeight()
	updatePeriod := height%updatePowerPeriod == 0

	// Pass 1 (every block): reap any validators tombstoned by a previous
	// tick's eviction. A tombstone is a state leaf carrying Power=0; the
	// previous FinalizeBlock already forwarded that Power=0 to CometBFT via
	// validatorUpdate, so CometBFT has ejected it from its ValidatorSet.
	// state.RemoveValidator drops the leaf and clears the per-validator IST
	// markers atomically. This is what makes eviction consistent with
	// CometBFT's set without any side-channel: state IS the source of truth
	// for what validatorUpdate emits.
	for idx, v := range validators {
		if v.Power == 0 {
			if err := c.state.RemoveValidator(v); err != nil {
				return fmt.Errorf("cannot remove tombstoned validator: %w", err)
			}
			delete(validators, idx)
			delete(modified, idx)
		}
	}

	// Pass 2 (update-period only): compute score, adjust power, maintain the
	// inactive-since marker. Eviction candidates are collected here but not
	// tombstoned yet — tombstoning is a separate deterministic pass so it
	// does not depend on Go's randomised map iteration order.
	var candidates []string
	if updatePeriod {
		for idx, v := range validators {
			windowStart, votesAtStart, hasWindow, err := c.state.ValidatorScoreWindow(v.Address, false)
			if err != nil {
				return fmt.Errorf("cannot read validator score window: %w", err)
			}
			// A stale window boundary from a previous life of this address
			// (validator was tombstoned + reaped + re-added; state.Validators
			// carried the stale marker if the re-add path bypassed the reap)
			// or a legacy activation where hasWindow=true but the boundary
			// predates v.Height, both must be treated as "no window" so the
			// seed branch takes over and starts fresh.
			if hasWindow && windowStart < uint32(v.Height) {
				hasWindow = false
			}
			if !hasWindow {
				// First scoring period for this validator (fresh add or the
				// first tick after a chain-restart activation). Seed the
				// window boundary at the validator's current lifetime state
				// and clear any stale inactive-since marker, then skip the
				// score branch: the next period will produce the first real
				// score against a full, fresh window. Without this, the
				// fallback would reconstruct a lifetime-average score from
				// the canonical v.Votes / v.Height, spuriously rewarding
				// validators whose accumulated Votes reflect activity from
				// a pre-activation rule set.
				if err := c.state.SetValidatorScoreWindow(v.Address, height, v.Votes); err != nil {
					return fmt.Errorf("cannot seed validator score window: %w", err)
				}
				if err := c.state.ClearValidatorInactiveSince(v.Address); err != nil {
					return fmt.Errorf("cannot clear validator inactive-since: %w", err)
				}
				if err := c.state.AddValidator(v); err != nil {
					return fmt.Errorf("cannot update validator score: %w", err)
				}
				delete(modified, idx)
				continue
			}
			// After a cometbft rollback or a chain restart whose InitialHeight
			// is below a previously recorded window boundary, the boundary
			// values can land ahead of the current chain state. Guard the two
			// uint underflows before they wrap to ~4e9 and score a zero
			// participation on every validator each period.
			var gap uint32
			if windowStart >= height {
				gap = updatePowerPeriod
			} else {
				gap = height - windowStart
			}
			var windowVotes uint64
			if v.Votes >= votesAtStart {
				windowVotes = v.Votes - votesAtStart
			}
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

			since, marked, err := c.state.ValidatorInactiveSince(v.Address, false)
			if err != nil {
				return fmt.Errorf("cannot read validator inactive-since: %w", err)
			}
			isCandidate := false
			switch {
			case v.Power <= minPower && !marked:
				if err := c.state.SetValidatorInactiveSince(v.Address, height); err != nil {
					return fmt.Errorf("cannot set validator inactive-since: %w", err)
				}
			case v.Power > minPower && marked:
				if err := c.state.ClearValidatorInactiveSince(v.Address); err != nil {
					return fmt.Errorf("cannot clear validator inactive-since: %w", err)
				}
			// height >= since guards against uint32 underflow after a cometbft
			// rollback / low InitialHeight: without it the subtraction wraps
			// to ~4e9 and every marked validator is evicted with no grace.
			case v.Power <= minPower && marked && height >= since && height-since >= inactiveGraceBlocks:
				candidates = append(candidates, idx)
				isCandidate = true
			}
			// Score / power / marker changed on every boundary iteration —
			// always persist. Skip when the validator is a candidate: Pass 3
			// is about to overwrite the leaf with Power=0.
			if !isCandidate {
				if err := c.state.AddValidator(v); err != nil {
					return fmt.Errorf("cannot update validator score: %w", err)
				}
			}
			delete(modified, idx)
		}
	}

	// Non-update-period blocks: persist only the validators whose Votes or
	// Proposals were bumped in the attribution loop above. Other leaves are
	// bit-identical to their committed value; rewriting them would burn CPU
	// on proto.Marshal and cache lines on Pebble WAL for no state change.
	for idx := range modified {
		if err := c.state.AddValidator(validators[idx]); err != nil {
			return fmt.Errorf("cannot update validator score: %w", err)
		}
	}

	// Pass 3 (update-period only): tombstone up to (len - 3) candidates in
	// sorted-by-hex(Address) order. Deterministic ordering is critical for
	// consensus: without it, when several validators pass the grace window
	// in the same tick, different nodes would pick different subsets to
	// evict and produce divergent AppHash.
	//
	// A tombstone is v.Power = 0 persisted to state. The next FinalizeBlock
	// forwards it to CometBFT via validatorUpdate (Power=0 leaves are
	// naturally emitted); CometBFT ejects; pass 1 of the next IST tick
	// reaps the leaf and clears markers. Clear the inactive-since marker
	// alongside the tombstone so the reap-tick view is atomic in intent
	// even before the reap actually runs.
	if slots := len(validators) - 3; slots > 0 && len(candidates) > 0 {
		slices.Sort(candidates)
		if slots > len(candidates) {
			slots = len(candidates)
		}
		for _, idx := range candidates[:slots] {
			validators[idx].Power = 0
			if err := c.state.ClearValidatorInactiveSince(validators[idx].Address); err != nil {
				return fmt.Errorf("cannot clear validator inactive-since: %w", err)
			}
			if err := c.state.AddValidator(validators[idx]); err != nil {
				return fmt.Errorf("cannot tombstone validator: %w", err)
			}
		}
	}
	return nil
}

package ist

import (
	"bytes"
	"encoding/hex"
	"fmt"

	"go.vocdoni.io/dvote/log"
)

/*
This mechanism is responsible for the management and updating of validator power based on their voting and proposing performance over time.
It operates by evaluating the performance of validators in terms of votes on proposals they have accrued.

As a general idea, when a validator does not participate on the block production, their power will decay over time.
On the other hand, if a validator participates in the block production, their power will increase over time until it reaches the maximum power.
When a new validator joins the validator set, it starts with the minimum power (currently 5).

The mechanism is based on the following parameters:

1. `maxPower`: This represents the maximum power that any validator can achieve. Currently set to 100.

2. `updatePowerPeriod`: The frequency with which the validator power and score updates are made.
    Power adjustments for each validator are carried out once every 10 blocks.

3. `positiveScoreThreshold`: Validators need to maintain a score equal to or above this threshold for their power to increase.
    The threshold is currently set at 80.

4. `powerDecayRate`: If a validator underperforms, i.e., if their new score is lower than their previous score or if it's zero,
    their power will decay at this rate. The current rate is set to 5%.

Workflow:

- Every `updatePowerPeriod` blocks, the mechanism calculates a new score for each validator based on their voting performance.
- If a validator's score is above or equal to the `positiveScoreThreshold` or if it has improved, their power is incremented by 1, until the `maxPower` is reached.
- If a validator's score drops or is zero, their power is subjected to decay at the rate of `powerDecayRate`.
- Finally, if a validator's power becomes zero (and there are more than 3 validators), they are removed from the validator set. Otherwise, their updated state is stored back.


Simulations:

- Scenario A: Validator stops working after 100,000 blocks:

With 5% exponential decay and considering an update every 10 blocks:
Approximately 1,320 blocks (or 132 adjustment periods) to decrease power from 100 to 0.

- Scenario B: A new validator starts working after 100,000 blocks:

The validator would take approximately 20,000 blocks (or 2,000 adjustment periods) to reach the maximum power of 100,
assuming they maintain an ideal score throughout.
*/

const (
	maxPower               = 100  // maximum power of a validator
	updatePowerPeriod      = 10   // number of blocks to wait before updating validators power
	positiveScoreThreshold = 80   // if this minimum score is kept, the validator power will be increased
	powerDecayRate         = 0.05 // 5% decay rate
)

func (c *Controller) updateValidatorScore(voteAddresses [][]byte, proposer []byte) error {
	// get the validators
	validators, err := c.state.Validators(true)
	if err != nil {
		return fmt.Errorf("cannot update validator score: %w", err)
	}
	// get the validator score
	log.Debugw("update validator score", "totalVoters", len(voteAddresses), "proposer", hex.EncodeToString(proposer))
	for _, voteAddr := range voteAddresses {
		validator := ""
		for k, v := range validators {
			if bytes.Equal(voteAddr, v.ValidatorAddress) {
				validator = k
				break
			}
		}
		if validator != "" {
			validators[validator].Votes++
			if bytes.Equal(proposer, validators[validator].ValidatorAddress) {
				validators[validator].Proposals++
			}
		}
	}
	// compute the new power and score
	for idx := range validators {
		if c.state.CurrentHeight()%updatePowerPeriod == 0 {
			newScore := uint32(float64(validators[idx].Votes) /
				float64(c.state.CurrentHeight()-uint32(validators[idx].Height)) * 100)
			if newScore > validators[idx].Score ||
				(newScore >= positiveScoreThreshold && validators[idx].Score == newScore) {
				if validators[idx].Power < maxPower {
					validators[idx].Power++
				}
			}
			if newScore < validators[idx].Score || newScore == 0 {
				validators[idx].Power = uint64(float64(validators[idx].Power) * (1 - powerDecayRate))
			}
			validators[idx].Score = newScore
		}
		// update or remove the validator
		if validators[idx].Power <= 0 {
			if len(validators) <= 3 {
				// cannot remove the last 3 validators
				validators[idx].Power = 1
			} else {
				if err := c.state.RemoveValidator(validators[idx]); err != nil {
					return fmt.Errorf("cannot remove validator: %w", err)
				}
				continue
			}
		}
		if err := c.state.AddValidator(validators[idx]); err != nil {
			return fmt.Errorf("cannot update validator score: %w", err)
		}
	}
	return nil
}

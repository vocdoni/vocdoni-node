package ist

import (
	"bytes"
	"fmt"
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
	powerDecayRate         = 0.20 // 20% decay rate
)

func (c *Controller) updateValidatorScore(voteAddresses [][]byte, proposer []byte) error {
	// get the validators
	validators, err := c.state.Validators(true)
	if err != nil {
		return fmt.Errorf("cannot update validator score: %w", err)
	}
	// update Votes and Proposals of each validator found in voteAddresses
	for _, voteAddr := range voteAddresses {
		for _, v := range validators {
			if bytes.Equal(voteAddr, v.ValidatorAddress) {
				v.Votes++
				if bytes.Equal(proposer, v.ValidatorAddress) {
					v.Proposals++
				}
				break
			}
		}
	}
	if c.state.CurrentHeight()%updatePowerPeriod != 0 {
		return nil
	}

	// compute the new power and score
	for _, v := range validators {
		// if a validator voted on every block since they joined the validator list,
		// they will have a score of 100
		// if they voted on only half of the blocks, they will have a score of 50
		newScore := uint32(float64(v.Votes) /
			float64(c.state.CurrentHeight()-uint32(v.Height)) * 100)
		if newScore > v.Score ||
			(newScore >= positiveScoreThreshold && v.Score == newScore) {
			// validators that contributed lots of votes to the network,
			// regain a significant share of their power after downtime
			contributionSinceGenesis := float64(v.Votes) / float64(c.state.CurrentHeight())
			if v.Power < uint64(contributionSinceGenesis*maxPower) {
				v.Power = uint64(contributionSinceGenesis * maxPower)
			}
			if v.Power < maxPower {
				v.Power++
			}
			if v.Power > maxPower {
				v.Power = maxPower
			}
		}
		if newScore < v.Score || newScore == 0 {
			v.Power = uint64(float64(v.Power) * (1 - powerDecayRate))
		}
		v.Score = newScore
		// update or remove the validator
		if v.Power <= 0 {
			if len(validators) <= 3 {
				// cannot remove the last 3 validators
				v.Power = 1
			} else {
				if err := c.state.RemoveValidator(v); err != nil {
					return fmt.Errorf("cannot remove validator: %w", err)
				}
				continue
			}
		}
		if err := c.state.AddValidator(v); err != nil {
			return fmt.Errorf("cannot update validator score: %w", err)
		}
	}

	return nil
}

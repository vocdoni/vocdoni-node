package indexertypes

import (
	"bytes"
	"fmt"
	"math/big"
	"sync"

	"go.vocdoni.io/dvote/types"
	"go.vocdoni.io/proto/build/go/models"
)

const (
	// MaxQuestions is the maximum number of questions allowed in a VotePackage
	MaxQuestions = 64
	// MaxOptions is the maximum number of options allowed in a VotePackage question
	MaxOptions = 128
)

// Results holds the final results and relevant process info for a vochain process
type Results struct {
	ProcessID      types.HexBytes             `badgerholdKey:"ProcessID" json:"processId"`
	Votes          [][]*big.Int               `json:"votes"`
	Weight         *big.Int                   `json:"weight"`
	EnvelopeHeight uint64                     `json:"envelopeHeight"`
	EnvelopeType   *models.EnvelopeType       `json:"envelopeType"`
	VoteOpts       *models.ProcessVoteOptions `json:"voteOptions"`
	Signatures     []types.HexBytes           `json:"signatures"`
	Final          bool                       `json:"final"`
	BlockHeight    uint32                     `json:"blockHeight"`
}

// String formats the results in a human-readable string
func (r *Results) String() string {
	results := bytes.Buffer{}
	for _, q := range r.Votes {
		results.WriteString(" [")
		for j, o := range q {
			results.WriteString(o.String())
			if j < len(q)-1 {
				results.WriteString(",")
			}
		}
		results.WriteString("]")
	}
	return results.String()
}

// Add adds the total weight and votes from the given Results to the
// containing Results making the method call.
func (r *Results) Add(new *Results) error {
	r.Weight.Add(r.Weight, new.Weight)
	if new.BlockHeight > r.BlockHeight {
		r.BlockHeight = new.BlockHeight
	}
	r.EnvelopeHeight += new.EnvelopeHeight
	// Update votes only if present
	if len(new.Votes) == 0 {
		return nil
	}
	if len(new.Votes) != len(r.Votes) {
		return fmt.Errorf("results.Add: incorrect number of fields")
	}
	for i := range new.Votes {
		if len(r.Votes[i]) < len(new.Votes[i]) {
			return fmt.Errorf("results.Add: values overflow (%d)", i)
		}
		// Example:
		//	[ [1,2,3], [0,1,5] ]
		//	[ [2,1,1], [1,0,2] ]
		//	+ -----------------
		//	[ [3,3,4], [1,1,7] ]
		for j := range new.Votes[i] {
			r.Votes[i][j].Add(r.Votes[i][j], new.Votes[i][j])
		}
	}
	return nil
}

// AddVote adds the voteValues and weight to the Results struct.
// Checks are performed according the Ballot Protocol.
func (r *Results) AddVote(voteValues []int, weight *big.Int, mutex *sync.Mutex) error {
	if r.VoteOpts == nil {
		return fmt.Errorf("addVote: processVoteOptions is nil")
	}

	if len(r.Votes) > 0 && len(r.Votes) != int(r.VoteOpts.MaxCount) {
		return fmt.Errorf("addVote: currentResults size mismatch %d != %d",
			len(r.Votes), r.VoteOpts.MaxCount)
	}

	if r.EnvelopeType == nil {
		return fmt.Errorf("addVote: envelopeType is nil")
	}
	// MaxCount
	if len(voteValues) > int(r.VoteOpts.MaxCount) || len(voteValues) > MaxOptions {
		return fmt.Errorf("max count overflow %d", len(voteValues))
	}

	// UniqueValues
	if r.EnvelopeType.UniqueValues {
		votes := make(map[int]bool, len(voteValues))
		for _, v := range voteValues {
			if votes[v] {
				return fmt.Errorf("values are not unique")
			}
			votes[v] = true
		}
	}

	// Max Value, check it only if greather than zero
	if r.VoteOpts.MaxValue > 0 {
		for _, v := range voteValues {
			if uint32(v) > r.VoteOpts.MaxValue {
				return fmt.Errorf("max value overflow %d", v)
			}
		}
	}

	// Max total cost
	if r.VoteOpts.MaxTotalCost > 0 || r.EnvelopeType.CostFromWeight {
		cost := new(big.Int).SetUint64(0)
		exponent := new(big.Int).SetUint64(uint64(r.VoteOpts.CostExponent))
		var maxCost *big.Int
		if r.EnvelopeType.CostFromWeight {
			maxCost = weight
		} else {
			maxCost = new(big.Int).SetUint64(uint64(r.VoteOpts.MaxTotalCost))
		}
		for _, v := range voteValues {
			cost.Add(cost, new(big.Int).Exp(new(big.Int).SetUint64(uint64(v)), exponent, nil))
			if cost.Cmp(maxCost) > 0 {
				return fmt.Errorf("max total cost overflow: %s", cost)
			}
		}
	}

	// If Mutex provided, Lock it
	if mutex != nil {
		mutex.Lock()
		defer mutex.Unlock()
	}

	// If weight not provided, assume weight = 1
	if weight == nil {
		weight = new(big.Int).SetUint64(1)
	}

	// Add the Election weight (tells how much voting power have already been processed)
	r.Weight.Add(r.Weight, weight)
	if len(r.Votes) == 0 {
		r.Votes = NewEmptyVotes(int(r.VoteOpts.MaxCount), int(r.VoteOpts.MaxValue)+1)
	}
	// Increase EnvelopeHeight by the number of votes added
	r.EnvelopeHeight++

	// If MaxValue is zero, consider discrete value couting. So for each questoin, the value
	// is aggregated. The weight is multiplied for the value if costFromWeight=False.
	// This is a special case for Quadratic voting where maxValue should be 0 (no limit).
	// The results are aggregated, so we use only the first column of the results matrix.
	if r.VoteOpts.MaxValue == 0 {
		// If CostFromWeight, the Weight is used for computing the cost and not as a value multiplier
		if r.EnvelopeType.CostFromWeight {
			weight = new(big.Int).SetUint64(1)
		}
		// Example if maxValue=0 and CostFromWeight=false
		// Vote1: [1, 2, 3] w=10
		// Vote2: [0, 3, 1] w=5
		//  [ [1*10+0*5], [2*10+3*5], [3*10+1*5] ]
		// Results: [ [10], [35], [35] ]
		//
		// If CostFromWeight=true then we assume the weight is already represented on the vote value.
		// This is why we set weight=1.
		for q, value := range voteValues {
			r.Votes[q][0].Add(
				r.Votes[q][0],
				new(big.Int).Mul(
					new(big.Int).SetUint64(uint64(value)),
					weight),
			)
		}
	} else {
		// For the other cases, we use the results matrix index weighted
		// as described in the Ballot Protocol.
		for q, opt := range voteValues {
			r.Votes[q][opt].Add(r.Votes[q][opt], weight)
		}
	}
	return nil
}

// NewEmptyVotes creates a new results struct with the given number of questions and options
func NewEmptyVotes(questions, options int) [][]*big.Int {
	if questions == 0 || options == 0 {
		return nil
	}
	results := [][]*big.Int{}
	for i := 0; i < questions; i++ {
		question := []*big.Int{}
		for j := 0; j < options; j++ {
			question = append(question, big.NewInt(0))
		}
		results = append(results, question)
	}
	return results
}

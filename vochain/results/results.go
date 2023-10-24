package results

import (
	"bytes"
	"encoding/gob"
	"fmt"
	"math/big"
	"sync"

	"go.vocdoni.io/dvote/types"
	"go.vocdoni.io/proto/build/go/models"
)

const (
	// MaxQuestions is the maximum number of questions allowed in a VotePackage
	MaxQuestions = 64
)

// Results holds the final results and relevant process info for a vochain process
type Results struct {
	ProcessID    types.HexBytes             `json:"processId"`
	Votes        [][]*types.BigInt          `json:"votes"`
	Weight       *types.BigInt              `json:"weight"`
	EnvelopeType *models.EnvelopeType       `json:"envelopeType"`
	VoteOpts     *models.ProcessVoteOptions `json:"voteOptions"`
	BlockHeight  uint32                     `json:"blockHeight"`
}

// String formats the results in a human-readable string
func (r *Results) String() string {
	results := bytes.Buffer{}
	for _, q := range r.Votes {
		results.WriteString("[")
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

// Encode serializes the Results using Gob.
func (r *Results) Encode() ([]byte, error) {
	w := bytes.Buffer{}
	if err := gob.NewEncoder(&w).Encode(r); err != nil {
		return nil, err
	}
	return w.Bytes(), nil
}

// Decode deserializes the Results using Gob.
func (r *Results) Decode(data []byte) error {
	return gob.NewDecoder(bytes.NewReader(data)).Decode(r)
}

// Add adds the total weight and votes from the given Results to the
// containing Results making the method call.
func (r *Results) Add(new *Results) error {
	r.Weight.Add(r.Weight, new.Weight)
	if new.BlockHeight > r.BlockHeight {
		r.BlockHeight = new.BlockHeight
	}
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

// Sub subtracts the total weight and votes from the given Results to the containing Results.
func (r *Results) Sub(new *Results) error {
	r.Weight.Sub(r.Weight, new.Weight)
	// Update votes only if present
	if len(new.Votes) == 0 {
		return nil
	}
	if len(new.Votes) != len(r.Votes) {
		return fmt.Errorf("results.Sub: incorrect number of fields")
	}
	for i := range new.Votes {
		if len(r.Votes[i]) < len(new.Votes[i]) {
			return fmt.Errorf("results.Sub: values overflow (%d)", i)
		}
		// Example:
		//	[ [3,2,3], [2,1,5] ]
		//	[ [2,1,1], [1,0,2] ]
		//	- -----------------
		//	[ [1,1,2], [1,1,3] ]
		for j := range new.Votes[i] {
			r.Votes[i][j].Sub(r.Votes[i][j], new.Votes[i][j])
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
	if len(voteValues) > int(r.VoteOpts.MaxCount) {
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

	// Max Value, check it only if greater than zero
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
	r.Weight.Add(r.Weight, (*types.BigInt)(weight))
	if len(r.Votes) == 0 {
		r.Votes = NewEmptyVotes(r.VoteOpts)
	}

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
				new(types.BigInt).Mul(
					new(types.BigInt).SetUint64(uint64(value)),
					(*types.BigInt)(weight)),
			)
		}
	} else {
		// For the other cases, we use the results matrix index weighted
		// as described in the Ballot Protocol.
		for q, opt := range voteValues {
			r.Votes[q][opt].Add(r.Votes[q][opt], (*types.BigInt)(weight))
		}
	}
	return nil
}

// NewEmptyVotes creates a new results struct with the given number of questions and options
func NewEmptyVotes(voteOpts *models.ProcessVoteOptions) [][]*types.BigInt {
	questions := voteOpts.MaxCount
	options := voteOpts.MaxValue + 1
	if questions == 0 || options == 0 {
		return nil
	}
	results := [][]*types.BigInt{}
	for i := uint32(0); i < questions; i++ {
		question := []*types.BigInt{}
		for j := uint32(0); j < options; j++ {
			question = append(question, new(types.BigInt).SetUint64(0))
		}
		results = append(results, question)
	}
	return results
}

// ResultsToProto takes the Results type and builds the protobuf type ProcessResult.
func ResultsToProto(results *Results) *models.ProcessResult {
	// build the protobuf type for Results
	qr := []*models.QuestionResult{}
	for i := range results.Votes {
		qr = append(qr, &models.QuestionResult{})
		for j := range results.Votes[i] {
			qr[i].Question = append(qr[i].Question, results.Votes[i][j].Bytes())
		}
	}
	return &models.ProcessResult{
		Votes: qr,
	}
}

// ProtoToResults takes the protobuf type ProcessResult and builds the Results type.
func ProtoToResults(pr *models.ProcessResult) *Results {
	r := &Results{
		Votes: [][]*types.BigInt{},
	}
	for i := range pr.Votes {
		r.Votes = append(r.Votes, []*types.BigInt{})
		for j := range pr.Votes[i].Question {
			r.Votes[i] = append(r.Votes[i], new(types.BigInt).SetBytes(pr.Votes[i].Question[j]))
		}
	}
	return r
}

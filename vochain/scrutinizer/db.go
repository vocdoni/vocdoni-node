package scrutinizer

import (
	"bytes"
	"encoding/json"
	"fmt"
	"math/big"
	"sync"
	"time"

	"github.com/timshannon/badgerhold/v3"
	"go.vocdoni.io/dvote/types"
	"go.vocdoni.io/proto/build/go/models"
)

// Process represents an election process handled by the Vochain.
// The scrutinizer Process data type is different from the vochain state data type
// since it is optimized for querying purposes and not for keeping a shared consensus state.
type Process struct {
	ID            types.HexBytes             `badgerholdKey:"ID" json:"processId"`
	EntityID      types.HexBytes             `badgerholdIndex:"EntityID" json:"entityId"`
	StartBlock    uint32                     `json:"startBlock"`
	EndBlock      uint32                     `badgerholdIndex:"EndBlock" json:"endBlock"`
	Rheight       uint32                     `badgerholdIndex:"Rheight" json:"-"`
	CensusRoot    types.HexBytes             `json:"censusRoot"`
	CensusURI     string                     `json:"censusURI"`
	CensusOrigin  int32                      `json:"censusOrigin"`
	Status        int32                      `badgerholdIndex:"Status" json:"status"`
	Namespace     uint32                     `badgerholdIndex:"Namespace" json:"namespace"`
	Envelope      *models.EnvelopeType       `json:"envelopeType"`
	Mode          *models.ProcessMode        `json:"processMode"`
	VoteOpts      *models.ProcessVoteOptions `json:"voteOptions"`
	PrivateKeys   []string                   `json:"-"`
	PublicKeys    []string                   `json:"-"`
	QuestionIndex uint32                     `json:"questionIndex"`
	CreationTime  time.Time                  `json:"creationTime"`
	HaveResults   bool                       `json:"haveResults"`
	FinalResults  bool                       `json:"finalResults"`
}

func (p Process) String() string {
	b, _ := json.Marshal(p)
	return string(b)
}

type Entity struct {
	ID           types.HexBytes `badgerholdKey:"ID"`
	CreationTime time.Time
}

type VoteReference struct {
	Nullifier    types.HexBytes `badgerholdKey:"Nullifier"`
	ProcessID    types.HexBytes `badgerholdIndex:"ProcessID"`
	Height       uint32
	Weight       *big.Int
	TxIndex      int32
	CreationTime time.Time
}

type Results struct {
	ProcessID    types.HexBytes `badgerholdKey:"ProcessID"`
	Votes        [][]*big.Int
	Weight       *big.Int
	EnvelopeType *models.EnvelopeType       `json:"envelopeType"`
	VoteOpts     *models.ProcessVoteOptions `json:"voteOptions"`
	Signatures   []types.HexBytes
	Final        bool
	Height       uint32
}

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

// Add adds the total weight and votes from the given Results to the containing Results making the method call.
func (r *Results) Add(new *Results) error {
	r.Weight.Add(r.Weight, new.Weight)
	if new.Height > r.Height {
		r.Height = new.Height
	}
	if len(new.Votes) == 0 {
		return nil
	}
	if len(new.Votes) != len(r.Votes) {
		return fmt.Errorf("results.Add: different number of fields")
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
		r.Votes = newEmptyVotes(int(r.VoteOpts.MaxCount), int(r.VoteOpts.MaxValue)+1)
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

func InitDB(dataDir string) (*badgerhold.Store, error) {
	opts := badgerhold.DefaultOptions
	opts.WithCompression(0)
	opts.WithBlockCacheSize(0)
	opts.SequenceBandwith = 10000
	opts.WithVerifyValueChecksum(false)
	opts.WithDetectConflicts(true)
	opts.Dir = dataDir
	opts.ValueDir = dataDir
	// TO-DO set custom logger
	return badgerhold.Open(opts)
}

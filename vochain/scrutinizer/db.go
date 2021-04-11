package scrutinizer

import (
	"bytes"
	"encoding/json"
	"fmt"
	"math"
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
	Nullifier    types.HexBytes `badgerhold:"key"`
	ProcessID    types.HexBytes `badgerhold:"index"`
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

func (r *Results) AddVote(voteValues []int, weight *big.Int, mutex *sync.Mutex) error {
	if len(r.Votes) > 0 && len(r.Votes) != int(r.VoteOpts.MaxCount) {
		return fmt.Errorf("addVote: currentResults size mismatch %d != %d",
			len(r.Votes), r.VoteOpts.MaxCount)
	}

	if r.VoteOpts == nil {
		return fmt.Errorf("addVote: processVoteOptions is nil")
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

	// Max Value
	if r.VoteOpts.MaxValue > 0 {
		for _, v := range voteValues {
			if uint32(v) > r.VoteOpts.MaxValue {
				return fmt.Errorf("max value overflow %d", v)
			}
		}
	}

	// Total cost
	if r.VoteOpts.MaxTotalCost > 0 {
		cost := float64(0)
		for _, v := range voteValues {
			cost += math.Pow(float64(v), float64(r.VoteOpts.CostExponent))
		}
		if cost > float64(r.VoteOpts.MaxTotalCost) {
			return fmt.Errorf("max total cost overflow: %f", cost)
		}
	}

	// If weight not provided, assume weight = 1
	if weight == nil {
		weight = new(big.Int).SetUint64(1)
	}
	if mutex != nil {
		mutex.Lock()
		defer mutex.Unlock()
	}
	r.Weight.Add(r.Weight, weight)
	if len(r.Votes) == 0 {
		r.Votes = newEmptyVotes(int(r.VoteOpts.MaxCount), int(r.VoteOpts.MaxValue)+1)
	}
	for q, opt := range voteValues {
		r.Votes[q][opt].Add(r.Votes[q][opt], weight)
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

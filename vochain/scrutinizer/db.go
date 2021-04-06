package scrutinizer

import (
	"bytes"
	"encoding/json"
	"math/big"
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

type Results struct {
	ProcessID  types.HexBytes `badgerholdKey:"ProcessID"`
	Votes      [][]*big.Int
	Weight     *big.Int
	Signatures []types.HexBytes
	Final      bool
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

func InitDB(dataDir string) (*badgerhold.Store, error) {
	options := badgerhold.DefaultOptions
	options.Dir = dataDir
	options.ValueDir = dataDir
	// TO-DO set custom logger
	return badgerhold.Open(options)
}

package types

import (
	"encoding/json"
	"math/big"
	"time"

	"go.vocdoni.io/proto/build/go/models"
)

// Process represents an election process handled by the Vochain.
// The scrutinizer Process data type is different from the vochain state data type
// since it is optimized for querying purposes and not for keeping a shared consensus state.
type Process struct {
	ID            HexBytes                   `badgerholdKey:"ID" json:"processId"`
	EntityID      HexBytes                   `badgerholdIndex:"EntityID" json:"entityId"`
	StartBlock    uint32                     `json:"startBlock"`
	EndBlock      uint32                     `badgerholdIndex:"EndBlock" json:"endBlock"`
	Rheight       uint32                     `badgerholdIndex:"Rheight" json:"-"`
	CensusRoot    HexBytes                   `json:"censusRoot"`
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
	ID           HexBytes `badgerholdKey:"ID"`
	CreationTime time.Time
}

type VoteReference struct {
	Nullifier    HexBytes `badgerholdKey:"Nullifier"`
	ProcessID    HexBytes `badgerholdIndex:"ProcessID"`
	Height       uint32
	Weight       *big.Int
	TxIndex      int32
	CreationTime time.Time
}

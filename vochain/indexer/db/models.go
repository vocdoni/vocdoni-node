// Code generated by sqlc. DO NOT EDIT.
// versions:
//   sqlc v1.25.0

package indexerdb

import (
	"time"

	"go.vocdoni.io/dvote/types"
)

type Block struct {
	Height   int64
	Time     time.Time
	DataHash []byte
}

type Process struct {
	ID                 types.ProcessID
	EntityID           types.EntityID
	StartDate          time.Time
	EndDate            time.Time
	VoteCount          int64
	ChainID            string
	HaveResults        bool
	FinalResults       bool
	ResultsVotes       string
	ResultsWeight      string
	ResultsBlockHeight int64
	CensusRoot         types.CensusRoot
	MaxCensusSize      int64
	CensusUri          string
	Metadata           string
	CensusOrigin       int64
	Status             int64
	Namespace          int64
	Envelope           []byte
	Mode               []byte
	VoteOpts           []byte
	PrivateKeys        string
	PublicKeys         string
	QuestionIndex      int64
	CreationTime       time.Time
	SourceBlockHeight  int64
	SourceNetworkID    int64
	ManuallyEnded      bool
}

type TokenTransfer struct {
	TxHash       types.Hash
	BlockHeight  int64
	FromAccount  types.AccountID
	ToAccount    types.AccountID
	Amount       int64
	TransferTime time.Time
}

type Transaction struct {
	ID          int64
	Hash        types.Hash
	BlockHeight int64
	BlockIndex  int64
	Type        string
}

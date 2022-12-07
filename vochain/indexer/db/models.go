// Code generated by sqlc. DO NOT EDIT.
// versions:
//   sqlc v1.16.0

package indexerdb

import (
	"time"

	"go.vocdoni.io/dvote/types"
	"go.vocdoni.io/dvote/vochain/state"
)

type Process struct {
	ID                    types.ProcessID
	EntityID              types.EntityID
	EntityIndex           int64
	StartBlock            int64
	EndBlock              int64
	ResultsHeight         int64
	HaveResults           bool
	FinalResults          bool
	CensusRoot            types.CensusRoot
	RollingCensusRoot     types.CensusRoot
	RollingCensusSize     int64
	MaxCensusSize         int64
	CensusUri             string
	Metadata              string
	CensusOrigin          int64
	Status                int64
	Namespace             int64
	EnvelopePb            types.EncodedProtoBuf
	ModePb                types.EncodedProtoBuf
	VoteOptsPb            types.EncodedProtoBuf
	PrivateKeys           string
	PublicKeys            string
	QuestionIndex         int64
	CreationTime          time.Time
	SourceBlockHeight     int64
	SourceNetworkID       string
	ResultsVotes          string
	ResultsWeight         string
	ResultsEnvelopeHeight int64
	ResultsSignatures     string
	ResultsBlockHeight    int64
}

type TxReference struct {
	ID           int64
	Hash         types.Hash
	BlockHeight  int64
	TxBlockIndex int64
	TxType       string
}

type VoteReference struct {
	Nullifier    types.Nullifier
	ProcessID    types.ProcessID
	Height       int64
	Weight       string
	TxIndex      int64
	CreationTime time.Time
	VoterID      state.VoterID
}

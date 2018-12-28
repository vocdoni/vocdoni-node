package types

import (
	"time"
)

type Ballot struct {
	Type string
	PID string
	Nullifier []byte
	Vote  []byte
	Franchise []byte
}

type Envelope struct {
	Type string
	Nonce uint64
	KeyProof []byte
	Ballot []byte
	Timestamp time.Time
}

type Batch struct {
	Type string
	Nullifiers []string
	URL string
	TXID string
	Nonce	[]byte
	Signature string
}


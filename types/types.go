package types

import (
	"time"
)

type Submission struct {
	Type string
	Nonce []byte
	KeyProof []byte
	Package []byte
	Timestamp time.Time
}

type Batch struct {
	Type string
	Nullifiers [][]byte
	URL string
	TXID string
	Nonce	[]byte
	Signature string
}

type Packet struct {
	PID uint
	Nullifier string
	Vote  string
	Franchise string
}


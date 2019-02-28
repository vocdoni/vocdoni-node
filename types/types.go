package types

import (
	"time"
)

type Message struct {
	Topic     string
	Data      []byte
	Address   string
	TimeStamp time.Time
}

type Connection struct {
	Topic   string
	Key     string
	Kind    string
	Address string
}

type Ballot struct {
	Type      string
	PID       string
	Nullifier []byte
	Vote      []byte
	Franchise []byte
}

type Envelope struct {
	Type      string
	Nonce     uint64
	KeyProof  []byte
	Ballot    []byte
	Timestamp time.Time
}

type Batch struct {
	Type       string
	Nullifiers []string
	URL        string
	TXID       string
	Nonce      []byte
	Signature  string
}

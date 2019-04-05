package types

import (
	"time"
)

//Message is a wrapper for messages from various net transport modules
type Message struct {
	Topic     string
	Data      []byte
	Address   string //this is the address of the other side
	TimeStamp time.Time
}

//Connection describes the settings for any of the transports defined in the net module, note that not all
//fields are used for all transport types.
type Connection struct {
	Topic   string //channel/topic for topic based messaging such as PSS, PubSub
	Encryption    string //what type of encryption to use
	Key     string //this node's key
	Address string //this node's address
	Path    string //specific path on which a transport should listen
	Port    string //specific port on which a transport should listen
}

//Ballot represents the choices of one user in one voting process
type Ballot struct {
	Type      string
	PID       string
	Nullifier []byte
	Vote      []byte
	Franchise []byte
}

//Envelope contains a Ballot, and additional metadata for processing
type Envelope struct {
	Type      string
	Nonce     uint64
	KeyProof  []byte
	Ballot    []byte
	Timestamp time.Time
}

//Batch contains a number of Ballots, ready to be counted
type Batch struct {
	Type       string
	Nullifiers []string
	URL        string
	TXID       string
	Nonce      []byte
	Signature  string
}

package net

import (
	"errors"

	"go.vocdoni.io/dvote/types"
)

type Transport interface {
	ConnectionType() string

	// Listen starts the transport synchronously and starts a few goroutines
	// asynchronously. Do not call in a goroutine, as that would generally
	// be racy since the transport might not have started completely.
	Listen(reciever chan<- types.Message)

	Send(msg types.Message)
	AddNamespace(namespace string)
	SendUnicast(address string, msg types.Message)
	Init(c *types.Connection) error
	Address() string
	SetBootnodes(bootnodes []string)
	AddPeer(peer string) error
	String() string
}

type TransportID int

const (
	SubPub TransportID = iota + 1
)

func TransportIDFromString(i string) TransportID {
	switch i {
	case "SubPub":
		return SubPub
	default:
		return -1
	}
}

func Init(t TransportID, c *types.Connection) (Transport, error) {
	switch t {
	case SubPub:
		p := new(SubPubHandle)
		p.Init(c)
		return p, nil
	default:
		return nil, errors.New("bad transport type ID or Connection specifier")
	}
}

func InitDefault(t TransportID) (Transport, error) {
	switch t {
	case SubPub:
		defaultConnection := new(types.Connection)
		defaultConnection.Topic = "vocdoni"
		defaultConnection.Encryption = "sym"
		defaultConnection.TransportKey = "vocdoni"
		return Init(t, defaultConnection)
	default:
		return nil, errors.New("bad transport type ID")
	}
}

package net

import (
	"errors"

	"gitlab.com/vocdoni/go-dvote/types"
)

type Transport interface {
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

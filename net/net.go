package net

import (
	"errors"

	"gitlab.com/vocdoni/go-dvote/types"
)

type Transport interface {
	Listen(reciever chan<- types.Message)
	Send(msg types.Message)
	Init(c *types.Connection) error
}

type TransportID int

const (
	PubSub TransportID = iota + 1
	PSS
)

func TransportIDFromString(i string) TransportID {

	switch i {
	case "PubSub":
		return PubSub
	case "PSS":
		return PSS
	default:
		return -1
	}
}

func Init(t TransportID, c *types.Connection) (Transport, error) {
	switch t {
	case PubSub:
		p := new(PubSubHandle)
		p.Init(c)
		return p, nil
	case PSS:
		p := new(PSSHandle)
		p.Init(c)
		return p, nil
	default:
		return nil, errors.New("Bad transport type ID or Connection specifier")
	}
}

func InitDefault(t TransportID) (Transport, error) {
	switch t {
	case PubSub:
		defaultConnection := new(types.Connection)
		defaultConnection.Topic = "vocdoni_testing"
		return Init(t, defaultConnection)
	case PSS:
		defaultConnection := new(types.Connection)
		defaultConnection.Topic = "vocdoni_testing"
		defaultConnection.Encryption = "sym"
		defaultConnection.Key = ""
		return Init(t, defaultConnection)
	default:
		return nil, errors.New("Bad transport type ID")
	}
}

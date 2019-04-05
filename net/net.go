package net

import (
	"errors"

	"github.com/vocdoni/go-dvote/types"
)

type Transport interface {
	Listen(reciever chan<- types.Message, errors chan<- error)
	Send(msg []byte, errors chan<- error)
	Init(c *types.Connection) error
}

type TransportID int

const (
	PubSub TransportID = iota + 1
	PSS
	Websockets
)

func TransportIDFromString(i string) TransportID {
	switch i {
	case "PubSub":
		return PubSub
	case "PSS":
		return PSS
	case "Websockets":
		return Websockets
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
	//case Websockets:
	default:
		return nil, errors.New("Bad transport type ID or Connection specifier")
	}
}

func InitDefault(t TransportID) (Transport, error) {
	switch t {
	case PubSub:
		p := new(PubSubHandle)
		defaultConnection := new(types.Connection)
		defaultConnection.Topic = "vocdoni_testing"
		Init(t, defaultConnection)
		return p, nil
	case PSS:
		p := new(PSSHandle)
		defaultConnection := new(types.Connection)
		defaultConnection.Topic = "vocdoni_testing"
		defaultConnection.Encryption = "sym"
		defaultConnection.Key = ""
		Init(t, defaultConnection)
		return p, nil
	//case Websockets:
	default:
		return nil, errors.New("Bad transport type ID")
	}
}

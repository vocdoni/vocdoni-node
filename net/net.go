package net

import (
	"errors"

	"github.com/vocdoni/go-dvote/types"
)

type Transport interface {
	Listen(reciever chan<- types.Message, errors chan<- error)
	Init() error
}

type RWTransport interface {
	Transport
	Send(msg []byte, errors chan<- error)
}

type TransportID int

const (
	PubSub TransportID = iota + 1
	PSS
	HTTP
	Websockets
)

func TransportIDFromString(i string) TransportID {
	switch i {
	case "PubSub":
		return PubSub
	case "PSS":
		return PSS
	case "HTTP":
		return HTTP
	case "Websockets":
		return Websockets
	default:
		return -1
	}
}

func Init(t TransportID) (Transport, error) {
	switch t {
	case PubSub:
		p := new(PubSubHandle)
		defaultConnection := new(types.Connection)
		defaultConnection.Topic = "vocdoni_testing"
		p.c = defaultConnection
		p.Init()
		return p, nil
	case PSS:
		p := new(PSSHandle)
		defaultConnection := new(types.Connection)
		defaultConnection.Topic = "vocdoni_testing"
		defaultConnection.Key = ""
		defaultConnection.Kind = "sym"
		p.c = defaultConnection
		p.Init()
		return p, nil
	//case HTTP:
	//case Websockets:
	default:
		return nil, errors.New("Bad transport type ID")
	}
}

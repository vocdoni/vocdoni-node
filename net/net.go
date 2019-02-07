package net

import (
	"errors"
)

type Transport interface {
	Init(c string) error
	Listen() error
}

type TransportID int

const (
	HTTP TransportID = iota + 1
	PubSub
)

func TransportIDFromString(i string) TransportID {
	switch i {
	case "PubSub" :
		return PubSub
	case "HTTP":
		return HTTP
	default:
		return -1
	}
}

func Init(t TransportID) (Transport, error) {
	switch t {
	case PubSub :
		var p PubSubHandle
		p.Init("vocdoni_pubsub_testing")
		return p, nil
	case HTTP :
		var h HttpHandle
		h.Init("8080/submit")
		return h, nil
	default:
		return nil, errors.New("Bad transport type specification")
	}
}

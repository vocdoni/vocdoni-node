package endpoint

import (
	"fmt"

	"go.vocdoni.io/dvote/crypto/ethereum"
	"go.vocdoni.io/dvote/multirpc/transports"
	"go.vocdoni.io/dvote/multirpc/transports/subpubtransport"
)

const (
	OptionPrivKey      = "setPrivKey"
	OptionTopic        = "setTopic"
	OptionID           = "setID"
	OptionBootnodes    = "setBootnodes"
	OptionTransportKey = "setTransportKey"
)

type SubPubEndpoint struct {
	port         int32
	privKey      string
	transportKey string
	topic        string
	transport    subpubtransport.SubPubHandle
	bootnodes    []string
}

func (sp *SubPubEndpoint) Init(listener chan transports.Message) error {
	conn := transports.Connection{
		Port:         sp.port,
		Key:          sp.privKey,
		Topic:        fmt.Sprintf("%x", ethereum.HashRaw([]byte(sp.topic))),
		TransportKey: sp.transportKey,
	}

	if err := sp.transport.Init(&conn); err != nil {
		return err
	}
	sp.transport.SetBootnodes(sp.bootnodes)
	sp.transport.Listen(listener)
	return nil
}

func (sp *SubPubEndpoint) SetOption(name string, value interface{}) error {
	var ok bool
	switch name {
	case OptionListenPort:
		if sp.port, ok = value.(int32); !ok {
			return fmt.Errorf("ListenPort must be int32")
		}
	case OptionBootnodes:
		if sp.bootnodes, ok = value.([]string); !ok {
			return fmt.Errorf("Botnodes must be []string")
		}
	case OptionPrivKey:
		if sp.privKey, ok = value.(string); !ok {
			return fmt.Errorf("PrivKey must be of type string")
		}
	case OptionTransportKey:
		if sp.transportKey, ok = value.(string); !ok {
			return fmt.Errorf("TransportKey must be of type string")
		}
	case OptionTopic:
		if sp.topic, ok = value.(string); !ok {
			return fmt.Errorf("topic must be of type string")
		}
	default:
		return fmt.Errorf("option %s is unknown", value)
	}
	return nil
}

func (sp *SubPubEndpoint) Transport() transports.Transport {
	return &sp.transport
}
func (sp *SubPubEndpoint) ID() string {
	return "subpub"
}

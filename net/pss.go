package net

import (
	"time"

	"gitlab.com/vocdoni/go-dvote/log"
	"gitlab.com/vocdoni/go-dvote/swarm"
	"gitlab.com/vocdoni/go-dvote/types"
)

type PSSHandle struct {
	Conn  *types.Connection
	Swarm *swarm.SimpleSwarm
}

func (p *PSSHandle) Init(c *types.Connection) error {
	p.Conn = c
	sn := new(swarm.SimpleSwarm)
	err := sn.InitPSS(swarm.VocdoniBootnodes)
	if err != nil {
		return err
	}
	sn.PssSub(p.Conn.Encryption, p.Conn.Key, p.Conn.Topic)
	p.Swarm = sn
	return nil
}

func (p *PSSHandle) Listen(reciever chan<- types.Message) {
	var msg types.Message
	for {
		select {
		case pssMessage := <-p.Swarm.PssTopics[p.Conn.Topic].Delivery:
			ctx := new(types.PssContext)
			ctx.Topic = p.Conn.Topic
			ctx.PeerAddress = pssMessage.Peer.String()
			msg.Data = pssMessage.Msg
			msg.TimeStamp = int32(time.Now().Unix())
			msg.Context = ctx
			reciever <- msg
		default:
			continue
		}

	}
}

func (p *PSSHandle) Send(msg types.Message) {

	err := p.Swarm.PssPub(p.Conn.Encryption, p.Conn.Key, p.Conn.Topic, string(msg.Data), p.Conn.Address)
	if err != nil {
		log.Warnf("PSS send error: %s", err)
	}
}

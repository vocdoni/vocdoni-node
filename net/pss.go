package net

import (
	"time"

	"gitlab.com/vocdoni/go-dvote/log"
	"gitlab.com/vocdoni/go-dvote/swarm"
	"gitlab.com/vocdoni/go-dvote/types"
)

type PSSHandle struct {
	c *types.Connection
	s *swarm.SimpleSwarm
}

func (p *PSSHandle) Init(c *types.Connection) error {
	p.c = c
	sn := new(swarm.SimpleSwarm)
	err := sn.InitPSS()
	if err != nil {
		return err
	}
	sn.PssSub(p.c.Encryption, p.c.Key, p.c.Topic)
	p.s = sn
	return nil
}

func (p *PSSHandle) Listen(reciever chan<- types.Message) {
	var msg types.Message
	for {
		select {
		case pssMessage := <-p.s.PssTopics[p.c.Topic].Delivery:
			ctx := new(types.PssContext)
			ctx.Topic = p.c.Topic
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

	err := p.s.PssPub(p.c.Encryption, p.c.Key, p.c.Topic, string(msg.Data), p.c.Address)
	if err != nil {
		log.Warnf("PSS send error: %s", err)
	}
}

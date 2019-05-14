package net

import (
	"fmt"
	"time"

	"github.com/vocdoni/go-dvote/swarm"
	"github.com/vocdoni/go-dvote/types"
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
	err = sn.SetLog("crit")
	if err != nil {
		return err
	}
	sn.PssSub(p.c.Encryption, p.c.Key, p.c.Topic, p.c.Address)
	p.s = sn
	fmt.Println("pss init")
	fmt.Println("%v", p)
	return nil
}

func (p *PSSHandle) Listen(reciever chan<- types.Message, errorReciever chan<- error) {
	fmt.Println("%v", p)
	var msg types.Message
	for {
		select {
		case pssMessage := <-p.s.PssTopics[p.c.Topic].Delivery:
			ctx := new(types.PssContext)
			ctx.Topic = p.c.Topic
			ctx.PeerAddress = pssMessage.Peer.String()
			msg.Data = pssMessage.Msg
			msg.TimeStamp = time.Now()
			msg.Context = ctx
			//add error check
			reciever <- msg
		default:
			continue
		}

	}
}

func (p *PSSHandle) Send(msg types.Message, errors chan<- error) {

	err := p.s.PssPub(p.c.Encryption, p.c.Key, p.c.Topic, string(msg.Data), p.c.Address)
	if err != nil {
		errors <- err
	}
}

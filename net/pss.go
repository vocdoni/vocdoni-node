package net

import (
	"time"

	"github.com/vocdoni/go-dvote/swarm"
	"github.com/vocdoni/go-dvote/types"
)

type PSSHandle struct {
	c *types.Connection
	s *swarm.SimplePss
}

func (p *PSSHandle) Init(c *types.Connection) error {
	p.c = c
	sn := new(swarm.SimplePss)
	err := sn.Init()
	if err != nil {
		return err
	}
	err = sn.SetLog("crit")
	if err != nil {
		return err
	}
	sn.PssSub(p.c.Encryption, p.c.Key, p.c.Topic, p.c.Address)
	p.s = sn
	return nil
}

func (p *PSSHandle) Listen(reciever chan<- types.Message, errors chan<- error) {
	var pssMessage swarm.PssMsg
	var msg types.Message
	for {
		pssMessage = <-p.s.PssTopics[p.c.Topic].Delivery
		msg.Topic = p.c.Topic
		msg.Data = pssMessage.Msg
		msg.Address = pssMessage.Peer.String()
		msg.TimeStamp = time.Now()

		reciever <- msg
	}
}

func (p *PSSHandle) Send(msg []byte, errors chan<- error) {

	err := p.s.PssPub(p.c.Encryption, p.c.Key, p.c.Topic, string(msg), p.c.Address)
	if err != nil {
		errors <- err
	}
}

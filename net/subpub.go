package net

import (
	"fmt"

	"gitlab.com/vocdoni/go-dvote/crypto/signature"
	"gitlab.com/vocdoni/go-dvote/log"
	"gitlab.com/vocdoni/go-dvote/subpub"
	"gitlab.com/vocdoni/go-dvote/types"
)

type SubPubHandle struct {
	Conn      *types.Connection
	SubPub    *subpub.SubPub
	BootNodes []string
}

func (p *SubPubHandle) Init(c *types.Connection) error {
	p.Conn = c
	var s signature.SignKeys
	if err := s.AddHexKey(p.Conn.Key); err != nil {
		return fmt.Errorf("cannot import privkey %s: %s", p.Conn.Key, err)
	}
	if len(p.Conn.Topic) == 0 {
		return fmt.Errorf("groupkey topic not specified")
	}
	if p.Conn.Port == 0 {
		p.Conn.Port = 45678
	}
	sp := subpub.NewSubPub(s.Private, p.Conn.Topic, int32(p.Conn.Port))
	p.SubPub = &sp
	return nil
}

func (s *SubPubHandle) Listen(reciever chan<- types.Message) {
	s.SubPub.Connect()
	go s.SubPub.Subcribe()
	for {
		var msg types.Message
		if err := msg.Decode(<-s.SubPub.Reader); err != nil {
			log.Warn(err)
		}
		reciever <- msg
	}
}

func (s *SubPubHandle) Send(msg types.Message) {
	b, err := msg.Encode()
	if err != nil {
		log.Warn(err)
	} else {
		s.SubPub.BroadcastWriter <- b
	}
}

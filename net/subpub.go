package net

import (
	"bufio"
	"context"
	"fmt"
	"time"

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
	if len(p.Conn.TransportKey) == 0 {
		return fmt.Errorf("groupkey topic not specified")
	}
	if p.Conn.Port == 0 {
		p.Conn.Port = 45678
	}
	private := p.Conn.Encryption == "private"
	sp := subpub.NewSubPub(s.Private, []byte(p.Conn.TransportKey), p.Conn.Port, private)
	c.Address = sp.PubKey
	p.SubPub = sp
	return nil
}

func (s *SubPubHandle) Listen(reciever chan<- types.Message) {
	ctx := context.TODO()
	s.SubPub.Connect(ctx)
	go s.SubPub.Subscribe(ctx)
	var msg types.Message
	for {
		msg.Data = <-s.SubPub.Reader
		msg.TimeStamp = int32(time.Now().Unix())
		reciever <- msg
	}
}

func (s *SubPubHandle) Address() string {
	return s.SubPub.PubKey
}

func (s *SubPubHandle) SetBootnodes(bootnodes []string) {
	s.SubPub.BootNodes = bootnodes
}

func (s *SubPubHandle) Send(msg types.Message) {
	s.SubPub.BroadcastWriter <- msg.Data
}

func (s *SubPubHandle) SendUnicast(address string, msg types.Message) {
	s.SubPub.PeerConnect(address, func(rw *bufio.ReadWriter) {
		if err := s.SubPub.SendMessage(rw.Writer, msg.Data); err != nil {
			log.Warnf("cannot send message to %s: %s", address, err)
		}
		rw.Flush()
	})
}

package net

import (
	"os"
	"time"

	shell "github.com/ipfs/go-ipfs-api"
	"gitlab.com/vocdoni/go-dvote/log"
	"gitlab.com/vocdoni/go-dvote/types"
)

type PubSubHandle struct {
	c *types.Connection
	s *shell.PubSubSubscription
}

func PsSubscribe(topic string) *shell.PubSubSubscription {
	sh := shell.NewShell("localhost:5001")
	sub, err := sh.PubSubSubscribe(topic)
	if err != nil {
		log.Fatal(err)
	}
	return sub
}

func PsPublish(topic, data string) error {
	sh := shell.NewShell("localhost:5001")
	err := sh.PubSubPublish(topic, data)
	if err != nil {
		return err
	}
	return nil
}

func (p *PubSubHandle) Init(c *types.Connection) error {
	p.c = c
	p.s = PsSubscribe(p.c.Topic)
	return nil
}

func (p *PubSubHandle) Listen(reciever chan<- types.Message) {
	var psMessage *shell.Message
	var msg types.Message
	var err error
	for {
		psMessage, err = p.s.Next()
		if err != nil {
			log.Warnf("PubSub recieve error: %s", err)
		}
		ctx := new(types.PubSubContext)
		ctx.Topic = p.c.Topic
		ctx.PeerAddress = psMessage.From.String()
		msg.Data = psMessage.Data
		msg.TimeStamp = int32(time.Now().Unix())
		msg.Context = ctx

		reciever <- msg
	}
}

func (p *PubSubHandle) Send(msg types.Message) {
	err := PsPublish(p.c.Topic, string(msg.Data))
	if err != nil {
		log.Warnf("PubSub send error: %s", err)
	}
}

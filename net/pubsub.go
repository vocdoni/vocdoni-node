package net

import (
	"fmt"
	"os"
	"time"

	shell "github.com/ipfs/go-ipfs-api"
	"github.com/vocdoni/go-dvote/types"
)

type PubSubHandle struct {
	c *types.Connection
	s *shell.PubSubSubscription
}

func PsSubscribe(topic string) *shell.PubSubSubscription {
	sh := shell.NewShell("localhost:5001")
	sub, err := sh.PubSubSubscribe(topic)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %s", err)
		os.Exit(1)
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

func (p *PubSubHandle) Listen(reciever chan<- types.Message, errors chan<- error) {
	var psMessage *shell.Message
	var msg types.Message
	var err error
	for {
		psMessage, err = p.s.Next()
		if err != nil {
			errors <- err
			fmt.Fprintf(os.Stderr, "recieve error: %s", err)
		}
		msg.Topic = p.c.Topic
		msg.Data = psMessage.Data
		msg.Address = psMessage.From.String()
		msg.TimeStamp = time.Now()

		reciever <- msg
	}
}

func (p *PubSubHandle) Send(data []byte, errors chan<- error) {
	err := PsPublish(p.c.Topic, string(data))
	if err != nil {
		errors <- err
	}
}

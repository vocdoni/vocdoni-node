package net

import (
	"time"

	ipfsapi "github.com/ipfs/go-ipfs-api"

	"gitlab.com/vocdoni/go-dvote/log"
	"gitlab.com/vocdoni/go-dvote/types"
)

type PubSubHandle struct {
	c *types.Connection
	s *ipfsapi.PubSubSubscription
}

func PsSubscribe(topic string) (*ipfsapi.PubSubSubscription, error) {
	sh := ipfsapi.NewShell("localhost:5001")
	return sh.PubSubSubscribe(topic)
}

func PsPublish(topic, data string) error {
	sh := ipfsapi.NewShell("localhost:5001")
	return sh.PubSubPublish(topic, data)
}

func (p *PubSubHandle) Init(c *types.Connection) error {
	p.c = c
	s, err := PsSubscribe(p.c.Topic)
	if err != nil {
		return err
	}
	p.s = s
	return nil
}

func (p *PubSubHandle) Listen(reciever chan<- types.Message) {
	for {
		psMessage, err := p.s.Next()
		if err != nil {
			log.Warnf("PubSub recieve error: %s", err)
			continue
		}
		reciever <- types.Message{
			Data:      psMessage.Data,
			TimeStamp: int32(time.Now().Unix()),
			Context: &types.PubSubContext{
				Topic:       p.c.Topic,
				PeerAddress: psMessage.From.String(),
			},
		}
	}
}

func (p *PubSubHandle) Address() string {
	return "" // To-Do
}

func (w *PubSubHandle) SetBootnodes(bootnodes []string) {
	// To-Do
}

func (p *PubSubHandle) Send(msg types.Message) {
	err := PsPublish(p.c.Topic, string(msg.Data))
	if err != nil {
		log.Warnf("PubSub send error: %s", err)
	}
}

func (p *PubSubHandle) SendUnicast(address string, msg types.Message) {
	// To-Do
}

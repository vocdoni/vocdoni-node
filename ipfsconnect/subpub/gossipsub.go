package subpub

import (
	"context"

	"git.sr.ht/~sircmpwn/go-bare"
	pubsub "github.com/libp2p/go-libp2p-pubsub"
	"github.com/libp2p/go-libp2p/core/peer"
	"go.vocdoni.io/dvote/log"
)

// GossipBufSize is the number of incoming messages to buffer for each topic.
const GossipBufSize = 128

// Gossip represents a subscription to a single PubSub topic. Messages
// can be published to the topic with Gossip.Publish, and received
// messages are pushed to the Messages channel.
type Gossip struct {
	// Messages is a channel of messages received from other peers in the topic
	Messages chan *Message

	ctx   context.Context
	ps    *pubsub.PubSub
	topic *pubsub.Topic
	sub   *pubsub.Subscription

	self peer.ID
}

// setupGossip creates a new PubSub service using the GossipSub router
// attaches it to ps.Host, and joins topic ps.Topic
// the resulting Gossip is available at ps.Gossip
func (ps *SubPub) setupGossip(ctx context.Context) error {
	// create a new PubSub service using the GossipSub router
	gs, err := pubsub.NewGossipSub(ctx, ps.Host)
	if err != nil {
		return err
	}

	// join the topic
	ps.Gossip, err = JoinGossip(ctx, gs, ps.Host.ID(), ps.Topic)
	if err != nil {
		return err
	}
	return nil
}

// JoinGossip tries to subscribe to the PubSub topic, returning
// a Gossip on success.
func JoinGossip(ctx context.Context, ps *pubsub.PubSub, selfID peer.ID, topic string) (*Gossip, error) {
	// join the pubsub topic
	t, err := ps.Join(topic)
	if err != nil {
		return nil, err
	}

	// and subscribe to it
	sub, err := t.Subscribe()
	if err != nil {
		return nil, err
	}
	log.Debugf("with gossipsub i just joined topic %s", sub.Topic())

	g := &Gossip{
		ctx:      ctx,
		ps:       ps,
		topic:    t,
		sub:      sub,
		self:     selfID,
		Messages: make(chan *Message, GossipBufSize),
	}

	// start reading messages from the subscription in a loop
	go g.readLoop() // this spawns a single background task per instance (since a single gossip topic is used)
	return g, nil
}

// Publish sends a message to the pubsub topic.
func (g *Gossip) Publish(message []byte) error {
	log.Debugf("gossiping %d bytes to peers in topic %v: %v",
		len(message),
		g.topic,
		g.topic.ListPeers())

	m := &Message{
		Data: message,
		Peer: g.self.Pretty(),
	}
	msgBytes, err := bare.Marshal(m)
	if err != nil {
		log.Error("couldn't gossip: ", err)
		return err
	}
	return g.topic.Publish(g.ctx, msgBytes)
}

// readLoop pulls messages from the pubsub topic and pushes them onto the Messages channel.
func (g *Gossip) readLoop() {
	for {
		msg, err := g.sub.Next(g.ctx)
		if err != nil {
			log.Warnf("gossipsub: closing topic %v: %v", g.topic, err)
			close(g.Messages)
			return
		}
		// only forward messages delivered by others
		if msg.ReceivedFrom == g.self {
			continue
		}

		m := new(Message)
		if err := bare.Unmarshal(msg.Data, m); err != nil {
			log.Warnf("gossipsub: err %v, couldn't unmarshal %q", err, msg.Data)
			continue
		}
		// send valid messages onto the Messages channel
		g.Messages <- m
	}
}

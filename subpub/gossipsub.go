package subpub

import (
	"bufio"
	"bytes"
	"context"

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
func (s *SubPub) setupGossip(ctx context.Context) error {
	// create a new PubSub service using the GossipSub router
	gs, err := pubsub.NewGossipSub(ctx, s.node.PeerHost)
	if err != nil {
		return err
	}
	// join the topic
	if err = s.joinGossip(ctx, gs, s.node.PeerHost.ID(), s.Topic); err != nil {
		return err
	}
	return nil
}

// joinGossip tries to subscribe to the PubSub topic, returning
// a Gossip on success.
func (s *SubPub) joinGossip(ctx context.Context, p2pPS *pubsub.PubSub, selfID peer.ID, topic string) error {
	// join the pubsub topic
	t, err := p2pPS.Join(topic)
	if err != nil {
		return err
	}

	// and subscribe to it
	sub, err := t.Subscribe()
	if err != nil {
		return err
	}
	s.gossip = &Gossip{
		ctx:      ctx,
		ps:       p2pPS,
		topic:    t,
		sub:      sub,
		self:     selfID,
		Messages: make(chan *Message, GossipBufSize),
	}
	log.Infow("joined to gossipsub topic", "topic", sub.Topic(), "peer", selfID.String())

	// start reading messages from the subscription in a loop
	go s.readGossipLoop() // this spawns a single background task per instance (since a single gossip topic is used)
	return nil
}

// Publish sends a message to the gossip pubsub topic.
func (g *Gossip) Publish(message []byte) error {
	log.Debugw("gossiping message", "size", len(message), "topic", g.topic.String(), "numPeers", len(g.topic.ListPeers()))
	return g.topic.Publish(g.ctx, message)
}

// readGossipLoop pulls messages from the pubsub topic and pushes them onto the Messages channel.
func (s *SubPub) readGossipLoop() {
	for {
		msg, err := s.gossip.sub.Next(s.gossip.ctx)
		if err != nil {
			log.Warnf("gossipsub: closing topic %v: %v", s.gossip.topic, err)
			close(s.gossip.Messages)
			return
		}
		// only forward messages delivered by others
		if msg.ReceivedFrom == s.gossip.self {
			continue
		}

		m, err := s.readMessage(bufio.NewReader(bytes.NewBuffer(msg.Data)))
		if err != nil {
			log.Warnf("gossipsub: err %v, couldn't read message %q", err, msg.Data)
			continue
		}
		// send valid messages onto the Messages channel
		s.gossip.Messages <- m
	}
}

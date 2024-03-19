package subpub

import (
	"bufio"
	"bytes"
	"context"
	"crypto/ecdsa"
	"fmt"
	"sync"
	"time"

	"git.sr.ht/~sircmpwn/go-bare"
	ipfslog "github.com/ipfs/go-log/v2"
	"github.com/ipfs/kubo/core"
	libpeer "github.com/libp2p/go-libp2p/core/peer"
	discrouting "github.com/libp2p/go-libp2p/p2p/discovery/routing"
	"go.vocdoni.io/dvote/log"
)

const (
	// UnicastBufSize is the number of unicast incoming messages to buffer.
	UnicastBufSize = 128

	// DiscoveryPeriodSeconds is the time between discovery rounds.
	DiscoveryPeriodSeconds = 120

	// We use go-bare for export/import the trie. In order to support
	// big census (up to 8 Million entries) we need to increase the maximums.
	bareMaxArrayLength    uint64 = 1024 * 1014 * 8         // 8 Million entries
	bareMaxUnmarshalBytes        = bareMaxArrayLength * 32 // Assuming 32 bytes per entry
)

// SubPub is a simplified PubSub protocol using libp2p.
// It uses a shared secret to encrypt messages.
// The shared secret is derived from a group key, which is also used to discover peers.
// It allows for broadcasting messages to all peers in the group, and sending messages to specific peers.
// Peers are discovered using the DHT, and automatically set as protected peers.
type SubPub struct {
	Key          ecdsa.PrivateKey
	GroupKey     [32]byte
	Topic        string
	NoBootStrap  bool
	BootNodes    []string
	NodeID       string
	node         *core.IpfsNode
	MaxDHTpeers  int
	OnlyDiscover bool

	gossip      *Gossip       // Gossip deals with broadcasts
	streams     sync.Map      // this is a thread-safe map[libpeer.ID]bufioWithMutex
	unicastMsgs chan *Message // UnicastMsgs passes unicasts around
	messages    chan *Message // both unicasts and broadcasts end up being passed to Messages

	DiscoveryPeriod time.Duration

	// TODO(mvdan): replace with a context
	close   chan bool
	routing *discrouting.RoutingDiscovery

	// These are used in testing
	OnPeerAdd    func(id libpeer.ID)
	OnPeerRemove func(id libpeer.ID)
}

// Message is the type of messages passed around by SubPub.
type Message struct {
	Data []byte
	Peer string
}

// NewSubPub creates a new SubPub instance.
// The groupKey is a secret shared among the PubSub participants.
// Only those with the key will be able to join.
func NewSubPub(groupKey [32]byte, node *core.IpfsNode) *SubPub {
	s := SubPub{
		GroupKey:        groupKey,
		Topic:           fmt.Sprintf("%x", groupKey),
		DiscoveryPeriod: time.Second * DiscoveryPeriodSeconds,
		node:            node,
		MaxDHTpeers:     1024,
		close:           make(chan bool),
		unicastMsgs:     make(chan *Message, UnicastBufSize),
	}
	bare.MaxArrayLength(bareMaxArrayLength)
	bare.MaxUnmarshalBytes(bareMaxUnmarshalBytes)

	return &s
}

// Start connects the SubPub networking stack
// and begins passing incoming messages to the receiver chan
func (s *SubPub) Start(ctx context.Context, receiver chan *Message) {
	if s.Topic == "" {
		log.Fatal("no group key provided")
	}
	ipfslog.SetLogLevel("*", "ERROR")
	s.NodeID = s.node.PeerHost.ID().String()
	s.messages = receiver
	s.setupDiscovery(ctx)
	s.setupGossip(ctx)
	go s.listen(receiver)
	go s.printStats() // this spawns a single background task per instance, that just prints logs
}

// listen listens for incoming messages and passes them to the receiver chan.
func (s *SubPub) listen(receiver chan<- *Message) {
	for {
		select {
		case <-s.close:
			return
		case spmsg := <-s.gossip.Messages:
			receiver <- spmsg
		case spmsg := <-s.unicastMsgs:
			receiver <- spmsg
		}
	}
}

// Close terminates the subpub networking stack
func (s *SubPub) Close() {
	log.Debug("received close signal")
	select {
	case <-s.close:
		// already closed
	default:
		close(s.close)
		close(s.messages)
	}
}

// String returns a string representation of the SubPub instance.
func (s *SubPub) String() string {
	return fmt.Sprintf("dhtPeers:%d dhtKnown:%d clusterPeers:%d",
		len(s.node.PeerHost.Network().Peers()),
		len(s.node.PeerHost.Peerstore().PeersWithAddrs()),
		len(s.gossip.topic.ListPeers()))
}

// Stats returns the current stats of the SubPub instance.
func (s *SubPub) Stats() map[string]any {
	return map[string]any{
		"peers":   len(s.node.PeerHost.Network().Peers()),
		"known":   len(s.node.PeerHost.Peerstore().PeersWithAddrs()),
		"cluster": len(s.gossip.topic.ListPeers()),
	}
}

// Address returns the node's ID.
func (s *SubPub) Address() string {
	return s.NodeID
}

// SendBroadcast sends a message to all peers in the cluster.
func (s *SubPub) SendBroadcast(data []byte) error {
	var buf bytes.Buffer
	writer := bufio.NewWriter(&buf)
	if err := s.writeMessage(writer, data); err != nil {
		return err
	}
	return s.gossip.Publish(buf.Bytes())
}

// SendUnicast sends a message to a specific peer in the cluster.
func (s *SubPub) SendUnicast(address string, data []byte) error {
	return s.sendStreamMessage(address, data)
}

func (s *SubPub) printStats() {
	for {
		time.Sleep(120 * time.Second)
		log.Monitor("subpub network", s.Stats())
	}
}

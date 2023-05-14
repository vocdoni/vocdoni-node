package subpub

import (
	"context"
	"crypto/ecdsa"
	"fmt"
	"sync"
	"time"

	"git.sr.ht/~sircmpwn/go-bare"
	ipfslog "github.com/ipfs/go-log/v2"
	"github.com/libp2p/go-libp2p"
	dht "github.com/libp2p/go-libp2p-kad-dht"
	"github.com/libp2p/go-libp2p/core/host"
	libpeer "github.com/libp2p/go-libp2p/core/peer"
	discrouting "github.com/libp2p/go-libp2p/p2p/discovery/routing"
	"github.com/libp2p/go-libp2p/p2p/net/connmgr"
	"go.vocdoni.io/dvote/log"
	"go.vocdoni.io/dvote/util"
)

const (
	// UnicastBufSize is the number of unicast incoming messages to buffer.
	UnicastBufSize = 128

	// We use go-bare for export/import the trie. In order to support
	// big census (up to 8 Million entries) we need to increase the maximums.
	bareMaxArrayLength    uint64 = 1024 * 1014 * 8         // 8 Million entries
	bareMaxUnmarshalBytes uint64 = bareMaxArrayLength * 32 // Assuming 32 bytes per entry
)

// SubPub is a simplified PubSub protocol using libp2p
type SubPub struct {
	Key         ecdsa.PrivateKey
	GroupKey    [32]byte
	Topic       string
	NoBootStrap bool
	BootNodes   []string
	NodeID      string
	Port        int32
	Host        host.Host
	MaxDHTpeers int

	Gossip      *Gossip       // Gossip deals with broadcasts
	Streams     sync.Map      // this is a thread-safe map[libpeer.ID]bufioWithMutex
	UnicastMsgs chan *Message // UnicastMsgs passes unicasts around
	Messages    chan *Message // both unicasts and broadcasts end up being passed to Messages

	DiscoveryPeriod time.Duration

	// TODO(mvdan): replace with a context
	close   chan bool
	dht     *dht.IpfsDHT
	routing *discrouting.RoutingDiscovery

	// These are used in testing
	OnPeerAdd    func(id libpeer.ID)
	OnPeerRemove func(id libpeer.ID)
}

type Message struct {
	Data []byte
	Peer string
}

// NewSubPub creates a new SubPub instance.
// The groupKey is a secret shared among the PubSub participants.
// Only those with the key will be able to join.
func NewSubPub(groupKey [32]byte, port int32) *SubPub {
	s := SubPub{
		GroupKey:        groupKey,
		Topic:           fmt.Sprintf("%x", groupKey),
		NodeID:          util.RandomHex(32),
		DiscoveryPeriod: time.Second * 10,
		Port:            port,
		Host:            nil,
		MaxDHTpeers:     128,
		close:           make(chan bool),
		UnicastMsgs:     make(chan *Message, UnicastBufSize),
	}
	bare.MaxArrayLength(bareMaxArrayLength)
	bare.MaxUnmarshalBytes(bareMaxUnmarshalBytes)

	return &s
}

// Start connects the SubPub networking stack
// and begins passing incoming messages to the receiver chan
func (s *SubPub) Start(ctx context.Context, receiver chan *Message) {
	if len(s.Topic) == 0 {
		log.Fatal("no group key provided")
	}
	ipfslog.SetLogLevel("*", "ERROR")

	connmgr, err := connmgr.NewConnManager(
		s.MaxDHTpeers/2, // Lowwater
		s.MaxDHTpeers,   // HighWater,
		connmgr.WithGracePeriod(time.Second*10),
	)
	if err != nil {
		log.Fatal(err)
	}

	s.Host, err = libp2p.New(
		// libp2p will listen on any interface device (both on IPv4 and IPv6)
		libp2p.ListenAddrStrings(
			fmt.Sprintf("/ip6/::/tcp/%d", s.Port),
			fmt.Sprintf("/ip4/0.0.0.0/tcp/%d", s.Port),
		),
		// support any other default transports (TCP)
		libp2p.DefaultTransports,
		// Let's prevent our peer from having too many
		// connections by attaching a connection manager.
		libp2p.ConnectionManager(connmgr),
		// Set RelayCustom = true, Relay = false
		libp2p.DisableRelay(),
	)
	if err != nil {
		log.Fatal(err)
	}
	// Note that we don't use ctx here, since we stop via the Close method.

	s.NodeID = s.Host.ID().String()
	s.Messages = receiver
	log.Infow("libp2p host listening", "port", s.Port, "id", s.NodeID)

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
		case spmsg := <-s.Gossip.Messages:
			receiver <- spmsg
		case spmsg := <-s.UnicastMsgs:
			receiver <- spmsg
		}
	}
}

// Close terminaters the subpub networking stack
func (s *SubPub) Close() error {
	log.Debug("received close signal")
	select {
	case <-s.close:
		// already closed
		return nil
	default:
		close(s.close)
		if err := s.dht.Close(); err != nil {
			return err
		}
		return s.Host.Close()
	}
}

// String returns a string representation of the SubPub instance.
func (s *SubPub) String() string {
	return fmt.Sprintf("dhtPeers:%d dhtKnown:%d clusterPeers:%d",
		len(s.Host.Network().Peers()),
		len(s.Host.Peerstore().PeersWithAddrs()),
		len(s.Gossip.topic.ListPeers()))
}

// Stats returns the current stats of the SubPub instance.
func (s *SubPub) Stats() map[string]interface{} {
	return map[string]interface{}{
		"peers":   len(s.Host.Network().Peers()),
		"known":   len(s.Host.Peerstore().PeersWithAddrs()),
		"cluster": len(s.Gossip.topic.ListPeers())}
}

// Address returns the node's ID.
func (s *SubPub) Address() string {
	return s.NodeID
}

// SendBroadcast sends a message to all peers in the cluster.
func (s *SubPub) SendBroadcast(msg Message) error {
	return s.Gossip.Publish(msg.Data)
}

// SendUniCast sends a message to a specific peer in the cluster.
func (s *SubPub) SendUnicast(address string, msg Message) error {
	return s.Unicast(address, msg.Data)
}

func (ps *SubPub) printStats() {
	for {
		time.Sleep(120 * time.Second)
		log.Monitor("subpub network", ps.Stats())
	}
}

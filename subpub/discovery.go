package subpub

import (
	"context"
	"time"

	corediscovery "github.com/libp2p/go-libp2p/core/discovery"
	libpeer "github.com/libp2p/go-libp2p/core/peer"
	"github.com/libp2p/go-libp2p/core/protocol"
	discrouting "github.com/libp2p/go-libp2p/p2p/discovery/routing"
	discutil "github.com/libp2p/go-libp2p/p2p/discovery/util"
	multiaddr "github.com/multiformats/go-multiaddr"
	"github.com/prometheus/client_golang/prometheus"
	"go.vocdoni.io/dvote/log"
	"go.vocdoni.io/dvote/metrics"
)

// Metrics exported via prometheus
var (
	dhtLatency = prometheus.NewHistogram(prometheus.HistogramOpts{
		Namespace: "file",
		Name:      "peers_dht_latency",
		Help:      "The time it takes FindPeers to discover peers",
	})
)

// setupDiscovery creates a DHT discovery service and attaches it to the libp2p Host.
// This lets us automatically discover peers and connect to them.
func (s *SubPub) setupDiscovery(ctx context.Context) {
	// Set a function as stream handler. This function is called when a peer
	// initiates a connection and starts a stream with this peer.
	if !s.OnlyDiscover {
		s.node.PeerHost.SetStreamHandler(protocol.ID(s.Topic), s.handleStream)
	}

	// We use a rendezvous point "meet me here" to announce our location.
	// This is like telling your friends to meet you at the Eiffel Tower.
	log.Infof("advertising myself periodically in topic %s", s.Topic)
	s.routing = discrouting.NewRoutingDiscovery(s.node.DHT)
	discutil.Advertise(ctx, s.routing, s.Topic)

	metrics.Register(dhtLatency)

	// Discover new peers periodically
	go func() { // this spawns a single background task per instance
		for {
			select {
			case <-s.close:
				return
			default:
				pctx, cancel := context.WithTimeout(ctx, s.DiscoveryPeriod)
				s.discover(pctx)
				cancel()
				time.Sleep(s.DiscoveryPeriod)
			}
		}
	}()
}

func (s *SubPub) discover(ctx context.Context) {
	dhtLatencyTimer := prometheus.NewTimer(dhtLatency)
	// Now, look for others who have announced.
	// This is like your friend telling you the location to meet you.
	log.Debugf("looking for peers in topic %s", s.Topic)
	peerChan, err := s.routing.FindPeers(ctx, s.Topic,
		corediscovery.Limit(4*s.MaxDHTpeers))
	if err != nil {
		log.Errorw(err, "error finding peers")
		return
	}
	for peer := range peerChan {
		select {
		case <-s.close:
			return
		default:
			// continues below
		}
		if peer.ID == s.node.PeerHost.ID() {
			continue // this is us; skip
		}
		if s.connectedPeer(peer.ID) {
			continue
		}
		// new peer; let's connect to it
		// first update the latency metrics
		dhtLatencyTimer.ObserveDuration()
		connectCtx, cancel := context.WithTimeout(ctx, time.Second*10)
		if err := s.node.PeerHost.Connect(connectCtx, peer); err != nil {
			cancel()
			continue
		}
		cancel()
		log.Infow("connected to cluster peer!", "address", peer.ID.Pretty())

		// protect the peer from being disconnected by the connection manager
		s.node.PeerHost.ConnManager().Protect(peer.ID, "discoveredPeer")

		// if only discover is set, we don't need to open a stream
		if s.OnlyDiscover {
			continue
		}

		// open a stream to the peer to start sending messages
		stream, err := s.node.PeerHost.NewStream(ctx, peer.ID, protocol.ID(s.Topic))
		if err != nil {
			// Since this error is pretty common in p2p networks.
			continue
		}
		s.handleStream(stream)
	}
}

// connectedPeer returns true if the peer has some stream
func (s *SubPub) connectedPeer(pid libpeer.ID) bool {
	for _, conn := range s.node.PeerHost.Network().ConnsToPeer(pid) {
		if len(conn.GetStreams()) > 0 {
			return true
		}
	}
	return false
}

// AddPeer creates a new libp2p peer connection (transport layer)
func (s *SubPub) AddPeer(peer string) error {
	m, err := multiaddr.NewMultiaddr(peer)
	if err != nil {
		return err
	}
	ai, err := libpeer.AddrInfoFromP2pAddr(m)
	if err != nil {
		return err
	}
	s.node.PeerHost.ConnManager().Protect(ai.ID, "customPeer")
	return s.node.PeerHost.Connect(context.Background(), *ai)
}

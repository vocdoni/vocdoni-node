package subpub

import (
	"context"
	"sync"
	"time"

	dht "github.com/libp2p/go-libp2p-kad-dht"
	dhtopts "github.com/libp2p/go-libp2p-kad-dht/opts"
	corediscovery "github.com/libp2p/go-libp2p/core/discovery"
	libpeer "github.com/libp2p/go-libp2p/core/peer"
	"github.com/libp2p/go-libp2p/core/protocol"
	discrouting "github.com/libp2p/go-libp2p/p2p/discovery/routing"
	discutil "github.com/libp2p/go-libp2p/p2p/discovery/util"
	multiaddr "github.com/multiformats/go-multiaddr"
	"go.vocdoni.io/dvote/log"
)

// setupDiscovery creates a DHT discovery service and attaches it to the libp2p Host.
// This lets us automatically discover peers and connect to them.
func (s *SubPub) setupDiscovery(ctx context.Context) {
	var err error

	// Set a function as stream handler. This function is called when a peer
	// initiates a connection and starts a stream with this peer.
	s.Host.SetStreamHandler(protocol.ID(s.Topic), s.handleStream)

	// Start a DHT, for use in peer discovery. We can't just make a new DHT
	// client because we want each peer to maintain its own local copy of the
	// DHT, so that the bootstrapping node of the DHT can go down without
	// inhibiting future peer discovery.

	// Let's try to apply some tunning for reducing the DHT fingerprint
	opts := []dhtopts.Option{
		dht.RoutingTableLatencyTolerance(time.Second * 20),
		dht.BucketSize(20),
		dht.MaxRecordAge(1 * time.Hour),
	}

	// Note that we don't use ctx here, since we stop via the Close method.
	s.dht, err = dht.New(context.Background(), s.Host, opts...)
	if err != nil {
		log.Fatal(err)
	}

	if !s.NoBootStrap {
		// Bootstrap the DHT. In the default configuration, this spawns a Background
		// thread that will refresh the peer table every five minutes.
		log.Info("bootstrapping the DHT")
		// Note that we don't use ctx here, since we stop via the Close method.
		if err := s.dht.Bootstrap(context.Background()); err != nil {
			log.Fatal(err)
		}

		// Let's connect to the bootstrap nodes first. They will tell us about the
		// other nodes in the network.
		bootnodes := dht.DefaultBootstrapPeers
		if len(s.BootNodes) > 0 {
			bootnodes, err = parseMultiaddress(s.BootNodes)
			if err != nil {
				log.Fatal(err)
			}
		}

		log.Info("connecting to bootstrap nodes...")
		log.Debugf("bootnodes: %+v", bootnodes)
		var wg sync.WaitGroup
		for _, peerAddr := range bootnodes {
			if peerAddr == nil {
				continue
			}
			peerinfo, err := libpeer.AddrInfoFromP2pAddr(peerAddr)
			if err != nil {
				log.Fatal(err)
			}
			if peerinfo == nil {
				continue // nothing to do
			}
			wg.Add(1)
			go func() { // try to connect to every bootnode in parallel, since ps.Host.Connect() is thread-safe
				defer wg.Done()
				log.Debugf("trying %s", *peerinfo)
				if err := s.Host.Connect(ctx, *peerinfo); err != nil {
					log.Debug(err)
				} else {
					log.Infof("connection established with bootstrap node: %s", peerinfo)
				}
			}()
		}
		wg.Wait()
	}

	// We use a rendezvous point "meet me here" to announce our location.
	// This is like telling your friends to meet you at the Eiffel Tower.
	log.Infof("advertising myself periodically in topic %s", s.Topic)
	s.routing = discrouting.NewRoutingDiscovery(s.dht)
	discutil.Advertise(ctx, s.routing, s.Topic)

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
	// Now, look for others who have announced.
	// This is like your friend telling you the location to meet you.
	log.Debugf("looking for peers in topic %s", s.Topic)
	peerChan, err := s.routing.FindPeers(ctx, s.Topic,
		corediscovery.Limit(4*s.MaxDHTpeers))
	if err != nil {
		log.Fatal(err)
	}
	for peer := range peerChan {
		select {
		case <-s.close:
			return
		default:
			// continues below
		}
		if peer.ID == s.Host.ID() {
			continue // this is us; skip
		}

		if s.connectedPeer(peer.ID) {
			continue
		}
		// new peer; let's connect to it
		if err := peer.ID.Validate(); err == nil {
			stream, err := s.Host.NewStream(context.Background(), peer.ID, protocol.ID(s.Topic))
			if err != nil {
				// Since this error is pretty common in p2p networks.
				continue
			}
			log.Debugf("found peer %s: %v", peer.ID.Pretty(), peer.Addrs)
			s.Host.ConnManager().Protect(peer.ID, "discoveredPeer")
			s.handleStream(stream)
		}
	}
}

// connectedPeer returns true if the peer has some stream
func (s *SubPub) connectedPeer(pid libpeer.ID) bool {
	for _, conn := range s.Host.Network().ConnsToPeer(pid) {
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
	s.Host.ConnManager().Protect(ai.ID, "customPeer")
	return s.Host.Connect(context.Background(), *ai)
}

func parseMultiaddress(maddress []string) (ma []multiaddr.Multiaddr, err error) {
	for _, m := range maddress {
		mad, err := multiaddr.NewMultiaddr(m)
		if err != nil {
			return nil, err
		}
		ma = append(ma, mad)
	}
	return ma, nil
}

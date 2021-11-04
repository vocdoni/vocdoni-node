package subpub

import (
	"context"
	"sync"
	"time"

	corediscovery "github.com/libp2p/go-libp2p-core/discovery"
	libpeer "github.com/libp2p/go-libp2p-core/peer"
	"github.com/libp2p/go-libp2p-core/protocol"
	discovery "github.com/libp2p/go-libp2p-discovery"
	dht "github.com/libp2p/go-libp2p-kad-dht"
	dhtopts "github.com/libp2p/go-libp2p-kad-dht/opts"
	multiaddr "github.com/multiformats/go-multiaddr"
	"go.vocdoni.io/dvote/log"
)

// setupDiscovery creates a DHT discovery service and attaches it to the libp2p Host.
// This lets us automatically discover peers and connect to them.
func (ps *SubPub) setupDiscovery(ctx context.Context) {
	var err error

	// Set a function as stream handler. This function is called when a peer
	// initiates a connection and starts a stream with this peer.
	ps.Host.SetStreamHandler(protocol.ID(ps.Topic), ps.handleStream)

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
	ps.dht, err = dht.New(context.Background(), ps.Host, opts...)
	if err != nil {
		log.Fatal(err)
	}

	if !ps.NoBootStrap {
		// Bootstrap the DHT. In the default configuration, this spawns a Background
		// thread that will refresh the peer table every five minutes.
		log.Info("bootstrapping the DHT")
		// Note that we don't use ctx here, since we stop via the Close method.
		if err := ps.dht.Bootstrap(context.Background()); err != nil {
			log.Fatal(err)
		}

		// Let's connect to the bootstrap nodes first. They will tell us about the
		// other nodes in the network.
		bootnodes := dht.DefaultBootstrapPeers
		if len(ps.BootNodes) > 0 {
			bootnodes, err = parseMultiaddress(ps.BootNodes)
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
				if err := ps.Host.Connect(ctx, *peerinfo); err != nil {
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
	log.Infof("advertising myself periodically in topic %s", ps.Topic)
	ps.routing = discovery.NewRoutingDiscovery(ps.dht)
	discovery.Advertise(ctx, ps.routing, ps.Topic)

	// Discover new peers periodically
	go func() { // this spawns a single background task per IPFSsync instance
		for {
			select {
			case <-ps.close:
				return
			default:
				pctx, cancel := context.WithTimeout(ctx, ps.DiscoveryPeriod)
				ps.discover(pctx)
				cancel()
				time.Sleep(ps.DiscoveryPeriod)
			}
		}
	}()
}

func (ps *SubPub) discover(ctx context.Context) {
	// Now, look for others who have announced.
	// This is like your friend telling you the location to meet you.
	log.Debugf("looking for peers in topic %s", ps.Topic)
	peerChan, err := ps.routing.FindPeers(ctx, ps.Topic,
		corediscovery.Limit(4*ps.MaxDHTpeers))
	if err != nil {
		log.Fatal(err)
	}
	for peer := range peerChan {
		select {
		case <-ps.close:
			return
		default:
			// continues below
		}
		if peer.ID == ps.Host.ID() {
			continue // this is us; skip
		}

		if ps.connectedPeer(peer.ID) {
			continue
		}
		// new peer; let's connect to it
		if err := peer.ID.Validate(); err == nil {
			stream, err := ps.Host.NewStream(context.Background(), peer.ID, protocol.ID(ps.Topic))
			if err != nil {
				// Since this error is pretty common in p2p networks.
				continue
			}
			log.Debugf("found peer %s: %v", peer.ID.Pretty(), peer.Addrs)
			ps.Host.ConnManager().Protect(peer.ID, "discoveredPeer")
			ps.handleStream(stream)
		}
	}
}

// connectedPeer returns true if the peer has some stream
func (ps *SubPub) connectedPeer(pid libpeer.ID) bool {
	for _, conn := range ps.Host.Network().ConnsToPeer(pid) {
		if len(conn.GetStreams()) > 0 {
			return true
		}
	}
	return false
}

// AddPeer creates a new libp2p peer connection (transport layer)
func (ps *SubPub) AddPeer(peer string) error {
	m, err := multiaddr.NewMultiaddr(peer)
	if err != nil {
		return err
	}
	ai, err := libpeer.AddrInfoFromP2pAddr(m)
	if err != nil {
		return err
	}
	ps.Host.ConnManager().Protect(ai.ID, "customPeer")
	return ps.Host.Connect(context.Background(), *ai)
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

package subpub

import (
	"context"
	"fmt"
	"time"

	corediscovery "github.com/libp2p/go-libp2p-core/discovery"
	"github.com/libp2p/go-libp2p-core/protocol"
	"go.vocdoni.io/dvote/log"
)

func (ps *SubPub) discover(ctx context.Context) {
	// Now, look for others who have announced.
	// This is like your friend telling you the location to meet you.
	log.Debugf("searching for SubPub group identity %s", ps.Topic)
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
			log.Debugf("found peer: %s", peer.ID.Pretty())
			ps.Host.ConnManager().Protect(peer.ID, "discoveredPeer")
			ps.handleStream(stream)
		}
	}
}

// Subscribe advertises and subscribes the SubPub host to the network topics
func (ps *SubPub) Subscribe(ctx context.Context) {
	go ps.peersManager()
	go ps.advertise(ctx, ps.Topic)
	go ps.advertise(ctx, ps.PubKey)

	// Distribute broadcast messages to all connected peers.
	go func() {
		for {
			select {
			case <-ps.close:
				return
			case msg := <-ps.BroadcastWriter:
				ps.PeersMu.Lock()
				for _, peer := range ps.Peers {
					if peer.write == nil {
						continue
					}
					select {
					case peer.write <- msg:
					default:
						log.Infof("dropping broadcast message for peer %s", peer.id)
					}
				}
				ps.PeersMu.Unlock()
			}
		}
	}()

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
}

// advertise uses the DHT for announcing ourselves, so other nodes can find us
func (ps *SubPub) advertise(ctx context.Context, topic string) {
	// Initially, we don't wait until the next advertise call.
	var duration time.Duration
	for {
		select {
		case <-time.After(duration):
			log.Infof("advertising topic %s", topic)

			// The duration should be updated, and be in the order
			// of multiple hours.
			var err error
			duration, err = ps.routing.Advertise(ctx, topic,
				corediscovery.Limit(4*ps.MaxDHTpeers))
			if err == nil && duration < time.Second {
				err = fmt.Errorf("refusing to advertise too often: %v", duration)
			}
			if err != nil {
				// TODO: do we care about this error? it happens
				// on the tests pretty often.

				log.Infof("could not advertise topic %q: %v", topic, err)
				// Since the duration is 0s now, reset it to
				// something sane, like 1m.
				duration = time.Minute
			}
		case <-ps.close:
			return
		}
	}
}

package subpub

import (
	"bufio"
	"context"
	"fmt"
	"time"

	corediscovery "github.com/libp2p/go-libp2p-core/discovery"
	libpeer "github.com/libp2p/go-libp2p-core/peer"
	"github.com/libp2p/go-libp2p-core/protocol"
	multiaddr "github.com/multiformats/go-multiaddr"
	"go.vocdoni.io/dvote/log"
)

type peerSub struct {
	id    libpeer.ID
	write chan []byte
}

// PeerStreamWrite looks for an existing connection with peerID and calls the callback function with the writer channel as parameter
func (ps *SubPub) PeerStreamWrite(peerID string, msg []byte) error {
	peerIdx := -1
	ps.PeersMu.RLock()
	defer ps.PeersMu.RUnlock()
	for i, p := range ps.Peers {
		if p.id.String() == peerID {
			peerIdx = i
			break
		}
	}
	if peerIdx < 0 {
		return fmt.Errorf("no connection with peer %s, cannot open stream", peerID)
	}
	ps.Peers[peerIdx].write <- msg
	return nil
}

// FindTopic opens one or multiple new streams with the peers announcing the namespace.
// The callback function is executed once a new stream connection is created
func (ps *SubPub) FindTopic(ctx context.Context, namespace string, callback func(*bufio.ReadWriter)) error {
	log.Infof("searching for topic %s", namespace)
	peerChan, err := ps.routing.FindPeers(ctx, namespace,
		corediscovery.Limit(4*ps.MaxDHTpeers))
	if err != nil {
		return err
	}
	for peer := range peerChan {
		// not myself
		if peer.ID == ps.Host.ID() {
			continue
		}
		log.Infof("found peer: %s", peer.ID)
		stream, err := ps.Host.NewStream(context.Background(), peer.ID, protocol.ID(ps.Topic))
		if err != nil {
			log.Debugf("connection failed: ", err)
			continue
		}
		log.Infof("connected to peer %s", peer.ID)
		callback(bufio.NewReadWriter(bufio.NewReader(stream), bufio.NewWriter(stream)))
	}
	return nil
}

// TransportConnectPeer creates a new libp2p peer connection (transport layer)
func (ps *SubPub) TransportConnectPeer(maddr string) error {
	m, err := multiaddr.NewMultiaddr(maddr)
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

func parseMultiaddress(maddress []string) (ma []multiaddr.Multiaddr) {
	for _, m := range maddress {
		mad, err := multiaddr.NewMultiaddr(m)
		if err != nil {
			log.Warn(err)
		}
		ma = append(ma, mad)
	}
	return ma
}

// peersManager keeps track of the connected peers and executes the network cleaning functions
func (ps *SubPub) peersManager() {
	for {
		select {
		case <-ps.close:
			return
		default:
			// continues below
		}
		ps.PeersMu.Lock()
		// We can't use a range, since we modify the slice in the loop.
		for i := 0; i < len(ps.Peers); i++ {
			peer := ps.Peers[i]
			if len(ps.Host.Network().ConnsToPeer(peer.id)) > 0 {
				continue
			}
			// Remove peer if no active connection
			ps.Peers[i] = ps.Peers[len(ps.Peers)-1]
			ps.Peers = ps.Peers[:len(ps.Peers)-1]
			if fn := ps.onPeerRemove; fn != nil {
				fn(peer.id)
			}
		}
		ps.PeersMu.Unlock()
		tctx, cancel := context.WithTimeout(context.Background(), ps.CollectionPeriod)
		ps.Host.ConnManager().TrimOpenConns(tctx) // Not sure if it works
		cancel()
		time.Sleep(ps.CollectionPeriod)
	}
}

// connectedPeer returns true if the peer is connected
func (ps *SubPub) connectedPeer(pid libpeer.ID) bool {
	ps.PeersMu.Lock()
	defer ps.PeersMu.Unlock()
	for _, peer := range ps.Peers {
		if peer.id.String() == pid.String() {
			return true
		}
	}
	return false
}

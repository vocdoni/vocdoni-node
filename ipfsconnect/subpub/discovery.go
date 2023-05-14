package subpub

import (
	"context"

	libpeer "github.com/libp2p/go-libp2p/core/peer"
	multiaddr "github.com/multiformats/go-multiaddr"
)

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

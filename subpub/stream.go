package subpub

import (
	"bufio"

	"github.com/libp2p/go-libp2p-core/network"
	"go.vocdoni.io/dvote/log"
)

func (ps *SubPub) handleStream(stream network.Stream) {
	// First, ensure that any messages read from the stream are sent to the
	// SubPub.Reader channel.
	go ps.readHandler(stream)

	// Second, ensure that, from now on, any broadcast message is sent to
	// this stream as well.
	// Allow up to 8 queued broadcast messages per peer. This allows us to
	// concurrently spread broadcast messages without blocking, falling back
	// to dropping messages if any peer is slow or disconnects.
	// TODO(mvdan): if another peer opens a stream with us, to just send
	// us a single message (unicast), it's wasteful to also send broadcasts
	// back via that stream.

	write := make(chan []byte, 8)
	pid := stream.Conn().RemotePeer()
	ps.PeersMu.Lock()
	defer ps.PeersMu.Unlock()
	ps.Peers = append(ps.Peers, peerSub{pid, write})
	if fn := ps.onPeerAdd; fn != nil {
		fn(pid)
	}
	log.Infof("connected to peer %s: %+v", pid, stream.Conn().RemoteMultiaddr())
	go ps.broadcastHandler(write, bufio.NewWriter(stream))
}

func (ps *SubPub) broadcastHandler(write <-chan []byte, w *bufio.Writer) {
	for {
		select {
		case <-ps.close:
			return
		case msg := <-write:
			if err := ps.SendMessage(w, msg); err != nil {
				log.Debugf("error writing to buffer: (%s)", err)
				return
			}
			if err := w.Flush(); err != nil {
				log.Debugf("error flushing write buffer: (%s)", err)
				return
			}
		}
	}
}

func (ps *SubPub) readHandler(stream network.Stream) {
	r := bufio.NewReader(stream)
	for {
		select {
		case <-ps.close:
			return
		default:
			// continues below
		}
		message, err := r.ReadBytes(byte(delimiter))
		if err != nil {
			stream.Close()
			log.Debugf("error reading from buffer %s: %s", stream.Conn().RemotePeer().Pretty(), err)
			return
		} else if len(message) == 0 {
			log.Debugf("no data could be read from stream: %s (%+v)", stream.Conn().RemotePeer().Pretty(), stream.Stat())
			continue
		}
		// Remove delimiter
		message = message[:len(message)-1]
		if !ps.Private {
			var ok bool
			message, ok = ps.decrypt(string(message))
			if !ok {
				log.Warn("cannot decrypt message")
				continue
			}
		}
		go func() { ps.Reader <- message }()
	}
}

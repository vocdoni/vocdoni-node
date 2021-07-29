package subpub

import (
	"bufio"
	"io"

	"git.sr.ht/~sircmpwn/go-bare"
	"github.com/libp2p/go-libp2p-core/network"
	"go.vocdoni.io/dvote/log"
)

func (ps *SubPub) handleStream(stream network.Stream) {
	peerClosed := make(chan bool)

	// First, ensure that any messages read from the stream are sent to the
	// SubPub.Reader channel.
	go ps.readHandler(peerClosed, stream)

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
	ps.Peers = append(ps.Peers, peerSub{ // TO-DO this should be a map
		id:         pid,
		peerClosed: peerClosed,
		write:      write,
	})
	if fn := ps.onPeerAdd; fn != nil {
		fn(pid)
	}
	log.Infof("connected to peer %s: %+v", pid, stream.Conn().RemoteMultiaddr())
	go ps.broadcastHandler(peerClosed, write, bufio.NewWriter(stream))
}

func (ps *SubPub) broadcastHandler(peerClosed <-chan bool, write <-chan []byte, w *bufio.Writer) {
	for {
		select {
		case <-ps.close:
			return
		case <-peerClosed:
			return
		case msg := <-write:
			if err := ps.SendMessage(w, msg); err != nil {
				log.Debugf("error writing to buffer: (%s)", err)
				continue
			}
			if err := w.Flush(); err != nil {
				log.Debugf("error flushing write buffer: (%s)", err)
				continue
			}
		}
	}
}

func (ps *SubPub) readHandler(peerClosed <-chan bool, stream network.Stream) {
	r := bufio.NewReader(stream)

	// Ensure that we always close the stream.
	defer stream.Close()

	for {
		select {
		case <-ps.close:
			return
		case <-peerClosed:
			return
		default:
			// continues below
		}
		message := new(Message)
		bare.MaxArrayLength(bareMaxArrayLength)
		bare.MaxUnmarshalBytes(bareMaxUnmarshalBytes)
		if err := bare.UnmarshalReader(io.Reader(r), message); err != nil {
			log.Debugf("error reading stream buffer %s: %v", stream.Conn().RemotePeer().Pretty(), err)
			return
		} else if len(message.Data) == 0 {
			log.Debugf("no data could be read from stream: %s (%+v)", stream.Conn().RemotePeer().Pretty(), stream.Stat())
			continue
		}
		if !ps.Private {
			var ok bool
			message.Data, ok = ps.decrypt(message.Data)
			if !ok {
				log.Warn("cannot decrypt message")
				continue
			}
			message.Peer = stream.Conn().RemotePeer().String()
		}
		go func() { ps.Reader <- message }()
	}
}

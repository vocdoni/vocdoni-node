package subpub

import (
	"bufio"
	"fmt"
	"strings"
	"sync"

	"github.com/libp2p/go-libp2p/core/network"
	libpeer "github.com/libp2p/go-libp2p/core/peer"
	"go.vocdoni.io/dvote/log"
)

// bufioWithMutex is a *bufio.Writer with methods Lock() and Unlock()
type bufioWithMutex struct {
	*bufio.Writer
	sync.Mutex
}

func (ps *SubPub) handleStream(stream network.Stream) {
	// 'stream' will stay open until you close it (or the other side closes it).

	peer := stream.Conn().RemotePeer()
	log.Infof("connected to peer %s: %+v", peer, stream.Conn().RemoteMultiaddr())

	// Any messages read from the stream are sent to the SubPub.Reader channel.
	go ps.readHandler(stream) // ps.readHandler just deals with chans so is thread-safe

	// Create a buffer stream for concurrent, non blocking writes.
	ps.streams.Store(peer, &bufioWithMutex{Writer: bufio.NewWriter(stream)})

	if fn := ps.OnPeerAdd; fn != nil {
		fn(peer)
	}
}

// sendStreamMessage sends a message to a single peer.
func (ps *SubPub) sendStreamMessage(address string, message []byte) error {
	peerID, err := libpeer.Decode(address)
	if err != nil {
		return fmt.Errorf("cannot decode %s into a peerID: %w", address, err)
	}
	value, _ := ps.streams.Load(peerID)
	stream, ok := value.(*bufioWithMutex) // check type to avoid panics
	if !ok {
		return fmt.Errorf("stream for peer %s not found", peerID)
	}

	log.Debugw("sending message", "size", len(message), "peer", peerID)
	stream.Lock()
	defer stream.Unlock()
	if err := ps.writeMessage(stream.Writer, message); err != nil {
		return fmt.Errorf("error trying to unicast to %s: %v", peerID, err)
	}
	return nil
}

// readHandler loops waiting for a Message from stream and passes them to ps.UnicastMsgs chan
func (ps *SubPub) readHandler(stream network.Stream) {
	r := bufio.NewReader(stream)
	peer := stream.Conn().RemotePeer()

	// Ensure that we always close the stream.
	defer stream.Close()

	for {
		select {
		case <-ps.close:
			return
		default:
			// continues below
		}
		message, err := ps.readMessage(r)
		if err != nil {
			if strings.Contains(err.Error(), "stream reset") {
				log.Infof("peer %s disconnected (stream reset)", peer)
			} else {
				log.Errorf("error reading stream from %s: %v", peer, err)
			}
			if fn := ps.OnPeerRemove; fn != nil {
				fn(peer)
			}
			return
		}

		message.Peer = peer.String()
		ps.unicastMsgs <- message
	}
}

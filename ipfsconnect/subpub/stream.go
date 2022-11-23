package subpub

import (
	"bufio"
	"fmt"
	"strings"
	"sync"

	"github.com/libp2p/go-libp2p-core/network"
	libpeer "github.com/libp2p/go-libp2p-core/peer"
	"go.vocdoni.io/dvote/log"
)

// bufioWithMutex is a *bufio.Writer with methods Lock() and Unlock()
type bufioWithMutex struct {
	*bufio.Writer
	*sync.Mutex
}

func (ps *SubPub) handleStream(stream network.Stream) {
	// 'stream' will stay open until you close it (or the other side closes it).

	peer := stream.Conn().RemotePeer()
	log.Infof("connected to peer %s: %+v", peer, stream.Conn().RemoteMultiaddr())

	// Any messages read from the stream are sent to the SubPub.Reader channel.
	go ps.readHandler(stream) // ps.readHandler just deals with chans so is thread-safe

	// Create a buffer stream for concurrent, non blocking writes.
	ps.Streams.Store(peer, bufioWithMutex{bufio.NewWriter(stream), new(sync.Mutex)})

	if fn := ps.OnPeerAdd; fn != nil {
		fn(peer)
	}
}

func (ps *SubPub) Unicast(address string, message []byte) error {
	peerID, err := libpeer.Decode(address)
	if err != nil {
		return fmt.Errorf("cannot decode %s into a peerID: %w", address, err)
	}
	value, found := ps.Streams.Load(peerID)
	stream, ok := value.(bufioWithMutex) // check type to avoid panics
	if !found || !ok {
		return fmt.Errorf("stream for peer %s not found", peerID)
	}

	log.Debugf("sending %d bytes to %s = %s", len(message), address, peerID)

	stream.Lock()
	defer stream.Unlock()
	if err := ps.SendMessage(stream.Writer, message); err != nil {
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
		message, err := ps.ReadMessage(r)
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
		ps.UnicastMsgs <- message
	}
}

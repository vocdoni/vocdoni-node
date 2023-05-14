// Package ipfsconnect provides a service to maintain persistent connections (PersistPeers) between two or more IPFS nodes
package ipfsconnect

import (
	"context"
	"fmt"
	"net"
	"strings"
	"time"

	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/multiformats/go-multiaddr"
	"go.vocdoni.io/dvote/crypto/ethereum"
	"go.vocdoni.io/dvote/data/ipfs"
	"go.vocdoni.io/dvote/ipfsconnect/subpub"
	"go.vocdoni.io/proto/build/go/models"
	"google.golang.org/protobuf/proto"

	"go.vocdoni.io/dvote/log"
)

const (
	// HelloIntervalSeconds is the interval between HELLO messages.
	HelloIntervalSeconds = 60
	// TimeToLiveForMessageSeconds is the time window for messages to be considered valid.
	TimeToLiveForMessageSeconds = 180
)

// IPFSConnect is the service that discover and maintains persistent connections between IPFS nodes.
// It uses a gossip protocol to broadcast messages to all peers sharing a same group key.
// The messages are encrypted with the group key.
type IPFSConnect struct {
	Port            int
	HelloInterval   time.Duration
	Bootnodes       []string
	IPFS            *ipfs.Handler
	Transport       *subpub.SubPub
	GroupKey        [32]byte
	TimestampWindow int32
	Messages        chan *subpub.Message
}

// New creates a new IPFSConnect instance
func New(groupKey string, port int, ipfsHandler *ipfs.Handler) *IPFSConnect {
	is := &IPFSConnect{
		Port:            port,
		HelloInterval:   time.Second * HelloIntervalSeconds,
		IPFS:            ipfsHandler,
		TimestampWindow: TimeToLiveForMessageSeconds, // seconds
		Messages:        make(chan *subpub.Message),
	}
	keyHash := ethereum.HashRaw([]byte(groupKey))
	copy(is.GroupKey[:], keyHash[:])
	return is
}

func (is *IPFSConnect) broadcastMsg(imsg *models.IpfsSync) error {
	imsg.Timestamp = uint32(time.Now().Unix())
	d, err := proto.Marshal(imsg)
	if err != nil {
		return fmt.Errorf("broadcastMsg: %w", err)
	}
	log.Debugw("broadcasting message",
		"msgtype", imsg.Msgtype.String(),
		"address", imsg.Address,
		"multiaddress", imsg.Multiaddress,
		"timestamp", imsg.Timestamp)
	return is.Transport.SendBroadcast(subpub.Message{Data: d})
}

// Handle handles a Message in a thread-safe way.
func (is *IPFSConnect) Handle(msg *models.IpfsSync) error {
	if msg.Address == is.Transport.Address() {
		return nil
	}
	if since := int32(time.Now().Unix()) - int32(msg.Timestamp); since > is.TimestampWindow {
		log.Debugf("discarding old message from %d seconds ago", since)
		return nil
	}
	switch msg.Msgtype {
	case models.IpfsSync_HELLO:
		peers, err := is.IPFS.CoreAPI.Swarm().Peers(is.IPFS.Node.Context())
		if err != nil {
			return err
		}
		found := false
		for _, p := range peers {
			if strings.Contains(msg.Multiaddress, p.ID().String()) {
				found = true
				break
			}
		}
		if !found {
			log.Infow("connecting IPFS to peer", "peer", msg.Multiaddress)
			multiAddr, err := multiaddr.NewMultiaddr(msg.Multiaddress)
			if err != nil {
				return err
			}
			peerInfo, err := peer.AddrInfoFromP2pAddr(multiAddr)
			if err != nil {
				return err
			}
			is.IPFS.Node.PeerHost.ConnManager().Protect(peerInfo.ID, "ipfsconnect")
			return is.IPFS.Node.PeerHost.Connect(context.Background(), *peerInfo)
		}
	}
	return nil
}

func (is *IPFSConnect) sendHelloWithAddr(multiaddress string) {
	var msg models.IpfsSync
	msg.Msgtype = models.IpfsSync_HELLO
	msg.Address = is.Transport.Address()
	msg.Multiaddress = multiaddress
	err := is.broadcastMsg(&msg)
	if err != nil {
		log.Warnw("sendHello: error broadcasting message", "error", err.Error())
	}
}

func (is *IPFSConnect) sendHello() {
	for _, addr := range is.ipfsAddrs() {
		is.sendHelloWithAddr(addr.String())
	}
}

// ipfsAddrs returns a list of peer multiaddresses for the IPFS node.
func (is *IPFSConnect) ipfsAddrs() (maddrs []multiaddr.Multiaddr) {
	ipfs, err := multiaddr.NewMultiaddr("/ipfs/" + is.IPFS.Node.PeerHost.ID().String())
	if err != nil {
		return nil
	}
	for _, maddr := range is.IPFS.Node.PeerHost.Addrs() {
		for _, p := range []int{multiaddr.P_IP4, multiaddr.P_IP6} {
			if v, _ := maddr.ValueForProtocol(p); v != "" {
				if ip := net.ParseIP(v); ip != nil {
					if !ip.IsLoopback() && !ip.IsPrivate() {
						maddrs = append(maddrs, maddr.Encapsulate(ipfs))
					}
				}
			}
		}
	}
	return maddrs
}

// Start initializes and starts an IPFSConnect instance.
func (is *IPFSConnect) Start() {
	is.Transport = subpub.NewSubPub(is.GroupKey, int32(is.Port), is.IPFS.Node)
	is.Transport.BootNodes = is.Bootnodes
	is.Transport.Start(context.Background(), is.Messages)
	go is.handleEvents() // this spawns a single background task per IPFSConnect instance
}

// handleEvents runs an event loop that
// * checks for incoming messages, passing them to is.Handle(),
// * at regular interval sends HELLOs, UPDATEs and calls syncPins()
func (is *IPFSConnect) handleEvents() {
	helloTicker := time.NewTicker(is.HelloInterval)
	defer helloTicker.Stop()

	for {
		select {
		case d := <-is.Messages:
			// receive unicast & broadcast messages and handle them
			var imsg models.IpfsSync
			err := proto.Unmarshal(d.Data, &imsg)
			if err != nil {
				log.Warnf("cannot unmarshal message %s", err)
			} else {
				go is.Handle(&imsg) // handle each incoming message in parallel, since is.Handle() is thread-safe
			}

		case <-helloTicker.C:
			// send hello messages
			is.sendHello()
		}
	}
}

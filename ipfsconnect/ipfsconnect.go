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
	"go.vocdoni.io/dvote/ipfsconnect/subpub"
	"go.vocdoni.io/proto/build/go/models"
	"google.golang.org/protobuf/proto"

	"go.vocdoni.io/dvote/data"
	"go.vocdoni.io/dvote/log"
)

const (
	MaxKeySize = 64
	IPv4       = 4
	IPv6       = 6
)

type IPFSConnect struct {
	Key             string
	PrivKey         string
	Port            int16
	HelloInterval   time.Duration
	Bootnodes       []string
	IPFS            *data.IPFSHandle
	Transport       *subpub.SubPub
	GroupKey        string
	TimestampWindow int32
	Messages        chan *subpub.Message

	private bool
}

// New creates a new IPFSConnect instance. Transports supported are "libp2p" or "privlibp2p"
func New(groupKey, privKeyHex, transport string, storage data.Storage) *IPFSConnect {
	is := &IPFSConnect{
		GroupKey:        groupKey,
		PrivKey:         privKeyHex,
		Port:            4171,
		HelloInterval:   time.Second * 60,
		IPFS:            storage.(*data.IPFSHandle),
		TimestampWindow: 180, // seconds
		Messages:        make(chan *subpub.Message),
	}
	if transport == "privlibp2p" {
		transport = "libp2p"
		is.private = true
	}
	return is
}

func (is *IPFSConnect) broadcastMsg(imsg *models.IpfsSync) error {
	imsg.Timestamp = uint32(time.Now().Unix())
	d, err := proto.Marshal(imsg)
	if err != nil {
		return fmt.Errorf("broadcastMsg: %w", err)
	}
	log.Debugf("broadcasting message %s {Address:%s Hash:%x MA:%s PL:%v Ts:%d}",
		imsg.Msgtype.String(), imsg.Address, imsg.Hash, imsg.Multiaddress, imsg.PinList, imsg.Timestamp)
	return is.Transport.SendBroadcast(subpub.Message{Data: d})
}

// Handle handles a Message in a thread-safe way
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
			log.Infof("connecting IPFS to peer %s", msg.Multiaddress)
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
		log.Warnf("sendHello: %v", err)
	}
}

func (is *IPFSConnect) sendHello() {
	for _, addr := range is.ipfsAddrs() {
		is.sendHelloWithAddr(addr.String())
	}
}

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

// Start initializes and start an IPFSConnect instance
func (is *IPFSConnect) Start() {
	// Init SubPub
	is.Transport = subpub.NewSubPub(is.PrivKey, []byte(is.GroupKey), int32(is.Port), is.private)
	is.Transport.BootNodes = is.Bootnodes
	is.Transport.Start(context.Background(), is.Messages)
	// end Init SubPub

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

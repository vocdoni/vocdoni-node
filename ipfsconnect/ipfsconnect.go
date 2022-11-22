// Package ipfssync provides a service to synchronize IPFS datasets over a p2p network between two or more nodes
package ipfssync

import (
	"context"
	"fmt"
	"net"
	"strings"
	"sync"
	"time"

	"github.com/libp2p/go-libp2p-core/peer"
	"github.com/multiformats/go-multiaddr"
	"go.vocdoni.io/dvote/ipfssync/subpub"
	statedb "go.vocdoni.io/dvote/statedblegacy"
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

type Message struct {
	Type     string `json:"type"`
	Address  string `json:"address,omitempty"`
	Maddress string `json:"mAddress,omitempty"`
	// NodeID    string   `json:"nodeId,omitempty"`
	Hash      string   `json:"hash,omitempty"`
	PinList   []string `json:"pinList,omitempty"`
	Timestamp int32    `json:"timestamp"`
}

type IPFSsync struct {
	DataDir         string
	Key             string
	PrivKey         string
	Port            int16
	HelloInterval   time.Duration
	UpdateInterval  time.Duration
	Bootnodes       []string
	Storage         *data.IPFSHandle
	Transport       *subpub.SubPub
	GroupKey        string
	Timeout         time.Duration
	TimestampWindow int32
	Messages        chan *subpub.Message
	OnlyConnect     bool

	hashTree   statedb.StateTree
	state      statedb.StateDB
	updateLock sync.RWMutex
	lastHash   []byte
	private    bool
}

// NewIPFSsync creates a new IPFSsync instance. Transports supported are "libp2p" or "privlibp2p"
func NewIPFSsync(dataDir, groupKey, privKeyHex, transport string, storage data.Storage) *IPFSsync {
	is := &IPFSsync{
		DataDir:         dataDir,
		GroupKey:        groupKey,
		PrivKey:         privKeyHex,
		Port:            4171,
		HelloInterval:   time.Second * 60,
		UpdateInterval:  time.Second * 20,
		Timeout:         time.Second * 600,
		Storage:         storage.(*data.IPFSHandle),
		TimestampWindow: 180, // seconds
		Messages:        make(chan *subpub.Message),
	}
	if transport == "privlibp2p" {
		transport = "libp2p"
		is.private = true
	}
	return is
}

func (is *IPFSsync) broadcastMsg(imsg *models.IpfsSync) error {
	imsg.Timestamp = uint32(time.Now().Unix())
	d, err := proto.Marshal(imsg)
	if err != nil {
		return fmt.Errorf("broadcastMsg: %w", err)
	}
	log.Debugf("broadcasting message %s {Address:%s Hash:%x MA:%s PL:%v Ts:%d}",
		imsg.Msgtype.String(), imsg.Address, imsg.Hash, imsg.Multiaddress, imsg.PinList, imsg.Timestamp)
	is.Transport.SendBroadcast(subpub.Message{Data: d})
	return nil
}

// Handle handles a Message in a thread-safe way
func (is *IPFSsync) Handle(msg *models.IpfsSync) error {
	if msg.Address == is.Transport.Address() {
		return nil
	}
	if since := int32(time.Now().Unix()) - int32(msg.Timestamp); since > is.TimestampWindow {
		log.Debugf("discarding old message from %d seconds ago", since)
		return nil
	}
	switch msg.Msgtype {
	case models.IpfsSync_HELLO:
		peers, err := is.Storage.CoreAPI.Swarm().Peers(is.Storage.Node.Context())
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
			is.Storage.Node.PeerHost.ConnManager().Protect(peerInfo.ID, "ipfsPeer")
			return is.Storage.Node.PeerHost.Connect(context.Background(), *peerInfo)
		}
	}
	return nil
}

func (is *IPFSsync) sendHelloWithAddr(multiaddress string) {
	var msg models.IpfsSync
	msg.Msgtype = models.IpfsSync_HELLO
	msg.Address = is.Transport.Address()
	msg.Multiaddress = multiaddress
	err := is.broadcastMsg(&msg)
	if err != nil {
		log.Warnf("sendHello: %v", err)
	}
}

func (is *IPFSsync) sendHello() {
	for _, addr := range is.ipfsAddrs() {
		is.sendHelloWithAddr(addr.String())
	}
}

func (is *IPFSsync) ipfsAddrs() (maddrs []multiaddr.Multiaddr) {
	ipfs, err := multiaddr.NewMultiaddr("/ipfs/" + is.Storage.Node.PeerHost.ID().String())
	if err != nil {
		return nil
	}
	for _, maddr := range is.Storage.Node.PeerHost.Addrs() {
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

func (is *IPFSsync) unicastMsg(address string, imsg *models.IpfsSync) error {
	var msg subpub.Message
	imsg.Timestamp = uint32(time.Now().Unix())
	d, err := proto.Marshal(imsg)
	if err != nil {
		return fmt.Errorf("unicastMsg: %w", err)
	}
	msg.Data = d
	log.Debugf("sending message %s {Addr:%s Hash:%x MA:%s PL:%v Ts:%d}",
		imsg.Msgtype.String(), imsg.Address, imsg.Hash, imsg.Multiaddress, imsg.PinList, imsg.Timestamp)
	return is.Transport.SendUnicast(address, msg)
}

// Start initializes and start an IPFSsync instance
func (is *IPFSsync) Start() {
	// Init SubPub
	is.Transport = subpub.NewSubPub(is.PrivKey, []byte(is.GroupKey), int32(is.Port), is.private)
	is.Transport.BootNodes = is.Bootnodes
	is.Transport.Start(context.Background(), is.Messages)
	// end Init SubPub

	go is.handleEvents() // this spawns a single background task per IPFSsync instance
}

// handleEvents runs an event loop that
// * checks for incoming messages, passing them to is.Handle(),
// * at regular interval sends HELLOs, UPDATEs and calls syncPins()
func (is *IPFSsync) handleEvents() {
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

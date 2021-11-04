package subpub

import (
	"context"
	"crypto/ecdsa"
	"encoding/hex"
	"fmt"
	"strings"
	"sync"
	"time"

	"git.sr.ht/~sircmpwn/go-bare"
	eth "github.com/ethereum/go-ethereum/crypto"
	ipfslog "github.com/ipfs/go-log"
	"github.com/libp2p/go-libp2p"
	connmanager "github.com/libp2p/go-libp2p-connmgr"
	crypto "github.com/libp2p/go-libp2p-core/crypto"
	"github.com/libp2p/go-libp2p-core/host"
	libpeer "github.com/libp2p/go-libp2p-core/peer"
	discovery "github.com/libp2p/go-libp2p-discovery"
	dht "github.com/libp2p/go-libp2p-kad-dht"
	"go.vocdoni.io/dvote/crypto/ethereum"
	"go.vocdoni.io/dvote/log"
	"go.vocdoni.io/dvote/util"
)

// UnicastBufSize is the number of unicast incoming messages to buffer.
const UnicastBufSize = 128

// We use go-bare for export/import the trie. In order to support
// big census (up to 8 Million entries) we need to increase the maximums.
const bareMaxArrayLength uint64 = 1024 * 1014 * 8            // 8 Million entries
const bareMaxUnmarshalBytes uint64 = bareMaxArrayLength * 32 // Assuming 32 bytes per entry

const (
	IPv4 = 4
	IPv6 = 6
)

// SubPub is a simplified PubSub protocol using libp2p
type SubPub struct {
	Key           ecdsa.PrivateKey
	GroupKey      [32]byte
	Topic         string
	NoBootStrap   bool
	BootNodes     []string
	PubKey        string
	Private       bool
	MultiAddrIPv4 string
	MultiAddrIPv6 string
	NodeID        string
	Port          int32
	Host          host.Host
	MaxDHTpeers   int

	Gossip      *Gossip       // Gossip deals with broadcasts
	Streams     sync.Map      // this is a thread-safe map[libpeer.ID]bufioWithMutex
	UnicastMsgs chan *Message // UnicastMsgs passes unicasts around
	Messages    chan *Message // both unicasts and broadcasts end up being passed to Messages

	DiscoveryPeriod time.Duration

	// TODO(mvdan): replace with a context
	close   chan bool
	privKey string
	dht     *dht.IpfsDHT
	routing *discovery.RoutingDiscovery

	// These are used in testing
	OnPeerAdd    func(id libpeer.ID)
	OnPeerRemove func(id libpeer.ID)
}

type Message struct {
	Data []byte
	Peer string
}

// NewSubPub creates a new SubPub instance.
// The private key is used to identify the node (by derivating its pubKey) on the p2p network.
// The groupKey is a secret shared among the PubSub participants. Only those with the key will be able to join.
// If private enabled, a libp2p private network is created using the groupKey as shared secret (experimental).
// If private enabled the default bootnodes will not work.
func NewSubPub(hexKey string, groupKey []byte, port int32, private bool) *SubPub {
	ps := new(SubPub)

	s := ethereum.NewSignKeys()
	if err := s.AddHexKey(hexKey); err != nil {
		log.Fatalf("cannot import privkey %s: %s", hexKey, err)
	}
	ps.Key = s.Private

	if len(groupKey) < 4 {
		panic("subpub group key is too small; 4 bytes at minimum")
	}
	copy(ps.GroupKey[:], ethereum.HashRaw(groupKey)[:32])
	ps.Topic = fmt.Sprintf("%x", ethereum.HashRaw([]byte("topic"+string(groupKey))))
	ps.PubKey = hex.EncodeToString(eth.CompressPubkey(&ps.Key.PublicKey))
	ps.privKey = hex.EncodeToString(ps.Key.D.Bytes())
	ps.Port = port
	ps.Private = private
	ps.DiscoveryPeriod = time.Second * 10
	ps.MaxDHTpeers = 128
	ps.close = make(chan bool)
	ps.UnicastMsgs = make(chan *Message, UnicastBufSize)

	bare.MaxArrayLength(bareMaxArrayLength)
	bare.MaxUnmarshalBytes(bareMaxUnmarshalBytes)

	return ps
}

// Start connects the SubPub networking stack
// and begins passing incoming messages to the receiver chan
func (ps *SubPub) Start(ctx context.Context, receiver chan *Message) {
	log.Infof("public address: %s", ps.PubKey)
	log.Infof("private key: %s", ps.privKey)
	if len(ps.Topic) == 0 {
		log.Fatal("no group key provided")
	}
	ipfslog.SetLogLevel("*", "ERROR")

	prvKey, _, err := crypto.GenerateKeyPairWithReader(crypto.ECDSA, 2048, strings.NewReader(ps.privKey))
	if err != nil {
		log.Fatal(err)
	}

	var c libp2p.Config
	libp2p.Defaults(&c)

	// libp2p will listen on any interface device (both on IPv4 and IPv6)
	c.ListenAddrs = nil
	err = c.Apply(libp2p.ListenAddrStrings(
		fmt.Sprintf("/ip6/::/tcp/%d", ps.Port),
		fmt.Sprintf("/ip4/0.0.0.0/tcp/%d", ps.Port),
	))
	if err != nil {
		log.Fatal(err)
	}

	c.RelayCustom = true
	c.Relay = false
	c.EnableAutoRelay = false
	c.PeerKey = prvKey
	if ps.Private {
		c.PSK = ps.GroupKey[:32]
	}
	c.ConnManager = connmanager.NewConnManager(ps.MaxDHTpeers/2, ps.MaxDHTpeers, time.Second*10)

	log.Debugf("libp2p config: %+v", c)
	// Note that we don't use ctx here, since we stop via the Close method.
	ps.Host, err = c.NewNode(context.Background())
	if err != nil {
		log.Fatal(err)
	}
	log.Debug("libp2p host created: ", ps.Host.ID())
	log.Debug("libp2p host addrs: ", ps.Host.Addrs())

	ipv4, err4 := util.PublicIP(IPv4)
	ipv6, err6 := util.PublicIP(IPv6)

	// Fail only if BOTH ipv4 and ipv6 failed.
	if err4 != nil && err6 != nil {
		log.Fatalf("ipv4: %s; ipv6: %s", err4, err6)
	}
	if ipv4 != nil {
		ps.MultiAddrIPv4 = fmt.Sprintf("/ip4/%s/tcp/%d/p2p/%s", ipv4, ps.Port, ps.Host.ID())
		log.Infof("my libp2p multiaddress ipv4: %s", ps.MultiAddrIPv4)
	}
	if ipv6 != nil {
		ps.MultiAddrIPv6 = fmt.Sprintf("/ip6/%s/tcp/%d/p2p/%s", ipv6, ps.Port, ps.Host.ID())
		log.Infof("my libp2p multiaddress ipv6: %s", ps.MultiAddrIPv6)
	}

	ps.NodeID = ps.Host.ID().String()
	ps.Messages = receiver

	ps.setupDiscovery(ctx)

	ps.setupGossip(ctx)

	go ps.Listen(receiver) // this spawns a single background task per IPFSsync instance, that just deals with chans

	go ps.printStats() // this spawns a single background task per IPFSsync instance, that just prints logs
}

func (ps *SubPub) Listen(receiver chan<- *Message) {
	for {
		select {
		case <-ps.close:
			return
		case spmsg := <-ps.Gossip.Messages:
			log.Debugf("received gossip (%d bytes) from %s", len(spmsg.Data), spmsg.Peer)
			receiver <- spmsg
		case spmsg := <-ps.UnicastMsgs:
			log.Debugf("received unicast (%d bytes) from %s", len(spmsg.Data), spmsg.Peer)
			receiver <- spmsg
		}
	}
}

// Close terminaters the subpub networking stack
func (ps *SubPub) Close() error {
	log.Debug("received close signal")
	select {
	case <-ps.close:
		// already closed
		return nil
	default:
		close(ps.close)
		if err := ps.dht.Close(); err != nil {
			return err
		}
		return ps.Host.Close()
	}
}

func (ps *SubPub) String() string {
	return fmt.Sprintf("dhtPeers:%d dhtKnown:%d clusterPeers:%d",
		len(ps.Host.Network().Peers()),
		len(ps.Host.Peerstore().PeersWithAddrs()),
		len(ps.Gossip.topic.ListPeers()))
}

func (s *SubPub) Address() string {
	return s.NodeID
}

func (s *SubPub) SendBroadcast(msg Message) error {
	return s.Gossip.Publish(msg.Data)
}

func (s *SubPub) SendUnicast(address string, msg Message) error {
	return s.Unicast(address, msg.Data)
}

func (ps *SubPub) printStats() {
	for {
		time.Sleep(120 * time.Second)
		log.Infof("[subPub info] %s", ps)
	}
}

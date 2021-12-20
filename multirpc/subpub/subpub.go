package subpub

import (
	"context"
	"crypto/ecdsa"
	"encoding/hex"
	"fmt"
	"strings"
	"sync"
	"time"

	eth "github.com/ethereum/go-ethereum/crypto"
	ipfslog "github.com/ipfs/go-log"
	"github.com/libp2p/go-libp2p"
	connmanager "github.com/libp2p/go-libp2p-connmgr"
	crypto "github.com/libp2p/go-libp2p-core/crypto"
	"github.com/libp2p/go-libp2p-core/host"
	libpeer "github.com/libp2p/go-libp2p-core/peer"
	"github.com/libp2p/go-libp2p-core/protocol"
	discovery "github.com/libp2p/go-libp2p-discovery"
	dht "github.com/libp2p/go-libp2p-kad-dht"
	dhtopts "github.com/libp2p/go-libp2p-kad-dht/opts"
	"go.vocdoni.io/dvote/crypto/ethereum"
	"go.vocdoni.io/dvote/log"
	"go.vocdoni.io/dvote/util"
)

// We use go-bare for export/import the trie. In order to support
// big census (up to 8 Million entries) we need to increase the maximums.

const bareMaxArrayLength uint64 = 1024 * 1014 * 8 // 8 Million entires

const bareMaxUnmarshalBytes uint64 = bareMaxArrayLength * 32 // Assuming 32 bytes per entry

const (
	IPv4 = 4
	IPv6 = 6
)

// SubPub is a simplified PubSub protocol using libp2p
type SubPub struct {
	Key             ecdsa.PrivateKey
	GroupKey        [32]byte
	Topic           string
	BroadcastWriter chan []byte
	Reader          chan *Message
	NoBootStrap     bool
	BootNodes       []string
	PubKey          string
	Private         bool
	MultiAddrIPv4   string
	MultiAddrIPv6   string
	NodeID          string
	Port            int32
	Host            host.Host
	MaxDHTpeers     int

	PeersMu sync.RWMutex
	Peers   []peerSub // TODO(p4u): this should be a map

	DiscoveryPeriod  time.Duration
	CollectionPeriod time.Duration

	// TODO(mvdan): replace with a context
	close   chan bool
	privKey string
	dht     *dht.IpfsDHT
	routing *discovery.RoutingDiscovery

	// These are useful for testing.
	onPeerAdd    func(id libpeer.ID)
	onPeerRemove func(id libpeer.ID)
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
func NewSubPub(key ecdsa.PrivateKey, groupKey []byte, port int32, private bool) *SubPub {
	ps := new(SubPub)
	ps.Key = key
	if len(groupKey) < 4 {
		panic("subpub group key is too small; 4 bytes at minimum")
	}
	copy(ps.GroupKey[:], ethereum.HashRaw(groupKey)[:32])
	ps.Topic = fmt.Sprintf("%x", ethereum.HashRaw([]byte("topic"+string(groupKey))))
	ps.PubKey = hex.EncodeToString(eth.CompressPubkey(&key.PublicKey))
	ps.privKey = hex.EncodeToString(key.D.Bytes())
	ps.BroadcastWriter = make(chan []byte)
	ps.Reader = make(chan *Message)
	ps.Port = port
	ps.Private = private
	ps.DiscoveryPeriod = time.Second * 10
	ps.CollectionPeriod = time.Second * 10
	ps.MaxDHTpeers = 128
	ps.close = make(chan bool)
	return ps
}

// Start connects  the SubPub networking stack
func (ps *SubPub) Start(ctx context.Context) {
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

	ipv4, err4 := util.PublicIP(IPv4)
	ipv6, err6 := util.PublicIP(IPv6)

	// Fail only if BOTH ipv4 and ipv6 failed.
	if err4 != nil && err6 != nil {
		log.Fatal("ipv4: %s; ipv6: %s", err4, err6)
	}
	if ipv4 != nil {
		ps.MultiAddrIPv4 = fmt.Sprintf("/ip4/%s/tcp/%d/p2p/%s", ipv4, ps.Port, ps.Host.ID())
		log.Infof("my subpub multiaddress ipv4: %s", ps.MultiAddrIPv4)
	}
	if ipv6 != nil {
		ps.MultiAddrIPv6 = fmt.Sprintf("/ip6/%s/tcp/%d/p2p/%s", ipv6, ps.Port, ps.Host.ID())
		log.Infof("my subpub multiaddress ipv6: %s", ps.MultiAddrIPv6)
	}

	ps.NodeID = ps.Host.ID().String()

	// Set a function as stream handler. This function is called when a peer
	// initiates a connection and starts a stream with this peer.
	ps.Host.SetStreamHandler(protocol.ID(ps.Topic), ps.handleStream)

	// Start a DHT, for use in peer discovery. We can't just make a new DHT
	// client because we want each peer to maintain its own local copy of the
	// DHT, so that the bootstrapping node of the DHT can go down without

	// Let's try to apply some tunning for reducing the DHT fingerprint
	opts := []dhtopts.Option{
		dht.RoutingTableLatencyTolerance(time.Second * 20),
		dht.BucketSize(20),
		dht.MaxRecordAge(1 * time.Hour),
	}

	// Note that we don't use ctx here, since we stop via the Close method.
	ps.dht, err = dht.New(context.Background(), ps.Host, opts...)
	if err != nil {
		log.Fatal(err)
	}

	if !ps.NoBootStrap {
		// Bootstrap the DHT. In the default configuration, this spawns a Background
		// thread that will refresh the peer table every five minutes.
		log.Info("bootstrapping the DHT")
		// Note that we don't use ctx here, since we stop via the Close method.
		if err := ps.dht.Bootstrap(context.Background()); err != nil {
			log.Fatal(err)
		}

		// Let's connect to the bootstrap nodes first. They will tell us about the
		// other nodes in the network.
		bootnodes := dht.DefaultBootstrapPeers
		if len(ps.BootNodes) > 0 {
			bootnodes = parseMultiaddress(ps.BootNodes)
		}

		log.Info("connecting to bootstrap nodes...")
		log.Debugf("bootnodes: %+v", bootnodes)
		var wg sync.WaitGroup
		for _, peerAddr := range bootnodes {
			if peerAddr == nil {
				continue
			}
			peerinfo, err := libpeer.AddrInfoFromP2pAddr(peerAddr)
			if err != nil {
				log.Fatal(err)
			}
			if peerinfo == nil {
				continue // nothing to do
			}
			wg.Add(1)
			go func() {
				defer wg.Done()
				log.Debugf("trying %s", *peerinfo)
				if err := ps.Host.Connect(ctx, *peerinfo); err != nil {
					log.Debug(err)
				} else {
					log.Infof("connection established with bootstrap node: %s", peerinfo)
				}
			}()
		}
		wg.Wait()
	}
	// We use a rendezvous point "meet me here" to announce our location.
	// This is like telling your friends to meet you at the Eiffel Tower.
	ps.routing = discovery.NewRoutingDiscovery(ps.dht)
	go ps.printStats()
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
		len(ps.Peers))
}

func (ps *SubPub) printStats() {
	for {
		time.Sleep(120 * time.Second)
		log.Infof("[subPub info] %s", ps)
	}
}

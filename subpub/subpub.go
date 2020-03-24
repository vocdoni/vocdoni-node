package subpub

import (
	"bufio"
	"context"
	"crypto/ecdsa"
	"crypto/rand"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"io"
	"strings"
	"sync"
	"time"

	"github.com/ethereum/go-ethereum/common/hexutil"

	eth "github.com/ethereum/go-ethereum/crypto"
	ipfslog "github.com/ipfs/go-log"
	"github.com/libp2p/go-libp2p"
	crypto "github.com/libp2p/go-libp2p-core/crypto"
	"github.com/libp2p/go-libp2p-core/host"
	"github.com/libp2p/go-libp2p-core/network"
	"github.com/libp2p/go-libp2p-core/peer"
	"github.com/libp2p/go-libp2p-core/protocol"
	discovery "github.com/libp2p/go-libp2p-discovery"
	dht "github.com/libp2p/go-libp2p-kad-dht"
	dhtopts "github.com/libp2p/go-libp2p-kad-dht/opts"
	multiaddr "github.com/multiformats/go-multiaddr"

	"gitlab.com/vocdoni/go-dvote/crypto/signature"
	"gitlab.com/vocdoni/go-dvote/log"
	"gitlab.com/vocdoni/go-dvote/util"
	"golang.org/x/crypto/nacl/secretbox"
)

const delimiter = '\x00'

type SubPub struct {
	Key             *ecdsa.PrivateKey
	GroupKey        [32]byte
	Topic           string
	BroadcastWriter chan []byte
	Reader          chan []byte
	NoBootStrap     bool
	BootNodes       []string
	PubKey          string
	Private         bool
	MultiAddr       string
	Port            int
	Host            host.Host
	GroupPeers      []peer.ID
	DiscoveryPeriod time.Duration
	GroupMu         sync.RWMutex

	// TODO(mvdan): replace with a context
	close   chan bool
	privKey string
	dht     *dht.IpfsDHT
	routing *discovery.RoutingDiscovery
}

// TODO(mvdan): we probably don't want to hard-code bufio in the API this much.

func (ps *SubPub) handleStream(stream network.Stream) {
	log.Info("got a new stream!")

	// Create a buffer stream for non blocking read and write.
	rw := bufio.NewReadWriter(bufio.NewReader(stream), bufio.NewWriter(stream))

	go ps.readHandler(rw)
	go ps.broadcastHandler(rw)
	// 'stream' will stay open until you close it (or the other side closes it).
}

// SendMessage encrypts and writes a message on the readwriter buffer
func (ps *SubPub) SendMessage(rw *bufio.ReadWriter, msg []byte) error {
	log.Debugf("sending message: %s", msg)
	if !ps.Private {
		msg = []byte(ps.encrypt(msg)) // TO-DO find a better way to encapsulate data! byte -> b64 -> byte is not the best...
	}
	if _, err := rw.Write(append(msg, byte(delimiter))); err != nil {
		return err
	}
	return rw.Flush()
}

func (ps *SubPub) broadcastHandler(rw *bufio.ReadWriter) {
	for {
		select {
		case <-ps.close:
			return
		default:
			if err := ps.SendMessage(rw, <-ps.BroadcastWriter); err != nil {
				log.Warnf("error writing to buffer: %s", err)
				return
			}
			rw.Flush()
		}
	}
}

func (ps *SubPub) readHandler(rw *bufio.ReadWriter) {
	for {
		select {
		case <-ps.close:
			return
		default:
			// continues below
		}
		message, err := rw.ReadBytes(byte(delimiter))
		if err != nil {
			log.Debugf("error reading from buffer: %s", err)
			return
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
		log.Debugf("message received: %s", message)
		go func() { ps.Reader <- message }()
	}
}

func NewSubPub(key *ecdsa.PrivateKey, groupKey string, port int, private bool) *SubPub {
	ps := new(SubPub)
	ps.Key = key
	copy(ps.GroupKey[:], signature.HashRaw(groupKey)[:32])
	ps.Topic = fmt.Sprintf("%x", signature.HashRaw("topic"+groupKey))
	ps.PubKey = hexutil.Encode(eth.CompressPubkey(&key.PublicKey))
	ps.privKey = hex.EncodeToString(key.D.Bytes())
	ps.BroadcastWriter = make(chan []byte)
	ps.Reader = make(chan []byte)
	ps.Port = port
	ps.Private = private
	ps.DiscoveryPeriod = time.Second * 10
	ps.close = make(chan bool)
	return ps
}

func (ps *SubPub) Connect(ctx context.Context) {
	log.Infof("public address: %s", ps.PubKey)
	log.Infof("private key: %s", ps.privKey)
	if len(ps.Topic) == 0 {
		log.Fatal("no group key provided")
		return
	}
	ipfslog.SetLogLevel("*", "ERROR")

	// 0.0.0.0 will listen on any interface device.
	sourceMultiAddr, err := multiaddr.NewMultiaddr(fmt.Sprintf("/ip4/0.0.0.0/tcp/%d", ps.Port))
	if err != nil {
		log.Fatal(err)
	}
	prvKey, _, err := crypto.GenerateKeyPairWithReader(crypto.ECDSA, 2048, strings.NewReader(ps.privKey))
	if err != nil {
		log.Fatal(err)
	}
	var c libp2p.Config
	libp2p.Defaults(&c)
	c.RelayCustom = true
	c.Relay = false
	c.EnableAutoRelay = false
	c.PeerKey = prvKey
	if ps.Private {
		c.PSK = ps.GroupKey[:32]
	}
	c.ListenAddrs = []multiaddr.Multiaddr{sourceMultiAddr}

	log.Debugf("libp2p config: %+v", c)
	ps.Host, err = c.NewNode(ctx)
	if err != nil {
		log.Fatal(err)
		return
	}

	ip, err := util.PublicIP()
	if err != nil {
		log.Fatal(err)
	}
	ps.MultiAddr = fmt.Sprintf("/ip4/%s/tcp/%d/p2p/%s", ip, ps.Port, ps.Host.ID())
	log.Infof("my subpub multiaddress %s", ps.MultiAddr)

	// Set a function as stream handler. This function is called when a peer
	// initiates a connection and starts a stream with this peer.
	ps.Host.SetStreamHandler(protocol.ID(ps.Topic), ps.handleStream)

	// Start a DHT, for use in peer discovery. We can't just make a new DHT
	// client because we want each peer to maintain its own local copy of the
	// DHT, so that the bootstrapping node of the DHT can go down without

	// Let's try to apply some tunning for reducing the DHT fingerprint
	opts := []dhtopts.Option{
		dhtopts.RoutingTableLatencyTolerance(time.Second * 20),
		dhtopts.BucketSize(5),
		dhtopts.MaxRecordAge(1 * time.Hour),
	}

	ps.dht, err = dht.New(ctx, ps.Host, opts...)
	if err != nil {
		log.Fatal(err)
		return
	}

	if !ps.NoBootStrap {
		// Bootstrap the DHT. In the default configuration, this spawns a Background
		// thread that will refresh the peer table every five minutes.
		log.Info("bootstrapping the DHT")
		if err := ps.dht.Bootstrap(ctx); err != nil {
			log.Fatal(err)
			return
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
			peerinfo, err := peer.AddrInfoFromP2pAddr(peerAddr)
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
}

func (ps *SubPub) Close() {
	log.Debug("received close signal")
	ps.close <- true
}

func (ps *SubPub) advertise(ctx context.Context, topic string) {
	// Initially, we don't wait until the next advertise call.
	var duration time.Duration
	for {
		select {
		case <-time.After(duration):
			log.Infof("advertising topic %s", topic)

			// The duration should be updated, and be in the order
			// of multiple hours.
			var err error
			duration, err = ps.routing.Advertise(ctx, topic)
			if err == nil && duration < time.Second {
				err = fmt.Errorf("refusing to advertise too often: %v", duration)
			}
			if err != nil {
				// TODO: do we care about this error? it happens
				// on the tests pretty often.

				log.Infof("could not advertise topic %q: %v", topic, err)
				// Since the duration is 0s now, reset it to
				// something sane, like 1m.
				duration = time.Minute
			}
		case <-ps.close:
			return
		}
	}
}

func (ps *SubPub) Subscribe(ctx context.Context) {
	go ps.peersManager()
	go ps.advertise(ctx, ps.Topic)
	go ps.advertise(ctx, ps.PubKey)

	for {
		select {
		case <-ps.close:
			return
		default:
			ps.discover(ctx)
			time.Sleep(ps.DiscoveryPeriod)
		}
	}
}

func (ps *SubPub) PeerConnect(pubKey string, callback func(*bufio.ReadWriter)) error {
	ctx := context.TODO()

	log.Infof("searching for peer %s", pubKey)
	peerChan, err := ps.routing.FindPeers(ctx, pubKey)
	if err != nil {
		return err
	}
	for peer := range peerChan {
		// not myself
		if peer.ID == ps.Host.ID() {
			continue
		}

		log.Infof("found peer: %s", peer.ID)

		stream, err := ps.Host.NewStream(ctx, peer.ID, protocol.ID(ps.Topic))
		if err != nil {
			log.Debugf("connection failed: ", err)
			continue
		}
		if !ps.connectedPeer(peer.ID) {
			ps.addConnectedPeer(peer.ID)
		}
		log.Infof("connected to peer %s", peer.ID)
		callback(bufio.NewReadWriter(bufio.NewReader(stream), bufio.NewWriter(stream)))
	}
	return nil
}

func parseMultiaddress(maddress []string) (ma []multiaddr.Multiaddr) {
	for _, m := range maddress {
		mad, err := multiaddr.NewMultiaddr(m)
		if err != nil {
			log.Warn(err)
		}
		ma = append(ma, mad)
	}
	return ma
}

func (ps *SubPub) peersManager() {
	for {
		select {
		case <-ps.close:
			return
		default:
			ps.GroupMu.Lock()
			for i, p := range ps.GroupPeers {
				if len(ps.Host.Network().ConnsToPeer(p)) == 0 {
					ps.GroupPeers[i] = ps.GroupPeers[len(ps.GroupPeers)-1]
					ps.GroupPeers[len(ps.GroupPeers)-1] = ""
					ps.GroupPeers = ps.GroupPeers[:len(ps.GroupPeers)-1]
				}
			}
			ps.GroupMu.Unlock()
			time.Sleep(10 * time.Second)
		}
	}
}

func (ps *SubPub) addConnectedPeer(pid peer.ID) {
	ps.GroupMu.Lock()
	defer ps.GroupMu.Unlock()
	ps.GroupPeers = append(ps.GroupPeers, pid)
}

func (ps *SubPub) connectedPeer(pid peer.ID) bool {
	ps.GroupMu.Lock()
	defer ps.GroupMu.Unlock()
	for _, p := range ps.GroupPeers {
		if p == pid {
			return true
		}
	}
	return false
}

func (ps *SubPub) discover(ctx context.Context) {
	// Now, look for others who have announced
	// This is like your friend telling you the location to meet you.
	log.Debugf("searching for SubPub group identity %s", ps.Topic)
	peerChan, err := ps.routing.FindPeers(ctx, ps.Topic)
	if err != nil {
		log.Fatal(err)
		return
	}
	for peer := range peerChan {
		select {
		case <-ps.close:
			return
		default:
			// continues below
		}
		if peer.ID == ps.Host.ID() {
			continue // this is us; skip
		}

		if ps.connectedPeer(peer.ID) {
			log.Debugf("peer %s already connected", peer.ID)
			continue
		}
		// new peer; let's connect to it
		log.Debugf("found peer: %s", peer.ID)

		stream, err := ps.Host.NewStream(ctx, peer.ID, protocol.ID(ps.Topic))
		if err != nil {
			// Since this error is pretty common in p2p networks.
			log.Debugf("could not connect to peer %s: %v", peer.ID, err)
			continue
		}
		ps.addConnectedPeer(peer.ID)
		log.Infof("connected to peer %s", peer.ID)
		rw := bufio.NewReadWriter(bufio.NewReader(stream), bufio.NewWriter(stream))
		go ps.broadcastHandler(rw)
		go ps.readHandler(rw)
	}
}

func (ps *SubPub) encrypt(msg []byte) string {
	var nonce [24]byte
	if _, err := io.ReadFull(rand.Reader, nonce[:]); err != nil {
		log.Error(err)
		return ""
	}
	return base64.StdEncoding.EncodeToString(secretbox.Seal(nonce[:], msg, &nonce, &ps.GroupKey))
}

func (ps *SubPub) decrypt(b64msg string) ([]byte, bool) {
	msg, err := base64.StdEncoding.DecodeString(b64msg)
	if err != nil {
		return nil, false
	}
	if len(msg) < 25 {
		return nil, false
	}
	var decryptNonce [24]byte
	copy(decryptNonce[:], msg[:24])
	return secretbox.Open(nil, msg[24:], &decryptNonce, &ps.GroupKey)
}

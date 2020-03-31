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
	libpeer "github.com/libp2p/go-libp2p-core/peer"
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

	PeersMu sync.RWMutex
	Peers   []peerSub

	DiscoveryPeriod  time.Duration
	CollectionPeriod time.Duration

	// TODO(mvdan): replace with a context
	close   chan bool
	privKey string
	dht     *dht.IpfsDHT
	routing *discovery.RoutingDiscovery
}

type peerSub struct {
	id    libpeer.ID
	write chan []byte
}

func (ps *SubPub) handleStream(stream network.Stream) {
	log.Info("got a new stream!")

	// First, ensure that any messages read from the stream are sent to the
	// SubPub.Reader channel.
	go ps.readHandler(bufio.NewReader(stream))

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
	log.Infof("connected to peer %s", pid)
	go ps.broadcastHandler(write, bufio.NewWriter(stream))
}

// SendMessage encrypts and writes a message on the readwriter buffer
func (ps *SubPub) SendMessage(w *bufio.Writer, msg []byte) error {
	log.Debugf("sending message: %s", msg)
	if !ps.Private {
		msg = []byte(ps.encrypt(msg)) // TO-DO find a better way to encapsulate data! byte -> b64 -> byte is not the best...
	}
	if _, err := w.Write(append(msg, byte(delimiter))); err != nil {
		return err
	}
	return w.Flush()
}

func (ps *SubPub) broadcastHandler(write <-chan []byte, w *bufio.Writer) {
	for {
		select {
		case <-ps.close:
			return
		case msg := <-write:
			if err := ps.SendMessage(w, msg); err != nil {
				log.Warnf("error writing to buffer: %s", err)
				return
			}
			if err := w.Flush(); err != nil {
				log.Warnf("error flushing write buffer: %s", err)
				return
			}
		}
	}
}

func (ps *SubPub) readHandler(r *bufio.Reader) {
	for {
		select {
		case <-ps.close:
			return
		default:
			// continues below
		}
		message, err := r.ReadBytes(byte(delimiter))
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
	ps.CollectionPeriod = time.Second * 10
	ps.close = make(chan bool)
	return ps
}

func (ps *SubPub) Connect(ctx context.Context) {
	log.Infof("public address: %s", ps.PubKey)
	log.Infof("private key: %s", ps.privKey)
	if len(ps.Topic) == 0 {
		log.Fatal("no group key provided")
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
	// Note that we don't use ctx here, since we stop via the Close method.
	ps.Host, err = c.NewNode(context.Background())
	if err != nil {
		log.Fatal(err)
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
}

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

	// Distribute broadcast messages to all connected peers.
	go func() {
		for {
			select {
			case <-ps.close:
				return
			case msg := <-ps.BroadcastWriter:
				ps.PeersMu.Lock()
				for _, peer := range ps.Peers {
					if peer.write == nil {
						continue
					}
					select {
					case peer.write <- msg:
					default:
						log.Infof("dropping broadcast message for peer %s", peer.id)
					}
				}
				ps.PeersMu.Unlock()
			}
		}
	}()

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

// TODO(mvdan): test and refactor this "unicast" method.

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
			// continues below
		}
		ps.PeersMu.Lock()
		// We can't use a range, since we modify the slice in the loop.
		for i := 0; i < len(ps.Peers); i++ {
			peer := ps.Peers[i]
			if len(ps.Host.Network().ConnsToPeer(peer.id)) > 0 {
				continue
			}
			// Remove peer if no active connection
			ps.Peers[i] = ps.Peers[len(ps.Peers)-1]
			ps.Peers = ps.Peers[:len(ps.Peers)-1]
		}
		ps.PeersMu.Unlock()
		time.Sleep(ps.CollectionPeriod)
	}
}

func (ps *SubPub) connectedPeer(pid libpeer.ID) bool {
	ps.PeersMu.Lock()
	defer ps.PeersMu.Unlock()
	for _, peer := range ps.Peers {
		if peer.id == pid {
			return true
		}
	}
	return false
}

func (ps *SubPub) discover(ctx context.Context) {
	// Now, look for others who have announced.
	// This is like your friend telling you the location to meet you.
	log.Debugf("searching for SubPub group identity %s", ps.Topic)
	peerChan, err := ps.routing.FindPeers(ctx, ps.Topic)
	if err != nil {
		log.Fatal(err)
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
		ps.handleStream(stream)
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

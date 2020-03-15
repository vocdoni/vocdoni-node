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
	BroadcastWriter chan ([]byte)
	Reader          chan ([]byte)
	BootNodes       []string
	PubKey          string
	Private         bool
	port            int32
	privKey         string
	host            host.Host
	groupPeers      []peer.ID
	dht             *dht.IpfsDHT
	routing         *discovery.RoutingDiscovery
}

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
		if err := ps.SendMessage(rw, <-ps.BroadcastWriter); err != nil {
			log.Warnf("error writing to buffer: %s", err)
			return
		}
		rw.Flush()
	}
}

func (ps *SubPub) readHandler(rw *bufio.ReadWriter) {
	for {
		var err error
		var message []byte
		if message, err = rw.ReadBytes(byte(delimiter)); err != nil {
			log.Debugf("error reading from buffer: %s", err)
			return
		}
		if !ps.Private {
			var ok bool
			message, ok = ps.decrypt(string(message[:len(message)-1]))
			if !ok {
				log.Warn("cannot decrypt message")
				continue
			}
		}
		log.Debugf("message received: %s", message)
		go func() { ps.Reader <- message }()
	}
}

func NewSubPub(key *ecdsa.PrivateKey, groupKey string, port int32, private bool) SubPub {
	var ps SubPub
	ps.Key = key
	copy(ps.GroupKey[:], signature.HashRaw(groupKey)[:32])
	ps.Topic = fmt.Sprintf("%x", signature.HashRaw("topic"+groupKey))
	ps.PubKey = hexutil.Encode(eth.CompressPubkey(&key.PublicKey))
	ps.privKey = hex.EncodeToString(key.D.Bytes())
	ps.BroadcastWriter = make(chan []byte)
	ps.Reader = make(chan []byte)
	ps.port = port
	ps.Private = private
	return ps
}

func (ps *SubPub) Connect() {
	ctx := context.Background()
	log.Infof("public address: %s", ps.PubKey)
	log.Infof("private key: %s", ps.privKey)
	if len(ps.Topic) == 0 {
		log.Fatal("no group key provided")
		return
	}
	ipfslog.SetLogLevel("*", "ERROR")

	// 0.0.0.0 will listen on any interface device.
	sourceMultiAddr, _ := multiaddr.NewMultiaddr(fmt.Sprintf("/ip4/0.0.0.0/tcp/%d", ps.port))
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
	ps.host, err = c.NewNode(ctx)
	/*ps.host, err = libp2p.New(ctx,
		libp2p.ListenAddrs(sourceMultiAddr),
		libp2p.DisableRelay(),
		libp2p.Identity(prvKey),
		libp2p.PrivateNetwork(psk),
		libp2p.NATPortMap(),
	)
	*/
	if err != nil {
		log.Fatal(err)
		return
	}
	//log.Infof("libp2p peerID: %s", ps.host.ID())
	ip, _ := util.PublicIP()
	log.Infof("my subpub multiaddr /ip4/%s/tcp/%d/p2p/%s ", ip, ps.port, ps.host.ID())

	// Set a function as stream handler. This function is called when a peer
	// initiates a connection and starts a stream with this peer.
	ps.host.SetStreamHandler(protocol.ID(ps.Topic), ps.handleStream)

	// Start a DHT, for use in peer discovery. We can't just make a new DHT
	// client because we want each peer to maintain its own local copy of the
	// DHT, so that the bootstrapping node of the DHT can go down without

	// Let's try to apply some tunning for reducing the DHT fingerprint
	var opts = []dhtopts.Option{
		dhtopts.RoutingTableLatencyTolerance(time.Second * 20),
		dhtopts.BucketSize(5),
		dhtopts.MaxRecordAge(1 * time.Hour),
	}

	ps.dht, err = dht.New(ctx, ps.host, opts...)
	if err != nil {
		log.Fatal(err)
		return
	}

	// Bootstrap the DHT. In the default configuration, this spawns a Background
	// thread that will refresh the peer table every five minutes.
	log.Info("bootstrapping the DHT")
	if err = ps.dht.Bootstrap(ctx); err != nil {
		log.Fatal(err)
		return
	}

	// Let's connect to the bootstrap nodes first. They will tell us about the
	// other nodes in the network.
	var bootnodes []multiaddr.Multiaddr
	if len(ps.BootNodes) > 0 {
		bootnodes = parseMultiaddress(ps.BootNodes)
	} else {
		bootnodes = dht.DefaultBootstrapPeers
	}

	log.Info("connecting to bootstrap nodes...")
	log.Debugf("bootnodes: %+v", bootnodes)
	var wg sync.WaitGroup
	for _, peerAddr := range bootnodes {
		peerinfo, _ := peer.AddrInfoFromP2pAddr(peerAddr)
		wg.Add(1)
		go func() {
			defer wg.Done()
			if peerinfo != nil {
				log.Debugf("trying %s", *peerinfo)
				if err := ps.host.Connect(ctx, *peerinfo); err != nil {
					log.Debug(err)
				} else {
					log.Infof("connection established with bootstrap node: %s", peerinfo)
				}
			}
		}()
	}
	wg.Wait()
	// We use a rendezvous point "meet me here" to announce our location.
	// This is like telling your friends to meet you at the Eiffel Tower.
	ps.routing = discovery.NewRoutingDiscovery(ps.dht)

	log.Info("advertise own pubKey so nodes can find me")
	discovery.Advertise(ctx, ps.routing, ps.PubKey)
}

func (ps *SubPub) Subcribe() {
	ctx := context.Background()
	go func() {
		for {
			ps.discover(ctx)
			time.Sleep(time.Second)
		}
	}()
	select {}
}

func (ps *SubPub) PeerConnect(pubKey string, callback func(*bufio.ReadWriter)) error {
	ctx := context.Background()

	log.Infof("searching for peer %s", pubKey)
	peerChan, err := ps.routing.FindPeers(ctx, pubKey)
	if err != nil {
		return err
	}

	for peer := range peerChan {
		// not myself
		if peer.ID == ps.host.ID() {
			continue
		}

		log.Infof("found peer: %s", peer.ID)
		ps.groupPeers = append(ps.groupPeers, peer.ID)

		stream, err := ps.host.NewStream(ctx, peer.ID, protocol.ID(ps.Topic))
		if err != nil {
			log.Debugf("connection failed: ", err)
		} else {
			log.Infof("connected to peer %s", peer.ID)
			callback(bufio.NewReadWriter(bufio.NewReader(stream), bufio.NewWriter(stream)))
		}
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

func (ps *SubPub) discover(ctx context.Context) {
	log.Debug("announcing ourselves as SubPub participants")
	discovery.Advertise(ctx, ps.routing, ps.Topic)

	// Now, look for others who have announced
	// This is like your friend telling you the location to meet you.
	log.Debugf("searching for SubPub group identity %s", ps.Topic)
	peerChan, err := ps.routing.FindPeers(ctx, ps.Topic)
	if err != nil {
		log.Fatal(err)
		return
	}

	for peer := range peerChan {
		// not myself
		if peer.ID == ps.host.ID() {
			continue
		}

		// if already connected
		if len(ps.host.Network().ConnsToPeer(peer.ID)) > 0 {
			log.Debugf("peer %s already connected", peer.ID)
			continue
		}
		// if new peer, let's connect to it
		log.Debugf("found peer: %s", peer.ID)
		ps.groupPeers = append(ps.groupPeers, peer.ID)

		stream, err := ps.host.NewStream(ctx, peer.ID, protocol.ID(ps.Topic))
		if err == nil {
			log.Infof("connected to peer %s", peer.ID)
			rw := bufio.NewReadWriter(bufio.NewReader(stream), bufio.NewWriter(stream))
			go ps.broadcastHandler(rw)
			go ps.readHandler(rw)
		}
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
		return []byte{}, false
	}
	if len(msg) < 25 {
		return []byte{}, false
	}
	var decryptNonce [24]byte
	copy(decryptNonce[:], msg[:24])
	return secretbox.Open(nil, msg[24:], &decryptNonce, &ps.GroupKey)
}

// Package ipfssync provides a service to synchronize IPFS datasets over a p2p network between two or more nodes
package ipfssync

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"time"

	"github.com/libp2p/go-libp2p-core/peer"
	ma "github.com/multiformats/go-multiaddr"

	"gitlab.com/vocdoni/go-dvote/crypto/signature"
	"gitlab.com/vocdoni/go-dvote/data"
	"gitlab.com/vocdoni/go-dvote/log"
	"gitlab.com/vocdoni/go-dvote/net"
	"gitlab.com/vocdoni/go-dvote/tree"
	"gitlab.com/vocdoni/go-dvote/types"
	"gitlab.com/vocdoni/go-dvote/util"
)

type Message struct {
	Type      string   `json:"type"`
	Address   string   `json:"address"`
	Maddress  string   `json:"mAddress"`
	NodeID    string   `json:"nodeId"`
	Hash      string   `json:"hash"`
	PinList   []string `json:"pinList"`
	Timestamp int32    `json:"timestamp"`
}

// shity function to workaround NAT problems (hope it's temporary)
func guessMyAddress(port int, id string) string {
	ip, err := util.PublicIP()
	if err != nil {
		log.Warn(err)
		return ""
	}
	if ip4 := ip.To4(); ip4 != nil {
		return fmt.Sprintf("/ip4/%s/tcp/%d/ipfs/%s", ip4, port, id)
	}
	if ip6 := ip.To16(); ip6 != nil {
		return fmt.Sprintf("/ip6/[%s]/tcp/%d/ipfs/%s", ip6, port, id)
	}
	return ""
}

func (is *IPFSsync) updatePinsTree(extraPins []string) {
	currentRoot := is.hashTree.Root()
	for _, v := range append(is.listPins(), extraPins...) {
		if len(v) > is.hashTree.MaxClaimSize() {
			log.Warnf("CID exceeds the claim size %d", len(v))
			continue
		}
		is.hashTree.AddClaim([]byte(v), []byte{})
	}
	if currentRoot != is.hashTree.Root() {
		is.lastHash = currentRoot
	}
}

func (is *IPFSsync) syncPins() error {
	mkPins, _, err := is.hashTree.DumpPlain(is.hashTree.Root(), false)
	if err != nil {
		return err
	}
	ctx := context.TODO() // the caller should probably provide it
	pins, err := is.Storage.ListPins(ctx)
	if err != nil {
		return err
	}
	for _, v := range mkPins {
		if _, e := pins[v]; e {
			continue
		}

		log.Infof("pinning %s", v)
		ctx, cancel := context.WithTimeout(ctx, is.Timeout)
		defer cancel()
		if err := is.Storage.Pin(ctx, v); err != nil {
			log.Warn(err)
		}
	}
	return nil
}

func (is *IPFSsync) askPins(address string, hash string) error {
	var msg Message
	msg.Type = "fetch"
	msg.Address = is.myAddress
	msg.Hash = hash
	msg.Timestamp = int32(time.Now().Unix())
	return is.unicastMsg(address, msg)
}

func (is *IPFSsync) sendPins(address string) error {
	var msg Message
	msg.Type = "fetchReply"
	msg.Address = is.myAddress
	msg.Hash = is.hashTree.Root()
	msg.PinList = is.listPins()
	msg.Timestamp = int32(time.Now().Unix())

	return is.unicastMsg(address, msg)
}

func (is *IPFSsync) broadcastMsg(ipfsmsg Message) error {
	d, err := json.Marshal(ipfsmsg)
	if err != nil {
		return err
	}
	is.Transport.Send(types.Message{
		Data:      d,
		TimeStamp: int32(time.Now().Unix()),
	})
	return nil
}

// Handle handles an Message
func (is *IPFSsync) Handle(msg Message) error {
	if msg.Address == is.myAddress {
		return nil
	}
	if int32(time.Now().Unix())-msg.Timestamp > is.TimestampWindow {
		log.Debug("discarting old message")
		return nil
	}
	switch msg.Type {
	case "hello":
		peers, err := is.Storage.CoreAPI.Swarm().Peers(is.Storage.Node.Context())
		if err != nil {
			return err
		}
		found := false
		for _, p := range peers {
			if p.ID().String() == msg.NodeID {
				found = true
			}
		}
		if !found {
			log.Infof("connecting to peer %s", msg.Maddress)
			multiAddr, err := ma.NewMultiaddr(msg.Maddress)
			if err != nil {
				return err
			}
			peerInfo, err := peer.AddrInfoFromP2pAddr(multiAddr)
			if err != nil {
				return err
			}
			return is.Storage.CoreAPI.Swarm().Connect(is.Storage.Node.Context(), *peerInfo)
		}

	case "update":
		if len(msg.Hash) > 31 && len(msg.Address) > 31 && !is.updateLock && len(is.askLock) == 0 {
			if exist, err := is.hashTree.HashExist(msg.Hash); err == nil && !exist && is.lastHash != msg.Hash {
				log.Infof("found new hash %s from %s", msg.Hash, msg.Address)
				is.askLock = msg.Hash
				return is.askPins(msg.Address, msg.Hash)
			}
		}

	case "fetchReply":
		if len(msg.Hash) > 31 && len(msg.Address) > 31 && !is.updateLock {
			if msg.Hash != is.hashTree.Root() {
				is.updateLock = true
				log.Infof("got new pin list %s from %s", msg.Hash, msg.Address)
				is.updatePinsTree(msg.PinList)
				is.updateLock = false
				if is.askLock == msg.Hash {
					is.askLock = ""
				}
				return nil
			}
		}

	case "fetch":
		if len(msg.Hash) > 31 && len(msg.Address) > 31 {
			if msg.Hash == is.hashTree.Root() {
				log.Infof("got fetch query, sending pin list to %s", msg.Address)
				return is.sendPins(msg.Address)
			}
		}
	}

	return nil
}

func (is *IPFSsync) sendUpdate() {
	var msg Message
	msg.Type = "update"
	msg.Address = is.myAddress
	msg.Hash = is.hashTree.Root()
	msg.Timestamp = int32(time.Now().Unix())
	if len(is.listPins()) > 0 {
		log.Debugf("current hash %s", msg.Hash)
		err := is.broadcastMsg(msg)
		if err != nil {
			log.Warn(err)
		}
	}
}

func (is *IPFSsync) sendHello() {
	var msg Message
	msg.Type = "hello"
	msg.Address = is.myAddress
	msg.Maddress = is.myMultiAddr.String()
	msg.NodeID = is.myNodeID
	msg.Timestamp = int32(time.Now().Unix())
	err := is.broadcastMsg(msg)
	if err != nil {
		log.Warn(err)
	}
}

func (is *IPFSsync) listPins() (pins []string) {
	list, err := is.Storage.ListPins(context.TODO())
	if err != nil {
		log.Error(err)
	}
	for i := range list {
		pins = append(pins, i)
	}
	return
}

type IPFSsync struct {
	DataDir         string
	Key             string
	PrivKey         string
	Port            int16
	HelloTime       int
	UpdateTime      int
	Bootnodes       []string
	Storage         *data.IPFSHandle
	Transport       net.Transport
	Topic           string
	Timeout         time.Duration
	TimestampWindow int32

	hashTree    tree.Tree
	updateLock  bool   // TODO(mvdan): this is super racy
	askLock     string // TODO(mvdan): this is super racy
	myAddress   string
	myNodeID    string
	myMultiAddr ma.Multiaddr
	lastHash    string
	private     bool
}

// NewIPFSsync creates a new IPFSsync instance. Transports supported are "libp2p" or "privlibp2p"
func NewIPFSsync(dataDir, groupKey, privKeyHex, transport string, storage data.Storage) *IPFSsync {
	is := &IPFSsync{
		DataDir:         dataDir,
		Topic:           groupKey,
		PrivKey:         privKeyHex,
		Port:            4171,
		HelloTime:       40,
		UpdateTime:      20,
		Timeout:         time.Second * 600,
		Storage:         storage.(*data.IPFSHandle),
		TimestampWindow: 30,
	}
	if transport == "privlibp2p" {
		transport = "libp2p"
		is.private = true
	}
	switch transport {
	case "libp2p":
		is.Transport = &net.SubPubHandle{}
	default:
		is.Transport = &net.SubPubHandle{}
	}
	return is
}

func (is *IPFSsync) unicastMsg(address string, ipfsmsg Message) error {
	var msg types.Message
	d, err := json.Marshal(ipfsmsg)
	if err != nil {
		return err
	}
	msg.Data = d
	msg.TimeStamp = int32(time.Now().Unix())
	go is.Transport.SendUnicast(address, msg)
	return nil
}

// Start initializes and start an IPFSsync instance
func (is *IPFSsync) Start() {
	log.Infof("initializing new pin storage")
	os.RemoveAll(is.DataDir + "/ipfsSync.db")
	is.hashTree.StorageDir = is.DataDir
	if err := is.hashTree.Init("ipfsSync.db"); err != nil {
		log.Fatal(err)
	}
	is.updatePinsTree([]string{})
	log.Infof("current hash %s", is.hashTree.Root())

	conn := types.Connection{
		Port:         int(is.Port),
		Key:          is.PrivKey,
		Topic:        fmt.Sprintf("%x", signature.HashRaw(is.Topic)),
		TransportKey: is.Topic,
	}
	// conn.Address, _ = signature.PubKeyFromPrivateKey(is.PrivKey)
	if is.private {
		conn.Encryption = "private"
	}

	if err := is.Transport.Init(&conn); err != nil {
		log.Fatal(err)
	}
	is.Transport.SetBootnodes(is.Bootnodes)

	msg := make(chan types.Message)
	go is.Transport.Listen(msg)
	is.myAddress = is.Transport.Address()
	is.myNodeID = is.Storage.Node.PeerHost.ID().String()
	var err error
	is.myMultiAddr, err = ma.NewMultiaddr(guessMyAddress(4001, is.myNodeID))
	if err != nil {
		log.Fatal(err)
	}
	log.Infof("my multiaddress: %s", is.myMultiAddr)

	go func() {
		for {
			d := <-msg
			var syncMsg Message
			err := json.Unmarshal(d.Data, &syncMsg)
			if err != nil {
				log.Warnf("cannot unmarshal message %s", err)
			} else {
				go is.Handle(syncMsg)
			}
		}
	}()

	go func() {
		for {
			is.sendHello()
			time.Sleep(time.Second * time.Duration(is.HelloTime))
		}
	}()

	go func() {
		for {
			time.Sleep(time.Second * time.Duration(is.UpdateTime))
			if !is.updateLock {
				is.updatePinsTree([]string{})
				is.sendUpdate()
			}
		}
	}()

	go func() {
		for {
			if len(is.askLock) > 0 {
				for i := 0; i < 100; i++ {
					if len(is.askLock) == 0 {
						break
					}
					time.Sleep(time.Millisecond * 100)
				}
				if len(is.askLock) > 0 {
					is.askLock = ""
					log.Warn("ask lock released due timeout")
				}
			}
			time.Sleep(time.Millisecond * 200)
		}
	}()

	for {
		err = is.syncPins()
		if err != nil {
			log.Warn(err)
		}
		time.Sleep(time.Second * 32)
	}
}

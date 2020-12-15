// Package ipfssync provides a service to synchronize IPFS datasets over a p2p network between two or more nodes
package ipfssync

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path"
	"strings"
	"sync"
	"time"

	"github.com/libp2p/go-libp2p-core/peer"
	ma "github.com/multiformats/go-multiaddr"

	"gitlab.com/vocdoni/go-dvote/censustree"
	tree "gitlab.com/vocdoni/go-dvote/censustree/gravitontree"
	"gitlab.com/vocdoni/go-dvote/crypto/ethereum"
	"gitlab.com/vocdoni/go-dvote/data"
	"gitlab.com/vocdoni/go-dvote/log"
	"gitlab.com/vocdoni/go-dvote/net"
	"gitlab.com/vocdoni/go-dvote/types"
	"gitlab.com/vocdoni/go-dvote/util"
)

type Message struct {
	Type      string   `json:"type"`
	Address   string   `json:"address,omitempty"`
	Maddress  string   `json:"mAddress,omitempty"`
	NodeID    string   `json:"nodeId,omitempty"`
	Hash      string   `json:"hash,omitempty"`
	PinList   []string `json:"pinList,omitempty"`
	Timestamp int32    `json:"timestamp"`
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

	hashTree    censustree.Tree
	updateLock  sync.RWMutex
	myMultiAddr ma.Multiaddr // The IPFS multiaddress
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

// myPins return the list of local stored pins base64 encoded
func (is *IPFSsync) myPins() (pins []string) {
	ctx, cancel := context.WithTimeout(context.Background(), is.Timeout)
	defer cancel()
	list, err := is.Storage.ListPins(ctx)
	if err != nil {
		log.Error(err)
		return
	}
	for i := range list {
		pins = append(pins, i)
	}
	return pins
}

// updateLocalPins gets the local IPFS pin list and add them to the Merkle Tree
func (is *IPFSsync) updateLocalPins() {
	for _, p := range is.myPins() {
		is.hashTree.Add([]byte(p), []byte{}) // errror is ignored, should be fine
	}
}

// addPins adds to the MerkleTree the new pins and updates the Root
func (is *IPFSsync) addPins(pins []string) {
	var err error
	var pin []byte
	currentRoot := is.hashTree.Root()
	for _, v := range pins {
		pin, err = base64.StdEncoding.DecodeString(v)
		if err != nil {
			log.Warnf("cannot decode pin %s: (%s)", v, err)
			continue
		}
		if len(pin) > is.hashTree.MaxKeySize() {
			log.Warnf("CID exceeds the claim size %d", len(v))
			continue
		}
		is.hashTree.Add(pin, []byte{})
	}
	if !bytes.Equal(currentRoot, is.hashTree.Root()) {
		is.lastHash = fmt.Sprintf("%x", currentRoot)
	}
}

// syncPins get the list of pins stored in the merkle tree and pin all of them
func (is *IPFSsync) syncPins() error {
	mkPins, _, err := is.hashTree.DumpPlain(nil, false)
	if err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(context.Background(), is.Timeout)
	defer cancel()
	pins, err := is.Storage.ListPins(ctx)
	if err != nil {
		return err
	}
	for _, v := range mkPins {
		if _, e := pins[v]; e {
			continue
		}

		log.Infof("pinning %s", v)
		if err := is.Storage.Pin(ctx, v); err != nil {
			log.Warn(err)
		}
	}
	return nil
}

func (is *IPFSsync) askPins(address string, hash []byte) error {
	var msg Message
	msg.Type = "fetch"
	msg.Address = is.Transport.Address()
	msg.Hash = fmt.Sprintf("%x", hash)
	msg.Timestamp = int32(time.Now().Unix())
	return is.unicastMsg(address, msg)
}

func (is *IPFSsync) sendPins(address string, theirHash string) error {
	var msg Message
	msg.Type = "fetchReply"
	msg.Address = is.Transport.Address()
	msg.Hash = fmt.Sprintf("%x", is.hashTree.Root())
	msg.PinList = is.listPins(theirHash)
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
	if msg.Address == is.Transport.Address() {
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
			if strings.Contains(msg.Maddress, p.ID().String()) {
				found = true
				break
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
			is.Storage.Node.PeerHost.ConnManager().Protect(peerInfo.ID, "ipfsPeer")
			return is.Storage.Node.PeerHost.Connect(context.Background(), *peerInfo)
			//return is.Storage.CoreAPI.Swarm().Connect(is.Storage.Node.Context(), *peerInfo)
		}

	case "update":
		if len(msg.Hash) > 31 && len(msg.Address) > 31 {
			is.updateLock.RLock()
			defer is.updateLock.RUnlock()
			hash, err := hex.DecodeString(msg.Hash)
			if err != nil {
				return err
			}
			if exist, err := is.hashTree.HashExists(hash); err == nil && !exist && is.lastHash != msg.Hash {
				log.Infof("found new hash %s from %s", msg.Hash, msg.Address)
				return is.askPins(msg.Address, is.hashTree.Root())
			}
		}

		// received a fetchReply, adding new pins
	case "fetchReply":
		if len(msg.Hash) > 31 && len(msg.Address) > 31 {
			is.updateLock.Lock()
			defer is.updateLock.Unlock()
			if util.TrimHex(msg.Hash) != fmt.Sprintf("%x", is.hashTree.Root()) {
				log.Infof("got new pin list %s from %s", msg.Hash, msg.Address)
				is.addPins(msg.PinList)
				return nil
			}
		}

	case "fetch":
		if len(msg.Hash) > 31 && len(msg.Address) > 31 {
			is.updateLock.RLock()
			defer is.updateLock.RUnlock()
			if util.TrimHex(msg.Hash) != fmt.Sprintf("%x", is.hashTree.Root()) {
				log.Infof("got fetch query, sending pin list to %s", msg.Address)
				return is.sendPins(msg.Address, msg.Hash)
			}
		}
	}

	return nil
}

func (is *IPFSsync) sendUpdate() {
	var msg Message
	msg.Type = "update"
	msg.Address = is.Transport.Address()
	msg.Hash = fmt.Sprintf("%x", is.hashTree.Root())
	msg.Timestamp = int32(time.Now().Unix())
	if s, err := is.hashTree.Size(is.hashTree.Root()); err == nil && s > 0 {
		log.Infof("[ipfsSync info] pins:%d hash:%s", s, msg.Hash)
		err := is.broadcastMsg(msg)
		if err != nil {
			log.Warn(err)
		}
	} else if err != nil {
		log.Error(err)
	}
}

func (is *IPFSsync) sendHello() {
	var msg Message
	msg.Type = "hello"
	msg.Address = is.Transport.Address()
	msg.Maddress = is.myMultiAddr.String()
	msg.Timestamp = int32(time.Now().Unix())
	err := is.broadcastMsg(msg)
	if err != nil {
		log.Warn(err)
	}
}

// difference returns the elements in `a` that aren't in `b`.
func diff(a, b []string) []string {
	mb := make(map[string]struct{}, len(b))
	for _, x := range b {
		mb[x] = struct{}{}
	}
	var diff []string
	for _, x := range a {
		if _, found := mb[x]; !found {
			diff = append(diff, x)
		}
	}
	return diff
}

// listPins return the current pins of the Merkle Tree
// if fromHash is a valid hash, returns only the difference between the root and the provided hash
func (is *IPFSsync) listPins(fromHash string) []string {
	myClaims, _, err := is.hashTree.DumpPlain(nil, true)
	if err != nil {
		log.Error(err)
		return []string{}
	}
	if fromHash == "" || fromHash == "0x0000000000000000000000000000000000000000000000000000000000000000" {
		return myClaims
	}
	fromHashBytes, err := hex.DecodeString(util.TrimHex(fromHash))
	if err != nil {
		log.Error(err)
		return []string{}
	}
	if exist, err := is.hashTree.HashExists(fromHashBytes); !exist {
		if err != nil {
			log.Error(err)
		}
		return myClaims
	}

	theirClaims, _, err := is.hashTree.DumpPlain(fromHashBytes, true)
	if err != nil {
		log.Error(err)
		return []string{}
	}
	return diff(myClaims, theirClaims)
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
	var err error
	log.Infof("initializing new pin storage")
	dbDir := path.Join(is.DataDir, "db")
	if err := os.RemoveAll(dbDir); err != nil {
		log.Fatal(err)
	}
	is.hashTree, err = tree.NewTree("ipfsSync", dbDir)
	if err != nil {
		log.Fatal(err)
	}
	is.updateLocalPins()
	log.Infof("current hash %s", is.hashTree.Root())

	conn := types.Connection{
		Port:         int(is.Port),
		Key:          is.PrivKey,
		Topic:        fmt.Sprintf("%x", ethereum.HashRaw([]byte(is.Topic))),
		TransportKey: is.Topic,
	}
	// conn.Address, _ = ethereum.PubKeyFromPrivateKey(is.PrivKey)
	if is.private {
		conn.Encryption = "private"
	}

	if err := is.Transport.Init(&conn); err != nil {
		log.Fatal(err)
	}
	is.Transport.SetBootnodes(is.Bootnodes)

	msg := make(chan types.Message)
	go is.Transport.Listen(msg)

	is.myMultiAddr, err = ma.NewMultiaddr(guessMyAddress(4001, is.Storage.Node.PeerHost.ID().String()))
	if err != nil {
		log.Fatal(err)
	}
	log.Infof("my multiaddress: %s", is.myMultiAddr)

	// receive messages and handle them
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

	// send hello messages
	go func() {
		for {
			is.sendHello()
			time.Sleep(time.Second * time.Duration(is.HelloTime)) // CHECK THIS
		}
	}()

	// send update messages
	go func() {
		for {
			time.Sleep(time.Duration(is.UpdateTime) * time.Second) //CHECK THIS
			is.updateLock.Lock()
			is.updateLocalPins()
			is.sendUpdate()
			is.updateLock.Unlock()
		}
	}()

	go func() {
		for {
			if err := is.syncPins(); err != nil {
				log.Warn(err)
			}
			time.Sleep(time.Second * 32)
		}
	}()
}

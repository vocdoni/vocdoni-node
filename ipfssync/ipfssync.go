// Package ipfssync provides a service to synchronize IPFS datasets over a p2p network between two or more nodes
package ipfssync

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"path"
	"strings"
	"sync"
	"time"

	"github.com/libp2p/go-libp2p-core/peer"
	ma "github.com/multiformats/go-multiaddr"
	"go.vocdoni.io/dvote/ipfssync/subpub"
	statedb "go.vocdoni.io/dvote/statedblegacy"
	"go.vocdoni.io/proto/build/go/models"
	"google.golang.org/protobuf/proto"

	"go.vocdoni.io/dvote/data"
	"go.vocdoni.io/dvote/log"
	"go.vocdoni.io/dvote/statedblegacy/gravitonstate"
	"go.vocdoni.io/dvote/util"
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

	hashTree        statedb.StateTree
	state           statedb.StateDB
	updateLock      sync.RWMutex
	myMultiAddrIPv4 ma.Multiaddr // The IPFS multiaddress (IPv4)
	myMultiAddrIPv6 ma.Multiaddr // The IPFS multiaddress (IPv6)
	lastHash        []byte
	private         bool
}

// NewIPFSsync creates a new IPFSsync instance. Transports supported are "libp2p" or "privlibp2p"
func NewIPFSsync(dataDir, groupKey, privKeyHex, transport string, storage data.Storage) *IPFSsync {
	is := &IPFSsync{
		DataDir:         dataDir,
		GroupKey:        groupKey,
		PrivKey:         privKeyHex,
		Port:            4171,
		HelloInterval:   time.Second * 40,
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

// shity function to workaround NAT problems (hope it's temporary)
func guessMyAddress(ipversion uint, port int, id string) string {
	ip, err := util.PublicIP(ipversion)
	if err != nil {
		log.Warnf("guessMyAddress: %v", err)
		return ""
	}
	if ip4 := ip.To4(); ip4 != nil {
		return fmt.Sprintf("/ip4/%s/tcp/%d/ipfs/%s", ip4, port, id)
	}
	if ip6 := ip.To16(); ip6 != nil {
		return fmt.Sprintf("/ip6/%s/tcp/%d/ipfs/%s", ip6, port, id)
	}
	return ""
}

// myPins return the list of local stored pins
func (is *IPFSsync) myPins() (pins []string, err error) {
	ctx, cancel := context.WithTimeout(context.Background(), is.Timeout)
	defer cancel()
	list, err := is.Storage.ListPins(ctx)
	if err != nil {
		return nil, fmt.Errorf("myPins: %w", err)
	}
	for i := range list {
		pins = append(pins, i)
	}
	return pins, nil
}

// updateLocalPins gets the local IPFS pin list and add them to the Merkle Tree
func (is *IPFSsync) updateLocalPins() {
	pins, err := is.myPins()
	if err != nil {
		log.Errorf("updateLocalPins: %v", err)
	}
	for _, p := range pins {
		is.hashTree.Add([]byte(p), []byte{}) // errror is ignored, should be fine
	}
	is.state.Commit()
}

// addPins adds to the MerkleTree the new pins and updates the Root
func (is *IPFSsync) addPins(pins []*models.IpfsPin) error {
	currentRoot := is.hashTree.Hash()
	for _, v := range pins {
		if len(v.Uri) > gravitonstate.GravitonMaxKeySize {
			log.Warnf("CID exceeds the max size (got %d)", len(v.Uri))
			continue
		}
		if err := is.hashTree.Add([]byte(v.Uri), []byte{}); err != nil {
			log.Warnf("cannot add pin %s", v.Uri)
		}
		log.Debugf("added pin %s", v.Uri)
	}
	if !bytes.Equal(currentRoot, is.hashTree.Hash()) {
		is.lastHash = currentRoot
	}
	_, err := is.state.Commit()
	return err
}

func (is *IPFSsync) getMyPins() []string {
	// Note that we return []string instead of [][]byte since we need to
	// make copies of the keys; the key parameter in the callback isn't safe
	// for use after the callback returns, nor can it be modified.
	// We could make []byte copies, but since all users want strings, this
	// is easier.
	var mkPins []string
	is.hashTree.Iterate(nil, func(key, value []byte) bool {
		mkPins = append(mkPins, string(key))
		return false
	})
	return mkPins
}

// syncPins get the list of pins stored in the merkle tree and pin all of them
func (is *IPFSsync) syncPins() error {
	mkPins := is.getMyPins()
	ctx, cancel := context.WithTimeout(context.Background(), is.Timeout)
	defer cancel()
	pins, err := is.Storage.ListPins(ctx)
	if err != nil {
		return fmt.Errorf("syncPins: %w", err)
	}
	for _, pin := range mkPins {
		if _, e := pins[pin]; e {
			continue
		}

		log.Infof("pinning %s", pin)
		if err := is.Storage.Pin(ctx, pin); err != nil {
			return fmt.Errorf("syncPins: %w", err)
		}
	}
	return nil
}

func (is *IPFSsync) askPins(address string, hash []byte) error {
	var msg models.IpfsSync
	msg.Msgtype = models.IpfsSync_FETCH
	msg.Address = is.Transport.Address()
	msg.Hash = hash
	return is.unicastMsg(address, &msg)
}

func (is *IPFSsync) sendPins(address string, theirHash []byte) error {
	var msg models.IpfsSync
	var err error
	msg.Msgtype = models.IpfsSync_FETCHREPLY
	msg.Address = is.Transport.Address()
	msg.Hash = is.hashTree.Hash()
	msg.PinList, err = is.listPins(theirHash)
	if err != nil {
		return fmt.Errorf("sendPins: %w", err)
	}
	if len(msg.PinList) == 0 {
		return nil
	}
	return is.unicastMsg(address, &msg)
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

// Handle handles a Message in a thread-safe way:
// is.updateLock RWMutex syncs the calls to:
// * addPins which modifies IPFS pin list
// * askPins and sendPins, which produce outgoing unicasts messages with thread-safe is.unicastMsg()
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
			multiAddr, err := ma.NewMultiaddr(msg.Multiaddress)
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

	case models.IpfsSync_UPDATE:
		if len(msg.Hash) == gravitonstate.GravitonHashSizeBytes && len(msg.Address) > 31 {
			is.updateLock.RLock()
			defer is.updateLock.RUnlock()
			tree := is.state.TreeWithRoot(msg.Hash)
			if tree == nil && !bytes.Equal(is.lastHash, msg.Hash) {
				log.Infof("found new hash %x from %s", msg.Hash, msg.Address)
				return is.askPins(msg.Address, is.hashTree.Hash())
			}
		}

	// received a fetchReply, adding new pins
	case models.IpfsSync_FETCHREPLY:
		if len(msg.Hash) == gravitonstate.GravitonHashSizeBytes && len(msg.Address) > 31 {
			is.updateLock.Lock()
			defer is.updateLock.Unlock()
			if !bytes.Equal(msg.Hash, is.hashTree.Hash()) {
				log.Infof("got new pin list %x from %s", msg.Hash, msg.Address)
				return is.addPins(msg.PinList)
			}
		}

	case models.IpfsSync_FETCH:
		if len(msg.Address) > 31 {
			is.updateLock.RLock()
			defer is.updateLock.RUnlock()
			log.Infof("got fetch query, sending pin list to %s", msg.Address)
			return is.sendPins(msg.Address, msg.Hash)
		}
	}
	return nil
}

func (is *IPFSsync) sendUpdate() {
	var msg models.IpfsSync
	msg.Msgtype = models.IpfsSync_UPDATE
	msg.Address = is.Transport.Address()
	msg.Hash = is.hashTree.Hash()
	if s := is.hashTree.Count(); s > 0 {
		log.Infof("[ipfsSync info] pins:%d hash:%x", s, msg.Hash)
		err := is.broadcastMsg(&msg)
		if err != nil {
			log.Warnf("sendUpdate: %v", err)
		}
	}
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
	// send HELLO with IPv4 address, if defined
	if is.myMultiAddrIPv4 != nil {
		is.sendHelloWithAddr(is.myMultiAddrIPv4.String())
	}
	// send HELLO with IPv6 address, if defined
	if is.myMultiAddrIPv6 != nil {
		is.sendHelloWithAddr(is.myMultiAddrIPv6.String())
	}
}

// listPins return the current pins of the Merkle Tree
// if fromHash is a valid hash, returns only the difference between the root and the provided hash
func (is *IPFSsync) listPins(fromHash []byte) ([]*models.IpfsPin, error) {
	var pins []*models.IpfsPin
	diff, err := is.state.KeyDiff(fromHash, is.hashTree.Hash())
	if err != nil {
		return nil, fmt.Errorf("listPins, failed KeyDiff: %w", err)
	}
	for _, c := range diff {
		pins = append(pins, &models.IpfsPin{Uri: string(c)})
	}
	log.Debugf("listPins: sending %d pins out of %d", len(diff), is.hashTree.Count())
	return pins, nil
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
	var err error

	// Init pin storage
	log.Infof("initializing new pin storage")
	dbDir := path.Join(is.DataDir, "db")
	if err := os.RemoveAll(dbDir); err != nil {
		log.Fatal(err)
	}
	is.state = &gravitonstate.GravitonState{}
	if err = is.state.Init(dbDir, "disk"); err != nil {
		log.Fatal(err)
	}
	if err = is.state.AddTree("ipfsSync"); err != nil {
		log.Fatal(err)
	}
	is.hashTree = is.state.Tree("ipfsSync")
	is.updateLocalPins()
	log.Infof("current hash %x", is.hashTree.Hash())
	// end Init pin storage

	// Init SubPub
	is.Transport = subpub.NewSubPub(is.PrivKey, []byte(is.GroupKey), int32(is.Port), is.private)
	is.Transport.BootNodes = is.Bootnodes
	is.Transport.Start(context.Background(), is.Messages)
	// end Init SubPub

	// guessMyAddress and print it
	var err4, err6 error
	is.myMultiAddrIPv4, err4 = ma.NewMultiaddr(guessMyAddress(IPv4, 4001, is.Storage.Node.PeerHost.ID().String()))
	is.myMultiAddrIPv6, err6 = ma.NewMultiaddr(guessMyAddress(IPv6, 4001, is.Storage.Node.PeerHost.ID().String()))
	if err4 != nil && err6 != nil {
		log.Fatal("ipv4: %s; ipv6: %s", err4, err6)
	}
	if is.myMultiAddrIPv4 != nil {
		log.Infof("my IPFS multiaddress ipv4: %s", is.myMultiAddrIPv4)
	}
	if is.myMultiAddrIPv6 != nil {
		log.Infof("my IPFS multiaddress ipv6: %s", is.myMultiAddrIPv6)
	}
	// end guessMyAddress and print it

	go is.handleEvents() // this spawns a single background task per IPFSsync instance
}

// handleEvents runs an event loop that
// * checks for incoming messages, passing them to is.Handle(),
// * at regular interval sends HELLOs, UPDATEs and calls syncPins()
func (is *IPFSsync) handleEvents() {
	helloTicker := time.NewTicker(is.HelloInterval)
	defer helloTicker.Stop()

	updateTicker := time.NewTicker(is.UpdateInterval)
	defer updateTicker.Stop()

	syncPinsTicker := time.NewTicker(time.Second * 10)
	defer syncPinsTicker.Stop()

	for {
		select {
		case d := <-is.Messages:
			// receive unicast & broadcast messages and handle them
			var imsg models.IpfsSync
			err := proto.Unmarshal(d.Data, &imsg)
			if err != nil {
				log.Warnf("cannot unmarshal message %s", err)
			} else {
				log.Debugf("received message %s {Address:%s Hash:%x MA:%s PL:%v Ts:%d}",
					imsg.Msgtype.String(), imsg.Address, imsg.Hash,
					imsg.Multiaddress, imsg.PinList, imsg.Timestamp)
				go is.Handle(&imsg) // handle each incoming message in parallel, since is.Handle() is thread-safe
			}

		case <-helloTicker.C:
			// send hello messages
			is.sendHello()

		case <-updateTicker.C:
			// send update messages
			is.updateLock.Lock()
			is.updateLocalPins()
			is.sendUpdate()
			is.updateLock.Unlock()

		case <-syncPinsTicker.C:
			// pin everything found in the merkle tree
			if err := is.syncPins(); err != nil {
				log.Warnf("syncPins: %v", err)
			}
		}
	}
}

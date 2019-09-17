package ipfssync

import (
	"encoding/json"
	"fmt"
	"os"
	"time"

	peer "github.com/libp2p/go-libp2p-core/peer"
	ma "github.com/multiformats/go-multiaddr"
	"gitlab.com/vocdoni/go-dvote/crypto/signature"
	"gitlab.com/vocdoni/go-dvote/data"
	"gitlab.com/vocdoni/go-dvote/log"
	"gitlab.com/vocdoni/go-dvote/net"
	"gitlab.com/vocdoni/go-dvote/tree"
	"gitlab.com/vocdoni/go-dvote/types"
	"gitlab.com/vocdoni/go-dvote/util"
)

type IPFSsyncMessage struct {
	Type     string   `json:type`
	Address  string   `json:address`
	Maddress string   `json:mAddress`
	NodeID   string   `json:nodeId`
	Hash     string   `json:hash`
	PinList  []string `json:pinList`
}

//shity function to workaround NAT problems (hope it's temporary)
func guessMyAddress(port int, id string) string {
	ip, err := util.GetPublicIP()
	if err != nil {
		log.Warn(err.Error())
		return ""
	}
	if len(ip.To4().String()) > 8 {
		return fmt.Sprintf("/ip4/%s/tcp/%d/ipfs/%s", ip.String(), port, id)
	}
	if len(ip.To16().String()) > 8 {
		return fmt.Sprintf("/ip6/[%s]/tcp/%d/ipfs/%s", ip.String(), port, id)
	}
	return ""
}

func (is *IPFSsync) updatePinsTree(extraPins []string) {
	for _, v := range append(is.listPins(), extraPins...) {
		if len(v) > is.hashTree.GetMaxClaimSize() {
			log.Warnf("CID exceeds the claim size %d", len(v))
			continue
		}
		is.hashTree.AddClaim([]byte(v))
	}
}

func (is *IPFSsync) syncPins() error {
	if is.syncLock {
		return nil
	}
	is.syncLock = true
	mkPins, err := is.hashTree.DumpPlain(is.hashTree.GetRoot(), false)
	if err != nil {
		return err
	}
	pins, err := is.Storage.ListPins()
	if err != nil {
		return err
	}
	for _, v := range mkPins {
		if _, e := pins[v]; !e {
			log.Infof("pinning %s", v)
			pinned := false
			go func() {
				err := is.Storage.Pin(v)
				if err != nil {
					log.Warn(err.Error())
				}
				pinned = true
			}()
			waitTime := 100
			for !pinned && waitTime > 0 {
				time.Sleep(100 * time.Millisecond)
				waitTime--
			}
			if waitTime < 1 {
				log.Warnf("pinning timeout for %s", v)
			}
		}
	}
	is.syncLock = false
	return nil
}

func (is *IPFSsync) sendMsg(ipfsmsg IPFSsyncMessage) error {
	var msg types.Message
	d, err := json.Marshal(ipfsmsg)
	if err != nil {
		return err
	}
	msg.Data = d
	msg.TimeStamp = int32(time.Now().Unix())
	is.Transport.Send(msg)
	return nil
}

// Handle handles an IPFSsyncMessage
func (is *IPFSsync) Handle(msg IPFSsyncMessage) error {
	if msg.Address == is.myAddress {
		return nil
	}
	log.Debugf("got %+v", msg)
	if msg.Type == "hello" {
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
	}
	if msg.Type == "update" {
		if len(msg.Hash) > 31 && len(msg.Address) > 31 && !is.updateLock {
			if msg.Hash != is.hashTree.GetRoot() {
				is.updateLock = true
				log.Infof("found new hash %s from %s", msg.Hash, msg.Address)
				is.updatePinsTree(msg.PinList)
				is.updateLock = false
			}
		}
	}
	return nil
}

func (is *IPFSsync) sendUpdate() {
	var msg IPFSsyncMessage
	msg.Type = "update"
	msg.Address = is.myAddress
	msg.Hash = is.hashTree.GetRoot()
	msg.PinList = is.listPins()
	if len(msg.PinList) > 0 {
		log.Infof("current hash %s", msg.Hash)
		err := is.sendMsg(msg)
		if err != nil {
			log.Warn(err.Error())
		}
	}
}

func (is *IPFSsync) sendHello() {
	var msg IPFSsyncMessage
	msg.Type = "hello"
	msg.Address = is.myAddress
	msg.Maddress = is.myMultiAddr.String()
	msg.NodeID = is.myNodeID
	err := is.sendMsg(msg)
	if err != nil {
		log.Warn(err.Error())
	}
}

func (is *IPFSsync) listPins() (pins []string) {
	list, _ := is.Storage.ListPins()
	for i := range list {
		pins = append(pins, i)
	}
	return
}

type IPFSsync struct {
	DataDir     string
	Key         string
	Port        int16
	HelloTime   int
	UpdateTime  int
	Storage     *data.IPFSHandle
	Transport   net.PSSHandle
	hashTree    tree.Tree
	updateLock  bool
	syncLock    bool
	myAddress   string
	myNodeID    string
	myMultiAddr ma.Multiaddr
}

//NewIPFSsync creates a new IPFSsync instance
func NewIPFSsync(dataDir, key string, storage data.Storage) IPFSsync {
	var is IPFSsync
	is.DataDir = dataDir
	is.Key = key
	is.Port = 4171
	is.HelloTime = 40
	is.UpdateTime = 20
	is.Storage = storage.(*data.IPFSHandle)
	return is
}

//Start initializes and start an IPFSsync instance
func (is *IPFSsync) Start() {
	log.Infof("initializing new pin storage")
	os.RemoveAll(is.DataDir + "/ipfsSync.db")
	is.hashTree.Storage = is.DataDir
	is.hashTree.Init("ipfsSync.db")
	is.updatePinsTree([]string{})
	log.Infof("current hash %s", is.hashTree.GetRoot())

	var conn types.Connection
	conn.Port = int(is.Port)
	conn.Key = is.Key
	conn.Encryption = "sym"
	conn.Topic = string(signature.HashRaw(conn.Key))

	err := is.Transport.Init(&conn)
	if err != nil {
		log.Fatal(err.Error())
	}

	msg := make(chan types.Message)
	go is.Transport.Listen(msg)
	is.myAddress = fmt.Sprintf("%x", is.Transport.Swarm.PssAddr)
	is.myNodeID = is.Storage.Node.PeerHost.ID().String()
	is.myMultiAddr, err = ma.NewMultiaddr(guessMyAddress(4001, is.myNodeID))
	if err != nil {
		panic(err)
	}
	log.Infof("my multiaddress: %s", is.myMultiAddr.String())

	go func() {
		var syncMsg IPFSsyncMessage
		var err error
		for {
			d := <-msg
			err = json.Unmarshal(d.Data, &syncMsg)
			if err != nil {
				log.Warnf("cannot unmarshal message %s", err.Error())
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
			log.Infof("my current hash %s", is.hashTree.GetRoot())
			if !is.updateLock {
				is.updatePinsTree([]string{})
				is.sendUpdate()
			}
		}
	}()

	for {
		err = is.syncPins()
		if err != nil {
			log.Warn(err.Error())
		}
		time.Sleep(time.Second * 32)
	}
}

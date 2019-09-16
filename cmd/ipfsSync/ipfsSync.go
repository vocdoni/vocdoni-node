package main

import (
	"encoding/json"
	"fmt"
	"os"
	"os/user"
	"time"

	peer "github.com/libp2p/go-libp2p-core/peer"
	ma "github.com/multiformats/go-multiaddr"
	flag "github.com/spf13/pflag"
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

var updateLock bool
var myAddress string
var myNodeID string
var myMultiAddr ma.Multiaddr

func updatePinsTree(storage data.IPFSHandle, mt *tree.Tree) {
	for _, v := range listPins(storage) {
		mt.AddClaim([]byte(v))
	}
}

// it won't work if global IPV6 configured in the host...
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

func sendMsg(ipfsmsg IPFSsyncMessage, t net.PSSHandle) error {
	var msg types.Message
	d, err := json.Marshal(ipfsmsg)
	if err != nil {
		return err
	}
	msg.Data = d
	msg.TimeStamp = int32(time.Now().Unix())
	t.Send(msg)
	return nil
}

func ipfsSyncHandle(msg IPFSsyncMessage, storage data.IPFSHandle, mt *tree.Tree, t net.PSSHandle) error {
	if msg.Address == myAddress {
		return nil
	}
	log.Debugf("got %+v", msg)
	if msg.Type == "hello" {
		peers, err := storage.CoreAPI.Swarm().Peers(storage.Node.Context())
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
			return storage.CoreAPI.Swarm().Connect(storage.Node.Context(), *peerInfo)
		}
	}
	if msg.Type == "update" {
		if len(msg.Hash) > 31 && len(msg.Address) > 31 && !updateLock {
			if msg.Hash != mt.GetRoot() {
				log.Infof("found new hash %s from %s", msg.Hash, msg.Address)
				updateLock = true
				pins, err := storage.ListPins()
				if err != nil {
					log.Warn(err.Error())
					updateLock = false
					return err
				}
				for _, v := range msg.PinList {
					if _, e := pins[v]; !e {
						log.Infof("pining %s", v)
						pinned := false
						go func() {
							err := storage.Pin(v)
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
							log.Warnf("pining timeout for %s", v)
						}
					} else {
						log.Debugf("we already have %v", v)
					}
				}
				updatePinsTree(storage, mt)
				updateLock = false
			}
		}
	}
	return nil
}

func sendUpdate(storage data.IPFSHandle, t net.PSSHandle, mt *tree.Tree) {
	var msg IPFSsyncMessage
	msg.Type = "update"
	msg.Address = myAddress
	msg.Hash = mt.GetRoot()
	msg.PinList = listPins(storage)
	if len(msg.PinList) > 0 {
		log.Infof("current hash %s", msg.Hash)
		err := sendMsg(msg, t)
		if err != nil {
			log.Warn(err.Error())
		}
	}
}

func sendHello(storage data.IPFSHandle, t net.PSSHandle) {
	var msg IPFSsyncMessage
	msg.Type = "hello"
	msg.Address = myAddress
	msg.Maddress = myMultiAddr.String()
	msg.NodeID = myNodeID
	err := sendMsg(msg, t)
	if err != nil {
		log.Warn(err.Error())
	}
}

func listPins(storage data.IPFSHandle) (pins []string) {
	list, _ := storage.ListPins()
	for i := range list {
		pins = append(pins, i)
	}
	return
}

func main() {
	usr, err := user.Current()
	if err != nil {
		log.Fatal(err)
	}
	userDir := usr.HomeDir + "/.dvote"
	logLevel := flag.String("logLevel", "info", "log level")
	dataDir := flag.String("dataDir", userDir, "directory for storing data")
	key := flag.String("key", "", "symetric key of the sync ipfs cluster")
	port := flag.Int16("port", 4171, "port for the sync network")
	flag.Parse()
	log.InitLoggerAtLevel(*logLevel)

	ipfsStore := data.IPFSNewConfig(*dataDir + "/.ipfs")
	var storage data.IPFSHandle
	err = storage.Init(ipfsStore)
	if err != nil {
		log.Fatal(err.Error())
	}

	log.Infof("initializing new pin storage")
	var mkFiles tree.Tree
	os.RemoveAll(*dataDir + "/ipfsSync.db")
	mkFiles.Storage = *dataDir
	mkFiles.Init("ipfsSync.db")
	updatePinsTree(storage, &mkFiles)
	log.Infof("current hash %s", mkFiles.GetRoot())

	var conn types.Connection
	conn.Port = int(*port)
	conn.Key = *key
	conn.Encryption = "sym"
	conn.Topic = string(signature.HashRaw(conn.Key))

	var transport net.PSSHandle
	err = transport.Init(&conn)
	if err != nil {
		log.Fatal(err.Error())
	}

	msg := make(chan types.Message)
	go transport.Listen(msg)
	myAddress = fmt.Sprintf("%x", transport.Swarm.PssAddr)
	myNodeID = storage.Node.PeerHost.ID().String()
	myMultiAddr, err = ma.NewMultiaddr(guessMyAddress(4001, myNodeID))
	if err != nil {
		panic(err)
	}

	go func() {
		//var psscontext *types.PssContext
		var syncMsg IPFSsyncMessage
		var err error
		for {
			d := <-msg
			//psscontext = d.Context.(*types.PssContext)
			err = json.Unmarshal(d.Data, &syncMsg)
			if err != nil {
				log.Warnf("cannot unmarshal message %s", err.Error())
			} else {
				go ipfsSyncHandle(syncMsg, storage, &mkFiles, transport)
			}
		}
	}()

	go func() {
		for {
			sendHello(storage, transport)
			time.Sleep(time.Second * 30)
		}
	}()

	for {
		time.Sleep(time.Second * 5)
		if !updateLock {
			updatePinsTree(storage, &mkFiles)
			sendUpdate(storage, transport, &mkFiles)
		}
	}
}

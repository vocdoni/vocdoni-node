package swarm

import (
	"context"
	"crypto/ecdsa"
	"fmt"
	"os"
	"os/user"
	"strings"
	"time"

	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/node"

	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/p2p"

	"github.com/ethereum/go-ethereum/log"
	"github.com/ethereum/go-ethereum/p2p/enode"
	"github.com/ethereum/go-ethereum/swarm"

	swarmapi "github.com/ethereum/go-ethereum/swarm/api"
	"github.com/ethereum/go-ethereum/swarm/api/client"
	"github.com/ethereum/go-ethereum/swarm/network"
	"github.com/ethereum/go-ethereum/swarm/pss"
)

const (
	// MaxPeers is the maximum number of p2p peer connections
	MaxPeers = 10
)

// SwarmBootnodes list of bootnodes for the SWARM network
var SwarmBootnodes = []string{
	"enode://9b2fe07e69ccc7db5fef15793dab7d7d2e697ed92132d6e9548218e68a34613a8671ad03a6658d862b468ed693cae8a0f8f8d37274e4a657ffb59ca84676e45b@52.232.7.187:30400",
	"enode://76c1059162c93ef9df0f01097c824d17c492634df211ef4c806935b349082233b63b90c23970254b3b7138d630400f7cf9b71e80355a446a8b733296cb04169a@52.232.7.187:30401",
	"enode://ce46bbe2a8263145d65252d52da06e000ad350ed09c876a71ea9544efa42f63c1e1b6cc56307373aaad8f9dd069c90d0ed2dd1530106200e16f4ca681dd8ae2d@52.232.7.187:30402",
	"enode://f431e0d6008a6c35c6e670373d828390c8323e53da8158e7bfc43cf07e632cc9e472188be8df01decadea2d4a068f1428caba769b632554a8fb0607bc296988f@52.232.7.187:30403",
	"enode://174720abfff83d7392f121108ae50ea54e04889afe020df883655c0f6cb95414db945a0228d8982fe000d86fc9f4b7669161adc89cd7cd56f78f01489ab2b99b@52.232.7.187:30404",
	"enode://2ae89be4be61a689b6f9ecee4360a59e185e010ab750f14b63b4ae43d4180e872e18e3437d4386ce44875dc7cc6eb761acba06412fe3178f3dac1dab3b65703e@52.232.7.187:30405",
}

func newNode(key *ecdsa.PrivateKey, port int, httpport int, wsport int,
	datadir string, modules ...string) (*node.Node, *node.Config, error) {
	if port == 0 {
		port = 30100
	}
	cfg := &node.DefaultConfig
	if key != nil {
		cfg.P2P.PrivateKey = key
	}
	cfg.P2P.MaxPeers = MaxPeers
	cfg.P2P.ListenAddr = fmt.Sprintf("0.0.0.0:%d", port)
	cfg.P2P.EnableMsgEvents = true
	cfg.P2P.NoDiscovery = false
	cfg.P2P.DiscoveryV5 = true
	cfg.IPCPath = datadir + "/node.ipc"
	cfg.DataDir = datadir
	if httpport > 0 {
		cfg.HTTPHost = node.DefaultHTTPHost
		cfg.HTTPPort = httpport
		cfg.HTTPCors = []string{"*"}
	}
	if wsport > 0 {
		cfg.WSHost = node.DefaultWSHost
		cfg.WSPort = wsport
		cfg.WSOrigins = []string{"*"}
		for i := 0; i < len(modules); i++ {
			cfg.WSModules = append(cfg.WSModules, modules[i])
		}
	}
	stack, err := node.New(cfg)
	if err != nil {
		return nil, nil, fmt.Errorf("ServiceNode create fail: %v", err)
	}
	return stack, cfg, nil
}

func newSwarm(privkey *ecdsa.PrivateKey, nodekey *ecdsa.PrivateKey, datadir string, port int) (*swarm.Swarm, *swarmapi.Config, node.ServiceConstructor) {
	// create swarm service
	swarmCfg := swarmapi.NewConfig()
	swarmCfg.SyncEnabled = true
	swarmCfg.Port = fmt.Sprintf("%d", port)
	swarmCfg.Path = datadir
	swarmCfg.HiveParams.Discovery = true
	swarmCfg.Discovery = true
	swarmCfg.Pss.MsgTTL = time.Second * 10
	swarmCfg.Pss.CacheTTL = time.Second * 30
	swarmCfg.Pss.AllowRaw = true
	swarmCfg.Init(privkey, nodekey)
	swarmNode, err := swarm.NewSwarm(swarmCfg, nil)
	if err != nil {
		log.Crit("cannot crate swarm node")
	}
	// register swarm service to the node
	var swarmService node.ServiceConstructor = func(ctx *node.ServiceContext) (node.Service, error) {
		return swarmNode, nil
	}
	return swarmNode, swarmCfg, swarmService
}

type swarmPorts struct {
	WebSockets int
	HTTPRPC    int
	Bzz        int
	P2P        int
}

func NewSwarmPorts() *swarmPorts {
	sp := new(swarmPorts)
	sp.WebSockets = 8544
	sp.HTTPRPC = 8543
	sp.Bzz = 8542
	sp.P2P = 31000
	return sp
}

type PssMsg struct {
	Msg   []byte
	Peer  *p2p.Peer
	Asym  bool
	Keyid string
}

type pssSub struct {
	Unregister func()
	Delivery   (chan PssMsg)
	Address    string
}

type SimpleSwarm struct {
	Node       *node.Node
	NodeConfig *node.Config
	EnodeID    string
	Datadir    string
	Key        *ecdsa.PrivateKey
	NodeKey    *ecdsa.PrivateKey
	Pss        *pss.API
	PssPubKey  string
	PssAddr    pss.PssAddress
	PssTopics  map[string]*pssSub
	Hive       *network.Hive
	Ports      *swarmPorts
	ListenAddr string
	Client     *client.Client
}

func (sn *SimpleSwarm) SetLog(level string) error {
	// ensure good log formats for terminal
	// handle verbosity flag
	loglevel, err := log.LvlFromString(level)
	if err != nil {
		return err
	}
	hs := log.StreamHandler(os.Stderr, log.TerminalFormat(true))
	hf := log.LvlFilterHandler(loglevel, hs)
	h := log.CallerFileHandler(hf)
	log.Root().SetHandler(h)
	return nil
}

func (sn *SimpleSwarm) PrintStats() {
	// statistics thread
	go func() {
		for {
			if sn.Node.Server() != nil && sn.Hive != nil {
				addr := fmt.Sprintf("%x", sn.PssAddr)
				var addrs [][]byte
				addrs = append(addrs, []byte(addr))
				peerCount := sn.Node.Server().PeerCount()
				log.Info(fmt.Sprintf("PeerCount:%d NeighDepth:%d", peerCount, sn.Hive.NeighbourhoodDepth))
			}
			time.Sleep(time.Second * 5)
		}
	}()
}

func (sn *SimpleSwarm) SetDatadir(datadir string) {
	sn.Datadir = datadir
}

func (sn *SimpleSwarm) SetKey(key *ecdsa.PrivateKey) {
	sn.Key = key
}

func (sn *SimpleSwarm) InitBZZ() error {
	var err error
	if len(sn.Datadir) < 1 {
		usr, err := user.Current()
		if err != nil {
			return err
		}
		sn.Datadir = usr.HomeDir + "/.dvote/bzz"
		os.MkdirAll(sn.Datadir, 0755)
	}

	sn.SetLog("info")
	sn.Ports = NewSwarmPorts()
	sn.Ports.Bzz += 100
	sn.Ports.HTTPRPC += 100
	sn.Ports.P2P += 100
	sn.Ports.WebSockets += 100

	/*
		// set private key
		keypath := sn.Datadir + "/ecdsa.key"
		if _, err := os.Stat(keypath); err == nil {
			// load key
			prvKey, err := crypto.LoadECDSA(keypath)
			if err != nil {
				return err
			}
			sn.Key = prvKey

		} else if os.IsNotExist(err) {
			// generate and store key
			newKey, err := crypto.GenerateKey()
			if err != nil {
				return err
			}
			//write to file
			err = crypto.SaveECDSA(keypath, newKey)
			if err != nil {
				return err
			}
			sn.Key = newKey

		} else {
			// Schrodinger: file may or may not exist. See err for details.
			// this could be caused by permissions errors or a failing disk
			return err
		}
	*/

	// create node
	sn.Node, sn.NodeConfig, err = newNode(sn.Key, sn.Ports.P2P,
		sn.Ports.HTTPRPC, sn.Ports.WebSockets, sn.Datadir, "")
	if err != nil {
		return err
	}

	// set node key, if not set use the storage one or generate it
	if sn.NodeKey == nil {
		sn.NodeKey = sn.NodeConfig.NodeKey()
	}

	// create and register Swarm service
	_, swarmConfig, swarmHandler := newSwarm(sn.NodeKey, sn.NodeKey, sn.Datadir, sn.Ports.Bzz)
	err = sn.Node.Register(swarmHandler)
	if err != nil {
		return fmt.Errorf("swarm register fail %v", err)
	}

	// start the node
	sn.Node.Start()
	for _, url := range SwarmBootnodes {
		log.Info("Add bootnode " + url)
		node, _ := enode.ParseV4(url)
		sn.Node.Server().AddPeer(node)
	}

	// wait to connect to the p2p network
	_, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	time.Sleep(time.Second * 5)

	// establish swarm client
	sn.ListenAddr = swarmConfig.ListenAddr
	swarmURL := sn.ListenAddr + ":" + string(sn.Ports.HTTPRPC)
	sn.Client = client.NewClient(swarmURL)

	// Get the services API
	//for _, a := range swarmNode.APIs() {
	//	switch a.Service.(type) {
	//	case *network.Hive:
	//		sn.Hive = a.Service.(*network.Hive)
	//	case *pss.API:
	//		sn.Pss = a.Service.(*pss.API)
	//	default:
	//		log.Info("interface: " + fmt.Sprintf("%T", a.Service))
	//	}
	//}

	// Create topics map
	//sn.PssTopics = make(map[string]*pssSub)

	// Set some extra data
	//sn.EnodeID = sn.Node.Server().NodeInfo().Enode
	//sn.PssPubKey = hexutil.Encode(crypto.FromECDSAPub(sn.Pss.PublicKey()))
	//sn.PssAddr, err = sn.Pss.BaseAddr()
	//if err != nil {
	//	return fmt.Errorf("pss API fail %v", err)
	//}

	// Print some information
	//log.Info(fmt.Sprintf("My PSS pubkey is %s", sn.PssPubKey))
	//log.Info(fmt.Sprintf("My PSS address is %x", sn.PssAddr))

	// Run statistics goroutine
	sn.PrintStats()

	return nil
}

func (sn *SimpleSwarm) InitPSS() error {
	var err error
	if len(sn.Datadir) < 1 {
		usr, err := user.Current()
		if err != nil {
			return err
		}
		sn.Datadir = usr.HomeDir + "/.dvote/pss"
		os.MkdirAll(sn.Datadir, 0755)
	}

	sn.SetLog("info")
	sn.Ports = NewSwarmPorts()
	/*
		keypath := sn.Datadir + "/ecdsa.key"
		if _, err := os.Stat(keypath); err == nil {
			// load key
			prvKey, err := crypto.LoadECDSA(keypath)
			if err != nil {
				return err
			}
			sn.Key = prvKey

		} else if os.IsNotExist(err) {
			// generate and store key
			newKey, err := crypto.GenerateKey()
			if err != nil {
				return err
			}
			//write to file
			err = crypto.SaveECDSA(keypath, newKey)
			if err != nil {
				return err
			}
			sn.Key = newKey

		} else {
			// Schrodinger: file may or may not exist. See err for details.
			// this could be caused by permissions errors or a failing disk
			return err
		}
	*/

	// create node
	sn.Node, sn.NodeConfig, err = newNode(sn.Key, sn.Ports.P2P,
		sn.Ports.HTTPRPC, sn.Ports.WebSockets, sn.Datadir, "pss")
	if err != nil {
		return err
	}

	// set node key, if not set use the storage one or generate it
	if sn.NodeKey == nil {
		sn.NodeKey = sn.NodeConfig.NodeKey()
	}

	// create and register Swarm service
	swarmNode, _, swarmHandler := newSwarm(sn.NodeKey, sn.NodeKey, sn.Datadir, sn.Ports.Bzz)
	err = sn.Node.Register(swarmHandler)
	if err != nil {
		return fmt.Errorf("swarm register fail %v", err)
	}

	// start the node
	sn.Node.Start()
	for _, url := range SwarmBootnodes {
		log.Info("Add bootnode " + url)
		node, _ := enode.ParseV4(url)
		sn.Node.Server().AddPeer(node)
	}

	// wait to connect to the p2p network
	_, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	time.Sleep(time.Second * 5)

	// Get the services API
	for _, a := range swarmNode.APIs() {
		switch a.Service.(type) {
		case *network.Hive:
			sn.Hive = a.Service.(*network.Hive)
		case *pss.API:
			sn.Pss = a.Service.(*pss.API)
		default:
			//log.Info("interface: " + fmt.Sprintf("%T", a.Service))
			continue
		}
	}

	// Create topics map
	sn.PssTopics = make(map[string]*pssSub)

	// Set some extra data
	sn.EnodeID = sn.Node.Server().NodeInfo().Enode
	sn.PssPubKey = hexutil.Encode(crypto.FromECDSAPub(sn.Pss.PublicKey()))
	sn.PssAddr, err = sn.Pss.BaseAddr()
	if err != nil {
		return fmt.Errorf("pss API fail %v", err)
	}

	// Print some information
	log.Info(fmt.Sprintf("My PSS pubkey is %s", sn.PssPubKey))
	log.Info(fmt.Sprintf("My PSS address is %x", sn.PssAddr))

	// Run statistics goroutine
	sn.PrintStats()

	return nil
}

func strTopic(topic string) pss.Topic {
	return pss.BytesToTopic([]byte(topic))
}

func strSymKey(key string) []byte {
	symKey := make([]byte, 32)
	copy(symKey, []byte(key))
	return symKey
}

func strAddress(addr string) pss.PssAddress {
	var pssAddress pss.PssAddress
	pssAddress = []byte(addr)
	return pssAddress
}

func (sn *SimpleSwarm) PssSub(subType, key, topic, address string) error {
	pssTopic := strTopic(topic)
	pssAddress := strAddress(address)
	if subType == "sym" {
		_, err := sn.Pss.SetSymmetricKey(strSymKey(key), pssTopic, pssAddress, true)
		if err != nil {
			return err
		}
	}

	sn.PssTopics[topic] = new(pssSub)
	sn.PssTopics[topic].Address = address
	sn.PssTopics[topic].Delivery = make(chan PssMsg)

	var pssHandler pss.HandlerFunc = func(msg []byte, peer *p2p.Peer, asym bool, keyid string) error {
		log.Debug("pss received", "msg", fmt.Sprintf("%s", msg), "keyid", fmt.Sprintf("%s", keyid))

		sn.PssTopics[topic].Delivery <- PssMsg{Msg: msg, Peer: peer, Asym: asym, Keyid: keyid}
		return nil
	}
	topicHandler := pss.NewHandler(pssHandler)
	if subType == "raw" {
		topicHandler = topicHandler.WithProxBin().WithRaw()
	}
	sn.PssTopics[topic].Unregister = sn.Pss.Register(&pssTopic, topicHandler)

	log.Info(fmt.Sprintf("Subscribed to [%s] topic %s", subType, pssTopic.String()))
	return nil
}

func (sn *SimpleSwarm) PssPub(subType, key, topic, msg, address string) error {
	var err error
	dstAddr := strAddress(address)
	dstTopic := strTopic(topic)
	if subType == "sym" {
		symKeyId, err := sn.Pss.SetSymmetricKey(strSymKey(key), dstTopic, dstAddr, false)
		if err != nil {
			return err
		}
		// send symetric message
		err = sn.Pss.SendSym(symKeyId, strTopic(topic), hexutil.Bytes(msg))
	}
	if subType == "raw" {
		// sed raw message
		err = sn.Pss.SendRaw(hexutil.Bytes(address), dstTopic, hexutil.Bytes(msg))
	}
	if subType == "asym" {
		// add 0x prefix if not present
		if hasHexPrefix := strings.HasPrefix(key, "0x"); !hasHexPrefix {
			key = "0x" + key
		}

		// check if topic+address is already set for a pubKey
		_, err := sn.Pss.GetPeerAddress(key, dstTopic)
		if err != nil {
			pubKeyBytes, err := hexutil.Decode(key)
			if err != nil {
				return err
			}
			err = sn.Pss.SetPeerPublicKey(pubKeyBytes, dstTopic, dstAddr)
			if err != nil {
				return err
			}
		}

		// send asymetric message
		err = sn.Pss.SendAsym(key, dstTopic, hexutil.Bytes(msg))
	}
	return err
}

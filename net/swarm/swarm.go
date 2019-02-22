package swarm

import (
	"context"
	"crypto/ecdsa"
	"fmt"
	"os"
	"os/user"
	"time"

	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/node"

	"github.com/ethereum/go-ethereum/p2p"

	"github.com/ethereum/go-ethereum/log"
	"github.com/ethereum/go-ethereum/p2p/enode"
	"github.com/ethereum/go-ethereum/swarm"

	swarmapi "github.com/ethereum/go-ethereum/swarm/api"
	"github.com/ethereum/go-ethereum/swarm/network"
	"github.com/ethereum/go-ethereum/swarm/pss"
)

const (
	// MaxPeers is the maximum number of p2p peer connections
	MaxPeers = 10
)

// SwarmBootnodes list of bootnodes for the SWARM network
var SwarmBootnodes = []string{
	// EF Swarm Bootnode - AWS - eu-central-1
	"enode://4c113504601930bf2000c29bcd98d1716b6167749f58bad703bae338332fe93cc9d9204f08afb44100dc7bea479205f5d162df579f9a8f76f8b402d339709023@3.122.203.99:30301",
	// EF Swarm Bootnode - AWS - us-west-2
	"enode://89f2ede3371bff1ad9f2088f2012984e280287a4e2b68007c2a6ad994909c51886b4a8e9e2ecc97f9910aca538398e0a5804b0ee80a187fde1ba4f32626322ba@52.35.212.179:30301",
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

func newSwarm(privkey *ecdsa.PrivateKey, datadir string, port int) (*swarm.Swarm, *swarmapi.Config, node.ServiceConstructor) {
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
	swarmCfg.Init(privkey)
	swarmNode, err := swarm.NewSwarm(swarmCfg, nil)
	if err != nil {
		log.Crit("cannot crate swarm node")
	}
	// register swarm service to the node
	var swarmService node.ServiceConstructor = func(ctx *node.ServiceContext) (node.Service, error) {
		//return swarm.NewSwarm(swarmCfg, nil)
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

type SwarmNet struct {
	Node       *node.Node
	NodeConfig *node.Config
	EnodeID    string
	Datadir    string
	Key        *ecdsa.PrivateKey
	Pss        *pss.API
	PssAddr    pss.PssAddress
	Hive       *network.Hive
	Ports      *swarmPorts
}

func (sn *SwarmNet) SetLog() {
	// ensure good log formats for terminal
	// handle verbosity flag
	hs := log.StreamHandler(os.Stderr, log.TerminalFormat(true))
	loglevel := log.LvlInfo
	hf := log.LvlFilterHandler(loglevel, hs)
	h := log.CallerFileHandler(hf)
	log.Root().SetHandler(h)
}

func (sn *SwarmNet) PrintStats() {
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

func (sn *SwarmNet) SetDatadir(datadir string) {
	sn.Datadir = datadir
}

func (sn *SwarmNet) SetKey(key *ecdsa.PrivateKey) {
	sn.Key = key
}

func (sn *SwarmNet) Init() error {
	var err error
	if len(sn.Datadir) < 1 {
		usr, err := user.Current()
		if err != nil {
			return err
		}
		sn.Datadir = usr.HomeDir + "/.dvote/swarm"
		os.MkdirAll(sn.Datadir, 0755)
	}

	sn.SetLog()
	sn.Ports = NewSwarmPorts()

	// create node
	sn.Node, sn.NodeConfig, err = newNode(sn.Key, sn.Ports.P2P,
		sn.Ports.HTTPRPC, sn.Ports.WebSockets, sn.Datadir, "pss")
	if err != nil {
		return err
	}
	// set node key, if not set use the storage one or generate it
	if sn.Key == nil {
		sn.Key = sn.NodeConfig.NodeKey()
	}

	// create and register Swarm service
	swarmNode, _, swarmHandler := newSwarm(sn.Key, sn.Datadir, sn.Ports.Bzz)
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
	//defer sn.Node.Stop()

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
		}
	}

	// Set the enode ID and the pss Address, fail if not available
	sn.EnodeID = sn.Node.Server().NodeInfo().Enode

	sn.PssAddr, err = sn.Pss.BaseAddr()
	if err != nil {
		return fmt.Errorf("pss API fail %v", err)
	}
	log.Info(fmt.Sprintf("My PSS address is %x", sn.PssAddr))

	sn.PrintStats()

	return nil
}

func (sn *SwarmNet) Test() error {
	topic := pss.BytesToTopic([]byte("vocdoni_test"))
	symKey := make([]byte, 32)
	copy(symKey, []byte("vocdoni"))

	var emptyAddress pss.PssAddress
	emptyAddress = []byte("")
	symKeyId, err := sn.Pss.SetSymmetricKey(symKey, topic, emptyAddress, true)
	if err != nil {
		fmt.Errorf("pss cannot set symkey %v", err)
	}
	var pssHandler pss.HandlerFunc = func(msg []byte, peer *p2p.Peer, asym bool, keyid string) error {
		log.Info("pss received", "msg", fmt.Sprintf("%s", msg), "from", fmt.Sprintf("%x", peer))
		return nil
	}
	topicHandler := pss.NewHandler(pssHandler)
	topicUnregister := sn.Pss.Register(&topic, topicHandler)
	defer topicUnregister()
	log.Info(fmt.Sprintf("Subscribed to topic %s", topic.String()))

	hostname, _ := os.Hostname()
	for {
		err = sn.Pss.SendSym(symKeyId, topic, hexutil.Bytes(fmt.Sprintf("Hello world from %s", hostname)))
		/*	err = pssCli.SendRaw(emptyAddress, topic, []byte("Hello world!"))
			if err != nil {
				log.Warn("pss cannot send raw", "err", err)
			}
		*/
		log.Info("pss sent", "err", err)
		time.Sleep(10 * time.Second)
	}
}

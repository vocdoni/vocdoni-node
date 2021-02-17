// Package chain provides the functions to interact with the Ethereum-like control blockchain
package chain

import (
	"context"
	"encoding/base64"
	"fmt"
	"math/rand"
	"net"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/accounts/keystore"
	"github.com/ethereum/go-ethereum/cmd/utils"
	"github.com/ethereum/go-ethereum/core"
	"github.com/ethereum/go-ethereum/eth"
	"github.com/ethereum/go-ethereum/eth/downloader"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/ethereum/go-ethereum/les"
	"github.com/ethereum/go-ethereum/node"
	"github.com/ethereum/go-ethereum/p2p/enode"
	"github.com/ethereum/go-ethereum/p2p/nat"
	"go.vocdoni.io/dvote/config"
	"go.vocdoni.io/dvote/log"
	"go.vocdoni.io/dvote/metrics"
	"go.vocdoni.io/dvote/types"
	"go.vocdoni.io/dvote/util"
)

type EthChainContext struct {
	Node          *node.Node
	API           *eth.EthAPIBackend // Ethereum backend API (if fastmode enabled)
	LAPI          *les.LesApiBackend // Ethereum Light API (if lightmode enabled)
	Config        *eth.Config
	Keys          *keystore.KeyStore
	DefaultConfig *EthChainConfig
	VotingHandle  *VotingHandle
	MetricsAgent  *metrics.Agent
	RestartLock   sync.RWMutex
}

type EthChainConfig struct {
	RPCHost        string
	RPCPort        int
	NodePort       int
	NetworkId      int
	NetworkGenesis []byte
	BootstrapNodes []string
	TrustedPeers   []*enode.Node
	KeyStore       string
	DataDir        string
	IPCPath        string
	LightMode      bool
	W3external     string
}

// NewConfig returns an Ethereum config using some default values
func NewConfig(ethCfg *config.EthCfg, w3Cfg *config.W3Cfg) (*EthChainConfig, error) {
	chainSpecs, err := SpecsFor(ethCfg.ChainType)
	if err != nil {
		return nil, err
	}

	cfg := new(EthChainConfig)
	cfg.RPCHost = w3Cfg.RPCHost
	cfg.RPCPort = w3Cfg.RPCPort
	cfg.NodePort = ethCfg.NodePort
	cfg.NetworkId = chainSpecs.NetworkId
	cfg.NetworkGenesis, err = base64.StdEncoding.DecodeString(chainSpecs.GenesisB64)
	cfg.LightMode = ethCfg.LightMode
	cfg.W3external = w3Cfg.W3External
	if err != nil {
		return nil, err
	}
	if len(ethCfg.BootNodes) > 0 && len(ethCfg.BootNodes[0]) > 32 {
		r := strings.NewReplacer("[", "", "]", "") // viper []string{} sanity
		for _, b := range ethCfg.BootNodes {
			cfg.BootstrapNodes = append(cfg.BootstrapNodes, r.Replace(b))
		}
	} else {
		cfg.BootstrapNodes = chainSpecs.BootNodes
	}
	if len(ethCfg.TrustedPeers) > 0 && len(ethCfg.TrustedPeers[0]) > 32 {
		r := strings.NewReplacer("[", "", "]", "") // viper []string{} sanity
		for _, b := range ethCfg.TrustedPeers {
			node, err := enode.ParseV4(r.Replace(b))
			if err != nil {
				log.Warn(err)
				continue
			}
			cfg.TrustedPeers = append(cfg.TrustedPeers, node)
		}
	}
	defaultDirPath := ethCfg.DataDir
	cfg.KeyStore = defaultDirPath + "/keystore"
	cfg.DataDir = defaultDirPath + "/data"
	cfg.IPCPath = defaultDirPath + "/ipc"
	return cfg, nil
}

func Init(c *EthChainConfig) (*EthChainContext, error) {
	e := new(EthChainContext)
	err := e.init(c)
	return e, err
}

func (e *EthChainContext) init(c *EthChainConfig) error {
	e.DefaultConfig = c
	nodeConfig := node.DefaultConfig
	nodeConfig.InsecureUnlockAllowed = true
	nodeConfig.NoUSB = true
	nodeConfig.WSHost = c.RPCHost
	nodeConfig.WSPort = c.RPCPort
	nodeConfig.WSModules = []string{}
	nodeConfig.HTTPHost = c.RPCHost
	nodeConfig.HTTPPort = c.RPCPort
	nodeConfig.HTTPCors = []string{""}
	nodeConfig.HTTPVirtualHosts = []string{"*"}
	nodeConfig.HTTPModules = []string{}
	nodeConfig.WSOrigins = []string{"*"}
	nodeConfig.IPCPath = c.IPCPath
	nodeConfig.DataDir = c.DataDir
	nodeConfig.P2P.DiscoveryV5 = true
	log.Infof("listening on 0.0.0.0:%d", c.NodePort)
	nodeConfig.P2P.ListenAddr = fmt.Sprintf(":%d", c.NodePort)

	myPublicIP, err := util.PublicIP()
	if err != nil {
		log.Warn("cannot get external public IPv4 address")
	} else {
		natInt, err := nat.Parse("extip:" + myPublicIP.String())
		if err != nil {
			return err
		}
		nodeConfig.P2P.NAT = natInt
	}
	nodeConfig.P2P.BootstrapNodes = make([]*enode.Node, 0, len(c.BootstrapNodes))
	for _, url := range c.BootstrapNodes {
		if url != "" {
			node, err := enode.ParseV4(url)
			if err != nil {
				return err
			}
			nodeConfig.P2P.BootstrapNodes = append(nodeConfig.P2P.BootstrapNodes, node)
		}
	}
	log.Debugf("using ethereum bootstrap nodes: %v", nodeConfig.P2P.BootstrapNodes)
	n, err := node.New(&nodeConfig)
	if err != nil {
		return err
	}
	ethConfig := eth.DefaultConfig

	if len(c.W3external) > 0 {
		log.Infof("using external web3 endpoint %s", c.W3external)
	} else {
		// network id 1 is mainet, the default go-ethereum network
		// only if network id is bigger than 1, we try to fetch the genesis file from our code
		if c.NetworkId > 1 {
			ethConfig.NetworkId = uint64(c.NetworkId)
			g := new(core.Genesis)
			err = g.UnmarshalJSON(c.NetworkGenesis)
			if err != nil {
				log.Errorf("cannot read genesis")
				return err
			}
			ethConfig.Genesis = g
		}
		if c.LightMode {
			log.Info("using chain light mode synchronization")
			ethConfig.SyncMode = downloader.LightSync
		} else {
			log.Info("using chain fast mode synchronization")
			ethConfig.SyncMode = downloader.FastSync
		}
	}

	ks := keystore.NewKeyStore(c.KeyStore, keystore.StandardScryptN, keystore.StandardScryptP)
	e.Node = n
	e.Config = &ethConfig
	e.Keys = ks

	return nil
}

// Start starts an Ethereum blockchain connection and web3 APIs
func (e *EthChainContext) Start() {
	api := utils.RegisterEthService(e.Node, e.Config)
	if e.Config.SyncMode == downloader.FastSync {
		e.API = api.(*eth.EthAPIBackend)
	}
	if e.Config.SyncMode == downloader.LightSync {
		e.LAPI = api.(*les.LesApiBackend)
	}

	if len(e.Keys.Accounts()) < 1 {
		if err := e.createAccount(); err != nil {
			log.Fatalf("cannot create ethereum account: %s", err)
		}
	} else {
		// phrase := getPassPhrase("please provide primary account passphrase", false)
		if err := e.Keys.TimedUnlock(e.Keys.Accounts()[0], "", time.Duration(0)); err != nil {
			log.Fatalf("cannot unlock ethereum account: %s", err)
		}
		log.Infof("my Ethereum address %x", e.Keys.Accounts()[0].Address)
	}
	if len(e.DefaultConfig.W3external) == 0 {
		// Don't use ethereum's utils.StartNode. It sets up a signal
		// handler for SIGINT, which interferes with the signal handler
		// we set up in our own main func. We want to use ethereum as a
		// "pure" library, so it shouldn't be using signals.
		if err := e.Node.Start(); err != nil {
			log.Fatalf("error starting ethereum node: %v", err)
		}

		log.Infof("started Ethereum Blockchain service with Network ID %d", e.DefaultConfig.NetworkId)
		if e.DefaultConfig.RPCPort >= 0 && e.DefaultConfig.RPCHost != "" { // if host == "" RPC API is not initialized
			if e.DefaultConfig.RPCPort == 0 {
				// assign a random port
				// 1-1024 are only available to root.
				// if port already binded generate new one
				for {
					e.DefaultConfig.RPCPort = 1025 + rand.Intn(50000)
					ln, err := net.Listen("tcp", ":"+strconv.Itoa(e.DefaultConfig.RPCPort))
					if err != nil {
						continue
					}
					_ = ln.Close()
					log.Infof("RPC port is set to 0. Using random port %d", e.DefaultConfig.RPCPort)
					break
				}
			}
			log.Infof("web3 websocket rpc api endpoint initialized at ws://%s:%d", e.DefaultConfig.RPCHost, e.DefaultConfig.RPCPort)
			log.Infof("web3 http rpc api endpoint initialized at http://%s:%d", e.DefaultConfig.RPCHost, e.DefaultConfig.RPCPort)
		}

		log.Infof("my enode address: %s", e.Node.Server().NodeInfo().Enode)
		for _, p := range e.DefaultConfig.TrustedPeers {
			log.Infof("adding tusted peer %s", p)
			// AddTrustedPeer does not work as expected, but AddPeer does. So using both.
			e.Node.Server().AddPeer(p)
			e.Node.Server().AddTrustedPeer(p)
		}
		go e.SyncGuard(context.Background())

	} else {

		var err error
		client, err := EthClientConnect(e.DefaultConfig.W3external, 10)
		if err != nil {
			log.Errorf("cannot connect to external web3 endpoint, skipping ethereum initialization")
			return
		}

		tctx, cancel := context.WithTimeout(context.Background(), types.EthereumReadTimeout)
		defer cancel()
		nid, err := client.NetworkID(tctx)
		if err != nil || nid == nil {
			log.Fatalf("cannot get network ID from external web3: (%s)", err)
		}
		if nid.Int64() != int64(e.DefaultConfig.NetworkId) {
			log.Fatalf("web3 external network ID do not match the expected %d != %d", nid.Int64(), e.DefaultConfig.NetworkId)
		}
		log.Infof("connected to external web3 endpoint on network ID %d", nid.Int64())
	}
}

func (e *EthChainContext) createAccount() error {
	// phrase := getPassPhrase("Your new account will be locked with a passphrase. Please give a passphrase. Do not forget it!.", true)
	_, err := e.Keys.NewAccount("")
	if err != nil {
		return fmt.Errorf("failed to create account: %w", err)
	}
	e.Keys.TimedUnlock(e.Keys.Accounts()[0], "", time.Duration(0))
	log.Infof("my Ethereum address %x", e.Keys.Accounts()[0].Address)
	return nil
}

// PrintInfo prints every N seconds some ethereum information (sync and height). It's blocking!
func (e *EthChainContext) PrintInfo(ctx context.Context, seconds time.Duration) {
	var lastHeight uint64
	var info EthSyncInfo
	var err error
	var syncingInfo string
	for {
		time.Sleep(seconds)
		tctx, cancel := context.WithTimeout(ctx, time.Minute)
		info, err = e.SyncInfo(tctx)
		cancel()
		if err != nil {
			log.Warn(err)
			continue
		}
		if !info.Synced {
			syncingInfo = fmt.Sprintf("syncSpeed:%d b/s", (info.Height-lastHeight)/uint64(seconds.Seconds()))
		} else {
			syncingInfo = ""
		}
		log.Infof("[ethereum info] synced:%t height:%d/%d peers:%d mode:%s %s",
			info.Synced, info.Height, info.MaxHeight, info.Peers, info.Mode, syncingInfo)
		lastHeight = info.Height
	}
}

type EthSyncInfo struct {
	Height    uint64
	MaxHeight uint64
	Synced    bool
	Peers     int
	Mode      string
}

// SyncInfo returns the height and syncing Ethereum blockchain information
func (e *EthChainContext) SyncInfo(ctx context.Context) (info EthSyncInfo, err error) {
	// External Web3
	if len(e.DefaultConfig.W3external) > 0 {
		info.Mode = "external"
		info.Synced = false
		info.Peers = 1 // force peers=1 if using external web3
		var client *ethclient.Client
		var sp *ethereum.SyncProgress

		client, _ = EthClientConnect(e.DefaultConfig.W3external, 0)

		defer client.Close()
		sp, err = client.SyncProgress(ctx)
		if err != nil {
			log.Warn(err)
			return
		}
		if sp != nil {
			info.MaxHeight = sp.HighestBlock
			info.Height = sp.CurrentBlock
		} else {
			header, err2 := client.HeaderByNumber(ctx, nil)
			if err2 != nil {
				err = err2
				log.Warn(err)
				return
			}
			info.Height = uint64(header.Number.Int64())
			info.MaxHeight = info.Height
			info.Synced = info.Height > 0
		}
		return
	}
	e.RestartLock.RLock()
	defer e.RestartLock.RUnlock()
	// Light sync
	if e.DefaultConfig.LightMode {
		info.Mode = "light"
		info.Synced = !e.LAPI.Downloader().Synchronising()
		info.MaxHeight = e.LAPI.Downloader().Progress().HighestBlock
		if info.Synced {
			info.Height = e.LAPI.CurrentBlock().Number().Uint64()
		} else {
			info.Height = e.LAPI.Downloader().Progress().CurrentBlock
		}
		info.Peers = e.Node.Server().PeerCount()
		if info.Synced && (info.MaxHeight == 0 || info.Height == 0 || info.Peers == 0) {
			info.Synced = false
		}
		return
	}
	// Fast sync
	if e.API != nil && e.Node != nil {
		info.Mode = "fast"
		info.Synced = !e.API.Downloader().Synchronising()
		info.MaxHeight = e.API.Downloader().Progress().HighestBlock
		if info.Synced {
			info.Height = e.API.CurrentBlock().Number().Uint64()
		} else {
			info.Height = e.API.Downloader().Progress().CurrentBlock
		}
		info.Peers = e.Node.Server().PeerCount()
		if info.Synced && (info.MaxHeight == 0 || info.Height == 0 || info.Peers == 0) {
			info.Synced = false
		}
		return
	}
	err = fmt.Errorf("cannot get sync info, unknown error")
	return
}

func (e *EthChainContext) SyncGuard(ctx context.Context) {
	log.Infof("starting ethereum sync guard")
	for {
		time.Sleep(time.Second * 120)
		si, err := e.SyncInfo(ctx)
		if err != nil {
			continue
		}
		if si.Synced && si.Height+200 < si.MaxHeight {
			log.Warn("ethereum is experiencing sync problems, restarting node...")
			e.RestartLock.Lock()
			if err = e.Node.Close(); err != nil {
				log.Fatal(err)
			}
			if err = e.Node.Start(); err != nil {
				log.Fatal(err)
			}
			e.RestartLock.Unlock()
		}
	}
}

// EthClientConnect is a utility blocking function for connecting to an endpoint
// that will be executed until a connection is established and the client created
// successfully. If attempts is not 0 the function will return an error
// and the connection loop will terminate
func EthClientConnect(dialAddr string, attempts int) (*ethclient.Client, error) {
	var attemptsCounter int
	for {
		if attempts != 0 && attemptsCounter == attempts {
			return nil, fmt.Errorf("tryied %d to reconnect, connection cannot be established", attemptsCounter)
		}
		client, err := ethclient.DialContext(context.Background(), dialAddr)
		if err != nil || client == nil {
			log.Errorf("cannot create a client connection: (%s), trying again ...", err)
			time.Sleep(time.Second * 10)
			attemptsCounter++
			continue
		}
		return client, nil
	}
}

// Package chain provides the functions to interact with the Ethereum-like control blockchain
package chain

import (
	"context"
	"encoding/base64"
	"fmt"
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
	"github.com/ethereum/go-ethereum/node"
	"github.com/ethereum/go-ethereum/p2p/enode"
	"github.com/ethereum/go-ethereum/p2p/nat"
	"github.com/ethereum/go-ethereum/rpc"
	"gitlab.com/vocdoni/go-dvote/config"
	"gitlab.com/vocdoni/go-dvote/log"
	"gitlab.com/vocdoni/go-dvote/metrics"
	"gitlab.com/vocdoni/go-dvote/util"
)

type EthChainContext struct {
	Node          *node.Node
	Eth           *eth.Ethereum
	Config        *eth.Config
	Keys          *keystore.KeyStore
	DefaultConfig *EthChainConfig
	ProcessHandle *ProcessHandle
	MetricsAgent  *metrics.Agent
	RestartLock   sync.RWMutex
}

type EthChainConfig struct {
	WSHost         string
	WSPort         int
	HTTPHost       string
	HTTPPort       int
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
	cfg.WSHost = w3Cfg.WsHost
	cfg.WSPort = w3Cfg.WsPort
	cfg.HTTPHost = w3Cfg.HTTPHost
	cfg.HTTPPort = w3Cfg.HTTPPort
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
	nodeConfig.WSHost = c.WSHost
	nodeConfig.WSPort = c.WSPort
	nodeConfig.WSModules = []string{}
	nodeConfig.HTTPHost = c.HTTPHost
	nodeConfig.HTTPPort = c.HTTPPort
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

	if c.NetworkId > 0 {
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
	if len(c.W3external) > 0 {
		log.Infof("using external web3 endpoint %s", c.W3external)
	}

	ks := keystore.NewKeyStore(c.KeyStore, keystore.StandardScryptN, keystore.StandardScryptP)

	e.Node = n
	e.Config = &ethConfig
	e.Keys = ks
	return nil
}

// Start starts an Ethereum blockchain connection and web3 APIs
func (e *EthChainContext) Start() {
	utils.RegisterEthService(e.Node, e.Config)
	if len(e.Keys.Accounts()) < 1 {
		if err := e.createAccount(); err != nil {
			log.Error(err)
		}
	} else {
		// phrase := getPassPhrase("please provide primary account passphrase", false)
		e.Keys.TimedUnlock(e.Keys.Accounts()[0], "", time.Duration(0))
		log.Infof("my Ethereum address %x", e.Keys.Accounts()[0].Address)
	}
	if len(e.DefaultConfig.W3external) == 0 {
		utils.StartNode(e.Node)

		log.Infof("started Ethereum Blockchain service with Network ID %d", e.DefaultConfig.NetworkId)
		if e.DefaultConfig.WSPort > 0 {
			log.Infof("web3 WebSockets endpoint ws://%s:%d", e.DefaultConfig.WSHost, e.DefaultConfig.WSPort)
		}
		if e.DefaultConfig.HTTPPort > 0 {
			log.Infof("web3 HTTP endpoint http://%s:%d", e.DefaultConfig.HTTPHost, e.DefaultConfig.HTTPPort)
		}

		if !e.DefaultConfig.LightMode {
			var et *eth.Ethereum
			err := e.Node.Service(&et)
			if err != nil {
				log.Fatal(err)
			}
			e.Eth = et
		}
		log.Infof("my enode address: %s", e.Node.Server().NodeInfo().Enode)
		for _, p := range e.DefaultConfig.TrustedPeers {
			log.Infof("adding tusted peer %s", p)
			// AddTrustedPeer does not work as expected, but AddPeer does. So using both.
			e.Node.Server().AddPeer(p)
			e.Node.Server().AddTrustedPeer(p)
		}
		go e.SyncGuard()
	}
}

func (e *EthChainContext) createAccount() error {
	// phrase := getPassPhrase("Your new account will be locked with a passphrase. Please give a passphrase. Do not forget it!.", true)
	_, err := e.Keys.NewAccount("")
	if err != nil {
		return fmt.Errorf("failed to create account: %v", err)
	}
	e.Keys.TimedUnlock(e.Keys.Accounts()[0], "", time.Duration(0))
	log.Infof("my Ethereum address %x", e.Keys.Accounts()[0].Address)
	return nil
}

// PrintInfo prints every N seconds some ethereum information (sync and height). It's blocking!
func (e *EthChainContext) PrintInfo(seconds time.Duration) {
	var lastHeight uint64
	var info EthSyncInfo
	var err error
	var syncingInfo string
	for {
		time.Sleep(seconds)
		info, err = e.SyncInfo()
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
func (e *EthChainContext) SyncInfo() (info EthSyncInfo, err error) {
	e.RestartLock.RLock()
	defer e.RestartLock.RUnlock()
	// Light sync
	if e.DefaultConfig.LightMode {
		info.Mode = "light"
		info.Synced = false
		var r *rpc.Client
		r, err = e.Node.Attach()
		if r == nil || err != nil {
			return
		}

		r.Call(&info.Synced, "eth_syncing") // true = syncing / false if synced
		info.Synced = !info.Synced
		var block string
		r.Call(&block, "eth_blockNumber")
		info.Height = uint64(util.Hex2int64(block))
		if info.Height == 0 {
			info.Synced = false // Workaround
		}
		// TODO find a way to get the maxHeight on light mode
		info.MaxHeight = info.Height
		info.Peers = e.Node.Server().PeerCount()
		return
	}
	// External Web3
	if len(e.DefaultConfig.W3external) > 0 {
		info.Mode = "external"
		info.Synced = false
		info.Peers = 1 // force peers=1 if using external web3
		var client *ethclient.Client
		var sp *ethereum.SyncProgress
		client, err = ethclient.Dial(e.DefaultConfig.W3external)
		if err != nil {
			return
		}
		sp, err = client.SyncProgress(context.Background())
		info.MaxHeight = sp.HighestBlock
		info.Height = sp.CurrentBlock
		return
	}
	// Fast sync
	if e.Eth != nil {
		info.Mode = "fast"
		info.Synced = e.Eth.Synced()
		if info.Synced {
			info.Height = e.Eth.BlockChain().CurrentBlock().Number().Uint64()
		} else {
			info.Height = e.Eth.Downloader().Progress().CurrentBlock
		}
		info.MaxHeight = e.Eth.Downloader().Progress().HighestBlock
		info.Peers = e.Node.Server().PeerCount()
		return
	}
	err = fmt.Errorf("cannot get sync info, unknown error")
	return
}

func (e *EthChainContext) SyncGuard() {
	log.Infof("starting ethereum sync guard")
	for {
		time.Sleep(time.Second * 120)
		si, err := e.SyncInfo()
		if err != nil {
			continue
		}
		if si.Synced && si.Height+200 < si.MaxHeight {
			log.Warn("ethereum is experiencing sync problems, restarting node...")
			e.RestartLock.Lock()
			if err = e.Node.Restart(); err != nil {
				log.Fatal(err)
			}
			e.RestartLock.Unlock()
		}
	}
}

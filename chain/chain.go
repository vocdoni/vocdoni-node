// Package chain provides the functions to interact with the Ethereum-like control blockchain
package chain

import (
	"context"
	"encoding/base64"
	"fmt"
	"strings"
	"time"

	"github.com/ethereum/go-ethereum/accounts/keystore"
	"github.com/ethereum/go-ethereum/cmd/utils"
	"github.com/ethereum/go-ethereum/core"
	"github.com/ethereum/go-ethereum/eth"
	"github.com/ethereum/go-ethereum/eth/downloader"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/ethereum/go-ethereum/node"
	"github.com/ethereum/go-ethereum/p2p/enode"
	"github.com/ethereum/go-ethereum/p2p/nat"
	"gitlab.com/vocdoni/go-dvote/config"
	"gitlab.com/vocdoni/go-dvote/log"
	"gitlab.com/vocdoni/go-dvote/util"
)

type EthChainContext struct {
	Node          *node.Node
	Eth           *eth.Ethereum
	Config        *eth.Config
	Keys          *keystore.KeyStore
	DefaultConfig *EthChainConfig
	ProcessHandle *ProcessHandle
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
		if err := e.Node.Start(); err != nil {
			log.Fatalf("error starting ethereum node: %v", err)
		}

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
	}
}

func (e *EthChainContext) createAccount() error {
	// phrase := getPassPhrase("Your new account will be locked with a passphrase. Please give a passphrase. Do not forget it!.", true)
	_, err := e.Keys.NewAccount("")
	if err != nil {
		return fmt.Errorf("failed to create account: %v", err)
	}
	e.Keys.TimedUnlock(e.Keys.Accounts()[0], "", time.Duration(0))
	log.Infof("my Ethereum address %x\n", e.Keys.Accounts()[0].Address)
	return nil
}

// PrintInfo prints every N seconds some ethereum information (sync and height). It's blocking!
func (e *EthChainContext) PrintInfo(seconds time.Duration) {
	for {
		time.Sleep(seconds)
		height, synced, peers, err := e.SyncInfo()
		if err != nil {
			log.Warn(err)
		}
		log.Infof("[ethereum info] synced:%t height:%s peers:%d mode:%s", synced, height, peers, e.Config.SyncMode)
	}
}

// SyncInfo returns the height and syncing Ethereum blockchain information
func (e *EthChainContext) SyncInfo() (height string, synced bool, peers int, err error) {
	if e.DefaultConfig.LightMode {
		synced = true
		r, err := e.Node.Attach()
		if r == nil || err != nil {
			return "0", false, 0, err
		}
		r.Call(&synced, "eth_syncing") // true = syncing / false if synced
		synced = !synced
		r.Call(&height, "eth_blockNumber")
		block := util.Hex2int64(height)
		if block == 0 {
			// Workaround
			synced = false
		}
		height = fmt.Sprintf("%d", block)
		peers = e.Node.Server().PeerCount()
		return height, synced, peers, err
	}
	if len(e.DefaultConfig.W3external) > 0 {
		client, err := ethclient.Dial(e.DefaultConfig.W3external)
		if err != nil {
			return "0", true, 0, err
		}
		sp, err := client.SyncProgress(context.Background())
		height = fmt.Sprintf("%d", sp.CurrentBlock)
		return height, true, 1, err
	}
	if e.Eth != nil {
		synced = e.Eth.Synced()
		if synced {
			height = e.Eth.BlockChain().CurrentBlock().Number().String()
		} else {
			height = fmt.Sprintf("%d/%d",
				e.Eth.Downloader().Progress().CurrentBlock,
				e.Eth.Downloader().Progress().HighestBlock)
		}
		peers = e.Node.Server().PeerCount()
		return height, synced, peers, nil
	}
	return "0", false, 0, fmt.Errorf("cannot get sync info, unknown error")
}

// Package chain provides the functions to interact with the Ethereum-like control blockchain
package chain

import (
	"context"
	"encoding/base64"
	"fmt"
	"math/big"
	"time"

	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/accounts/keystore"
	"github.com/ethereum/go-ethereum/cmd/utils"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/console"
	"github.com/ethereum/go-ethereum/core"
	ethTypes "github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/eth"
	"github.com/ethereum/go-ethereum/eth/downloader"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/ethereum/go-ethereum/node"
	"github.com/ethereum/go-ethereum/p2p/enode"
	"github.com/ethereum/go-ethereum/rpc"

	"gitlab.com/vocdoni/go-dvote/config"
	"gitlab.com/vocdoni/go-dvote/log"
	//	"github.com/ethereum/go-ethereum/accounts/abi"
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
	KeyStore       string
	DataDir        string
	IPCPath        string
	LightMode      bool
	W3external     string
}

// available chains: vctestnet
func NewConfig(w3Cfg config.W3Cfg) (*EthChainConfig, error) {
	chainSpecs, err := ChainSpecsFor(w3Cfg.ChainType)
	if err != nil {
		return nil, err
	}

	cfg := new(EthChainConfig)
	cfg.WSHost = w3Cfg.WsHost
	cfg.WSPort = w3Cfg.WsPort
	cfg.HTTPHost = w3Cfg.HttpHost
	cfg.HTTPPort = w3Cfg.HttpPort
	cfg.NodePort = w3Cfg.NodePort
	cfg.NetworkId = chainSpecs.NetworkId
	cfg.NetworkGenesis, err = base64.StdEncoding.DecodeString(chainSpecs.GenesisB64)
	cfg.LightMode = w3Cfg.LightMode
	cfg.W3external = w3Cfg.W3External
	if err != nil {
		return nil, err
	}
	cfg.BootstrapNodes = chainSpecs.BootNodes
	defaultDirPath := w3Cfg.DataDir + "/ethereum"
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
	if len(e.DefaultConfig.W3external) == 0 {
		utils.RegisterEthService(e.Node, e.Config)
		utils.StartNode(e.Node)

		if len(e.Keys.Accounts()) < 1 {
			if err := e.createAccount(); err != nil {
				log.Error(err)
			}
		} else {
			// phrase := getPassPhrase("please provide primary account passphrase", false)
			e.Keys.TimedUnlock(e.Keys.Accounts()[0], "", time.Duration(0))
			log.Infof("my Ethereum address %x", e.Keys.Accounts()[0].Address)
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
	}
}

// might be worthwhile to create generic SendTx to call contracttx, deploytx, etc
func (e *EthChainContext) sendTx(addr string, limit uint64, amount int) error {
	client, err := ethclient.Dial(e.Node.IPCEndpoint())
	deadline := time.Now().Add(1000 * time.Millisecond)
	ctx, cancel := context.WithDeadline(context.TODO(), deadline)
	defer cancel()

	accounts := e.Keys.Accounts()
	acc := accounts[0]
	sendAddr := acc.Address
	nonce, _ := client.NonceAt(ctx, sendAddr, nil)
	if err != nil {
		return err
	}
	// create tx
	price, _ := client.SuggestGasPrice(ctx)
	var empty []byte
	tx := ethTypes.NewTransaction(nonce, common.HexToAddress(addr), big.NewInt(int64(amount)), limit, price, empty)
	signedTx, err := e.Keys.SignTx(acc, tx, big.NewInt(int64(e.Config.NetworkId)))
	if err != nil {
		return err
	}
	// create ctx
	err = client.SendTransaction(ctx, signedTx)
	log.Error(err)
	// fix return*/
	return err
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

func getPassPhrase(prompt string, confirmation bool) string {
	// Otherwise prompt the user for the password
	if prompt != "" {
		log.Info(prompt)
	}
	phrase, err := console.Stdin.PromptPassword("Passphrase: ")
	if err != nil {
		utils.Fatalf("failed to read passphrase: %v", err)
	}
	if confirmation {
		confirm, err := console.Stdin.PromptPassword("repeat passphrase: ")
		if err != nil {
			utils.Fatalf("failed to read passphrase confirmation: %v", err)
		}
		if phrase != confirm {
			utils.Fatalf("passphrases do not match")
		}
	}
	return phrase
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
		var r *rpc.Client
		r, err = e.Node.Attach()
		r.Call(&synced, "eth_syncing") // true = syncing / false if synced
		synced = !synced
		err = r.Call(&height, "eth_blockNumber")
		peers = e.Node.Server().PeerCount()
		return
	}
	if len(e.DefaultConfig.W3external) > 0 {
		var client *ethclient.Client
		var sp *ethereum.SyncProgress
		client, err = ethclient.Dial(e.DefaultConfig.W3external)
		if err != nil {
			return "0", true, 0, err
		}
		sp, err = client.SyncProgress(context.Background())
		height = fmt.Sprintf("%d", sp.CurrentBlock)
		synced = true
		peers = 1
		return
	}
	if e.Eth != nil {
		synced = e.Eth.Synced()
		height = e.Eth.BlockChain().CurrentBlock().Number().String()
		peers = e.Node.Server().PeerCount()
		return
	}
	return "0", false, 0, fmt.Errorf("cannot get sync info, unknown error")
}

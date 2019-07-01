package chain

import (
	"context"
	"encoding/base64"
	"math/big"
	"time"

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
	"github.com/vocdoni/go-dvote/config"
	"github.com/vocdoni/go-dvote/log"

	//	"github.com/ethereum/go-ethereum/accounts/abi"

	votingprocess "github.com/vocdoni/go-dvote/chain/contracts"
)

type EthChainContext struct {
	Node          *node.Node
	Eth           *eth.Ethereum
	Config        *eth.Config
	Keys          *keystore.KeyStore
	DefaultConfig *EthChainConfig
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
}

// available chains: vctestnet
func NewConfig(w3Cfg config.W3Cfg) (*EthChainConfig, error) {
	chainSpecs, err := GetChainSpecs(w3Cfg.ChainType)
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
	if err != nil {
		return nil, err
	}
	cfg.BootstrapNodes = chainSpecs.BootNodes
	cfg.KeyStore = "run/eth/keystore"
	cfg.DataDir = "run/eth/data"
	cfg.IPCPath = "run/eth/ipc"
	cfg.LightMode = false
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
	nodeConfig.HTTPCors = []string{"*"}
	nodeConfig.HTTPVirtualHosts = []string{"*"}
	nodeConfig.HTTPModules = []string{}
	nodeConfig.WSOrigins = []string{"*"}
	nodeConfig.NoUSB = true
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
	ethConfig.NetworkId = uint64(c.NetworkId)
	if c.LightMode {
		ethConfig.SyncMode = downloader.LightSync
	}

	g := new(core.Genesis)
	err = g.UnmarshalJSON(c.NetworkGenesis)
	if err != nil {
		log.Errorf("Cannot read genesis file")
		return err
	}
	ethConfig.Genesis = g

	ks := keystore.NewKeyStore(c.KeyStore, keystore.StandardScryptN, keystore.StandardScryptP)

	e.Node = n
	e.Config = &ethConfig
	e.Keys = ks

	return nil
}

func (e *EthChainContext) Start() {
	utils.RegisterEthService(e.Node, e.Config)
	utils.StartNode(e.Node)
	if len(e.Keys.Accounts()) < 1 {
		e.createAccount()
	} else {
		//phrase := getPassPhrase("please provide primary account passphrase", false)
		e.Keys.TimedUnlock(e.Keys.Accounts()[0], "", time.Duration(0))
		log.Infof("My Ethereum address %x\n", e.Keys.Accounts()[0].Address)
	}
	log.Infof("Started Ethereum Blockchain service with Network ID %d", e.DefaultConfig.NetworkId)
	if e.DefaultConfig.WSPort > 0 {
		log.Infof("Web3 WebSockets endpoint ws://%s:%d\n", e.DefaultConfig.WSHost, e.DefaultConfig.WSPort)
	}
	if e.DefaultConfig.HTTPPort > 0 {
		log.Infof("Web3 HTTP endpoint http://%s:%d\n", e.DefaultConfig.HTTPHost, e.DefaultConfig.HTTPPort)
	}
}

func (e *EthChainContext) LinkBatch(ref []byte) error {
	client, err := ethclient.Dial(e.Node.IPCEndpoint())
	if err != nil {
		return err
	}
	//account := e.k.Accounts()[0]

	/*
		nonce, err := client.PendingNonceAt(context.Background(), account.Address)
		if err != nil {
			return err
		}
	*/

	/*
		deadline := time.Now().Add(1000 * time.Millisecond)
		ctx, cancel := context.WithDeadline(context.TODO(), deadline)
		defer cancel()
		gasPrice, err := client.SuggestGasPrice(ctx)
	*/

	contractAddr := common.HexToAddress("0x3e4FfefF898580eC8132A97A91543c8fdeF1210E")
	votingProcessInstance, err := votingprocess.NewVotingProcess(contractAddr, client)
	if err != nil {
		log.Fatalf("Error: %v", err)
	}

	log.Infof("contract is loaded")
	_ = votingProcessInstance
	return nil

}

func (e *EthChainContext) TestTx(amount int) error {
	bigWalletAddr := "0x781b6544b1a73c6d779eb23c7369cf8039640793"
	var gasLimit uint64
	gasLimit = 8000000
	return e.sendTx(bigWalletAddr, gasLimit, amount)
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
	//create tx
	price, _ := client.SuggestGasPrice(ctx)
	log.Infof("Price: $v", price)
	var empty []byte
	tx := ethTypes.NewTransaction(nonce, common.HexToAddress(addr), big.NewInt(int64(amount)), limit, price, empty)
	signedTx, err := e.Keys.SignTx(acc, tx, big.NewInt(int64(e.Config.NetworkId)))
	if err != nil {
		return err
	}
	//create ctx
	err = client.SendTransaction(ctx, signedTx)
	log.Errorf("Error: %v", err)
	//fix return*/
	return err
}

func (e *EthChainContext) createAccount() error {
	//phrase := getPassPhrase("Your new account will be locked with a passphrase. Please give a passphrase. Do not forget it!.", true)
	_, err := e.Keys.NewAccount("")

	if err != nil {
		utils.Fatalf("Failed to create account: %v", err)
		return err
	}
	e.Keys.TimedUnlock(e.Keys.Accounts()[0], "", time.Duration(0))
	log.Infof("My Ethereum address %x\n", e.Keys.Accounts()[0].Address)

	return nil
}

func getPassPhrase(prompt string, confirmation bool) string {
	// Otherwise prompt the user for the password
	if prompt != "" {
		log.Info(prompt)
	}
	phrase, err := console.Stdin.PromptPassword("Passphrase: ")
	if err != nil {
		utils.Fatalf("Failed to read passphrase: %v", err)
	}
	if confirmation {
		confirm, err := console.Stdin.PromptPassword("Repeat passphrase: ")
		if err != nil {
			utils.Fatalf("Failed to read passphrase confirmation: %v", err)
		}
		if phrase != confirm {
			utils.Fatalf("Passphrases do not match")
		}
	}
	return phrase
}

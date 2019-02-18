package chain

import (
	"context"
	"fmt"
	"log"
	"math/big"
	"time"

	"github.com/ethereum/go-ethereum/cmd/utils"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/eth"
	"github.com/ethereum/go-ethereum/eth/downloader"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/ethereum/go-ethereum/node"
	"github.com/ethereum/go-ethereum/p2p/enode"

	//	"github.com/ethereum/go-ethereum/accounts/abi"
	hdwallet "github.com/miguelmota/go-ethereum-hdwallet"
)

type Node interface {
	Init() error
	Start()
	LinkBatch([]byte) error
}

type EthNodeHandle struct {
	n *node.Node
	c *eth.Config
}

func Init() (Node, error) {
	e := new(EthNodeHandle)
	err := e.Init()
	return e, err
}

func (e *EthNodeHandle) Init() error {

	nodeConfig := node.DefaultConfig
	nodeConfig.IPCPath = "~/.go-dvote-eth.ipc"

	//might also make sense to use staticnodes instead of bootstrap
	var urls = []string{
		"enode://e97bbd1b66c91a3e042703ec1a9af9361e7c84d892632cfa995bad96f3fa630b7048c8194e2bbc8984a612db7f5f8ca29be5d807d8abe5db86d5505b62eaa382@51.15.114.224:30303",
		"enode://480fbf14bab92f6203140403f5a1d4b4a8af143b4bc2c48d7f09fd3a294d08955611c362064ea01681663e42638440a888c326977e178a35295f5c1ca99dfb0f@51.15.71.154:30303",
		"enode://d6165c23b1d4415f845922b6589fcf19c62e07555c867848a890978993230dad039c248975578216b69a9f7a8fbc2b58db4817bd693f464a6aa45af90964b1e2@51.15.102.251:30303",
		"enode://3722cb9af22c61294fe6a0a7beb1db176ef456eb4cab1f6551cefb44e253450342de6459bcf576bd97797cf0df9e818db3d048532f3068351ed1bacaa6ec0d20@51.15.89.137:30303",
	}
	nodeConfig.P2P.BootstrapNodes = make([]*enode.Node, 0, len(urls))
	for _, url := range urls {
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
	ethConfig.NetworkId = 1714
	ethConfig.SyncMode = downloader.LightSync

	e.n = n
	e.c = &ethConfig
	return nil
}

func (e *EthNodeHandle) Start() {
	utils.RegisterEthService(e.n, e.c)
	// start the stack and watch for SIG events
	utils.StartNode(e.n)
	//api := node.NewPublicAdminAPI(e.n)
	//info, _ := api.NodeInfo()
	//fmt.Println(info)
	//client, _ := ethclient.Dial(e.n.IPCEndpoint())
	//ctx, _ := context.WithCancel(context.TODO())
	//sync, _ := client.SyncProgress(ctx)
	//fmt.Println(sync)
	// wait for explicit shutdown
	//e.n.Wait()
	//download chain?
}

func (e *EthNodeHandle) LinkBatch(data []byte) error {
	contractAddr := "0x3e4FfefF898580eC8132A97A91543c8fdeF1210E"
	var gasLimit uint64
	gasLimit = 100000000000000000
	return e.sendContractTx(contractAddr, gasLimit, data)
}

// might be worthwhile to create generic SendTx to call contracttx, deploytx, etc

func (e *EthNodeHandle) sendContractTx(addr string, limit uint64, data []byte) error {
	fmt.Println(e.n)
	client, err := ethclient.Dial(e.n.IPCEndpoint())
	fmt.Println("Got IPC Endpoint:" + e.n.IPCEndpoint())
	deadline := time.Now().Add(1000 * time.Millisecond)
	ctx, cancel := context.WithDeadline(context.TODO(), deadline)
	defer cancel()
	fmt.Println("context created")

	mnemonic := "perfect kite link property simple eight welcome spring enforce universe barely cargo"
	wallet, err := hdwallet.NewFromMnemonic(mnemonic)
	if err != nil {
		log.Fatal(err)
	}

	path := hdwallet.MustParseDerivationPath("m/44'/60'/0'/0/0")
	account, err := wallet.Derive(path, true)
	if err != nil {
		log.Fatal(err)
	}

	fmt.Println("recovered acct")
	key, err := wallet.PrivateKey(account)
	sendaddr, err := wallet.Address(account)
	nonce, _ := client.NonceAt(ctx, sendaddr, nil)
	if err != nil {
		fmt.Println("error")
		return err
	}
	//create tx
	fmt.Println("creating tx")
	price, _ := client.SuggestGasPrice(ctx)
	fmt.Println(price)
	tx := types.NewTransaction(nonce, common.HexToAddress(addr), big.NewInt(0), limit, price, data)
	signedTx, _ := types.SignTx(tx, types.HomesteadSigner{}, key)
	//create ctx
	err = client.SendTransaction(ctx, signedTx)
	fmt.Println(err)
	//fix return
	return err
}

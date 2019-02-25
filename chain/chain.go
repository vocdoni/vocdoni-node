package chain

import (
	"context"
	"fmt"
	"io/ioutil"
	"math/big"
	"os"
	"time"

	"github.com/ethereum/go-ethereum/accounts/keystore"
	"github.com/ethereum/go-ethereum/cmd/utils"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/console"
	"github.com/ethereum/go-ethereum/core"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/eth"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/ethereum/go-ethereum/node"
	"github.com/ethereum/go-ethereum/p2p/enode"
	//	"github.com/ethereum/go-ethereum/accounts/abi"

	"github.com/vocdoni/go-dvote/chain/contracts"
)

type Node interface {
	Init() error
	Start()
	LinkBatch([]byte) error
}

type EthNodeHandle struct {
	n *node.Node
	s *eth.Ethereum
	c *eth.Config
	k *keystore.KeyStore
}

func Init() (Node, error) {
	e := new(EthNodeHandle)
	err := e.init()
	return e, err
}

func (e *EthNodeHandle) init() error {

	nodeConfig := node.DefaultConfig
	nodeConfig.IPCPath = "run/eth/ipc"
	nodeConfig.DataDir = "run/eth/data"

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
	//ethConfig.SyncMode = downloader.LightSync
	var genesisJson *os.File
	genesisJson, err = os.Open("genesis.json")
	genesisBytes, _ := ioutil.ReadAll(genesisJson)
	g := new(core.Genesis)
	g.UnmarshalJSON(genesisBytes)
	ethConfig.Genesis = g

	ks := keystore.NewKeyStore("run/eth/keystore", keystore.StandardScryptN, keystore.StandardScryptP)

	e.n = n
	e.c = &ethConfig
	e.k = ks

	return nil
}

func (e *EthNodeHandle) Start() {
	utils.RegisterEthService(e.n, e.c)
	utils.StartNode(e.n)
	if len(e.k.Accounts()) < 1 {
		e.createAccount()
	} else {
		phrase := getPassPhrase("please provide primary account passphrase", false)
		e.k.TimedUnlock(e.k.Accounts()[0], phrase, time.Duration(0))
	}
}

func (e *EthNodeHandle) LinkBatch(ref []byte) error {
	contractAddr := "0x3e4FfefF898580eC8132A97A91543c8fdeF1210E"
	processHandle, err := process.
}

func (e *EthNodeHandle) TestTx(amount int) error {
	bigWalletAddr := "0x781b6544b1a73c6d779eb23c7369cf8039640793"
	var gasLimit uint64
	gasLimit = 8000000
	return e.sendContractTx(bigWalletAddr, gasLimit, int64(amount))
}

// might be worthwhile to create generic SendTx to call contracttx, deploytx, etc

func (e *EthNodeHandle) sendTx(addr string, limit uint64, amount int) error {

	client, err := ethclient.Dial(e.n.IPCEndpoint())
	deadline := time.Now().Add(1000 * time.Millisecond)
	ctx, cancel := context.WithDeadline(context.TODO(), deadline)
	defer cancel()

	accounts := e.k.Accounts()
	acc := accounts[0]
	sendAddr := acc.Address
	nonce, _ := client.NonceAt(ctx, sendAddr, nil)
	if err != nil {
		return err
	}
	//create tx
	price, _ := client.SuggestGasPrice(ctx)
	fmt.Println(price)
	var empty []byte
	tx := types.NewTransaction(nonce, common.HexToAddress(addr), big.NewInt(amount), limit, price, empty)
	signedTx, err := e.k.SignTx(acc, tx, big.NewInt(int64(e.c.NetworkId)))
	if err != nil {
		return err
	}
	//create ctx
	err = client.SendTransaction(ctx, signedTx)
	fmt.Println(err)
	//fix return*/
	return err
}

func (e *EthNodeHandle) createAccount() error {
	phrase := getPassPhrase("Your new account will be locked with a passphrase. Please give a passphrase. Do not forget it!.", true)
	acc, err := e.k.NewAccount(phrase)

	if err != nil {
		utils.Fatalf("Failed to create account: %v", err)
		return err
	}
	fmt.Printf("Address: {%x}\n", acc.Address)
	e.k.TimedUnlock(e.k.Accounts()[0], phrase, time.Duration(0))

	return nil
}

func getPassPhrase(prompt string, confirmation bool) string {
	// Otherwise prompt the user for the password
	if prompt != "" {
		fmt.Println(prompt)
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

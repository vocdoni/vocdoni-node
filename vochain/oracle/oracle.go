package vochain

// CONNECT TO WEB3
// CONNECT TO TENDERMINT

// INSTANTIATE THE CONTRACT

// GET METHODS FOR THE CONTRACT
//		PROCESS
//		VALIDATORS
// 		ORACLES

// SUBSCRIBE TO EVENTS

// CREATE TM TX BASED ON EVENTS

// WRITE TO ETH SM IF PROCESS FINISHED

import (
	"context"
	"fmt"
	"log"

	eth "github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/ethclient"
	tmclient "github.com/tendermint/tendermint/abci/client"
	"gitlab.com/vocdoni/go-dvote/chain"
)

// Oracle represents an oracle with a connection to Ethereum and Vochain
type Oracle struct {
	// ethereum connection
	ethereumConnection *chain.EthChainContext
	// vochain connection
	vochainConnection *tmclient.Client
	// ethereum subscribed events
	ethereumEventList []string
	// vochain subscribed events
	vochainEventList []string
}

// NewOracle creates an Oracle given an existing Ethereum and Vochain connection
func NewOracle(ethCon *chain.EthChainContext, voCon *tmclient.Client) *Oracle {
	return &Oracle{
		ethereumConnection: ethCon,
		vochainConnection:  voCon,
	}
}

// BlockInfo represents the basic Ethereum block information
type BlockInfo struct {
	Hash     string
	Number   uint64
	Time     uint64
	Nonce    uint64
	TxNumber int
}

// NewBlockInfo creates a pointer to a new BlockInfo
func NewBlockInfo() *BlockInfo {
	return &BlockInfo{}
}

// Info retuns information about the Oracle
func (o *Oracle) Info() {}

// Start starts the oracle
func (o *Oracle) Start() {}

// GetEthereumContract gets a contract from ethereum if exists
func (o *Oracle) GetEthereumContract() {}

// GetEthereumEventList gets the ethereum events to which we are subscribed
func (o *Oracle) GetEthereumEventList() {}

// GetVochainEventList gets the vochain events to which we are subscribed
func (o *Oracle) GetVochainEventList() {}

// SetEthereumEventList sets the ethereum events to which we want to be subscribed
func (o *Oracle) SetEthereumEventList() {}

// SetVochainEventList sets the vochain events to which we want to be subscribed
func (o *Oracle) SetVochainEventList() {}

// SendEthereumTx sends a transaction to ethereum
func (o *Oracle) SendEthereumTx() {}

// SendVochainTx sends a transaction to vochain
func (o *Oracle) SendVochainTx() {}

// EthereumBlockListener returns a block info when a new block is created on Ethereum
func (o *Oracle) EthereumBlockListener() BlockInfo {
	client, err := ethclient.Dial(o.ethereumConnection.Node.WSEndpoint())
	if err != nil {
		log.Fatal(err)
	}
	headers := make(chan *types.Header)
	sub, err := client.SubscribeNewHead(context.Background(), headers)
	if err != nil {
		log.Fatal(err)
	}
	for {
		select {
		case err := <-sub.Err():
			log.Fatal(err)
		case header := <-headers:
			fmt.Println(header.Hash().Hex()) // 0xbc10defa8dda384c96a17640d84de5578804945d347072e091b4e5f390ddea7f

			block, err := client.BlockByHash(context.Background(), header.Hash())
			if err != nil {
				log.Fatal(err)
			}

			blockInfo := NewBlockInfo()

			blockInfo.Hash = block.Hash().Hex()            // 0xbc10defa8dda384c96a17640d84de5578804945d347072e091b4e5f390ddea7f
			blockInfo.Number = block.Number().Uint64()     // 3477413
			blockInfo.Time = block.Time()                  // 1529525947
			blockInfo.Nonce = block.Nonce()                // 130524141876765836
			blockInfo.TxNumber = len(block.Transactions()) // 7
		}
	}
}

// SubscribeToEthereumContract sets the ethereum contract to which oracle should listen
// and listens for events on this contract
func (o *Oracle) SubscribeToEthereumContract(address string) {
	// create ws client
	client, err := ethclient.Dial(o.ethereumConnection.Node.WSEndpoint())
	if err != nil {
		log.Fatal(err)
	}

	contractAddress := common.HexToAddress(address)
	query := eth.FilterQuery{
		Addresses: []common.Address{contractAddress},
	}

	logs := make(chan types.Log)
	sub, err := client.SubscribeFilterLogs(context.Background(), query, logs)
	if err != nil {
		log.Fatal(err)
	}

	for {
		select {
		case err := <-sub.Err():
			log.Fatal(err)
		case vLog := <-logs:
			fmt.Println(vLog) // pointer to event log
		}
	}
}

func (o *Oracle) ReadEthereumEventLogs(from, to uint, contractAddr string) {

}

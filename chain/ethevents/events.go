package ethevents

import (
	"context"
	"fmt"
	"math/big"
	"strings"
	"time"

	eth "github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	ethtypes "github.com/ethereum/go-ethereum/core/types"
	cttypes "github.com/tendermint/tendermint/rpc/core/types"
	ttypes "github.com/tendermint/tendermint/types"
	"gitlab.com/vocdoni/go-dvote/census"
	"gitlab.com/vocdoni/go-dvote/chain/contracts"
	"gitlab.com/vocdoni/go-dvote/data"
	"gitlab.com/vocdoni/go-dvote/types"
	"gitlab.com/vocdoni/go-dvote/vochain"

	"github.com/ethereum/go-ethereum/ethclient"
	"gitlab.com/vocdoni/go-dvote/chain"
	"gitlab.com/vocdoni/go-dvote/log"
)

// EthereumEvents type is used to monitorize an Ethereum smart contract and call custom EventHandler functions
type EthereumEvents struct {
	// voting process contract address
	ContractAddress common.Address
	// voting process contract abi
	ContractABI abi.ABI
	// voting process contract handle
	ProcessHandle *chain.ProcessHandle
	// dial web3 address
	DialAddr string
	// list of handler functions that will be called on events
	EventHandlers []EventHandler
	// ethereum subscribed events
	Signer Signer
	// VochainApp is a pointer to the Vochain BaseApplication
	// allowing to call SendTx method
	VochainApp *vochain.BaseApplication

	// Census is the census manager service
	Census CensusManager
	// EventProcessor handles events pending to process
	EventProcessor *EventProcessor
}

type Signer interface {
	// TODO(mvdan): expose Sign, not SignJSON
	// TODO(mvdan): the result should be []byte, not string
	SignJSON(message interface{}) (string, error)

	fmt.Stringer
}

type VochainClient interface {
	// TODO(mvdan): do we want a more generic API?
	BroadcastTxSync(tx ttypes.Tx) (*cttypes.ResultBroadcastTx, error)
}

type CensusManager interface {
	AddToImportQueue(censusID, censusURI string)
	// TODO(mvdan): is this too wide? maybe just URIprefix?
	Data() data.Storage
}

// EventHandler function type is executed on each Ethereum event
type EventHandler func(event ethtypes.Log, ethEvents *EthereumEvents) error

// BlockInfo represents the basic Ethereum block information
type BlockInfo struct {
	Hash     string
	Number   uint64
	Time     uint64
	Nonce    uint64
	TxNumber int
}

type EventProcessor struct {
	CurrentBlock int64
	Events       chan ethtypes.Log
}

// NewEthEvents creates a new Ethereum events handler
func NewEthEvents(contractAddressHex string, signer Signer, w3Endpoint string, cens *census.Manager, vocapp *vochain.BaseApplication) (*EthereumEvents, error) {
	// try to connect to default addr if w3Endpoint is empty
	if len(w3Endpoint) == 0 {
		w3Endpoint = "ws://127.0.0.1:9091"
	}
	contractAddr := common.HexToAddress(contractAddressHex)
	ph, err := chain.NewVotingProcessHandle(contractAddressHex, w3Endpoint)
	if err != nil {
		return nil, err
	}
	contractABI, err := abi.JSON(strings.NewReader(contracts.VotingProcessABI))
	if err != nil {
		log.Fatal(err)
	}

	return &EthereumEvents{
		ContractAddress: contractAddr,
		ContractABI:     contractABI,
		ProcessHandle:   ph,
		Signer:          signer,
		DialAddr:        w3Endpoint,
		Census:          cens,
		VochainApp:      vocapp,
		EventProcessor: &EventProcessor{
			Events: make(chan ethtypes.Log),
		},
	}, nil
}

// AddEventHandler adds a new handler even log function
func (ev *EthereumEvents) AddEventHandler(h EventHandler) {
	ev.EventHandlers = append(ev.EventHandlers, h)
}

// EthereumBlockListener returns a block info when a new block is created on Ethereum
func (ev *EthereumEvents) EthereumBlockListener() BlockInfo {
	client, err := ethclient.Dial(ev.DialAddr)
	if err != nil {
		log.Fatal(err)
	}
	headers := make(chan *ethtypes.Header)
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
			blockInfo := &BlockInfo{}
			blockInfo.Hash = block.Hash().Hex()            // 0xbc10defa8dda384c96a17640d84de5578804945d347072e091b4e5f390ddea7f
			blockInfo.Number = block.Number().Uint64()     // 3477413
			blockInfo.Time = block.Time()                  // 1529525947
			blockInfo.Nonce = block.Nonce()                // 130524141876765836
			blockInfo.TxNumber = len(block.Transactions()) // 7
		}
	}
}

// SubscribeEthereumEventLogs subscribe to the oracle
// defined smart contract via websocket. Blocking function (use go routine)
func (ev *EthereumEvents) SubscribeEthereumEventLogs() {
	client, err := ethclient.Dial(ev.DialAddr)
	if err != nil {
		log.Fatal(err)
	}

	query := eth.FilterQuery{
		Addresses: []common.Address{ev.ContractAddress},
	}

	logs := make(chan ethtypes.Log)
	sub, err := client.SubscribeFilterLogs(context.Background(), query, logs)
	if err != nil {
		log.Fatal(err)
	}

	go ev.runEventProcessor(types.EthereumConfirmationsThreshold, client)

	for {
		select {
		case err := <-sub.Err():
			log.Fatal(err)
		case event := <-logs:
			ev.EventProcessor.Events <- event
		}
	}
}

// ReadEthereumEventLogs reads the oracle
// defined smart contract and looks for events.
func (ev *EthereumEvents) ReadEthereumEventLogs(from, to int64) error {
	log.Infof("reading ethereum events from block %d to %d", from, to)
	client, err := ethclient.Dial(ev.DialAddr)
	if err != nil {
		log.Fatal(err)
	}

	query := eth.FilterQuery{
		FromBlock: big.NewInt(from),
		ToBlock:   big.NewInt(to),
		Addresses: []common.Address{
			ev.ContractAddress,
		},
	}

	logs, err := client.FilterLogs(context.Background(), query)
	if err != nil {
		return err
	}

	for _, event := range logs {
		for _, h := range ev.EventHandlers {
			if err := h(event, ev); err != nil {
				log.Warn(err)
			}
		}
	}
	return nil
}

func (ev *EthereumEvents) runEventProcessor(threshold int64, client *ethclient.Client) {
	for {
		evt := <-ev.EventProcessor.Events
		for {
			if (ev.EventProcessor.CurrentBlock - int64(evt.BlockNumber)) >= threshold {
				for _, h := range ev.EventHandlers {
					if err := h(evt, ev); err != nil {
						log.Error(err)
					}
				}
				break
			}
			time.Sleep(time.Second * 15)
			header, err := client.HeaderByNumber(context.Background(), nil)
			if err != nil {
				log.Warnf("Cannot sync event processor block")
			}
			ev.EventProcessor.CurrentBlock = header.Number.Int64()
		}
	}
}

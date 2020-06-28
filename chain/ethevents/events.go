package ethevents

import (
	"context"
	"fmt"
	"math/big"
	"strings"
	"sync"
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
	// VochainApp is a pointer to the Vochain BaseApplication allowing to call SendTx method
	VochainApp *vochain.BaseApplication
	// Census is the census manager service
	Census CensusManager
	// EventProcessor handles events pending to process
	EventProcessor *EventProcessor
}

type logEvent struct {
	event *ethtypes.Log
	added time.Time
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
type EventHandler func(event *ethtypes.Log, ethEvents *EthereumEvents) error

// EventProcessor is in charge of processing Ethereum event logs asynchronously.
// Uses a Queue mechanism and waits for EventProcessThreshold before processing a queued event.
// If during this time window the Ethereum block is reversed, the event will be deleted.
type EventProcessor struct {
	Events                chan ethtypes.Log
	EventProcessThreshold time.Duration
	eventProcessorRunning bool
	eventQueue            map[string]*logEvent
	eventQueueLock        sync.RWMutex
}

// NewEthEvents creates a new Ethereum events handler
func NewEthEvents(contractAddressHex string, signer Signer, w3Endpoint string, cens *census.Manager, vocapp *vochain.BaseApplication) (*EthereumEvents, error) {
	// try to connect to default addr if w3Endpoint is empty
	if len(w3Endpoint) == 0 {
		return nil, fmt.Errorf("no w3Endpoint specified on Ethereum Events")
	}
	contractAddr := common.HexToAddress(contractAddressHex)
	ph, err := chain.NewVotingProcessHandle(contractAddressHex, w3Endpoint)
	if err != nil {
		return nil, err
	}
	contractABI, err := abi.JSON(strings.NewReader(contracts.VotingProcessABI))
	if err != nil {
		return nil, err
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
			Events:                make(chan ethtypes.Log),
			EventProcessThreshold: time.Duration(60 * time.Second),
			eventQueue:            make(map[string]*logEvent),
			eventProcessorRunning: false,
		},
	}, nil
}

// AddEventHandler adds a new handler even log function
func (ev *EthereumEvents) AddEventHandler(h EventHandler) {
	ev.EventHandlers = append(ev.EventHandlers, h)
}

// SubscribeEthereumEventLogs enables the subscription of Ethereum events for new blocks.
// Events are Queued for 60 seconds before processed in order to avoid possible blockchain reversions.
// If fromBlock nil, subscription will start on current block
// Blocking function (use go routine).
func (ev *EthereumEvents) SubscribeEthereumEventLogs(fromBlock *int64) {
	log.Debugf("dialing for %s", ev.DialAddr)
	ctx := context.Background()
	client, err := ethclient.Dial(ev.DialAddr)
	if err != nil {
		log.Fatal(err)
	}

	// Get current block
	blk, err := client.BlockByNumber(ctx, nil)
	if err != nil {
		log.Fatal(err)
	}

	// If fromBlock not nil, process past events
	if fromBlock != nil {
		startBlock := blk.Number().Int64()
		ev.processEventLogsFromTo(ctx, *fromBlock, startBlock, client)
		// Update block number
		if blk, err = client.BlockByNumber(ctx, nil); err != nil {
			log.Fatal(err)
		}
		// For security, read also the new passed blocks before subscribing
		ev.processEventLogsFromTo(ctx, startBlock, blk.Number().Int64(), client)
	} else {
		// For security, even if subscribe only, force to process at least the past 1024
		ev.processEventLogsFromTo(ctx, blk.Number().Int64()-1024, blk.Number().Int64(), client)
	}

	// And then subscribe to new events
	log.Infof("subscribing to Ethereum Events from block %d", blk.Number().Int64())
	query := eth.FilterQuery{
		Addresses: []common.Address{ev.ContractAddress},
		FromBlock: blk.Number(), // not sure it works
	}

	logs := make(chan ethtypes.Log, 10) // give it some buffer as recommended by the package library
	sub, err := client.SubscribeFilterLogs(ctx, query, logs)
	if err != nil {
		log.Fatal(err)
	}

	if !ev.EventProcessor.eventProcessorRunning {
		go ev.runEventProcessor()
	}

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
func (ev *EthereumEvents) processEventLogsFromTo(ctx context.Context, from, to int64, client *ethclient.Client) error {
	log.Infof("reading ethereum events from block %d to %d", from, to)
	query := eth.FilterQuery{
		FromBlock: big.NewInt(from),
		ToBlock:   big.NewInt(to),
		Addresses: []common.Address{
			ev.ContractAddress,
		},
	}

	logs, err := client.FilterLogs(ctx, query)
	if err != nil {
		return err
	}

	for _, event := range logs {
		log.Infof("processing event log from block %d", event.BlockNumber)
		for _, h := range ev.EventHandlers {
			if err := h(&event, ev); err != nil {
				log.Warn(err)
			}
		}
	}
	return nil
}

func (ep *EventProcessor) add(e *ethtypes.Log) {
	ep.eventQueueLock.Lock()
	defer ep.eventQueueLock.Unlock()
	eventID := fmt.Sprintf("%x%d", e.TxHash, e.TxIndex)
	ep.eventQueue[eventID] = &logEvent{event: e, added: time.Now()}
}

func (ep *EventProcessor) del(e *ethtypes.Log) {
	ep.eventQueueLock.Lock()
	defer ep.eventQueueLock.Unlock()
	eventID := fmt.Sprintf("%x%d", e.TxHash, e.TxIndex)
	delete(ep.eventQueue, eventID)
}

// next returns the frist log event rady to be processed
func (ep *EventProcessor) next() *ethtypes.Log {
	ep.eventQueueLock.Lock()
	defer ep.eventQueueLock.Unlock()
	for id, el := range ep.eventQueue {
		if time.Since(el.added) >= ep.EventProcessThreshold {
			delete(ep.eventQueue, id)
			return el.event
		}
	}
	return nil
}

func (ev *EthereumEvents) runEventProcessor() {
	ev.EventProcessor.eventProcessorRunning = true
	go func() {
		var evt ethtypes.Log
		var evtJSON []byte
		var err error
		for {
			evt = <-ev.EventProcessor.Events
			if evtJSON, err = evt.MarshalJSON(); err != nil {
				log.Error(err)
				continue
			}
			if evt.Removed {
				log.Warnf("removing reversed log event: %s", evtJSON)
				ev.EventProcessor.del(&evt)
			} else {
				log.Debugf("queued event log: %s", evtJSON)
				ev.EventProcessor.add(&evt)
			}
		}
	}()

	for {
		time.Sleep(1 * time.Second)
		if e := ev.EventProcessor.next(); e != nil {
			log.Infof("processing event log: (txhash:%x txid:%d)", e.TxHash, e.TxIndex)
			for _, h := range ev.EventHandlers {
				if err := h(e, ev); err != nil {
					log.Error(err)
				}
			}

		}
	}

}

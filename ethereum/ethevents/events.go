package ethevents

import (
	"bytes"
	"context"
	"fmt"
	"math/big"
	"sync"
	"sync/atomic"
	"time"

	eth "github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/common"
	ethtypes "github.com/ethereum/go-ethereum/core/types"
	cttypes "github.com/tendermint/tendermint/rpc/core/types"
	ttypes "github.com/tendermint/tendermint/types"
	"go.vocdoni.io/dvote/census"
	"go.vocdoni.io/dvote/crypto/ethereum"
	"go.vocdoni.io/dvote/data"
	"go.vocdoni.io/dvote/types"
	"go.vocdoni.io/dvote/vochain"
	"go.vocdoni.io/dvote/vochain/scrutinizer"
	"go.vocdoni.io/proto/build/go/models"

	"github.com/ethereum/go-ethereum/ethclient"
	ethereumhandler "go.vocdoni.io/dvote/ethereum/handler"
	"go.vocdoni.io/dvote/log"
)

const (
	readBlocksPast = 100
)

var blockConfirmThreshold = map[models.SourceNetworkId]time.Duration{
	models.SourceNetworkId_UNKNOWN:     time.Second * 30,
	models.SourceNetworkId_POA_XDAI:    time.Second * 20,
	models.SourceNetworkId_ETH_MAINNET: time.Second * 60,
}

// EthereumEvents type is used to monitorize Ethereum smart
// contracts and call custom EventHandler functions
type EthereumEvents struct {
	// contracts handle
	VotingHandle *ethereumhandler.EthereumHandler
	// dial web3 addresses
	DialAddrs []string
	// list of handler functions that will be called on events
	// TODO: add context on callbacks
	// TODO: return errors on callbacks
	EventHandlers []EventHandler
	// ethereum subscribed events
	Signer *ethereum.SignKeys
	// VochainApp is a pointer to the Vochain BaseApplication allowing to call SendTx method
	VochainApp *vochain.BaseApplication
	// Census is the census manager service
	Census CensusManager
	// EventProcessor handles events pending to process
	EventProcessor *EventProcessor
	// Scrutinizer
	Scrutinizer *scrutinizer.Scrutinizer
	// EthereumWhiteListAddrs
	EthereumWhiteListAddrs map[common.Address]bool
	// ContractsAddress
	ContractsAddress []common.Address
	// ContractsInfo holds useful info for working with the desired contracts
	ContractsInfo map[string]*ethereumhandler.EthereumContract
	// EthereumLastKnownBlock keeps track of the latest Ethereum known block
	EthereumLastKnownBlock atomic.Value
}

type timedEvent struct {
	event *ethtypes.Log
	added time.Time
}

// VochainClient is the interface that any vochain client should fullfy
type VochainClient interface {
	// TODO(mvdan): do we want a more generic API?
	BroadcastTxSync(tx ttypes.Tx) (*cttypes.ResultBroadcastTx, error)
}

// CensusManager is the interface that any census manager should fullfy
type CensusManager interface {
	AddToImportQueue(censusID, censusURI string)
	// TODO(mvdan): is this too wide? maybe just URIprefix?
	Data() data.Storage
}

// EventHandler function type is executed on each Ethereum event
type EventHandler func(ctx context.Context, event *ethtypes.Log, ethEvents *EthereumEvents) error

// EventProcessor is in charge of processing Ethereum event logs asynchronously.
// Uses a Queue mechanism and waits for EventProcessThreshold before processing a queued event.
// If during this time window the Ethereum block is reversed, the event will be deleted.
type EventProcessor struct {
	Events                chan ethtypes.Log
	EventProcessThreshold time.Duration
	eventProcessorRunning bool
	eventQueue            map[string]timedEvent
	eventQueueLock        sync.RWMutex
}

// NewEthEvents creates a new Ethereum events handler
func NewEthEvents(
	contracts map[string]*ethereumhandler.EthereumContract,
	srcNetworkId models.SourceNetworkId,
	signer *ethereum.SignKeys,
	w3Endpoints []string,
	cens *census.Manager,
	vocapp *vochain.BaseApplication,
	ethereumWhiteList []string,
) (*EthereumEvents, error) {
	secureAddrList := make(map[common.Address]bool, len(ethereumWhiteList))
	if len(ethereumWhiteList) != 0 {
		for _, sAddr := range ethereumWhiteList {
			addr := common.HexToAddress(sAddr)
			if (addr == common.Address{} || len(addr) != types.EthereumAddressSize) {
				return nil, fmt.Errorf("cannot create ethereum whitelist, address %q not valid", sAddr)
			}
			secureAddrList[addr] = true
		}
	}
	contractsAddress := []common.Address{}
	for _, contract := range contracts {
		if !bytes.Equal(contract.Address.Bytes(), common.Address{}.Bytes()) {
			if contract.ListenForEvents {
				contractsAddress = append(contractsAddress, contract.Address)
			}
		}
	}
	confirmThreshold := blockConfirmThreshold[0]
	if _, ok := blockConfirmThreshold[srcNetworkId]; ok {
		confirmThreshold = blockConfirmThreshold[srcNetworkId]
	}
	log.Infof("chain %s found, block confirmation threshold set to %s",
		srcNetworkId, confirmThreshold)
	ethev := &EthereumEvents{
		Signer:     signer,
		DialAddrs:  w3Endpoints,
		Census:     cens,
		VochainApp: vocapp,
		EventProcessor: &EventProcessor{
			Events:                make(chan ethtypes.Log),
			EventProcessThreshold: confirmThreshold,
			eventQueue:            make(map[string]timedEvent),
		},
		EthereumWhiteListAddrs: secureAddrList,
		ContractsAddress:       contractsAddress,
		ContractsInfo:          contracts,
	}

	return ethev, nil
}

// AddEventHandler adds a new handler even log function
func (ev *EthereumEvents) AddEventHandler(handler EventHandler) {
	ev.EventHandlers = append(ev.EventHandlers, handler)
}

// SubscribeEthereumEventLogs enables the subscription of Ethereum events for new blocks.
// Events are Queued for 60 seconds before processed in order to avoid possible blockchain
// reversions.
// Blocking function (use go routine).
func (ev *EthereumEvents) SubscribeEthereumEventLogs(ctx context.Context) {

	// Get current block
	tctx, cancel := context.WithTimeout(ctx, types.EthereumReadTimeout)
	defer cancel()
	var err error
	uLastBlockNumber, err := ev.VotingHandle.EthereumClient.BlockNumber(tctx)
	if err != nil {
		log.Fatalf("cannot get last block number: (%v)", err)
	}
	lastBlockNumber := int64(uLastBlockNumber)
	fromBlock := ev.EthereumLastKnownBlock.Load().(int64)
	if fromBlock == 0 {
		ev.EthereumLastKnownBlock.Store(lastBlockNumber)
		fromBlock = lastBlockNumber
	}

	// For security, even if subscribe only, force to process
	// from some past blocks defined by `readBlocksPast`
	if err := ev.processEventLogsFromTo(ctx, fromBlock-readBlocksPast,
		lastBlockNumber, ev.VotingHandle.EthereumClient); err != nil {
		log.Errorf("cannot process event logs: %v", err)
	}

	// Subscribing from latest known block
	log.Infof("subscribing to Ethereum Events from block %d", fromBlock)
	query := eth.FilterQuery{
		Addresses: ev.ContractsAddress,
		FromBlock: big.NewInt(fromBlock),
	}
	logs := make(chan ethtypes.Log, 30) // give it some buffer as recommended by the package library
	var sub eth.Subscription
	sub, err = ev.VotingHandle.EthereumClient.SubscribeFilterLogs(ctx, query, logs)
	if err != nil {
		log.Fatalf("cannot subscribe to ethereum events: (%v)", err)
	}

	// Run event processor
	if !ev.EventProcessor.eventProcessorRunning {
		go ev.runEventProcessor(ctx)
	}

	// Monitorize ethereum client connection
	connFailure := make(chan bool, 1)
	go func(chan bool) {
		for {
			time.Sleep(time.Second * 5)
			tctx, cancel := context.WithTimeout(ctx, types.EthereumReadTimeout)
			defer cancel()
			block, err := ev.VotingHandle.EthereumClient.BlockNumber(tctx)
			if err != nil {
				log.Warnf("cannot check ethereum connection while running the event processor, %s", err)
				connFailure <- false
				return
			}
			ev.EthereumLastKnownBlock.Store(int64(block))
		}
	}(connFailure)

	// Block forever and start processing events
	for {
		select {
		case <-sub.Err():
			ev.EventProcessor.eventProcessorRunning = false
			ev.VotingHandle = nil
			log.Warn("ethereum events subscription error on channel")
			log.Infof("restarting ethereum events subscription with web3: %+v", ev.DialAddrs)
			return
		case event := <-logs:
			ev.EventProcessor.Events <- event
		case <-connFailure:
			ev.EventProcessor.eventProcessorRunning = false
			ev.VotingHandle = nil
			log.Warn("ethereum events subscription error on channel")
			log.Infof("restarting ethereum events subscription with web3: %+v", ev.DialAddrs)
			return
		}
	}
}

// ReadEthereumEventLogs reads the oracle
// defined smart contract and looks for events.
func (ev *EthereumEvents) processEventLogsFromTo(ctx context.Context,
	from, to int64, client *ethclient.Client) error {
	for name, contract := range ev.ContractsInfo {
		if !contract.ListenForEvents {
			continue
		}
		ev.ContractsAddress = append(ev.ContractsAddress, contract.Address)
		log.Infof("subscribing to contract: %s at address %s", name, contract.Address)
	}
	log.Infof("reading ethereum events from block %d to %d", from, to)
	query := eth.FilterQuery{
		FromBlock: big.NewInt(from),
		ToBlock:   big.NewInt(to),
		Addresses: ev.ContractsAddress,
	}

	logs, err := client.FilterLogs(ctx, query)
	if err != nil {
		return fmt.Errorf("cannot execute ethereum logs filter query: %w", err)
	}

	for _, event := range logs {
		log.Infof("processing event log from block %d", event.BlockNumber)
		for _, handler := range ev.EventHandlers {

			// Use a pointer to a copy of the event for each handler.
			// Just in case the handler runs asynchronously,
			// or for some reason ends up modifying the event.
			event := event

			if err := handler(ctx, &event, ev); err != nil {
				// TODO: handle when event cannot be processed
				log.Warnf("cannot handle event (%+v) with error (%s)", event, err)
			}
		}
	}
	return nil
}

func (ep *EventProcessor) id(event *ethtypes.Log) string {
	return fmt.Sprintf("%x%d", event.TxHash, event.TxIndex)
}

func (ep *EventProcessor) add(event *ethtypes.Log) {
	eventID := ep.id(event)
	ep.eventQueueLock.Lock()
	defer ep.eventQueueLock.Unlock()
	ep.eventQueue[eventID] = timedEvent{event: event, added: time.Now()}
}

func (ep *EventProcessor) del(event *ethtypes.Log) {
	eventID := ep.id(event)
	ep.eventQueueLock.Lock()
	defer ep.eventQueueLock.Unlock()
	delete(ep.eventQueue, eventID)
}

// next returns the first log event ready to be processed
func (ep *EventProcessor) next() *ethtypes.Log {
	ep.eventQueueLock.Lock()
	defer ep.eventQueueLock.Unlock()
	for id, event := range ep.eventQueue {
		if time.Since(event.added) >= ep.EventProcessThreshold {
			delete(ep.eventQueue, id)
			return event.event
		}
	}
	return nil
}

func (ev *EthereumEvents) runEventProcessor(ctx context.Context) {
	ev.EventProcessor.eventProcessorRunning = true
	log.Info("starting event processor routine")
	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			case event := <-ev.EventProcessor.Events:
				evtJSON, err := event.MarshalJSON()
				if err != nil {
					log.Error(err)
					continue
				}
				if event.Removed {
					log.Warnf("removing reversed log event: %s", evtJSON)
					ev.EventProcessor.del(&event)
				} else {
					if event.BlockNumber == 0 {
						log.Fatalf("unexpected event block number 0", event)
					}
					log.Debugf("queued event log: %s", evtJSON)
					ev.EventProcessor.add(&event)
				}
			}
		}
	}()

	for {
		// TODO(mvdan): Perhaps refactor this so we don't need a sleep.
		// Maybe use a queue-like structure sorted by added time,
		// and separately mark the removed events to ignore them.
		select {
		case <-ctx.Done():
			return
		case <-time.After(2 * time.Second):
		}
		if tev := ev.EventProcessor.next(); tev != nil {
			for i, handler := range ev.EventHandlers {
				log.Infof("executing event handler %d for %s", i, ev.EventProcessor.id(tev))
				if err := handler(ctx, tev, ev); err != nil {
					log.Error(err)
				}
			}
		}
	}
}

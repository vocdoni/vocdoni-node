package ethevents

import (
	"bytes"
	"context"
	"fmt"
	"math/big"
	"sync"
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

// EthereumEvents type is used to monitorize Ethereum smart
// contracts and call custom EventHandler functions
type EthereumEvents struct {
	// contracts handle
	VotingHandle *ethereumhandler.EthereumHandler
	// dial web3 address
	DialAddr string
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
}

type logEvent struct {
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
	eventQueue            map[string]*logEvent
	eventQueueLock        sync.RWMutex
}

// NewEthEvents creates a new Ethereum events handler
func NewEthEvents(
	contracts map[string]*ethereumhandler.EthereumContract,
	srcNetworkId models.SourceNetworkId,
	signer *ethereum.SignKeys,
	w3Endpoint string,
	cens *census.Manager,
	vocapp *vochain.BaseApplication,
	ethereumWhiteList []string,
) (*EthereumEvents, error) {
	// try to connect to default addr if w3Endpoint is empty
	if len(w3Endpoint) == 0 {
		return nil, fmt.Errorf("no w3Endpoint specified on Ethereum Events")
	}
	ph, err := ethereumhandler.NewEthereumHandler(contracts, srcNetworkId, w3Endpoint)
	if err != nil {
		return nil, fmt.Errorf("cannot create voting handle: %w", err)
	}
	ph.WaitSync()
	go ph.PrintInfo(context.Background(), time.Second*60)
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
			contractsAddress = append(contractsAddress, contract.Address)
		}
	}

	ethev := &EthereumEvents{
		VotingHandle: ph,
		Signer:       signer,
		DialAddr:     w3Endpoint,
		Census:       cens,
		VochainApp:   vocapp,
		EventProcessor: &EventProcessor{
			Events:                make(chan ethtypes.Log),
			EventProcessThreshold: 60 * time.Second,
			eventQueue:            make(map[string]*logEvent),
		},
		EthereumWhiteListAddrs: secureAddrList,
		ContractsAddress:       contractsAddress,
		ContractsInfo:          contracts,
	}

	return ethev, nil
}

// AddEventHandler adds a new handler even log function
func (ev *EthereumEvents) AddEventHandler(h EventHandler) {
	ev.EventHandlers = append(ev.EventHandlers, h)
}

// SubscribeEthereumEventLogs enables the subscription of Ethereum events for new blocks.
// Events are Queued for 60 seconds before processed in order to avoid possible blockchain
// reversions. If fromBlock nil, subscription will start on current block.
// Blocking function (use go routine).
func (ev *EthereumEvents) SubscribeEthereumEventLogs(ctx context.Context, fromBlock *int64) {
	var sub eth.Subscription
	var err error

	// Get current block
	blockTctx, cancel := context.WithTimeout(ctx, types.EthereumReadTimeout*2)
	defer cancel()
	var lastBlock int64
	var blk *ethtypes.Block
	errors := 0
	for {
		if errors > 5 {
			log.Fatal("the web3 client connection is not working")
		}
		blk, err = ev.VotingHandle.EthereumClient.BlockByNumber(blockTctx, nil)
		if err != nil {
			log.Errorf("cannot get ethereum block: %s", err)
			errors++
			time.Sleep(time.Second * 2)
			if err := ev.VotingHandle.Connect(ev.DialAddr); err != nil {
				log.Warn(err)
			}
			continue
		}
		errors = 0
		lastBlock = blk.Number().Int64()
		break
	}

	// If fromBlock not nil, process past events
	if fromBlock != nil {
		// do not retry if error processing old logs
		// Unless this is the first set of oracles, it is almost
		// sure that the events are already processed so do not
		// block and do not fatal here
		if err := ev.processEventLogsFromTo(ctx, *fromBlock, lastBlock,
			ev.VotingHandle.EthereumClient); err != nil {
			log.Errorf("cannot process event logs: %v", err)
		}
		// Update block number
		// Expect the client to be connected here
		// and the call is successful, if not
		// block the execution and alert
		errors = 0
		for {
			if errors > 5 {
				log.Fatal("the web3 connection is not working")
			}
			blk, err = ev.VotingHandle.EthereumClient.BlockByNumber(blockTctx, nil)
			if err != nil {
				log.Errorf("cannot update block number: %v", err)
				errors++
				time.Sleep(time.Second * 2)
				// If any error try close the client and reconnect
				if err := ev.VotingHandle.Connect(ev.DialAddr); err != nil {
					log.Warn(err)
				}
				continue
			}
			lastBlock = blk.Number().Int64()
			break
		}
		// For security, read also the new passed blocks before subscribing
		// Same as the last processEventLogsFromTo function call
		if err := ev.processEventLogsFromTo(ctx, lastBlock, blk.Number().Int64(),
			ev.VotingHandle.EthereumClient); err != nil {
			log.Errorf("cannot process event logs: %s", err)
		}
	} else {
		// For security, even if subscribe only, force to process at least the past 1024
		// Same as the last processEventLogsFromTo function call
		if err := ev.processEventLogsFromTo(ctx, blk.Number().Int64()-1024,
			blk.Number().Int64(), ev.VotingHandle.EthereumClient); err != nil {
			log.Errorf("cannot process event logs: %v", err)
		}
	}

	blockTctx, cancel = context.WithTimeout(ctx, types.EthereumReadTimeout*2)
	defer cancel()
	// And then subscribe to new events
	// Update block number
	blk, err = ev.VotingHandle.EthereumClient.BlockByNumber(blockTctx, nil)
	if err != nil {
		// accept to not update the block number here
		// if any error, the old blk fetched will be used instead.
		log.Errorf("cannot upate block number: %s", err)
	}
	log.Infof("subscribing to Ethereum Events from block %d", blk.Number().Int64())
	query := eth.FilterQuery{
		Addresses: ev.ContractsAddress,
		FromBlock: blk.Number(),
	}

	logs := make(chan ethtypes.Log, 10) // give it some buffer as recommended by the package library
	// block here, since it is not acceptable to
	// start the event processor if the client
	// cannot be subscribed to the logs.
	// Use the same policy as processEventsLogsFromTo
	errors = 0
	for {
		if errors > 5 {
			log.Fatal("the web3 connection is not working")
		}
		sub, err = ev.VotingHandle.EthereumClient.SubscribeFilterLogs(ctx, query, logs)
		if err != nil {
			log.Errorf("cannot subscribe to ethereum client log: %s", err)
			errors++
			time.Sleep(time.Second * 2)
			if err := ev.VotingHandle.Connect(ev.DialAddr); err != nil {
				log.Warn(err)
			}
			continue
		}
		break
	}

	if !ev.EventProcessor.eventProcessorRunning {
		go ev.runEventProcessor(ctx)
	}

	for {
		select {
		case err := <-sub.Err():
			// TODO: @jordipainan do not fatal here, handle error on event subscription channel
			log.Fatalf("ethereum events subscription error on channel: %s", err)
		case event := <-logs:
			ev.EventProcessor.Events <- event
		}
	}
}

// ReadEthereumEventLogs reads the oracle
// defined smart contract and looks for events.
func (ev *EthereumEvents) processEventLogsFromTo(ctx context.Context,
	from, to int64, client *ethclient.Client) error {
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
		for _, h := range ev.EventHandlers {
			if err := h(ctx, &event, ev); err != nil {
				// TODO: handle when event cannot be processed
				log.Warnf("cannot handle event (%+v) with error (%s)", event, err)
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

// next returns the first log event rady to be processed
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

func (ev *EthereumEvents) runEventProcessor(ctx context.Context) {
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
				if err := h(ctx, e, ev); err != nil {
					log.Error(err)
				}
			}

		}
	}
}

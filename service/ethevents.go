package service

import (
	"bytes"
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"go.vocdoni.io/dvote/crypto/ethereum"
	chain "go.vocdoni.io/dvote/ethereum"
	"go.vocdoni.io/dvote/ethereum/ethevents"
	ethereumhandler "go.vocdoni.io/dvote/ethereum/handler"
	"go.vocdoni.io/dvote/log"
	"go.vocdoni.io/dvote/vochain"
)

// EthEvents service registers on the Ethereum smart contract specified in
// ethProcDomain, the provided event handlers.
// w3host and w3port must point to a working web3 websocket endpoint.
// If endBlock=0 is enabled the service will only subscribe for new blocks
func EthEvents(
	ctx context.Context,
	w3uris []string,
	networkName string,
	signer *ethereum.SignKeys,
	vocapp *vochain.BaseApplication,
	evh []ethevents.EventHandler,
	ethereumWhiteList []string,
) error {
	log.Infof("creating ethereum events service")
	specs, err := chain.SpecsFor(networkName)
	if err != nil {
		return fmt.Errorf("cannot get specs for the selected network: %w", err)
	}

	ev, err := ethevents.NewEthEvents(
		specs.Contracts,
		specs.NetworkSource,
		signer,
		vocapp,
		ethereumWhiteList,
	)
	if err != nil {
		return fmt.Errorf("couldn't create ethereum events listener: %w", err)
	}

	// Save as last known block the starting block for the selected chain.
	// Events will start to be monitorized from this block.
	ev.EthereumLastKnownBlock = uint64(specs.StartingBlock)

	for _, e := range evh {
		ev.AddEventHandler(e)
	}

	// The web3 queue manages the list of web3 endpoints
	w3q := new(w3queue)
	w3q.SetEndpoints(w3uris)

	// This goroutine sets a context with cancel to free all the resources
	// used by the subscription on the ethereum logs in case the mentioned
	// subscription breaks (e.g connection is dropped).
	// Once all the resources are freed the Ethereum handler is initialized
	// again as well as the subscription mechanism
	go func() {
		for {
			if ctx.Err() != nil {
				return
			}
			if ev.VotingHandle, err = ethereumhandler.NewEthereumHandler(
				ev.ContractsInfo,
				specs.NetworkSource,
				w3q.Get(),
			); err != nil {
				log.Warnf("cannot create ethereum handler: %v", err)
				w3q.Next()
				time.Sleep(time.Second * 2)
				continue
			}
			ctx, cancel := context.WithCancel(ctx)
			go ev.VotingHandle.PrintInfo(ctx, time.Second*20)
			ev.SubscribeEthereumEventLogs(ctx)
			// stop all child goroutines if error on subscription
			cancel()
			w3q.Next()
		}
	}()

	go func() {
		for {
			time.Sleep(time.Second * 300)
			log.Infof("web3 failures %s", w3q.Failures())
		}
	}()

	return nil
}

type w3queue struct {
	endpoints []string
	failures  []int
	index     int
	lock      sync.Mutex
}

func (w *w3queue) SetEndpoints(endpoints []string) {
	w.lock.Lock()
	defer w.lock.Unlock()
	w.endpoints = endpoints
	w.failures = make([]int, len(endpoints))
	w.index = 0
}

func (w *w3queue) Get() string {
	w.lock.Lock()
	defer w.lock.Unlock()
	return w.endpoints[w.index]
}

func (w *w3queue) Next() {
	w.lock.Lock()
	defer w.lock.Unlock()
	w.failures[w.index]++
	w.index = (w.index + 1) % len(w.endpoints)
}

func (w *w3queue) Failures() string {
	buf := bytes.Buffer{}
	w.lock.Lock()
	defer w.lock.Unlock()
	for i, e := range w.endpoints {
		e = strings.TrimPrefix(e, "https://")
		e = strings.TrimPrefix(e, "http://")
		e = strings.TrimPrefix(e, "wss://")
		e = strings.TrimPrefix(e, "ws://")
		if len(e) > 10 {
			e = e[:10]
		}
		buf.WriteString(fmt.Sprintf("[%s]:%d ", e, w.failures[i]))
	}
	return buf.String()
}

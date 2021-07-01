package service

import (
	"context"
	"fmt"
	"time"

	"go.vocdoni.io/dvote/census"
	"go.vocdoni.io/dvote/crypto/ethereum"
	chain "go.vocdoni.io/dvote/ethereum"
	"go.vocdoni.io/dvote/ethereum/ethevents"
	ethereumhandler "go.vocdoni.io/dvote/ethereum/handler"
	"go.vocdoni.io/dvote/log"
	"go.vocdoni.io/dvote/vochain"
	"go.vocdoni.io/dvote/vochain/scrutinizer"
)

// EthEvents service registers on the Ethereum smart contract specified in
// ethProcDomain, the provided event handlers.
// w3host and w3port must point to a working web3 websocket endpoint.
// If endBlock=0 is enabled the service will only subscribe for new blocks
func EthEvents(
	ctx context.Context,
	w3uris []string,
	networkName string,
	startBlock int64,
	cm *census.Manager,
	signer *ethereum.SignKeys,
	vocapp *vochain.BaseApplication,
	evh []ethevents.EventHandler,
	sc *scrutinizer.Scrutinizer,
	ethereumWhiteList []string,
) error {
	// TO-DO remove cm (add it on the eventHandler instead)
	log.Infof("creating ethereum events service")
	specs, err := chain.SpecsFor(networkName)
	if err != nil {
		return fmt.Errorf("cannot get specs for the selected network: %w", err)
	}

	ev, err := ethevents.NewEthEvents(
		specs.Contracts,
		specs.NetworkSource,
		signer,
		w3uris,
		cm,
		vocapp,
		ethereumWhiteList,
	)

	ev.EthereumLastKnownBlock.Store(startBlock)

	if err != nil {
		return fmt.Errorf("couldn't create ethereum events listener: %w", err)
	}
	for _, e := range evh {
		ev.AddEventHandler(e)
	}

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
			if ev.VotingHandle, err = ethereumhandler.NewEthereumHandler(ev.ContractsInfo, specs.NetworkSource, w3uris); err != nil {
				log.Fatalf("cannot restart ethereum events, cannot create ethereum handler: %v", err)
			}
			ctx, cancel := context.WithCancel(ctx)
			go ev.VotingHandle.PrintInfo(ctx, time.Second*20)
			ev.SubscribeEthereumEventLogs(ctx)
			// stop all child goroutines if error on subscription
			cancel()
		}
	}()

	return nil
}

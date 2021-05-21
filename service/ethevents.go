package service

import (
	"context"
	"fmt"

	"github.com/ethereum/go-ethereum/ethclient"
	"go.vocdoni.io/dvote/census"
	"go.vocdoni.io/dvote/crypto/ethereum"
	chain "go.vocdoni.io/dvote/ethereum"
	"go.vocdoni.io/dvote/ethereum/ethevents"
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
	w3uri string,
	networkName string,
	startBlock *int64,
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

	web3Client, err := ethclient.Dial(w3uri)
	if err != nil {
		return fmt.Errorf("cannot connect to the web3 endpoint: %w", err)
	}

	for name, contract := range specs.Contracts {
		if !contract.ListenForEvents {
			continue
		}
		if err := contract.InitContract(ctx, name, specs.Contracts["ensRegistry"].Address, web3Client); err != nil {
			return fmt.Errorf("cannot initialize contracts: %w", err)
		}
	}
	ev, err := ethevents.NewEthEvents(
		specs.Contracts,
		signer,
		w3uri,
		cm,
		vocapp,
		ethereumWhiteList,
	)
	if err != nil {
		return fmt.Errorf("couldn't create ethereum events listener: %w", err)
	}
	for _, e := range evh {
		ev.AddEventHandler(e)
	}
	go func() {
		ev.SubscribeEthereumEventLogs(ctx, startBlock)
	}()

	return nil
}

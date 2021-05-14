package service

import (
	"context"
	"fmt"

	"github.com/ethereum/go-ethereum/common"
	"go.vocdoni.io/dvote/census"
	"go.vocdoni.io/dvote/chain"
	"go.vocdoni.io/dvote/chain/ethereumhandler"
	"go.vocdoni.io/dvote/chain/ethevents"
	"go.vocdoni.io/dvote/crypto/ethereum"
	"go.vocdoni.io/dvote/log"
	"go.vocdoni.io/dvote/types"
	"go.vocdoni.io/dvote/vochain"
	"go.vocdoni.io/dvote/vochain/scrutinizer"
)

// EthEvents service registers on the Ethereum smart contract specified in ethProcDomain, the provided event handlers
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
	for k, contract := range specs.Contracts {
		if !contract.ListenForEvents {
			continue
		}
		var addr string
		for i := 0; i < types.EthereumDialMaxRetry; i++ {
			addr, err = ethereumhandler.EnsResolve(ctx, specs.ENSregistryAddr, contract.Domain, w3uri)
			if err != nil {
				log.Errorf("cannot resolve domain: %s, error: %s, trying again ...", contract.Domain, err)
				continue
			}
			break
		}
		if addr == "" {
			return fmt.Errorf("cannot resolve domain contract addresses")
		}
		contract.Address = common.HexToAddress(addr)
		if err := contract.SetABI(k); err != nil {
			return fmt.Errorf("couldn't set contract ABI: %w", err)
		}
		log.Infof("loaded contract %s at address: %s", contract.Domain, contract.Address)
	}

	ev, err := ethevents.NewEthEvents(specs.Contracts, signer, w3uri, cm, vocapp, sc, ethereumWhiteList)
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

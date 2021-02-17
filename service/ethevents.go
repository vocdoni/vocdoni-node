package service

import (
	"context"
	"fmt"

	"github.com/ethereum/go-ethereum/common"
	"go.vocdoni.io/dvote/census"
	"go.vocdoni.io/dvote/chain"
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
	contractsAddresses := make([]common.Address, len(specs.ENSdomains))
	for idx, domain := range specs.ENSdomains {
		var addr string
		for i := 0; i < types.EthereumDialMaxRetry; i++ {
			addr, err = chain.EnsResolve(ctx, specs.ENSregistryAddr, domain, w3uri)
			if err != nil {
				log.Errorf("cannot resolve domain: %s, error: %s, trying again ...", domain, err)
				continue
			}
			break
		}
		if addr == "" {
			return fmt.Errorf("cannot resolve domain address, cannot proceed without resolving contract addresses")
		}
		contractsAddresses[idx] = common.HexToAddress(addr)
	}
	log.Debugf("contracts addresses on eth events: %+v", contractsAddresses)

	ev, err := ethevents.NewEthEvents(contractsAddresses, signer, w3uri, cm, vocapp, sc, ethereumWhiteList)
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

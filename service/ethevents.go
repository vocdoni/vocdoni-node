package service

import (
	"context"
	"fmt"

	"github.com/ethereum/go-ethereum/common"
	"gitlab.com/vocdoni/go-dvote/census"
	"gitlab.com/vocdoni/go-dvote/chain"
	"gitlab.com/vocdoni/go-dvote/chain/ethevents"
	"gitlab.com/vocdoni/go-dvote/crypto/ethereum"
	"gitlab.com/vocdoni/go-dvote/log"
	"gitlab.com/vocdoni/go-dvote/vochain"
)

// EthEvents service registers on the Ethereum smart contract specified in ethProcDomain, the provided event handlers
// w3host and w3port must point to a working web3 websocket endpoint.
// If endBlock=0 is enabled the service will only subscribe for new blocks
func EthEvents(ctx context.Context, ensDomains []string, w3uri string, networkName string, startBlock *int64,
	cm *census.Manager, signer *ethereum.SignKeys, vocapp *vochain.BaseApplication, evh []ethevents.EventHandler) error {
	// TO-DO remove cm (add it on the eventHandler instead)
	log.Infof("creating ethereum events service")
	specs, err := chain.SpecsFor(networkName)
	if err != nil {
		return fmt.Errorf("cannot get specs for the selected network: %w", err)
	}
	contractsAddresses := make([]common.Address, len(ensDomains))
	for idx, domain := range ensDomains {
		addr, err := chain.EnsResolve(ctx, specs.ENSregistryAddr, domain, w3uri)
		if err != nil {
			return fmt.Errorf("cannot resolve domain: %s, error: %w", domain, err)
		}
		contractsAddresses[idx] = common.HexToAddress(addr)
	}
	log.Debugf("contracts addresses on eth events: %+v", contractsAddresses)

	ev, err := ethevents.NewEthEvents(contractsAddresses, signer, w3uri, cm, vocapp)
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

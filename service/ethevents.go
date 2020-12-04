package service

import (
	"context"
	"encoding/hex"
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
		return err
	}
	var contractsAddresses []common.Address = make([]common.Address, len(ensDomains))
	for _, domain := range ensDomains {
		addr, err := chain.EnsResolve(ctx, specs.ENSregistryAddr, domain, w3uri)
		if err != nil {
			return err
		}
		bAddr, err := hex.DecodeString(addr)
		if err != nil {
			return err
		}
		contractsAddresses = append(contractsAddresses, common.BytesToAddress(bAddr))
	}

	ev, err := ethevents.NewEthEvents(contractsAddresses, signer, w3uri, cm, vocapp)
	if err != nil {
		return fmt.Errorf("couldn't create ethereum events listener: (%s)", err)
	}
	for _, e := range evh {
		ev.AddEventHandler(e)
	}
	go func() {
		ev.SubscribeEthereumEventLogs(ctx, startBlock)
	}()

	return nil
}

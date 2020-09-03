package service

import (
	"fmt"

	"gitlab.com/vocdoni/go-dvote/census"
	"gitlab.com/vocdoni/go-dvote/chain"
	"gitlab.com/vocdoni/go-dvote/chain/ethevents"
	"gitlab.com/vocdoni/go-dvote/crypto/ethereum"
	"gitlab.com/vocdoni/go-dvote/log"
	"gitlab.com/vocdoni/go-dvote/vochain"
)

// EthEvents service registers on the Ethereum smart contract specified in ethProcDomain, the provided event handlers
// we3host and w3port must point to a working web3 websocket endpoint.
// If endBlock=0 is enabled the service will only subscribe for new blocks
func EthEvents(ethProcDomain, w3uri string, networkName string, startBlock *int64,
	cm *census.Manager, signer *ethereum.SignKeys, vocapp *vochain.BaseApplication, evh []ethevents.EventHandler) error {
	// TO-DO remove cm (add it on the eventHandler instead)
	log.Infof("creating ethereum events service")
	specs, err := chain.SpecsFor(networkName)
	if err != nil {
		return err
	}
	contractAddr, err := chain.EnsResolve(specs.ENSregistryAddr, ethProcDomain, w3uri)
	if err != nil {
		return err
	}
	ev, err := ethevents.NewEthEvents(contractAddr, signer, w3uri, cm, vocapp)
	if err != nil {
		return fmt.Errorf("couldn't create ethereum events listener: (%s)", err)
	}
	for _, e := range evh {
		ev.AddEventHandler(e)
	}
	go func() {
		ev.SubscribeEthereumEventLogs(startBlock)
	}()

	return nil
}

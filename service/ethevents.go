package service

import (
	"fmt"
	"strings"
	"time"

	"gitlab.com/vocdoni/go-dvote/census"
	"gitlab.com/vocdoni/go-dvote/chain"
	"gitlab.com/vocdoni/go-dvote/chain/ethevents"
	"gitlab.com/vocdoni/go-dvote/crypto/ethereum"
	"gitlab.com/vocdoni/go-dvote/log"
	"gitlab.com/vocdoni/go-dvote/vochain"
)

const ensRegistryAddr = "0x00000000000C2E074eC69A0dFb2997BA6C7d2e1e"
const maxRetries = 30

// EthEvents service registers on the Ethereum smart contract specified in ethProcDomain, the provided event handlers
// we3host and w3port must point to a working web3 websocket endpoint.
// If endBlock=0 is enabled the service will only subscribe for new blocks
func EthEvents(ethProcDomain, w3uri string, startBlock *int64,
	cm *census.Manager, signer *ethereum.SignKeys, vocapp *vochain.BaseApplication, evh []ethevents.EventHandler) error {
	// TO-DO remove cm (add it on the eventHandler instead)
	log.Infof("creating ethereum events service")
	contractAddr, err := ensResolve(ensRegistryAddr, ethProcDomain, w3uri)
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

func ensResolve(ensRegistryAddr, ethDomain, w3uri string) (contractAddr string, err error) {
	for i := 0; i < maxRetries; i++ {
		contractAddr, err = chain.VotingProcessAddress(ensRegistryAddr, ethDomain, w3uri)
		if err != nil {
			if strings.Contains(err.Error(), "no suitable peers available") {
				time.Sleep(time.Second)
				continue
			}
			err = fmt.Errorf("cannot get voting process contract: %s", err)
			return
		}
		log.Infof("loaded voting contract at address: %s", contractAddr)
		break
	}
	return
}

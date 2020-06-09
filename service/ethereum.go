package service

import (
	"fmt"
	"os"
	"time"

	"github.com/ethereum/go-ethereum/accounts/keystore"
	"gitlab.com/vocdoni/go-dvote/chain"
	"gitlab.com/vocdoni/go-dvote/config"
	"gitlab.com/vocdoni/go-dvote/crypto/ethereum"
	"gitlab.com/vocdoni/go-dvote/log"
	"gitlab.com/vocdoni/go-dvote/metrics"
	"gitlab.com/vocdoni/go-dvote/net"
)

func Ethereum(ethconfig *config.EthCfg, w3config *config.W3Cfg, pxy *net.Proxy, signer *ethereum.SignKeys, ma *metrics.Agent) (node *chain.EthChainContext, err error) {
	// Ethereum
	log.Info("creating ethereum service")

	// Set Ethereum node context
	w3cfg, err := chain.NewConfig(ethconfig, w3config)
	if err != nil {
		return nil, err
	}
	node, err = chain.Init(w3cfg)
	if err != nil {
		return
	}

	os.RemoveAll(ethconfig.DataDir + "/keystore/tmp")
	node.Keys = keystore.NewPlaintextKeyStore(ethconfig.DataDir + "/keyStore/tmp")
	if _, err := node.Keys.ImportECDSA(&signer.Private, ""); err != nil {
		if err != keystore.ErrAccountAlreadyExists {
			return nil, err
		}
	}

	// Start Ethereum node
	node.Start()
	go node.PrintInfo(time.Second * 20)

	// Grab ethereum metrics loop
	go node.CollectMetrics(ma)

	log.Infof("ethereum node listening on %s", node.Node.Server().NodeInfo().ListenAddr)
	if w3config.Enabled && pxy != nil {
		pxy.AddHandler(w3config.Route, pxy.AddEndpoint(fmt.Sprintf("http://%s:%d", w3cfg.RPCHost, w3cfg.RPCPort)))
		log.Infof("web3 endpoint available at %s", w3config.Route)
		pxy.AddWsHandler(w3config.Route+"ws", pxy.AddWsHTTPBridge(fmt.Sprintf("ws://%s:%d", w3cfg.RPCHost, w3cfg.RPCPort)))
		log.Infof("web3 endpoint available at %s", w3config.Route+"ws")
	}
	return
}

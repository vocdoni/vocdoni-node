package service

import (
	"fmt"
	"os"
	"time"

	"github.com/ethereum/go-ethereum/accounts/keystore"
	"gitlab.com/vocdoni/go-dvote/chain"
	"gitlab.com/vocdoni/go-dvote/config"
	"gitlab.com/vocdoni/go-dvote/crypto/signature"
	"gitlab.com/vocdoni/go-dvote/log"
	"gitlab.com/vocdoni/go-dvote/net"
)

func Ethereum(ethconfig *config.EthCfg, w3config *config.W3Cfg, pxy *net.Proxy, signer *signature.SignKeys) (node *chain.EthChainContext, err error) {
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
	if _, err := node.Keys.ImportECDSA(signer.Private, ""); err != nil {
		if err.Error() != "account already exists" {
			return nil, err
		}
	}

	// Start Ethereum node
	node.Start()
	go node.PrintInfo(time.Second * 20)

	log.Infof("ethereum node listening on %s", node.Node.Server().NodeInfo().ListenAddr)
	if w3config.Enabled {
		pxy.AddHandler(w3config.Route, pxy.AddEndpoint(fmt.Sprintf("http://%s:%d", w3cfg.HTTPHost, w3cfg.HTTPPort)))
		log.Infof("web3 available at %s", w3config.Route)
		pxy.AddWsHandler(w3config.Route+"ws", pxy.AddWsHTTPBridge(fmt.Sprintf("http://%s:%d", w3cfg.HTTPHost, w3cfg.HTTPPort)))
		log.Infof("web3 Websocket available at %s", w3config.Route+"ws")
	}
	return
}

package service

import (
	"fmt"
	"os"
	"strings"
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
	if _, err := node.Keys.ImportECDSA(&signer.Private, ""); err != nil && err != keystore.ErrAccountAlreadyExists {
		return nil, err
	}

	// Start Ethereum node
	node.Start()
	go node.PrintInfo(time.Second * 20)
	w3uri := w3cfg.W3external
	if w3uri == "" {
		// Grab ethereum metrics loop
		go node.CollectMetrics(ma)
		log.Infof("ethereum node listening on %s", node.Node.Server().NodeInfo().ListenAddr)
		w3uri = fmt.Sprintf("http://%s:%d", w3cfg.RPCHost, w3cfg.RPCPort)
	}

	if !w3config.Enabled || pxy == nil {
		return
	}
	if strings.HasPrefix(w3uri, "http") {
		pxy.AddHandler(w3config.Route, pxy.AddEndpoint(w3uri))
		log.Infof("web3 http endpoint available at %s", w3config.Route)
	} else if strings.HasSuffix(w3uri, ".ipc") {
		info, err := os.Stat(w3uri)
		if err != nil {
			return nil, fmt.Errorf("could not stat IPC path: %v", err)
		}
		if info.Mode()&os.ModeSocket == 0 {
			return nil, fmt.Errorf("IPC path is not a socket: %s", w3uri)
		}
		pxy.AddHandler(w3config.Route, pxy.ProxyIPC(w3uri))
		log.Infof("web3 http endpoint available at %s", w3config.Route)
	} else {
		log.Warnf("web3 http API requires http/https/ipc web3 external, disabling it")
	}
	if strings.HasPrefix(w3uri, "http") {
		pxy.AddWsHandler(w3config.Route+"ws", pxy.AddWsHTTPBridge(w3uri))
		log.Infof("web3 websocket endpoint available at %s", w3config.Route+"ws")
	} else if strings.HasPrefix(w3uri, "ws") {
		pxy.AddWsHandler(w3config.Route+"ws", pxy.AddWsWsBridge(w3uri))
		log.Infof("web3 websocket endpoint available at %s", w3config.Route+"ws")
	} else {
		log.Warnf("external web3 protocol is not http or ws, clients won't be able to use the web3 ws endpoint")
	}
	return
}

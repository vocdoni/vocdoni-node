package service

import (
	"context"
	"fmt"
	"os"
	"path"
	"strings"
	"time"

	"github.com/ethereum/go-ethereum/accounts/keystore"
	"go.vocdoni.io/dvote/chain"
	"go.vocdoni.io/dvote/config"
	"go.vocdoni.io/dvote/crypto/ethereum"
	"go.vocdoni.io/dvote/log"
	"go.vocdoni.io/dvote/metrics"
	"go.vocdoni.io/dvote/multirpc/transports/mhttp"
	"go.vocdoni.io/dvote/types"
)

var ValidChains = map[string]bool{
	"goerli":  true,
	"mainnet": true,
	"rinkeby": true,
}

func Ethereum(ethconfig *config.EthCfg,
	w3config *config.W3Cfg,
	pxy *mhttp.Proxy,
	signer *ethereum.SignKeys,
	ma *metrics.Agent,
) (node *chain.EthChainContext, err error) {
	// Ethereum
	log.Info("creating ethereum service")
	if _, ok := ValidChains[ethconfig.ChainType]; !ok {
		if w3config.W3External == "" {
			return nil, fmt.Errorf(`ethereum chain type %s is not supported in native mode by go-ethereum.
A web3 compatible external endpoint should be used instead`, ethconfig.ChainType)
		}
	}
	// Set Ethereum node context
	w3cfg, err := chain.NewConfig(ethconfig, w3config)
	if err != nil {
		return nil, err
	}
	node, err = chain.Init(w3cfg)
	if err != nil {
		return
	}

	_ = os.RemoveAll(path.Join(ethconfig.DataDir, "/keystore/tmp"))
	node.Keys = keystore.NewPlaintextKeyStore(ethconfig.DataDir + "/keyStore/tmp")
	if _, err := node.Keys.ImportECDSA(&signer.Private, ""); err != nil &&
		err != keystore.ErrAccountAlreadyExists {
		return nil, err
	}

	// Start Ethereum node
	node.Start()
	go node.PrintInfo(context.Background(), time.Second*20)
	w3uri := w3cfg.W3external
	if w3uri == "" {
		// Grab ethereum metrics loop
		go node.CollectMetrics(context.Background(), ma)
		log.Infof("ethereum node listening on %s", node.Node.Server().NodeInfo().ListenAddr)
		w3uri = fmt.Sprintf("http://%s:%d", w3cfg.RPCHost, w3cfg.RPCPort)
	}

	if !w3config.Enabled || pxy == nil {
		return
	}
	if strings.HasPrefix(w3uri, "http") {
		pxy.AddMixedHandler(w3config.Route,
			pxy.AddEndpoint(w3uri), pxy.AddWsHTTPBridge(w3uri), types.Web3WsReadLimit) // 5MB read limit
		log.Infof("web3 http/websocket endpoint available at %s", w3config.Route)
	} else if strings.HasPrefix(w3uri, "ws") {
		pxy.AddWsHandler(w3config.Route+"ws",
			pxy.AddWsWsBridge(w3uri, types.Web3WsReadLimit), types.Web3WsReadLimit) // 5MB read limit
		log.Infof("web3 websocket endpoint available at %s", w3config.Route)
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
		log.Warnf("web3 http API requires http/https/ipc web3 external")
	}
	return
}

package service

import (
	"time"

	"gitlab.com/vocdoni/go-dvote/census"
	"gitlab.com/vocdoni/go-dvote/config"
	"gitlab.com/vocdoni/go-dvote/crypto/ethereum"
	"gitlab.com/vocdoni/go-dvote/data"
	"gitlab.com/vocdoni/go-dvote/log"
	"gitlab.com/vocdoni/go-dvote/metrics"
	"gitlab.com/vocdoni/go-dvote/net"
	"gitlab.com/vocdoni/go-dvote/router"
	"gitlab.com/vocdoni/go-dvote/types"
	"gitlab.com/vocdoni/go-dvote/vochain"
	"gitlab.com/vocdoni/go-dvote/vochain/scrutinizer"
	"gitlab.com/vocdoni/go-dvote/vochain/vochaininfo"
)

// TBD: user the net.Transport interface
func API(apiconfig *config.API, pxy *net.Proxy, storage data.Storage, cm *census.Manager, vapp *vochain.BaseApplication,
	sc *scrutinizer.Scrutinizer, vi *vochaininfo.VochainInfo, vochainRPCaddr string, signer *ethereum.SignKeys, ma *metrics.Agent,
) error {
	log.Infof("creating API service")
	// API Endpoint initialization
	var transport net.Transport
	listenerOutput := make(chan types.Message)

	// WebSocket transport
	if apiconfig.Websockets {
		transport = new(net.WebsocketHandle)
		if err := transport.Init(new(types.Connection)); err != nil {
			return err
		}
		transport.(*net.WebsocketHandle).SetProxy(pxy)
		go transport.Listen(listenerOutput)
	} else {
		// HTTP transport
		transport = new(net.HttpHandler)
		if err := transport.Init(new(types.Connection)); err != nil {
			return err
		}
		transport.(*net.HttpHandler).SetProxy(pxy)
		go transport.Listen(listenerOutput)
	}
	routerAPI := router.InitRouter(listenerOutput, storage, transport, signer, ma, apiconfig.AllowPrivate)
	if apiconfig.File {
		log.Info("enabling file API")
		routerAPI.EnableFileAPI()
	}
	if apiconfig.Census {
		log.Info("enabling census API")
		routerAPI.EnableCensusAPI(cm)
	}
	if apiconfig.Vote {
		// todo: client params as cli flags
		log.Info("enabling vote API")
		routerAPI.Scrutinizer = sc
		routerAPI.EnableVoteAPI(vapp, vi)
	}
	transport.AddNamespace(apiconfig.Route)
	go routerAPI.Route()
	log.Infof("%s API available at %s", transport.String(), apiconfig.Route)
	go func() {
		for {
			time.Sleep(60 * time.Second)
			log.Infof("[router info] privateReqs:%d publicReqs:%d", routerAPI.PrivateCalls, routerAPI.PublicCalls)
		}
	}()
	return nil
}

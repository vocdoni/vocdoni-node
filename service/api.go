package service

import (
	"fmt"
	"time"

	"go.vocdoni.io/dvote/census"
	"go.vocdoni.io/dvote/config"
	"go.vocdoni.io/dvote/crypto/ethereum"
	"go.vocdoni.io/dvote/data"
	"go.vocdoni.io/dvote/log"
	"go.vocdoni.io/dvote/metrics"
	"go.vocdoni.io/dvote/net"
	"go.vocdoni.io/dvote/router"
	"go.vocdoni.io/dvote/types"
	"go.vocdoni.io/dvote/vochain"
	"go.vocdoni.io/dvote/vochain/scrutinizer"
	"go.vocdoni.io/dvote/vochain/vochaininfo"
)

// TBD: user the net.Transport interface
func API(apiconfig *config.API, pxy *net.Proxy, storage data.Storage, cm *census.Manager, vapp *vochain.BaseApplication,
	sc *scrutinizer.Scrutinizer, vi *vochaininfo.VochainInfo, vochainRPCaddr string, signer *ethereum.SignKeys, ma *metrics.Agent,
) error {
	log.Infof("creating API service")
	// API Endpoint initialization
	listenerOutput := make(chan types.Message)

	var htransport net.Transport

	if apiconfig.Websockets && apiconfig.HTTP {
		htransport = net.NewHttpWsHandleWithWsReadLimit(apiconfig.WebsocketsReadLimit)
		htransport.(*net.HttpWsHandler).SetProxy(pxy)
	} else {
		if apiconfig.Websockets {
			htransport = net.NewWebSocketHandleWithReadLimit(apiconfig.WebsocketsReadLimit)
			htransport.(*net.WebsocketHandle).SetProxy(pxy)
		} else if apiconfig.HTTP {
			htransport = new(net.HttpHandler)
			htransport.(*net.HttpHandler).SetProxy(pxy)
		} else {
			return fmt.Errorf("no transports available. At least one of HTTP and WS should be enabled")
		}
	}

	if err := htransport.Init(new(types.Connection)); err != nil {
		return err
	}
	go htransport.Listen(listenerOutput)
	htransport.AddNamespace(apiconfig.Route + "dvote")
	log.Infof("%s API available at %s", htransport.ConnectionType(), apiconfig.Route+"dvote")

	routerAPI := router.InitRouter(listenerOutput, storage, signer, ma, apiconfig.AllowPrivate)
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

	go routerAPI.Route()

	go func() {
		for {
			time.Sleep(60 * time.Second)
			log.Infof("[router info] privateReqs:%d publicReqs:%d", routerAPI.PrivateCalls, routerAPI.PublicCalls)
		}
	}()
	return nil
}

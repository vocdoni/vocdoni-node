package service

import (
	"fmt"
	"sync/atomic"
	"time"

	"go.vocdoni.io/dvote/census"
	"go.vocdoni.io/dvote/config"
	"go.vocdoni.io/dvote/crypto/ethereum"
	"go.vocdoni.io/dvote/data"
	"go.vocdoni.io/dvote/log"
	"go.vocdoni.io/dvote/metrics"

	"go.vocdoni.io/dvote/multirpc/transports"
	"go.vocdoni.io/dvote/multirpc/transports/mhttp"

	"go.vocdoni.io/dvote/router"
	"go.vocdoni.io/dvote/vochain"
	"go.vocdoni.io/dvote/vochain/scrutinizer"
	"go.vocdoni.io/dvote/vochain/vochaininfo"
)

func API(apiconfig *config.API, pxy *mhttp.Proxy, storage data.Storage, cm *census.Manager,
	vapp *vochain.BaseApplication, sc *scrutinizer.Scrutinizer, vi *vochaininfo.VochainInfo,
	vochainRPCaddr string, signer *ethereum.SignKeys, ma *metrics.Agent) (*router.Router, error) {
	log.Infof("creating API service")
	// API Endpoint initialization
	listenerOutput := make(chan transports.Message)

	var htransport transports.Transport

	if apiconfig.Websockets && apiconfig.HTTP {
		htransport = mhttp.NewHttpWsHandleWithWsReadLimit(apiconfig.WebsocketsReadLimit)
		htransport.(*mhttp.HttpWsHandler).SetProxy(pxy)
	} else {
		if apiconfig.Websockets {
			htransport = mhttp.NewWebSocketHandleWithReadLimit(apiconfig.WebsocketsReadLimit)
			htransport.(*mhttp.WebsocketHandle).SetProxy(pxy)
		} else if apiconfig.HTTP {
			htransport = new(mhttp.HttpHandler)
			htransport.(*mhttp.HttpHandler).SetProxy(pxy)
		} else {
			return nil, fmt.Errorf("no transports available. At least one of HTTP and WS should be enabled")
		}
	}

	if err := htransport.Init(new(transports.Connection)); err != nil {
		return nil, err
	}
	htransport.Listen(listenerOutput)
	if err := htransport.AddNamespace(apiconfig.Route + "dvote"); err != nil {
		return nil, err
	}
	log.Infof("%s API available at %s", htransport.ConnectionType(), apiconfig.Route+"dvote")

	routerAPI := router.InitRouter(listenerOutput, storage, signer, ma, apiconfig.AllowPrivate)
	if apiconfig.File && storage != nil {
		log.Info("enabling file API")
		routerAPI.EnableFileAPI()
	}
	if apiconfig.Census && cm != nil {
		log.Info("enabling census API")
		routerAPI.EnableCensusAPI(cm)
	}
	if apiconfig.Vote || apiconfig.Results || apiconfig.Indexer {
		routerAPI.Scrutinizer = sc
		if apiconfig.Vote && vapp != nil {
			log.Info("enabling vote API")
			routerAPI.EnableVoteAPI(vapp, vi)
		}
		if apiconfig.Results && sc != nil {
			log.Info("enabling results API")
			routerAPI.EnableResultsAPI(vapp, vi)
		}
		if apiconfig.Indexer && sc != nil {
			log.Info("enabling indexer API")
			routerAPI.EnableIndexerAPI(vapp, vi)
		}
	}
	go routerAPI.Route()

	go func() {
		for {
			time.Sleep(120 * time.Second)
			log.Infof("[router info] privateReqs:%d publicReqs:%d",
				atomic.LoadUint64(&routerAPI.PrivateCalls), atomic.LoadUint64(&routerAPI.PublicCalls))
		}
	}()
	return routerAPI, nil
}

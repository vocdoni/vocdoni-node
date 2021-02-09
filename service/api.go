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

	"github.com/vocdoni/multirpc/transports"
	"github.com/vocdoni/multirpc/transports/mhttp"

	"go.vocdoni.io/dvote/router"
	"go.vocdoni.io/dvote/vochain"
	"go.vocdoni.io/dvote/vochain/scrutinizer"
	"go.vocdoni.io/dvote/vochain/vochaininfo"
)

func API(apiconfig *config.API, pxy *mhttp.Proxy, storage data.Storage, cm *census.Manager, vapp *vochain.BaseApplication,
	sc *scrutinizer.Scrutinizer, vi *vochaininfo.VochainInfo, vochainRPCaddr string, signer *ethereum.SignKeys, ma *metrics.Agent,
) error {
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
			return fmt.Errorf("no transports available. At least one of HTTP and WS should be enabled")
		}
	}

	if err := htransport.Init(new(transports.Connection)); err != nil {
		return err
	}
	htransport.Listen(listenerOutput)
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

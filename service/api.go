package service

import (
	"sync/atomic"
	"time"

	"go.vocdoni.io/dvote/census"
	"go.vocdoni.io/dvote/config"
	"go.vocdoni.io/dvote/crypto/ethereum"
	"go.vocdoni.io/dvote/data"
	"go.vocdoni.io/dvote/log"
	"go.vocdoni.io/dvote/metrics"
	"go.vocdoni.io/dvote/rpcapi"

	"go.vocdoni.io/dvote/httprouter"
	"go.vocdoni.io/dvote/vochain"
	"go.vocdoni.io/dvote/vochain/scrutinizer"
	"go.vocdoni.io/dvote/vochain/vochaininfo"
)

func API(apiconfig *config.API, router *httprouter.HTTProuter, storage data.Storage, cm *census.Manager,
	vapp *vochain.BaseApplication, sc *scrutinizer.Scrutinizer, vi *vochaininfo.VochainInfo,
	signer *ethereum.SignKeys, ma *metrics.Agent) (*rpcapi.RPCAPI, error) {
	log.Infof("creating API service")

	rpc, err := rpcapi.NewAPI(signer, router, apiconfig.Route+"dvote", ma, apiconfig.AllowPrivate)
	if err != nil {
		return nil, err
	}
	log.Infof("rpc API available at %s", apiconfig.Route+"dvote")

	if apiconfig.File && storage != nil {
		log.Info("enabling file API")
		if err := rpc.EnableFileAPI(storage); err != nil {
			return nil, err
		}
	}
	if apiconfig.Census && cm != nil {
		log.Info("enabling census API")
		if err := rpc.EnableCensusAPI(cm); err != nil {
			return nil, err
		}
	}
	if apiconfig.Vote || apiconfig.Results || apiconfig.Indexer {
		if apiconfig.Vote && vapp != nil {
			log.Info("enabling vote API")
			if err := rpc.EnableVoteAPI(vapp, vi); err != nil {
				return nil, err
			}
		}
		if apiconfig.Results && sc != nil {
			log.Info("enabling results API")
			if err := rpc.EnableResultsAPI(vapp, sc); err != nil {
				return nil, err
			}
		}
		if apiconfig.Indexer && sc != nil {
			log.Info("enabling indexer API")
			if err := rpc.EnableIndexerAPI(vapp, vi, sc); err != nil {
				return nil, err
			}
		}
	}

	go func() {
		for {
			time.Sleep(120 * time.Second)
			log.Infof("[router info] privateReqs:%d publicReqs:%d",
				atomic.LoadUint64(&rpc.PrivateCalls), atomic.LoadUint64(&rpc.PublicCalls))
		}
	}()
	return rpc, nil
}

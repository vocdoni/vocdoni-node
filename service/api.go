package service

import (
	"go.vocdoni.io/dvote/api/censusdb"
	"go.vocdoni.io/dvote/config"
	"go.vocdoni.io/dvote/crypto/ethereum"
	"go.vocdoni.io/dvote/data"
	"go.vocdoni.io/dvote/log"
	"go.vocdoni.io/dvote/metrics"
	"go.vocdoni.io/dvote/rpcapi"
	census "go.vocdoni.io/dvote/rpccensus"

	"go.vocdoni.io/dvote/vochain"
	"go.vocdoni.io/dvote/vochain/indexer"
	"go.vocdoni.io/dvote/vochain/vochaininfo"
)

func API(apiconfig *config.API, rpc *rpcapi.RPCAPI, storage data.Storage, censusdb *censusdb.CensusDB,
	vapp *vochain.BaseApplication, sc *indexer.Indexer, vi *vochaininfo.VochainInfo,
	signer *ethereum.SignKeys, ma *metrics.Agent) (*rpcapi.RPCAPI, error) {
	log.Infof("creating API service")

	if apiconfig.File && storage != nil {
		log.Info("enabling file API")
		if err := rpc.EnableFileAPI(storage); err != nil {
			return nil, err
		}
	}
	if apiconfig.Census {
		log.Info("enabling census API")
		cm := census.NewCensusManager(censusdb, storage)
		if err := rpc.EnableCensusAPI(cm); err != nil {
			return nil, err
		}
	}
	if apiconfig.Vote || apiconfig.Results || apiconfig.Indexer {
		if apiconfig.Vote {
			log.Info("enabling vote API")
			if err := rpc.EnableVoteAPI(vapp, vi); err != nil {
				return nil, err
			}
		}
		if apiconfig.Results {
			log.Info("enabling results API")
			if err := rpc.EnableResultsAPI(vapp, sc); err != nil {
				return nil, err
			}
		}
		if apiconfig.Indexer {
			log.Info("enabling indexer API")
			if err := rpc.EnableIndexerAPI(vapp, vi, sc); err != nil {
				return nil, err
			}
		}
	}

	return rpc, nil
}

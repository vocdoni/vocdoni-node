package service

import (
	"go.vocdoni.io/dvote/log"
	"go.vocdoni.io/dvote/rpcapi"
	"go.vocdoni.io/dvote/rpccensus"
)

func (vs *VocdoniService) LegacyRPC() (*rpcapi.RPCAPI, error) {
	log.Infof("creating JSON-RPC service")

	// Initialize the RPC API
	rpc, err := rpcapi.NewAPI(vs.Signer, vs.Router, "/dvote", vs.MetricsAgent, true)
	if err != nil {
		return nil, err
	}
	log.Info("JSON-RPC available at /dvote")

	if vs.Storage != nil {
		log.Info("enabling file JSON-RPC")
		if err := rpc.EnableFileAPI(vs.Storage); err != nil {
			return nil, err
		}
	}

	if vs.CensusDB != nil {
		log.Info("enabling census JSON-RPC")
		cm := rpccensus.NewCensusManager(vs.CensusDB, vs.Storage)
		if err := rpc.EnableCensusAPI(cm); err != nil {
			return nil, err
		}
	}

	if vs.App != nil && vs.Stats != nil {
		log.Info("enabling vote JSON-RPC")
		if err := rpc.EnableVoteAPI(vs.App, vs.Stats); err != nil {
			return nil, err
		}
	}

	if vs.App != nil && vs.Indexer != nil {
		log.Info("enabling results JSON-RPC")
		if err := rpc.EnableResultsAPI(vs.App, vs.Indexer); err != nil {
			return nil, err
		}
	}

	if vs.App != nil && vs.Indexer != nil && vs.Stats != nil {
		log.Info("enabling indexer JSON-RPC")
		if err := rpc.EnableIndexerAPI(vs.App, vs.Stats, vs.Indexer); err != nil {
			return nil, err
		}
	}

	return rpc, nil
}

package rpcapi

import (
	"fmt"

	"go.vocdoni.io/dvote/data"
	"go.vocdoni.io/dvote/log"
	census "go.vocdoni.io/dvote/rpccensus"
	"go.vocdoni.io/dvote/vochain"
	"go.vocdoni.io/dvote/vochain/indexer"
	"go.vocdoni.io/dvote/vochain/vochaininfo"
)

// EnableFileAPI enables the FILE API in the Router
func (r *RPCAPI) EnableFileAPI(storage data.Storage) error {
	if storage == nil {
		return fmt.Errorf("storage cannot be nil for file RPC")
	}
	r.storage = storage
	r.APIs = append(r.APIs, "file")

	r.RegisterPublic("fetchFile", false, r.fetchFile)
	if r.allowPrivate {
		r.RegisterPrivate("addFile", r.addFile)
	} else {
		r.RegisterPublic("addFile", false, r.addJSONfile)
	}
	r.RegisterPrivate("pinList", r.pinList)
	r.RegisterPrivate("pinFile", r.pinFile)
	r.RegisterPrivate("unpinFile", r.unpinFile)
	return nil
}

// EnableCensusAPI enables the Census API in the Router
func (r *RPCAPI) EnableCensusAPI(cm *census.Manager) error {
	if cm == nil {
		return fmt.Errorf("census manager cannot be nil for census RPC")
	}
	r.APIs = append(r.APIs, "census")
	r.census = cm
	if cm.RemoteStorage == nil {
		cm.RemoteStorage = r.storage
	}

	r.RegisterPublic("getRoot", false, r.censusLocal)
	r.RegisterPrivate("dump", r.censusLocal)
	r.RegisterPublic("getSize", false, r.censusLocal)
	r.RegisterPublic("getCensusWeight", false, r.censusLocal)
	r.RegisterPublic("genProof", false, r.censusLocal)
	r.RegisterPublic("checkProof", false, r.censusLocal)
	if r.allowPrivate {
		r.RegisterPrivate("addCensus", r.censusLocal)
		r.RegisterPrivate("addClaim", r.censusLocal)
		r.RegisterPrivate("addClaimBulk", r.censusLocal)
		r.RegisterPrivate("publish", r.censusLocal)
		r.RegisterPrivate("importRemote", r.censusLocal)
		r.RegisterPrivate("getCensusList", r.censusLocal)
	}

	return nil
}

// EnableVoteAPI enabled the Vote API in the Router
func (r *RPCAPI) EnableVoteAPI(vocapp *vochain.BaseApplication, vocinfo *vochaininfo.VochainInfo) error {
	if (r.vocapp == nil && vocapp == nil) || (r.vocinfo == nil && vocinfo == nil) {
		return fmt.Errorf("vocdoni APP or vocdoni Info are nil")
	}
	if r.vocapp == nil {
		r.vocapp = vocapp
	}
	if r.vocinfo == nil {
		r.vocinfo = vocinfo
	}
	r.APIs = append(r.APIs, "vote")
	r.RegisterPublic("submitRawTx", false, r.submitRawTx)
	r.RegisterPublic("submitEnvelope", false, r.submitEnvelope)
	r.RegisterPublic("getEnvelopeStatus", false, r.getEnvelopeStatus)
	r.RegisterPublic("getEnvelopeHeight", false, r.getEnvelopeHeight)
	r.RegisterPublic("getBlockHeight", false, r.getBlockHeight)
	r.RegisterPublic("getProcessKeys", false, r.getProcessKeys)
	r.RegisterPublic("getProcessCircuitConfig", false, r.getProcessCircuitConfig)
	r.RegisterPublic("getProcessRollingCensusSize", false, r.getProcessRollingCensusSize)
	r.RegisterPublic("getBlockStatus", false, r.getBlockStatus)
	r.RegisterPublic("getOracleResults", false, r.getOracleResults)
	r.RegisterPublic("getProcessCircuitConfig", false, r.getProcessCircuitConfig)
	r.RegisterPublic("getProcessRollingCensusSize", false, r.getProcessRollingCensusSize)
	r.RegisterPublic("getPreregisterVoterWeight", false, r.getPreRegisterWeight)

	return nil
}

// EnableResultsAPI enabled the vote results API in the Router
func (r *RPCAPI) EnableResultsAPI(vocapp *vochain.BaseApplication, indexer *indexer.Indexer) error {
	if (r.vocapp == nil && vocapp == nil) || (r.indexer == nil && indexer == nil) {
		return fmt.Errorf("vocdoni APP or indexer are nil")
	}
	if r.vocapp == nil {
		r.vocapp = vocapp
	}
	if r.indexer == nil {
		r.indexer = indexer
	}
	r.APIs = append(r.APIs, "results")

	r.RegisterPublic("getProcessList", false, r.getProcessList)
	r.RegisterPublic("getProcessInfo", false, r.getProcessInfo)
	r.RegisterPublic("getProcessSummary", false, r.getProcessSummary)
	r.RegisterPublic("getProcessCount", false, r.getProcessCount)
	r.RegisterPublic("getResults", false, r.getResults)
	r.RegisterPublic("getResultsWeight", false, r.getResultsWeight)
	r.RegisterPublic("getEntityList", false, r.getEntityList)
	r.RegisterPublic("getEntityCount", false, r.getEntityCount)
	r.RegisterPublic("getEnvelope", false, r.getEnvelope)
	r.RegisterPublic("getAccount", false, r.getAccount)
	r.RegisterPublic("getTreasurer", false, r.getTreasurer)
	r.RegisterPublic("getTxCost", false, r.getTransactionCost)
	return nil
}

// EnableIndexerAPI enables the vote indexer API in the Router
func (r *RPCAPI) EnableIndexerAPI(vocapp *vochain.BaseApplication,
	vocinfo *vochaininfo.VochainInfo, indexer *indexer.Indexer) error {
	if (r.vocapp == nil && vocapp == nil) || (r.indexer == nil && indexer == nil) ||
		(r.vocinfo == nil && vocinfo == nil) {
		return fmt.Errorf("vocdoni APP or indexer are nil")
	}
	if r.vocapp == nil {
		r.vocapp = vocapp
	}
	if r.vocinfo == nil {
		r.vocinfo = vocinfo
	}
	if r.indexer == nil {
		r.indexer = indexer
	}

	if r.indexer == nil {
		log.Fatal("cannot enable indexer RPC without indexer")
	}
	r.APIs = append(r.APIs, "indexer")

	r.RegisterPublic("getStats", false, r.getStats)
	r.RegisterPublic("getEnvelopeList", false, r.getEnvelopeList)
	r.RegisterPublic("getBlock", false, r.getBlock)
	r.RegisterPublic("getBlockByHash", false, r.getBlockByHash)
	r.RegisterPublic("getBlockList", false, r.getBlockList)
	r.RegisterPublic("getTx", false, r.getTx)
	r.RegisterPublic("getTxById", false, r.getTxById)
	r.RegisterPublic("getTxByHash", false, r.getTxByHash)
	r.RegisterPublic("getValidatorList", false, r.getValidatorList)
	r.RegisterPublic("getOracleList", false, r.getOracleList)
	r.RegisterPublic("getTxListForBlock", false, r.getTxListForBlock)
	return nil
}

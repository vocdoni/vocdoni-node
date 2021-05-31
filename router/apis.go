package router

import (
	"go.vocdoni.io/dvote/census"
	"go.vocdoni.io/dvote/log"
	"go.vocdoni.io/dvote/vochain"
	"go.vocdoni.io/dvote/vochain/vochaininfo"
)

// EnableFileAPI enables the FILE API in the Router
func (r *Router) EnableFileAPI() {
	r.APIs = append(r.APIs, "file")
	r.RegisterPublic("fetchFile", r.fetchFile)
	if r.allowPrivate {
		r.RegisterPrivate("addFile", r.addFile)
	} else {
		r.RegisterPublic("addFile", r.addJSONfile)
	}
	r.RegisterPrivate("pinList", r.pinList)
	r.RegisterPrivate("pinFile", r.pinFile)
	r.RegisterPrivate("unpinFile", r.unpinFile)
}

// EnableCensusAPI enables the Census API in the Router
func (r *Router) EnableCensusAPI(cm *census.Manager) {
	r.APIs = append(r.APIs, "census")
	r.census = cm
	if cm.RemoteStorage == nil {
		cm.RemoteStorage = r.storage
	}
	r.RegisterPublic("getRoot", r.censusLocal)
	r.RegisterPrivate("dump", r.censusLocal)
	r.RegisterPrivate("dumpPlain", r.censusLocal)
	r.RegisterPublic("getSize", r.censusLocal)
	r.RegisterPublic("genProof", r.censusLocal)
	r.RegisterPublic("checkProof", r.censusLocal)
	r.RegisterPrivate("addCensus", r.censusLocal)
	r.RegisterPrivate("addClaim", r.censusLocal)
	r.RegisterPrivate("addClaimBulk", r.censusLocal)
	r.RegisterPrivate("publish", r.censusLocal)
	r.RegisterPrivate("importRemote", r.censusLocal)
	r.RegisterPrivate("getCensusList", r.censusLocal)
}

// EnableVoteAPI enabled the Vote API in the Router
func (r *Router) EnableVoteAPI(vocapp *vochain.BaseApplication, vocInfo *vochaininfo.VochainInfo) {
	r.APIs = append(r.APIs, "vote")
	r.vocapp = vocapp
	r.vocinfo = vocInfo
	r.RegisterPublic("submitRawTx", r.submitRawTx)
	r.RegisterPublic("submitEnvelope", r.submitEnvelope)
	r.RegisterPublic("getEnvelopeStatus", r.getEnvelopeStatus)
	r.RegisterPublic("getEnvelopeHeight", r.getEnvelopeHeight)
	r.RegisterPublic("getBlockHeight", r.getBlockHeight)
	r.RegisterPublic("getProcessKeys", r.getProcessKeys)
	r.RegisterPublic("getBlockStatus", r.getBlockStatus)
	r.RegisterPublic("getOracleResults", r.getOracleResults)
}

// EnableVoteAPI enabled the Vote API in the Router
func (r *Router) EnableResultsAPI(vocapp *vochain.BaseApplication,
	vocInfo *vochaininfo.VochainInfo) {
	if r.Scrutinizer == nil {
		log.Fatal("cannot enable results API without scrutinizer")
	}
	r.APIs = append(r.APIs, "results")
	r.RegisterPublic("getProcessList", r.getProcessList)
	r.RegisterPublic("getProcessInfo", r.getProcessInfo)
	r.RegisterPublic("getProcessSummary", r.getProcessSummary)
	r.RegisterPublic("getProcessCount", r.getProcessCount)
	r.RegisterPublic("getResults", r.getResults)
	r.RegisterPublic("getResultsWeight", r.getResultsWeight)
	r.RegisterPublic("getEntityList", r.getEntityList)
	r.RegisterPublic("getEntityCount", r.getEntityCount)
	r.RegisterPublic("getEnvelope", r.getEnvelope)
}

// EnableVoteAPI enabled the Vote API in the Router
func (r *Router) EnableIndexerAPI(vocapp *vochain.BaseApplication,
	vocInfo *vochaininfo.VochainInfo) {
	if r.Scrutinizer == nil {
		log.Fatal("cannot enable indexer API without scrutinizer")
	}
	r.APIs = append(r.APIs, "indexer")
	r.RegisterPublic("getStats", r.getStats)
	r.RegisterPublic("getEnvelopeList", r.getEnvelopeList)
	r.RegisterPublic("getBlock", r.getBlock)
	r.RegisterPublic("getBlockByHash", r.getBlockByHash)
	r.RegisterPublic("getBlockList", r.getBlockList)
	r.RegisterPublic("getTx", r.getTx)
	r.RegisterPublic("getTxByHeight", r.getTxByHeight)
	r.RegisterPublic("getValidatorList", r.getValidatorList)
	r.RegisterPublic("getTxListForBlock", r.getTxListForBlock)
}

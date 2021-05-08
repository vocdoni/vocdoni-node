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
	r.registerPublic("fetchFile", r.fetchFile)
	if r.allowPrivate {
		r.registerPrivate("addFile", r.addFile)
	} else {
		r.registerPublic("addFile", r.addJSONfile)
	}
	r.registerPrivate("pinList", r.pinList)
	r.registerPrivate("pinFile", r.pinFile)
	r.registerPrivate("unpinFile", r.unpinFile)
}

// EnableCensusAPI enables the Census API in the Router
func (r *Router) EnableCensusAPI(cm *census.Manager) {
	r.APIs = append(r.APIs, "census")
	r.census = cm
	if cm.RemoteStorage == nil {
		cm.RemoteStorage = r.storage
	}
	r.registerPublic("getRoot", r.censusLocal)
	r.registerPrivate("dump", r.censusLocal)
	r.registerPrivate("dumpPlain", r.censusLocal)
	r.registerPublic("getSize", r.censusLocal)
	r.registerPublic("genProof", r.censusLocal)
	r.registerPublic("checkProof", r.censusLocal)
	r.registerPrivate("addCensus", r.censusLocal)
	r.registerPrivate("addClaim", r.censusLocal)
	r.registerPrivate("addClaimBulk", r.censusLocal)
	r.registerPrivate("publish", r.censusLocal)
	r.registerPrivate("importRemote", r.censusLocal)
	r.registerPrivate("getCensusList", r.censusLocal)
}

// EnableVoteAPI enabled the Vote API in the Router
func (r *Router) EnableVoteAPI(vocapp *vochain.BaseApplication, vocInfo *vochaininfo.VochainInfo) {
	r.APIs = append(r.APIs, "vote")
	r.vocapp = vocapp
	r.vocinfo = vocInfo
	r.registerPublic("submitRawTx", r.submitRawTx)
	r.registerPublic("submitEnvelope", r.submitEnvelope)
	r.registerPublic("getEnvelopeStatus", r.getEnvelopeStatus)
	r.registerPublic("getEnvelopeHeight", r.getEnvelopeHeight)
	r.registerPublic("getBlockHeight", r.getBlockHeight)
	r.registerPublic("getProcessKeys", r.getProcessKeys)
	r.registerPublic("getBlockStatus", r.getBlockStatus)
}

// EnableVoteAPI enabled the Vote API in the Router
func (r *Router) EnableResultsAPI(vocapp *vochain.BaseApplication,
	vocInfo *vochaininfo.VochainInfo) {
	if r.Scrutinizer == nil {
		log.Fatal("cannot enable results API without scrutinizer")
	}
	r.APIs = append(r.APIs, "results")
	r.registerPublic("getProcessList", r.getProcessList)
	r.registerPublic("getProcessInfo", r.getProcessInfo)
	r.registerPublic("getProcessCount", r.getProcessCount)
	r.registerPublic("getResults", r.getResults)
	r.registerPublic("getResultsWeight", r.getResultsWeight)
	r.registerPublic("getEntityList", r.getEntityList)
	r.registerPublic("getEntityCount", r.getEntityCount)
	r.registerPublic("getEnvelope", r.getEnvelope)
}

// EnableVoteAPI enabled the Vote API in the Router
func (r *Router) EnableIndexerAPI(vocapp *vochain.BaseApplication,
	vocInfo *vochaininfo.VochainInfo) {
	if r.Scrutinizer == nil {
		log.Fatal("cannot enable indexer API without scrutinizer")
	}
	r.APIs = append(r.APIs, "indexer")
	r.registerPublic("getStats", r.getStats)
	r.registerPublic("getEnvelopeList", r.getEnvelopeList)
	r.registerPublic("getBlock", r.getBlock)
	r.registerPublic("getBlockByHash", r.getBlockByHash)
	r.registerPublic("getBlockList", r.getBlockList)
	r.registerPublic("getTx", r.getTx)
	r.registerPublic("getValidatorList", r.getValidatorList)
	r.registerPublic("getTxListForBlock", r.getTxListForBlock)
}

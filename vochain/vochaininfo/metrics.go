package vochaininfo

import "github.com/VictoriaMetrics/metrics"

var (
	height          = metrics.NewCounter("vochain_height")       // Height of the vochain (last block)
	voteCount       = metrics.NewCounter("vochain_vote_tree")    // Total vote count
	processTreeSize = metrics.NewCounter("vochain_process_tree") // Size of the process tree
	accountTreeSize = metrics.NewCounter("vochain_account_tree") // Size of the account tree
	sikTreeSize     = metrics.NewCounter("vochain_sik_tree")     // Size of the SIK tree
	mempoolSize     = metrics.NewCounter("vochain_mempool")      // Number of Txs in the mempool
	voteCacheSize   = metrics.NewCounter("vochain_vote_cache")   // Size of the current vote cache
)

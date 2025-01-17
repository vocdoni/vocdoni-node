package vochaininfo

import "github.com/VictoriaMetrics/metrics"

var (
	viMetrics         = metrics.NewSet()
	height            = viMetrics.NewCounter("vochain_height")              // Height of the vochain (last block)
	voteCount         = viMetrics.NewCounter("vochain_vote_tree")           // Total vote count
	processTreeSize   = viMetrics.NewCounter("vochain_process_tree")        // Size of the process tree
	accountTreeSize   = viMetrics.NewCounter("vochain_account_tree")        // Size of the account tree
	sikTreeSize       = viMetrics.NewCounter("vochain_sik_tree")            // Size of the SIK tree
	mempoolSize       = viMetrics.NewCounter("vochain_mempool")             // Number of Txs in the mempool
	voteCacheSize     = viMetrics.NewCounter("vochain_vote_cache")          // Size of the current vote cache
	blockPeriodMinute = viMetrics.NewCounter("vochain_block_period_minute") // Block period for the last minute

	blocksyncMetrics = metrics.NewSet()
	blocksyncHeight  = blocksyncMetrics.NewCounter("vochain_sync_height")
	blocksyncBPM     = blocksyncMetrics.NewCounter("vochain_sync_blocks_per_minute")
)

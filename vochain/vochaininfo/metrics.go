package vochaininfo

import (
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"go.vocdoni.io/dvote/metrics"
)

// Vochain collectors
var (
	// VochainHeight ...
	VochainHeight = prometheus.NewGauge(prometheus.GaugeOpts{
		Namespace: "vochain",
		Name:      "height",
		Help:      "Height of the vochain (last block)",
	})
	// VochainMempool ...
	VochainMempool = prometheus.NewGauge(prometheus.GaugeOpts{
		Namespace: "vochain",
		Name:      "mempool",
		Help:      "Number of Txs in the mempool",
	})
	// VochainAppTree ...
	VochainAppTree = prometheus.NewGauge(prometheus.GaugeOpts{
		Namespace: "vochain",
		Name:      "app_tree",
		Help:      "Size of the app tree",
	})
	// VochainProcessTree ...
	VochainProcessTree = prometheus.NewGauge(prometheus.GaugeOpts{
		Namespace: "vochain",
		Name:      "process_tree",
		Help:      "Size of the process tree",
	})
	// VochainVoteTree ...
	VochainVoteTree = prometheus.NewGauge(prometheus.GaugeOpts{
		Namespace: "vochain",
		Name:      "vote_tree",
		Help:      "Size of the vote tree",
	})
	// VochainVotesPerMinute ...
	VochainVotesPerMinute = prometheus.NewGauge(prometheus.GaugeOpts{
		Namespace: "vochain",
		Name:      "vote_tree_increase_last_minute",
		Help:      "Number of votes included in the vote tree the last 60 seconds",
	})
	// VochainAppTree ...
	VochainVoteCache = prometheus.NewGauge(prometheus.GaugeOpts{
		Namespace: "vochain",
		Name:      "vote_cache",
		Help:      "Size of the current vote cache",
	})

	VochainAccountTree = prometheus.NewGauge(prometheus.GaugeOpts{
		Namespace: "vochain",
		Name:      "account_tree",
		Help:      "Size of the account tree",
	})

	VochainSIKTree = prometheus.NewGauge(prometheus.GaugeOpts{
		Namespace: "vochain",
		Name:      "sik_tree",
		Help:      "Size of the SIK tree",
	})

	VochainTokensBurned = prometheus.NewGauge(prometheus.GaugeOpts{
		Namespace: "vochain",
		Name:      "tokens_burned",
		Help:      "Balance of the burn address",
	})
)

// registerMetrics registers each of the vochain prometheus metrics
func (*VochainInfo) registerMetrics(ma *metrics.Agent) {
	ma.Register(VochainHeight)
	ma.Register(VochainMempool)
	ma.Register(VochainAppTree)
	ma.Register(VochainProcessTree)
	ma.Register(VochainVoteTree)
	ma.Register(VochainVotesPerMinute)
	ma.Register(VochainVoteCache)
	ma.Register(VochainAccountTree)
	ma.Register(VochainSIKTree)
	ma.Register(VochainTokensBurned)
}

// setMetrics updates the metrics values to the current state
func (vi *VochainInfo) setMetrics() {
	VochainHeight.Set(float64(vi.Height()))
	VochainMempool.Set(float64(vi.MempoolSize()))
	p, v, vxm := vi.TreeSizes()
	VochainProcessTree.Set(float64(p))
	VochainVoteTree.Set(float64(v))
	VochainVotesPerMinute.Set(float64(vxm))
	VochainVoteCache.Set(float64(vi.VoteCacheSize()))
	VochainAccountTree.Set(float64(vi.AccountTreeSize()))
	VochainSIKTree.Set(float64(vi.SIKTreeSize()))
	VochainTokensBurned.Set(float64(vi.TokensBurned()))
}

// CollectMetrics constantly updates the metric values for prometheus
// The function is blocking, should be called in a go routine
// If the metrics Agent is nil, do nothing
func (vi *VochainInfo) CollectMetrics(ma *metrics.Agent) {
	if ma != nil {
		vi.registerMetrics(ma)
		for {
			time.Sleep(ma.RefreshInterval)
			vi.setMetrics()
		}
	}
}

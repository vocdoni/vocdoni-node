package vochaininfo

import (
	"github.com/prometheus/client_golang/prometheus"
	"go.vocdoni.io/dvote/metrics"
)

// registerMetrics registers each of the vochain prometheus metrics
func (vi *VochainInfo) registerMetrics() {
	metrics.Register(prometheus.NewGaugeFunc(prometheus.GaugeOpts{
		Namespace: "vochain",
		Name:      "height",
		Help:      "Height of the vochain (last block)",
	},
		func() float64 { return float64(vi.Height()) }))

	metrics.Register(prometheus.NewGaugeFunc(prometheus.GaugeOpts{
		Namespace: "vochain",
		Name:      "mempool",
		Help:      "Number of Txs in the mempool",
	},
		func() float64 { return float64(vi.MempoolSize()) }))

	metrics.Register(prometheus.NewGaugeFunc(prometheus.GaugeOpts{
		Namespace: "vochain",
		Name:      "process_tree",
		Help:      "Size of the process tree",
	},
		func() float64 { p, _, _ := vi.TreeSizes(); return float64(p) }))

	metrics.Register(prometheus.NewGaugeFunc(prometheus.GaugeOpts{
		Namespace: "vochain",
		Name:      "vote_tree",
		Help:      "Size of the vote tree",
	},
		func() float64 { _, v, _ := vi.TreeSizes(); return float64(v) }))

	metrics.Register(prometheus.NewGaugeFunc(prometheus.GaugeOpts{
		Namespace: "vochain",
		Name:      "vote_tree_increase_last_minute",
		Help:      "Number of votes included in the vote tree the last 60 seconds",
	},
		func() float64 { _, _, vxm := vi.TreeSizes(); return float64(vxm) }))

	metrics.Register(prometheus.NewGaugeFunc(prometheus.GaugeOpts{
		Namespace: "vochain",
		Name:      "vote_cache",
		Help:      "Size of the current vote cache",
	},
		func() float64 { return float64(vi.VoteCacheSize()) }))

	metrics.Register(prometheus.NewGaugeFunc(prometheus.GaugeOpts{
		Namespace: "vochain",
		Name:      "account_tree",
		Help:      "Size of the account tree",
	},
		func() float64 { return float64(vi.AccountTreeSize()) }))

	metrics.Register(prometheus.NewGaugeFunc(prometheus.GaugeOpts{
		Namespace: "vochain",
		Name:      "sik_tree",
		Help:      "Size of the SIK tree",
	},
		func() float64 { return float64(vi.SIKTreeSize()) }))

	metrics.Register(prometheus.NewGaugeFunc(prometheus.GaugeOpts{
		Namespace: "vochain",
		Name:      "tokens_burned",
		Help:      "Balance of the burn address",
	},
		func() float64 { return float64(vi.TokensBurned()) }))
}

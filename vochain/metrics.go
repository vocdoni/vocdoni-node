package vochain

import "github.com/prometheus/client_golang/prometheus"

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
)

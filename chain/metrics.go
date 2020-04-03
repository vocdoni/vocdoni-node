package chain

import "github.com/prometheus/client_golang/prometheus"

// Ethereum collectors
var (
	EthereumSynced = prometheus.NewGauge(prometheus.GaugeOpts{
		Namespace: "ethereum",
		Name:      "synced",
		Help:      "Boolean, 1 if chain is synced",
	})
	EthereumHeight = prometheus.NewGauge(prometheus.GaugeOpts{
		Namespace: "ethereum",
		Name:      "height",
		Help:      "Current height of the ethereum chain",
	})
	EthereumMaxHeight = prometheus.NewGauge(prometheus.GaugeOpts{
		Namespace: "ethereum",
		Name:      "max_height",
		Help:      "Height of the ethereum chain (last block)",
	})
	EthereumPeers = prometheus.NewGauge(prometheus.GaugeOpts{
		Namespace: "ethereum",
		Name:      "peers",
		Help:      "Number of ethereum peers connected",
	})
)

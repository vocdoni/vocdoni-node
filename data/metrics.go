package data

import "github.com/prometheus/client_golang/prometheus"

// File collectors
var (
	// FilePeers ...
	FilePeers = prometheus.NewGauge(prometheus.GaugeOpts{
		Namespace: "file",
		Name:      "peers",
		Help:      "The number of connected peers",
	})
	// FileAddresses ...
	FileAddresses = prometheus.NewGauge(prometheus.GaugeOpts{
		Namespace: "file",
		Name:      "addresses",
		Help:      "The number of registered addresses",
	})
	// FilePins ...
	FilePins = prometheus.NewGauge(prometheus.GaugeOpts{
		Namespace: "file",
		Name:      "pins",
		Help:      "The number of pinned files",
	})
)

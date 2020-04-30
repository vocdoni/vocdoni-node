package chain

import (
	"time"

	"github.com/prometheus/client_golang/prometheus"

	"gitlab.com/vocdoni/go-dvote/log"
	"gitlab.com/vocdoni/go-dvote/metrics"
)

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

// RegisterMetrics to the prometheus server
func (e *EthChainContext) registerMetrics(ma *metrics.Agent) {
	ma.Register(EthereumSynced)
	ma.Register(EthereumHeight)
	ma.Register(EthereumMaxHeight)
	ma.Register(EthereumPeers)
}

// GetMetrics grabs diferent metrics about ethereum chain.
func (e *EthChainContext) getMetrics() {
	info, err := e.SyncInfo()
	if err != nil {
		log.Warn(err)
		return
	}
	if info.Synced {
		EthereumSynced.Set(1)
	} else {
		EthereumSynced.Set(0)
	}
	EthereumHeight.Set(float64(info.Height))
	EthereumMaxHeight.Set(float64(info.MaxHeight))
	EthereumPeers.Set(float64(info.Peers))

}

// CollectMetrics constantly updates the metric values for prometheus
// The function is blocking, should be called in a go routine
// If the metrics Agent is nil, do nothing
func (e *EthChainContext) CollectMetrics(ma *metrics.Agent) {
	if ma != nil {
		e.registerMetrics(ma)
		for {
			time.Sleep(ma.RefreshInterval)
			e.getMetrics()
		}
	}
}

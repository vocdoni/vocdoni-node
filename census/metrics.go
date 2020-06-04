package census

import (
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"gitlab.com/vocdoni/go-dvote/metrics"
)

// Ethereum collectors
var (
	CensusLocal = prometheus.NewGauge(prometheus.GaugeOpts{
		Namespace: "census",
		Name:      "local",
		Help:      "Local created census",
	})
	CensusImported = prometheus.NewGauge(prometheus.GaugeOpts{
		Namespace: "census",
		Name:      "imported",
		Help:      "Remote imported census",
	})
	CensusLoaded = prometheus.NewGauge(prometheus.GaugeOpts{
		Namespace: "census",
		Name:      "loaded",
		Help:      "Loaded and active census",
	})
	CensusQueue = prometheus.NewGauge(prometheus.GaugeOpts{
		Namespace: "census",
		Name:      "queue",
		Help:      "Active queued census for importing",
	})
	CensusRetryQueue = prometheus.NewGauge(prometheus.GaugeOpts{
		Namespace: "census",
		Name:      "retryQueue",
		Help:      "Active queued census that failed but will be retried",
	})
)

// RegisterMetrics to the prometheus server
func (m *Manager) registerMetrics(ma *metrics.Agent) {
	ma.Register(CensusLocal)
	ma.Register(CensusImported)
	ma.Register(CensusLoaded)
	ma.Register(CensusQueue)
	ma.Register(CensusRetryQueue)
}

// GetMetrics to the prometheus server
func (m *Manager) getMetrics() {
	local, imported, loaded := m.Count()
	CensusLocal.Set(float64(local))
	CensusImported.Set(float64(imported))
	CensusLoaded.Set(float64(loaded))
	CensusQueue.Set(float64(m.ImportQueueSize()))
	CensusQueue.Set(float64(m.ImportFailedQueueSize()))
}

// CollectMetrics constantly updates the metric values for prometheus
// The function is blocking, should be called in a go routine
// If the metrics Agent is nil, do nothing
func (m *Manager) CollectMetrics(ma *metrics.Agent) {
	if ma != nil {
		m.registerMetrics(ma)
		for {
			time.Sleep(ma.RefreshInterval)
			m.getMetrics()
		}
	}
}

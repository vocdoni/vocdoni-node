package vochain

import (
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"gitlab.com/vocdoni/go-dvote/metrics"
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
	// VochainAppTree ...
	VochainVoteCache = prometheus.NewGauge(prometheus.GaugeOpts{
		Namespace: "vochain",
		Name:      "vote_cache",
		Help:      "Size of the current vote cache",
	})
)

func (app *BaseApplication) registerMetrics(ma *metrics.Agent) {
	ma.Register(VochainHeight)
	ma.Register(VochainMempool)
	ma.Register(VochainAppTree)
	ma.Register(VochainProcessTree)
	ma.Register(VochainVoteTree)
	ma.Register(VochainVoteCache)

}

func (app *BaseApplication) getMetrics() {
	VochainHeight.Set(float64(app.Node.BlockStore().Height()))
	VochainMempool.Set(float64(app.Node.Mempool().Size()))
	VochainAppTree.Set(float64(app.State.AppTree.Size()))
	VochainProcessTree.Set(float64(app.State.ProcessTree.Size()))
	VochainVoteTree.Set(float64(app.State.VoteTree.Size()))
	VochainVoteCache.Set(float64(app.State.VoteCacheSize()))
}

// CollectMetrics constantly updates the metric values for prometheus
// The function is blocking, should be called in a go routine
// If the metrics Agent is nil, do nothing
func (app *BaseApplication) CollectMetrics(ma *metrics.Agent) {
	if ma != nil {
		app.registerMetrics(ma)
		for {
			time.Sleep(ma.RefreshInterval)
			app.getMetrics()
		}
	}
}

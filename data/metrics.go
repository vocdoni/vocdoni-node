package data

import (
	"context"
	"time"

	"github.com/prometheus/client_golang/prometheus"

	"gitlab.com/vocdoni/go-dvote/metrics"
)

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

// RegisterMetrics to initialize the metrics to the agent
func (i *IPFSHandle) registerMetrics(ma *metrics.Agent) {
	ma.Register(FilePeers)
	ma.Register(FileAddresses)
	ma.Register(FilePins)
}

// GetMetrics to be called as a loop and grab metrics
func (i *IPFSHandle) getMetrics(ctx context.Context) error {
	peers, err := i.CoreAPI.Swarm().Peers(ctx)
	if err != nil {
		return err
	}
	FilePeers.Set(float64(len(peers)))
	addresses, err := i.CoreAPI.Swarm().KnownAddrs(ctx)
	if err != nil {
		return err
	}
	FileAddresses.Set(float64(len(addresses)))
	pins, err := i.countPins(ctx)
	if err != nil {
		return err
	}
	FilePins.Set(float64(pins))
	return nil
}

// CollectMetrics constantly updates the metric values for prometheus
// The function is blocking, should be called in a go routine
// If the metrics Agent is nil, do nothing
func (i *IPFSHandle) CollectMetrics(ctx context.Context, ma *metrics.Agent) error {
	if ma != nil {
		i.registerMetrics(ma)
		for {
			time.Sleep(ma.RefreshInterval)
			tctx, cancel := context.WithTimeout(ctx, time.Minute)
			err := i.getMetrics(tctx)
			cancel()
			if err != nil {
				return err
			}
		}
	}
	return nil
}

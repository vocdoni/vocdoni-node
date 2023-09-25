package ipfs

import (
	"context"
	"sync"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"go.uber.org/atomic"

	"go.vocdoni.io/dvote/metrics"
)

var stats struct {
	Peers      atomic.Float64
	KnownAddrs atomic.Float64
	Pins       atomic.Float64
}

// registerMetrics registers prometheus metrics
func (i *Handler) registerMetrics() {
	metrics.Register(prometheus.NewGaugeFunc(prometheus.GaugeOpts{
		Namespace: "file",
		Name:      "peers",
		Help:      "The number of connected peers",
	},
		stats.Peers.Load))

	metrics.Register(prometheus.NewGaugeFunc(prometheus.GaugeOpts{
		Namespace: "file",
		Name:      "addresses",
		Help:      "The number of registered addresses",
	},
		stats.KnownAddrs.Load))

	metrics.Register(prometheus.NewGaugeFunc(prometheus.GaugeOpts{
		Namespace: "file",
		Name:      "pins",
		Help:      "The number of pinned files",
	},
		stats.Pins.Load))
}

// updateStats constantly updates the ipfs stats (Peers, KnownAddrs, Pins)
// The function is blocking, should be called in a go routine
func (i *Handler) updateStats(interval time.Duration) {
	for {
		t := time.Now()

		ctx, cancel := context.WithTimeout(context.Background(), interval)

		var wg sync.WaitGroup

		wg.Add(1)
		go func() {
			list, err := i.CoreAPI.Swarm().Peers(ctx)
			if err == nil {
				stats.Peers.Store(float64(len(list)))
			}
			wg.Done()
		}()

		wg.Add(1)
		go func() {
			list, err := i.CoreAPI.Swarm().KnownAddrs(ctx)
			if err == nil {
				stats.KnownAddrs.Store(float64(len(list)))
			}
			wg.Done()
		}()

		wg.Add(1)
		go func() {
			count, err := i.countPins(ctx)
			if err == nil {
				stats.Pins.Store(float64(count))
			}
			wg.Done()
		}()

		wg.Wait()

		cancel()

		time.Sleep(interval - time.Since(t))
	}
}

package ipfs

import (
	"context"
	"sync"
	"time"

	"github.com/VictoriaMetrics/metrics"
)

var stats = struct {
	Peers      *metrics.Counter
	KnownAddrs *metrics.Counter
	Pins       *metrics.Counter
}{
	Peers:      metrics.NewCounter("file_peers"),
	KnownAddrs: metrics.NewCounter("file_addresses"),
	Pins:       metrics.NewCounter("file_pins"),
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
				stats.Peers.Set(uint64(len(list)))
			}
			wg.Done()
		}()

		wg.Add(1)
		go func() {
			list, err := i.CoreAPI.Swarm().KnownAddrs(ctx)
			if err == nil {
				stats.KnownAddrs.Set(uint64(len(list)))
			}
			wg.Done()
		}()

		wg.Add(1)
		go func() {
			count, err := i.countPins(ctx)
			if err == nil {
				stats.Pins.Set(uint64(count))
			}
			wg.Done()
		}()

		wg.Wait()

		cancel()

		time.Sleep(interval - time.Since(t))
	}
}

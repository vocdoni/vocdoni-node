package metrics

import (
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/vocdoni/multirpc/transports/mhttp"
	"go.vocdoni.io/dvote/log"
)

// Agent struct with options
type Agent struct {
	Path            string
	RefreshInterval time.Duration
}

// NewAgent creates and initializes the metrics agent with a HTTP server
func NewAgent(path string, interval time.Duration, proxy *mhttp.Proxy) *Agent {
	ma := Agent{Path: path, RefreshInterval: interval}
	proxy.AddHandler(path, promhttp.Handler().ServeHTTP)
	log.Infof("prometheus metrics ready at: %s", path)
	return &ma
}

// Register adds a prometheus collector
func (ma *Agent) Register(c prometheus.Collector) {
	err := prometheus.Register(c)
	if err != nil {
		log.Warnf("cannot register metrics: (%s) (%+v)", err, c)
	}
}

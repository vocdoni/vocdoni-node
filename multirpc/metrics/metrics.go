package metrics

import (
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"go.vocdoni.io/dvote/log"
	"go.vocdoni.io/dvote/multirpc/transports/mhttp"
)

// MetricsCfg initializes the metrics config
type Metrics struct {
	Enabled         bool
	RefreshInterval int
}

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

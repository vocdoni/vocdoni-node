package metrics

import (
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"go.vocdoni.io/dvote/httprouter"
	"go.vocdoni.io/dvote/log"
)

// Agent struct with options
type Agent struct {
	Path            string
	RefreshInterval time.Duration
}

// NewAgent creates and initializes the metrics agent with a HTTP server
func NewAgent(path string, interval time.Duration, router *httprouter.HTTProuter) *Agent {
	ma := Agent{Path: path, RefreshInterval: interval}
	router.AddRawHTTPHandler(path, "GET", promhttp.Handler().ServeHTTP)
	log.Infof("prometheus metrics ready at: %s", path)
	return &ma
}

// Register the provided prometheus collector, ignoring any error returned (simply logs a Warn)
func Register(c prometheus.Collector) {
	err := prometheus.Register(c)
	if err != nil {
		log.Warnf("cannot register metrics: (%s) (%+v)", err, c)
	}
}

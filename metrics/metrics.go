package metrics

import "time"

import "github.com/prometheus/client_golang/prometheus/promhttp"
import "github.com/prometheus/client_golang/prometheus"
import "gitlab.com/vocdoni/go-dvote/log"
import "gitlab.com/vocdoni/go-dvote/net"

// Agent struct with options
type Agent struct {
	Path            string
	RefreshInterval time.Duration
}

// NewAgent creates and initializes the metrics agent with a HTTP server
func NewAgent(path string, interval time.Duration, proxy *net.Proxy) *Agent {
	ma := Agent{Path: path, RefreshInterval: interval}
	proxy.AddHandler(path, promhttp.Handler().ServeHTTP)
	log.Infof("prometheus metrics ready at: %s", path)
	return &ma
}

// Register adds a prometheus collector
func (ma *Agent) Register(c prometheus.Collector) {
	err := prometheus.Register(c)
	if err != nil {
		log.Warnf("Cannot register metrics: %s", err)
	}
}

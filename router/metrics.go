package router

import (
	"github.com/prometheus/client_golang/prometheus"

	"go.vocdoni.io/dvote/metrics"
)

// Router collectors
var (
	// RouterPrivateReqs ...
	RouterPrivateReqs = prometheus.NewCounterVec(prometheus.CounterOpts{
		Namespace: "router",
		Name:      "private_reqs",
		Help:      "The number of private requests processed",
	}, []string{"method"})
	// RouterPublicReqs ...
	RouterPublicReqs = prometheus.NewCounterVec(prometheus.CounterOpts{
		Namespace: "router",
		Name:      "public_reqs",
		Help:      "The number of public requests processed",
	}, []string{"method"})
)

func (r *Router) registerMetrics(ma *metrics.Agent) {
	ma.Register(RouterPrivateReqs)
	ma.Register(RouterPublicReqs)
}

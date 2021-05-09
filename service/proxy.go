package service

import (
	"go.vocdoni.io/dvote/log"
	"go.vocdoni.io/dvote/multirpc/transports/mhttp"
)

// Proxy creates a new service for routing HTTP connections using go-chi server
// if tlsDomain is specified, it will use letsencrypt to fetch a valid TLS certificate
func Proxy(host string, port int, tlsDomain, tlsDir string) (*mhttp.Proxy, error) {
	pxy := mhttp.NewProxy()
	pxy.Conn.TLSdomain = tlsDomain
	pxy.Conn.TLScertDir = tlsDir
	pxy.Conn.Address = host
	pxy.Conn.Port = int32(port)
	log.Infof("creating proxy service, listening on %s:%d", host, port)
	if pxy.Conn.TLSdomain != "" {
		log.Infof("configuring proxy with TLS domain %s", tlsDomain)
	}
	return pxy, pxy.Init()
}

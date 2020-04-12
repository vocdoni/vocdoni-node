package service

import (
	"gitlab.com/vocdoni/go-dvote/log"
	"gitlab.com/vocdoni/go-dvote/net"
)

// Proxy creates a new service for routing HTTP connections using go-chi server
// if tlsDomain is specified, it will use letsencrypt to fetch a valid TLS certificate
func Proxy(host string, port int, tlsDomain, tlsDir string) (*net.Proxy, error) {
	pxy := net.NewProxy()
	pxy.C.SSLDomain = tlsDomain
	pxy.C.SSLCertDir = tlsDir
	pxy.C.Address = host
	pxy.C.Port = port
	log.Infof("creating proxy service, listening on %s:%d", host, port)
	if pxy.C.SSLDomain != "" {
		log.Infof("configuring proxy with TLS domain %s", tlsDomain)
	}
	return pxy, pxy.Init()
}

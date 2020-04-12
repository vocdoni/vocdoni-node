package service

import (
	"gitlab.com/vocdoni/go-dvote/log"
	"gitlab.com/vocdoni/go-dvote/net"
)

func Proxy(host string, port int, domain, dir string) (*net.Proxy, error) {
	pxy := net.NewProxy()
	pxy.C.SSLDomain = domain
	pxy.C.SSLCertDir = dir
	pxy.C.Address = host
	pxy.C.Port = port
	log.Infof("creating proxy service, listening on %s:%d", host, port)
	if pxy.C.SSLDomain != "" {
		log.Infof("configuring proxy with TLS domain %s", domain)
	}
	return pxy, pxy.Init()
}

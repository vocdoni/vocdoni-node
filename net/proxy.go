package net

import (
	"crypto/tls"
	"net/http"
	"strconv"
	"fmt"

	"golang.org/x/crypto/acme"
	"golang.org/x/crypto/acme/autocert"

	"gitlab.com/vocdoni/go-dvote/log"
)

// ProxyHandler function signature required to add a handler in the net/http Server
type ProxyHandler func(http.ResponseWriter, *http.Request)

// Proxy represents a proxy
type Proxy struct {
	Address    string //this node's address
	SSLDomain  string //ssl domain
	SSLCertDir string //ssl certificates directory
	Port       int    //specific port on which a transport should listen
}

// NewProxy creates a new proxy instance
func NewProxy() *Proxy {
	p := new(Proxy)
	return p
}

// Init checks if SSL is activated or not and runs a http server consequently
func (p *Proxy) Init() {

	/*
		http.HandleFunc("/ping", func(w http.ResponseWriter, r *http.Request) {
			fmt.Fprintf(w, "pong")
		})
	*/

	if p.SSLDomain != "" {
		s := p.GenerateSSLCertificate()
		go func() {
			log.Fatal(s.ListenAndServeTLS("", ""))
		}()
		log.Infof("Proxy with SSL initialized on domain %v, port %v", p.SSLDomain, strconv.Itoa(p.Port))
	}
	if p.SSLDomain == "" {
		s := &http.Server{
			Addr: p.Address + ":" + strconv.Itoa(p.Port),
		}
		go func() {
			log.Fatal(s.ListenAndServe())
		}()
		log.Infof("Proxy initialized, ssl not activated. Address: %v, Port: %v", p.Address, strconv.Itoa(p.Port))
	}
}

// GenerateSSLCertificate generates a SSL certificated for the proxy
func (p *Proxy) GenerateSSLCertificate() *http.Server {
	m := autocert.Manager{
		Prompt:     autocert.AcceptTOS,
		HostPolicy: autocert.HostWhitelist(p.SSLDomain),
		Cache:      autocert.DirCache(p.SSLCertDir),
	}

	serverConfig := &http.Server{
		Addr: fmt.Sprintf("%s:%d", p.Address, p.Port), // 443 ssl
		TLSConfig: &tls.Config{
			GetCertificate: m.GetCertificate,
		},
	}
	serverConfig.TLSConfig.NextProtos = append(serverConfig.TLSConfig.NextProtos, acme.ALPNProto)
	return serverConfig
}

// AddHandler adds a handler for the proxy
func (p *Proxy) AddHandler(path string, handler ProxyHandler) {
	http.HandleFunc(path, handler)
}

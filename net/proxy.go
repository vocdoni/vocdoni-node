package net

import (
	"crypto/tls"
	"fmt"
	"log"
	"net/http"
	"strconv"

	"golang.org/x/crypto/acme"
	"golang.org/x/crypto/acme/autocert"
)

type ProxyHandler func(http.ResponseWriter, *http.Request)

type Proxy struct {
	Address    string //this node's address
	SSLDomain  string //ssl domain
	SSLCertDir string //ssl certificates directory
	Port       int    //specific port on which a transport should listen
}

func NewProxy() *Proxy {
	p := new(Proxy)
	return p
}

func (p *Proxy) Init() {

	http.HandleFunc("/ping", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintf(w, "pong")
	})

	if p.SSLDomain != "" {
		s := p.GenerateSSLCertificate()
		go func() {
			log.Fatal(s.ListenAndServeTLS("", ""))
		}()
		log.Printf("Proxy with SSL initialized on %s", p.SSLDomain+":"+strconv.Itoa(p.Port))
	}
	if p.SSLDomain == "" {
		s := &http.Server{
			Addr: p.Address + ":" + strconv.Itoa(p.Port),
		}
		go func() {
			log.Fatal(s.ListenAndServe())
		}()
		log.Printf("Proxy initialized on %s, ssl not activated", p.Address+":"+strconv.Itoa(p.Port))
	}
}

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

func (p *Proxy) AddHandler(path string, handler ProxyHandler) {
	http.HandleFunc(path, handler)
}

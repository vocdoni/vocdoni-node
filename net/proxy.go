package net

import (
	"bytes"
	"crypto/tls"
	"fmt"
	"io/ioutil"
	"net/http"
	"strconv"

	"github.com/vocdoni/go-dvote/types"
	"golang.org/x/crypto/acme"
	"golang.org/x/crypto/acme/autocert"

	"gitlab.com/vocdoni/go-dvote/log"
)

// ProxyHandler function signature required to add a handler in the net/http Server
type ProxyHandler func(http.ResponseWriter, *http.Request)

// Proxy represents a proxy
type Proxy struct {
	C *types.Connection
}

// NewProxy creates a new proxy instance
func NewProxy() *Proxy {
	p := new(Proxy)
	return p
}

// Init checks if SSL is activated or not and runs a http server consequently
func (p *Proxy) Init() {
	if p.C.SSLDomain != "" {
		s := p.GenerateSSLCertificate()
		go func() {
			log.Fatal(s.ListenAndServeTLS("", ""))
		}()
		log.Infof("Proxy with SSL initialized on https://%s", p.C.SSLDomain+":"+strconv.Itoa(p.C.Port))
	}
	if p.C.SSLDomain == "" {
		s := &http.Server{
			Addr: p.C.Address + ":" + strconv.Itoa(p.C.Port),
		}
		go func() {
			log.Fatal(s.ListenAndServe())
		}()
		log.Infof("Proxy initialized on http://%s, ssl not activated", p.C.Address+":"+strconv.Itoa(p.C.Port))
	}
}

// GenerateSSLCertificate generates a SSL certificated for the proxy
func (p *Proxy) GenerateSSLCertificate() *http.Server {
	m := autocert.Manager{
		Prompt:     autocert.AcceptTOS,
		HostPolicy: autocert.HostWhitelist(p.C.SSLDomain),
		Cache:      autocert.DirCache(p.C.SSLCertDir),
	}

	serverConfig := &http.Server{
		Addr: fmt.Sprintf("%s:%d", p.C.Address, p.C.Port), // 443 ssl
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

// AddEndpoint adds an endpoint representing the url where the request will be handled
func (p *Proxy) AddEndpoint(host string, port int) func(writer http.ResponseWriter, reader *http.Request) {
	fn := func(writer http.ResponseWriter, reader *http.Request) {
		body, err := ioutil.ReadAll(reader.Body)
		if err != nil {
			panic(err)
		}
		var req *http.Request
		req, err = http.NewRequest(http.MethodPost, fmt.Sprintf("%s:%s", "http://"+host, strconv.Itoa(port)), bytes.NewReader(body))

		if err != nil {
			log.Infof("Cannot create http request: %s", err)
		}
		const contentType = "application/json"
		req.Header.Set("Content-Type", contentType)
		req.Header.Set("Accept", contentType)
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			log.Infof("Request failed: %s", err)
		}

		respBody, err := ioutil.ReadAll(resp.Body)
		if err != nil {
			log.Infof("Cannot read response: %s", err)
		}

		log.Infof("Response: %s", respBody)
	}
	return fn
}

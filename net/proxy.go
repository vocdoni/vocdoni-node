package net

import (
	"bytes"
	"crypto/tls"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"strconv"

	"golang.org/x/crypto/acme"
	"golang.org/x/crypto/acme/autocert"
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

// RegisterW3 enables the comunication with a running blockchain node through the proxy
func (p *Proxy) RegisterW3(w3httpHost string, w3httpPort int, sslDomain string) func(writer http.ResponseWriter, reader *http.Request) {
	fn := func(writer http.ResponseWriter, reader *http.Request) {
		body, err := ioutil.ReadAll(reader.Body)
		if err != nil {
			panic(err)
		}
		var req *http.Request
		if sslDomain == "" {
			req, err = http.NewRequest(http.MethodPost, fmt.Sprintf("%s:%s", "http://"+w3httpHost, strconv.Itoa(w3httpPort)), bytes.NewReader(body))
		} else {
			req, err = http.NewRequest(http.MethodPost, fmt.Sprintf("%s:%s", "https://"+w3httpHost, strconv.Itoa(w3httpPort)), bytes.NewReader(body))
		}
		if err != nil {
			log.Printf("Cannot create Web3 request: %s", err)
		}
		const contentType = "application/json"
		req.Header.Set("Content-Type", contentType)
		req.Header.Set("Accept", contentType)
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			log.Printf("Web3 request failed: %s", err)
		}

		respBody, err := ioutil.ReadAll(resp.Body)
		if err != nil {
			log.Printf("Cannot read Web3 node response: %s", err)
		}

		log.Printf("web3 response: %s", respBody)
	}

	return fn
}

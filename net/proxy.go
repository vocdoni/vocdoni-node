package net

import (
	"bytes"
	"crypto/tls"
	"fmt"
	"io/ioutil"
	"net/http"
	"strconv"
	"time"

	"gitlab.com/vocdoni/go-dvote/types"
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
	p.C = new(types.Connection)
	return p
}

func getCertificates(domain string, m *autocert.Manager) [][]byte {
	hello := &tls.ClientHelloInfo{
		ServerName:   domain,
		CipherSuites: []uint16{tls.TLS_ECDHE_RSA_WITH_CHACHA20_POLY1305},
	}
	hello.CipherSuites = append(hello.CipherSuites, tls.TLS_ECDHE_ECDSA_WITH_CHACHA20_POLY1305)

	cert, err := m.GetCertificate(hello)
	if err != nil {
		return nil
	}
	return cert.Certificate
}

// Init checks if SSL is activated or not and runs a http server consequently
func (p *Proxy) Init() error {
	var s *http.Server
	var m *autocert.Manager
	forceNonTLS := true

	if len(p.C.SSLDomain) > 0 {
		s, m = p.GenerateSSLCertificate()
		go func() {
			log.Warn(s.ListenAndServeTLS("", ""))
		}()

		time.Sleep(time.Second * 5)
		certs := getCertificates(p.C.SSLDomain, m)
		if certs == nil {
			log.Warn("letsencrypt TLS certificate cannot be obtained. Maybe port 443 is not accessible or domain name is wrong.")
			log.Infof(`you might redirect port 443 with iptables using the following command (required just the first time): 
								sudo iptables -t nat -I PREROUTING -p tcp --dport 443 -j REDIRECT --to-ports %d`, p.C.Port)
			s.Close()
		} else {
			forceNonTLS = false
			log.Infof("proxy with SSL initialized on https://%s", p.C.SSLDomain+":"+strconv.Itoa(p.C.Port))
		}
	}
	if forceNonTLS {
		s = &http.Server{
			Addr: p.C.Address + ":" + strconv.Itoa(p.C.Port),
		}
		go func() {
			log.Fatal(s.ListenAndServe())
		}()
		log.Infof("proxy initialized on http://%s, ssl not activated", p.C.Address+":"+strconv.Itoa(p.C.Port))

	}

	return nil
}

// GenerateSSLCertificate generates a SSL certificated for the proxy
func (p *Proxy) GenerateSSLCertificate() (*http.Server, *autocert.Manager) {
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

	return serverConfig, &m
}

// AddHandler adds a handler for the proxy
func (p *Proxy) AddHandler(path string, handler ProxyHandler) {
	http.HandleFunc(path, handler)
}

// AddEndpoint adds an endpoint representing the url where the request will be handled
func (p *Proxy) AddEndpoint(url string) func(writer http.ResponseWriter, reader *http.Request) {
	fn := func(writer http.ResponseWriter, reader *http.Request) {
		body, err := ioutil.ReadAll(reader.Body)
		if err != nil {
			panic(err)
		}
		var req *http.Request
		log.Debugf("%s", url)
		req, err = http.NewRequest(reader.Method, url, bytes.NewReader(body))

		if err != nil {
			log.Warnf("cannot create request: %s", err)
		}

		req.Header.Set("Content-Type", reader.Header.Get("Content-Type"))
		req.Header.Set("Accept", reader.Header.Get("Accept"))
		req.Header.Set("Content-Length", reader.Header.Get("Content-Length"))
		req.Header.Set("User-Agent", reader.Header.Get("User-Agent"))

		resp, err := http.DefaultClient.Do(req)

		if err != nil {
			log.Warnf("request failed: %s", err)
		}

		respBody, err := ioutil.ReadAll(resp.Body)
		if err != nil {
			log.Infof("cannot read response: %s", err)
		}
		writer.Header().Set("Access-Control-Allow-Origin", "*")
		writer.Write(respBody)
		log.Debugf("response: %s", respBody)
	}
	return fn
}

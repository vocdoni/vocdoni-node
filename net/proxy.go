package net

import (
	"bytes"
	"crypto/tls"
	"fmt"
	"io/ioutil"
	"net"
	"net/http"

	"golang.org/x/crypto/acme"
	"golang.org/x/crypto/acme/autocert"

	"gitlab.com/vocdoni/go-dvote/types"

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
//
// When it returns, the server is ready. The returned address is useful if the
// port was left as 0, to retrieve the randomly allocated port.
func (p *Proxy) Init() (net.Addr, error) {
	var s *http.Server
	var m *autocert.Manager

	ln, err := net.Listen("tcp", fmt.Sprintf("%s:%d", p.C.Address, p.C.Port))
	if err != nil {
		return nil, err
	}
	addr := ln.Addr()
	log.Infof("proxy listening on %s", addr)

	if len(p.C.SSLDomain) > 0 {
		log.Infof("fetching letsencrypt TLS certificate for %s", p.C.SSLDomain)
		s, m = p.GenerateSSLCertificate()
		go func() {
			log.Fatal(s.ServeTLS(ln, "", ""))
		}()

		certs := getCertificates(p.C.SSLDomain, m)
		if len(certs) == 0 {
			log.Warnf(`letsencrypt TLS certificate cannot be obtained. Maybe port 443 is not accessible or domain name is wrong.
						You might want to redirect port 443 with iptables using the following command:
						sudo iptables -t nat -I PREROUTING -p tcp --dport 443 -j REDIRECT --to-ports %d`, p.C.Port)
			s.Close()
			return nil, fmt.Errorf("cannot get letsencrypt TLS certificate")
		} else {
			log.Infof("proxy with TLS ready at https://%s:%d", p.C.SSLDomain, p.C.Port)
		}
	} else {
		s = &http.Server{}
		go func() {
			log.Fatal(s.Serve(ln))
		}()
		log.Infof("proxy ready at http://%s", addr)
	}
	return addr, nil
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
			http.Error(writer, "", http.StatusInternalServerError)
			log.Errorf("failed to read request body: %v", err)
			return
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
		writer.Header().Set("Access-Control-Allow-Methods", "POST, GET")
		writer.Header().Set("Access-Control-Allow-Headers", "Accept, Content-Type, Content-Length, Accept-Encoding, X-CSRF-Token")
		writer.Write(respBody)
		log.Debugf("response: %s", respBody)
	}
	return fn
}

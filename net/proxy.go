package net

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net"
	"net/http"
	"time"

	reuse "github.com/libp2p/go-reuseport"
	"github.com/p4u/recws"
	"gitlab.com/vocdoni/go-dvote/log"
	"gitlab.com/vocdoni/go-dvote/types"
	"go.uber.org/zap"
	"golang.org/x/crypto/acme"
	"golang.org/x/crypto/acme/autocert"

	"github.com/go-chi/chi"
	"github.com/go-chi/chi/middleware"
	"github.com/go-chi/cors"

	"nhooyr.io/websocket"
)

const desiredSoMaxConn = 4096

// ProxyWsHandler function signature required to add a handler in the net/http Server
type ProxyWsHandler func(c *websocket.Conn)

// Proxy represents a proxy
type Proxy struct {
	C      *types.Connection
	Server *chi.Mux
	Addr   net.Addr
}

// NewProxy creates a new proxy instance
func NewProxy() *Proxy {
	p := new(Proxy)
	p.C = new(types.Connection)
	return p
}

func getCertificates(domain string, m *autocert.Manager) ([][]byte, error) {
	hello := &tls.ClientHelloInfo{
		ServerName:   domain,
		CipherSuites: []uint16{tls.TLS_ECDHE_RSA_WITH_CHACHA20_POLY1305},
	}
	hello.CipherSuites = append(hello.CipherSuites, tls.TLS_ECDHE_ECDSA_WITH_CHACHA20_POLY1305)

	cert, err := m.GetCertificate(hello)
	if err != nil {
		return nil, err
	}
	return cert.Certificate, nil
}

type stdLogger struct {
	log *zap.SugaredLogger
}

func (l stdLogger) Print(v ...interface{}) { l.log.Info(v...) }

// Init checks if SSL is activated or not and runs a http server consequently
//
// When it returns, the server is ready. The returned address is useful if the
// port was left as 0, to retrieve the randomly allocated port.
func (p *Proxy) Init() error {
	ln, err := reuse.Listen("tcp", fmt.Sprintf("%s:%d", p.C.Address, p.C.Port))
	if err != nil {
		return err
	}

	if n := somaxconn(); n < desiredSoMaxConn {
		log.Warnf("operating system SOMAXCONN is smaller than recommended (%d). "+
			"Consider increasing it: echo %d | sudo tee /proc/sys/net/core/somaxconn", n, desiredSoMaxConn)
	}

	p.Server = chi.NewRouter()
	p.Server.Use(middleware.RealIP)
	// If we want rich logging (e.g. with fields), we could implement our
	// own version of DefaultLogFormatter.
	p.Server.Use(middleware.RequestLogger(&middleware.DefaultLogFormatter{
		Logger:  stdLogger{log.Logger()},
		NoColor: true,
	}))
	p.Server.Use(middleware.Recoverer)
	p.Server.Use(middleware.Throttle(5000))
	p.Server.Use(middleware.Timeout(30 * time.Second))
	cors := cors.New(cors.Options{
		AllowedOrigins: []string{"*"},
		AllowedMethods: []string{"GET", "POST", "OPTIONS"},
		AllowedHeaders: []string{"Content-Type"},
		MaxAge:         300, // Maximum value not ignored by any of major browsers
	})
	p.Server.Use(cors.Handler)

	p.Server.Get("/ping", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("pong"))
	})

	if len(p.C.SSLDomain) > 0 {
		log.Infof("fetching letsencrypt TLS certificate for %s", p.C.SSLDomain)
		s, m := p.GenerateSSLCertificate()
		s.ReadTimeout = 5 * time.Second
		s.WriteTimeout = 10 * time.Second
		s.IdleTimeout = 30 * time.Second
		s.ReadHeaderTimeout = 2 * time.Second
		s.Handler = p.Server
		log.Info("starting go-chi https server")
		go func() {
			log.Fatal(s.ServeTLS(ln, "", ""))
		}()
		certs, err := getCertificates(p.C.SSLDomain, m)
		if len(certs) == 0 || err != nil {
			log.Warnf(`letsencrypt TLS certificate cannot be obtained. Maybe port 443 is not accessible or domain name is wrong.
							You might want to redirect port 443 with iptables using the following command:
							sudo iptables -t nat -I PREROUTING -p tcp --dport 443 -j REDIRECT --to-ports %d`, p.C.Port)
			return fmt.Errorf("cannot get letsencrypt TLS certificate: (%s)", err)
		}
		log.Infof("proxy ready at https://%s", ln.Addr())

	} else {
		log.Info("starting go-chi http server")
		s := &http.Server{
			ReadTimeout:       5 * time.Second,
			WriteTimeout:      10 * time.Second,
			IdleTimeout:       60 * time.Second,
			ReadHeaderTimeout: 2 * time.Second,
			Handler:           p.Server,
		}
		go func() {
			log.Fatal(s.Serve(ln))
		}()
		log.Infof("proxy ready at http://%s", ln.Addr())
	}
	p.Addr = ln.Addr()
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

// AddWsHandler adds a websocket handler in the proxy
func (p *Proxy) AddWsHandler(path string, handler ProxyWsHandler) {
	p.Server.HandleFunc(path, func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Upgrade") == "websocket" {
			wshandler(w, r, handler)
		} else {
			log.Warn("receied a non upgrade websockets connection to a only WS endpoint")
		}
	})
}

// AddMixedHandler adds a mixed (websockets and HTTP) handler in the proxy
func (p *Proxy) AddMixedHandler(path string, HTTPhandler http.HandlerFunc, WShandler ProxyWsHandler) {
	p.Server.HandleFunc(path, func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Upgrade") == "websocket" {
			wshandler(w, r, WShandler)
		} else {
			HTTPhandler(w, r)
		}
	})
}

// AddHandler adds a HTTP handler in the proxy
func (p *Proxy) AddHandler(path string, handler http.HandlerFunc) {
	p.Server.HandleFunc(path, func(w http.ResponseWriter, r *http.Request) {
		handler(w, r)
	})
}

// AddEndpoint adds an endpoint representing the url where the request will be handled
func (p *Proxy) AddEndpoint(url string) func(writer http.ResponseWriter, reader *http.Request) {
	// TODO(mvdan): this should just be a reverse proxy
	fn := func(writer http.ResponseWriter, reader *http.Request) {
		body, err := ioutil.ReadAll(reader.Body)
		if err != nil {
			http.Error(writer, "", http.StatusInternalServerError)
			log.Errorf("failed to read request body: %v", err)
			return
		}
		log.Debugf("%s", url)
		req, err := http.NewRequest(reader.Method, url, bytes.NewReader(body))
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

func (p *Proxy) ProxyIPC(path string) http.HandlerFunc {
	return http.HandlerFunc(func(rw http.ResponseWriter, r *http.Request) {
		log.Debugf("%s", path)
		body, err := ioutil.ReadAll(r.Body)
		if err != nil {
			http.Error(rw, "", http.StatusInternalServerError)
			log.Errorf("failed to read request body: %v", err)
			return
		}
		log.Debugf("request: %s", body)
		conn, err := net.Dial("unix", path)
		if err != nil {
			http.Error(rw, "could not dial IPC socket", http.StatusInternalServerError)
			log.Errorf("could not dial IPC socket: %v", err)
			return
		}
		if _, err := conn.Write(body); err != nil {
			http.Error(rw, "could not write to IPC socket", http.StatusInternalServerError)
			log.Errorf("could not write to IPC socket: %v", err)
			return
		}
		// We can't use ioutil.ReadAll, because the server keeps the
		// connection open for more requests. Read a single json object.
		var respBody json.RawMessage
		if err := json.NewDecoder(conn).Decode(&respBody); err != nil {
			http.Error(rw, "could not read response from IPC socket", http.StatusInternalServerError)
			log.Errorf("could not read response from IPC socket: %v", err)
			return
		}
		rw.Header().Set("Access-Control-Allow-Origin", "*")
		rw.Header().Set("Access-Control-Allow-Methods", "POST, GET")
		rw.Header().Set("Access-Control-Allow-Headers", "Accept, Content-Type, Content-Length, Accept-Encoding, X-CSRF-Token")
		rw.Write(respBody)
		rw.Write([]byte("\n")) // a json object doesn't include a newline
		log.Debugf("response: %s", respBody)
	})
}

// AddWsHTTPBridge adds a WS endpoint to interact with the underlying web3
func (p *Proxy) AddWsHTTPBridge(url string) ProxyWsHandler {
	return func(c *websocket.Conn) {
		for {
			msgType, msg, err := c.Reader(context.TODO())
			if err != nil {
				log.Debugf("websocket closed by the client: %s", err)
				c.Close(websocket.StatusAbnormalClosure, "ws closed by client")
				return
			}
			req, err := http.NewRequest("POST", url, msg)
			if err != nil {
				log.Warnf("invalid request: %s", err)
				continue
			}
			req.Header.Set("Content-Type", "application/json")
			req.Header.Set("Accept", "application/json")
			resp, err := http.DefaultClient.Do(req)
			if err != nil {
				log.Warnf("request failed: %s", err)
				continue
			}
			respBody, err := ioutil.ReadAll(resp.Body)
			if err != nil {
				log.Warnf("cannot read response: %s", err)
				continue
			}
			if err := c.Write(context.TODO(), msgType, respBody); err != nil {
				log.Warnf("cannot write message: %s", err)
			}
		}
	}
}

// AddWsWsBridge adds a WS endpoint to interact with the underlying web3
func (p *Proxy) AddWsWsBridge(url string) ProxyWsHandler {
	return func(wslocal *websocket.Conn) {
		ws := recws.RecConn{
			KeepAliveTimeout: 10 * time.Second,
		}
		ws.Dial(url, nil)
		ws.SetReadLimit(int64(22 * 1024 * 1024)) // tendermint needs up to 20 MB
		ctx, cancel := context.WithCancel(context.Background())
		// Read from remote and write to local
		go func() {
			for {
				select {
				case <-ctx.Done():
					log.Debugf("websocket connection to %s closed, received context Done", url)
					return
				default:
					t, data, err := ws.ReadMessage()
					if err != nil {
						log.Debugf("websocket connection to %s closed", url)
						return
					}
					ctx2, cancel := context.WithTimeout(ctx, time.Second*10)
					if err := wslocal.Write(ctx2, websocket.MessageType(t), data); err != nil {
						log.Warnf("cannot write message to local websocket: (%s)", err)
						cancel()
						return
					}
					cancel()
				}
			}
		}()

		// Read local messages and write to remote
		for {
			msgType, msg, err := wslocal.Reader(context.TODO())
			if err != nil {
				log.Debugf("websocket closed by the client: %s", err)
				break
			}
			respBody, err := ioutil.ReadAll(msg)
			if err != nil {
				log.Warnf("cannot read response: %s", err)
				continue
			}
			if err := ws.WriteMessage(int(msgType), respBody); err != nil {
				break
			}
		}
		cancel()
		wslocal.Close(websocket.StatusAbnormalClosure, "ws closed by client")
		ws.Close()
	}
}

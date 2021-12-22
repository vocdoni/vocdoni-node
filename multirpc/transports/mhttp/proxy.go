package mhttp

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"strconv"
	"time"

	reuse "github.com/libp2p/go-reuseport"
	"github.com/p4u/recws"
	"go.uber.org/zap"
	"go.vocdoni.io/dvote/log"
	"go.vocdoni.io/dvote/multirpc/transports"
	"golang.org/x/crypto/acme"
	"golang.org/x/crypto/acme/autocert"
	"golang.org/x/net/http2"

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
	Conn      *transports.Connection
	Server    *chi.Mux
	Addr      net.Addr
	TLSConfig *tls.Config
}

// NewProxy creates a new proxy instance
func NewProxy() *Proxy {
	p := new(Proxy)
	p.Conn = new(transports.Connection)
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

func (l stdLogger) Print(v ...interface{}) { l.log.Debug(v...) }

// Init checks if SSL is activated or not and runs a http server consequently
//
// When it returns, the server is ready. The returned address is useful if the
// port was left as 0, to retrieve the randomly allocated port.
func (p *Proxy) Init() error {
	ln, err := reuse.Listen("tcp", net.JoinHostPort(p.Conn.Address, strconv.Itoa(int(p.Conn.Port))))
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
	p.Server.Use(middleware.Heartbeat("/ping"))
	p.Server.Use(middleware.ThrottleBacklog(5000, 40000, 30*time.Second))
	p.Server.Use(middleware.Timeout(30 * time.Second))
	cors := cors.New(cors.Options{
		AllowOriginFunc: func(r *http.Request, origin string) bool {
			return true
		}, // Kind of equivalent to AllowedOrigin: []string{"*"} but it returns the origin as allowed origin.
		AllowedMethods:   []string{"GET", "POST", "OPTIONS"},
		AllowedHeaders:   []string{"Content-Type"},
		AllowCredentials: true,
		MaxAge:           300, // Maximum value not ignored by any of major browsers
	})
	p.Server.Use(cors.Handler)

	if len(p.Conn.TLSdomain) > 0 {
		log.Infof("fetching letsencrypt TLS certificate for %s", p.Conn.TLSdomain)
		s, m := p.GenerateSSLCertificate(p.TLSConfig)
		s.ReadTimeout = 20 * time.Second
		s.WriteTimeout = 15 * time.Second
		s.IdleTimeout = 10 * time.Second
		s.ReadHeaderTimeout = 5 * time.Second
		s.Handler = p.Server
		if err := http2.ConfigureServer(s, nil); err != nil {
			return err
		}
		go func() {
			log.Info("starting go-chi https server")
			log.Fatal(s.ServeTLS(ln, "", ""))
		}()
		certs, err := getCertificates(p.Conn.TLSdomain, m)
		if len(certs) == 0 || err != nil {
			log.Warnf(`letsencrypt TLS certificate cannot be obtained. Maybe port 443 is not accessible or domain name is wrong.
							You might want to redirect port 443 with iptables using the following command:
							sudo iptables -t nat -I PREROUTING -p tcp --dport 443 -j REDIRECT --to-ports %d`, p.Conn.Port)
			return fmt.Errorf("cannot get letsencrypt TLS certificate: (%s)", err)
		}
		log.Infof("proxy ready at https://%s", ln.Addr())

	} else {
		log.Info("starting go-chi http server")
		s := &http.Server{
			ReadTimeout:       10 * time.Second,
			WriteTimeout:      10 * time.Second,
			IdleTimeout:       10 * time.Second,
			ReadHeaderTimeout: 3 * time.Second,
			Handler:           p.Server,
		}
		if err := http2.ConfigureServer(s, nil); err != nil {
			return err
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
func (p *Proxy) GenerateSSLCertificate(tlsConfig *tls.Config) (*http.Server, *autocert.Manager) {
	m := autocert.Manager{
		Prompt:     autocert.AcceptTOS,
		HostPolicy: autocert.HostWhitelist(p.Conn.TLSdomain),
		Cache:      autocert.DirCache(p.Conn.TLScertDir),
	}
	if tlsConfig == nil {
		tlsConfig = &tls.Config{}
	}
	tlsConfig.GetCertificate = m.GetCertificate
	serverConfig := &http.Server{
		Addr:      net.JoinHostPort(p.Conn.Address, strconv.Itoa(int(p.Conn.Port))), // 443 tls
		TLSConfig: tlsConfig,
	}
	serverConfig.TLSConfig.NextProtos = append(serverConfig.TLSConfig.NextProtos, acme.ALPNProto)

	return serverConfig, &m
}

// AddWsHandler adds a websocket handler in the proxy
func (p *Proxy) AddWsHandler(path string, handler ProxyWsHandler, readLimit int64) {
	p.Server.HandleFunc(path, func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Upgrade") == "websocket" {
			wshandler(w, r, handler, readLimit)
		} else {
			log.Warn("receied a non-upgrade websockets connection to a WS-only endpoint")
		}
	})
}

// AddMixedHandler adds a mixed (websockets and HTTP) handler in the proxy
func (p *Proxy) AddMixedHandler(path string, HTTPhandler http.HandlerFunc, WShandler ProxyWsHandler, WSreadLimit int64) {
	p.Server.HandleFunc(path, func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Upgrade") == "websocket" {
			wshandler(w, r, WShandler, WSreadLimit)
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
		body, err := io.ReadAll(reader.Body)
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

		respBody, err := io.ReadAll(resp.Body)
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
		body, err := io.ReadAll(r.Body)
		if err != nil {
			http.Error(rw, "", http.StatusInternalServerError)
			log.Errorf("failed to read request body: %v", err)
			return
		}
		log.Debugf("ipc request: %s", body)
		conn, err := net.Dial("unix", path)
		if err != nil {
			http.Error(rw, "could not dial IPC socket", http.StatusInternalServerError)
			log.Errorf("could not dial IPC socket: %v", err)
			return
		}
		if _, err := conn.Write(append(body, byte('\n'))); err != nil { // Write a \n because some readers split by lines
			http.Error(rw, "could not write to IPC socket", http.StatusInternalServerError)
			log.Errorf("could not write to IPC socket: %v", err)
			return
		}

		// We can't use io.ReadAll, because the server keeps the
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
		rw.Write([]byte("\n"))
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
			respBody, err := io.ReadAll(resp.Body)
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

// AddWsWsBridge adds a WS endpoint to interact with the underlying websocket server
func (p *Proxy) AddWsWsBridge(url string, readLimit int64) ProxyWsHandler {
	return func(wsServer *websocket.Conn) {
		// connection to web3 or vochain
		wsClient := recws.RecConn{
			KeepAliveTimeout: 10 * time.Second,
		}
		wsClient.Dial(url, nil)
		attempts := 50
		for wsClient.Conn == nil && attempts > 0 {
			time.Sleep(time.Millisecond * 100)
			attempts--
		}
		if wsClient.Conn == nil {
			log.Errorf("websocket bridge cannot connect with %s", url)
			wsServer.Close(websocket.StatusAbnormalClosure, "cannot reach destination")
			return
		}
		wsClient.SetReadLimit(readLimit)
		// Read from remote and write to local
		go func() {
			for {
				// TODO: ReadMessage acepting context
				t, data, err := wsClient.ReadMessage()
				if err != nil {
					log.Debugf("websocket connection to %s closed", url)
					return
				}
				if err := wsServer.Write(context.TODO(), websocket.MessageType(t), data); err != nil {
					log.Warnf("cannot write message to local websocket: (%s)", err)
					return
				}
			}
		}()

		// Read local messages and write to remote
		for {
			msgType, msg, err := wsServer.Reader(context.TODO())
			if err != nil {
				log.Debugf("websocket closed by the client: %s", err)
				break
			}
			respBody, err := io.ReadAll(msg)
			if err != nil {
				log.Warnf("cannot read response: %s", err)
				continue
			}
			if err := wsClient.WriteMessage(int(msgType), respBody); err != nil {
				break
			}
		}
		wsServer.Close(websocket.StatusAbnormalClosure, "ws closed by client")
		wsClient.Close()
	}
}

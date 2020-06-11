package net

import (
	"bytes"
	"context"
	"crypto/tls"
	"fmt"
	"io/ioutil"
	"net"
	"net/http"
	"strconv"
	"strings"
	"syscall"
	"time"

	reuse "github.com/libp2p/go-reuseport"
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
		wshandler(w, r, handler)
	})
}

// AddHandler adds a handler in the proxy
func (p *Proxy) AddHandler(path string, handler http.HandlerFunc) {
	p.Server.HandleFunc(path, func(w http.ResponseWriter, r *http.Request) {
		handler(w, r)
	})
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
		wsremote := newWsPoll()
		wsremote.addServer(url)
		if err := wsremote.dial(); err != nil {
			log.Errorf("dial failed: (%s)", err)
			return
		}
		go wsremote.read()
		go func() {
			for {
				msgType, msg, err := wslocal.Reader(context.TODO())
				if err != nil {
					log.Debugf("websocket closed by the client: %s", err)
					wslocal.Close(websocket.StatusAbnormalClosure, "ws closed by client")
					return
				}
				respBody, err := ioutil.ReadAll(msg)
				if err != nil {
					log.Warnf("cannot read response: %s", err)
					continue
				}
				if err := wsremote.write(msgType, respBody); err != nil {
					log.Fatal(err)
					return
				}
			}
		}()
		for {
			wsData := <-wsremote.readChan
			respBody, err := ioutil.ReadAll(wsData.reader)
			if err != nil {
				log.Warnf("cannot read response: %s", err)
				continue
			}
			ctx, cancel := context.WithTimeout(context.Background(), time.Second*10)
			if err := wslocal.Write(ctx, wsData.msgType, respBody); err != nil {
				log.Warnf("cannot write message to local websocket: (%s)", err)
			}
			cancel()
		}
	}
}

// WebsocketHandle handles the websockets connection on the go-dvote proxy
type WebsocketHandle struct {
	Connection *types.Connection // the ws connection
	WsProxy    *Proxy            // proxy where the ws will be associated

	internalReceiver chan types.Message
}

type WebsocketContext struct {
	Conn *websocket.Conn
}

func (c WebsocketContext) ConnectionType() string {
	return "Websocket"
}

// SetProxy sets the proxy for the ws
func (w *WebsocketHandle) SetProxy(p *Proxy) {
	w.WsProxy = p
}

// Init initializes the websockets handler and the internal channel to communicate with other go-dvote components
func (w *WebsocketHandle) Init(c *types.Connection) error {
	w.internalReceiver = make(chan types.Message, 1)
	return nil
}

// AddProxyHandler adds the current websocket handler into the Proxy
func (w *WebsocketHandle) AddProxyHandler(path string) {
	serveWs := func(conn *websocket.Conn) {
		// Read websocket messages until the connection is closed. HTTP
		// handlers are run in new goroutines, so we don't need to spawn
		// another goroutine.
		for {
			_, payload, err := conn.Read(context.TODO())
			if err != nil {
				conn.Close(websocket.StatusAbnormalClosure, "ws closed by client")
				break
			}
			msg := types.Message{
				Data:      payload,
				TimeStamp: int32(time.Now().Unix()),
				Context:   &WebsocketContext{Conn: conn},
				Namespace: path,
			}
			w.internalReceiver <- msg
		}
	}
	w.WsProxy.AddWsHandler(path, serveWs)
}

// Listen will listen the websockets handler and write the received data into the channel
func (w *WebsocketHandle) Listen(receiver chan<- types.Message) {
	for {
		msg := <-w.internalReceiver
		receiver <- msg
	}
}

// Listen will listen the websockets handler and write the received data into the channel
func (w *WebsocketHandle) AddNamespace(namespace string) {
	w.AddProxyHandler(namespace)
}

// Send sends the response given a message
func (w *WebsocketHandle) Send(msg types.Message) {
	clientConn := msg.Context.(*WebsocketContext).Conn
	clientConn.Write(context.TODO(), websocket.MessageBinary, msg.Data)
}

func (w *WebsocketHandle) SendUnicast(address string, msg types.Message) {
	// WebSocket is not p2p so sendUnicast makes the same of Send()
	w.Send(msg)
}

func (w *WebsocketHandle) SetBootnodes(bootnodes []string) {
	// No bootnodes on websockets handler
}

func (w *WebsocketHandle) AddPeer(peer string) error {
	// No peers on websockets handler
	return nil
}

func (w *WebsocketHandle) Address() string {
	return w.Connection.Address
}

func (w *WebsocketHandle) String() string {
	return w.WsProxy.Addr.String()
}

func wshandler(w http.ResponseWriter, r *http.Request, ph ProxyWsHandler) {
	conn, err := websocket.Accept(w, r, &websocket.AcceptOptions{OriginPatterns: []string{"*"}})
	if err != nil {
		log.Errorf("failed to set websocket upgrade: %s", err)
		return
	}
	ph(conn)
}

func somaxconn() int {
	content, err := ioutil.ReadFile("/proc/sys/net/core/somaxconn")
	if err != nil {
		return syscall.SOMAXCONN
	}
	n, err := strconv.Atoi(strings.Trim(fmt.Sprintf("%s", content), "\n"))
	if err != nil {
		return syscall.SOMAXCONN
	}
	return n
}

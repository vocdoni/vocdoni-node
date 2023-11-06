package httprouter

import (
	"crypto/tls"
	"fmt"
	"net"
	"net/http"
	"os"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	chiprometheus "github.com/766b/chi-prometheus"
	"github.com/VictoriaMetrics/metrics"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/cors"
	reuse "github.com/libp2p/go-reuseport"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/rs/zerolog"
	"go.vocdoni.io/dvote/log"
	"golang.org/x/crypto/acme"
	"golang.org/x/crypto/acme/autocert"
	"golang.org/x/net/http2"
)

const (
	desiredSoMaxConn = 4096
)

// HTTProuter is a thread-safe multiplexer http(s) router using go-chi and autocert with a set of
// preconfigured options. The router abstracts the HTTP layer and uses a custom Message type that
// allows create handlers in a comfortable manner.
// Allows multiple different kind of data processors by using the RouterNamespace interface. Each processor
// is identified by a unique namespace string which must be specified when adding handlers.
// Handlers can be Public, Private and Administrator. The proper checks must be implemented by the
// RouterNamespace implementation.
type HTTProuter struct {
	Mux            *chi.Mux
	TLSconfig      *tls.Config
	TLSdomain      string
	TLSdirCert     string
	address        net.Addr
	namespaces     map[string]RouterNamespace
	namespacesLock sync.RWMutex
}

type AuthAccessType int

const (
	AccessTypePublic AuthAccessType = iota
	AccessTypeQuota
	AccessTypePrivate
	AccessTypeAdmin
)

// RouterNamespace is the interface that a HTTProuter handler should follow in order
// to become a valid namespace.
type RouterNamespace interface {
	AuthorizeRequest(data any, accessType AuthAccessType) (valid bool, err error)
	ProcessData(req *http.Request) (data any, err error)
}

// RouterHandlerFn is the function signature for adding handlers to the HTTProuter.
type RouterHandlerFn = func(msg Message)

// Init initializes the router
func (r *HTTProuter) Init(host string, port int) error {
	r.namespaces = make(map[string]RouterNamespace, 32)
	ln, err := reuse.Listen("tcp", net.JoinHostPort(host, fmt.Sprintf("%d", port)))
	if err != nil {
		return err
	}

	if n := somaxconn(); n < desiredSoMaxConn {
		log.Warnf("operating system SOMAXCONN is smaller than recommended (%d). "+
			"Consider increasing it: echo %d | sudo tee /proc/sys/net/core/somaxconn", n, desiredSoMaxConn)
	}

	r.Mux = chi.NewRouter()
	r.Mux.Use(middleware.RealIP)

	// If we want rich logging (e.g. with fields), we could implement our
	// own version of DefaultLogFormatter.
	r.Mux.Use(middleware.RequestLogger(&middleware.DefaultLogFormatter{
		Logger:  stdLogger{log.Logger()},
		NoColor: true,
	}))
	r.Mux.Use(middleware.Recoverer)
	r.Mux.Use(middleware.Heartbeat("/ping"))
	r.Mux.Use(middleware.ThrottleBacklog(5000, 40000, 30*time.Second))
	r.Mux.Use(middleware.Timeout(30 * time.Second))
	r.Mux.Use(middleware.Compress(5))

	// Cors handler
	cors := cors.New(cors.Options{
		AllowOriginFunc: func(r *http.Request, origin string) bool {
			return true
		}, // Kind of equivalent to AllowedOrigin: []string{"*"} but it returns the origin as allowed origin.
		AllowedMethods: []string{
			http.MethodGet,
			http.MethodPost,
			http.MethodPut,
			http.MethodDelete,
			http.MethodOptions,
		},
		AllowedHeaders:     []string{"*"},
		AllowCredentials:   true,
		MaxAge:             300, // Maximum value not ignored by any of major browsers
		OptionsPassthrough: false,
		Debug:              false,
	})
	r.Mux.Use(cors.Handler)

	// For some reason the Cors handler is not returning 200 on OPTIONS.
	// So as a workaround we declare explicitly the Options handler
	r.Mux.Options("/*", func(w http.ResponseWriter, r *http.Request) {})

	if len(r.TLSdomain) > 0 {
		log.Infof("fetching letsencrypt TLS certificate for %s", r.TLSdomain)
		s, m := r.generateTLScert(host, port)
		s.ReadTimeout = 20 * time.Second
		s.WriteTimeout = 15 * time.Second
		s.IdleTimeout = 10 * time.Second
		s.ReadHeaderTimeout = 5 * time.Second
		s.Handler = r.Mux
		if err := http2.ConfigureServer(s, nil); err != nil {
			return err
		}
		go func() {
			log.Info("starting go-chi https server")
			log.Fatal(s.ServeTLS(ln, "", ""))
		}()
		certs, err := r.getCertificates(m)
		if len(certs) == 0 || err != nil {
			log.Warnf(`letsencrypt TLS certificate cannot be obtained. Maybe port 443 is not accessible or domain name is wrong.
							You might want to redirect port 443 with iptables using the following command:
							sudo iptables -t nat -I PREROUTING -p tcp --dport 443 -j REDIRECT --to-ports %d`, port)
			return fmt.Errorf("cannot get letsencrypt TLS certificate: (%s)", err)
		}
		log.Infof("router ready at https://%s", ln.Addr())

	} else {
		log.Info("starting go-chi http server")
		s := &http.Server{
			ReadTimeout:       10 * time.Second,
			WriteTimeout:      10 * time.Second,
			IdleTimeout:       10 * time.Second,
			ReadHeaderTimeout: 3 * time.Second,
			Handler:           r.Mux,
		}
		if err := http2.ConfigureServer(s, nil); err != nil {
			return err
		}
		go func() {
			log.Fatal(s.Serve(ln))
		}()
		log.Infof("router ready at http://%s", ln.Addr())
	}
	r.address = ln.Addr()
	return nil
}

// EnablePrometheusMetrics enables go-chi prometheus metrics under specified ID.
// If ID empty, the default "gochi_http" is used.
func (r *HTTProuter) EnablePrometheusMetrics(prometheusID string) {
	// Prometheus handler
	if prometheusID == "" {
		prometheusID = "gochi_http"
	}
	r.Mux.Use(chiprometheus.NewMiddleware(prometheusID))
}

// ExposePrometheusEndpoint registers a HTTPHandler at the passed path
// that will expose the metrics collected by VictoriaMetrics and Prometheus
func (r *HTTProuter) ExposePrometheusEndpoint(path string) {
	r.AddRawHTTPHandler(path, "GET", func(w http.ResponseWriter, req *http.Request) {
		// upstream packages (cometbft, libp2p) call prometheus.Register(), so expose those metrics first
		//
		// we need to remove the "Accept-Encoding: gzip" to ensure promhttp.Handler().ServeHTTP() outputs plaintext,
		// to be able to append the plaintext from metrics.WritePrometheus,
		// and let go-chi take care of compression of the combined output
		req.Header.Del("Accept-Encoding")
		promhttp.Handler().ServeHTTP(w, req)

		// then append the metrics registered in VictoriaMetrics (our metrics, basically)
		// don't exposeProcessMetrics here since the go_* and process_* are already part of the output of
		// the previous promhttp.Handler().ServeHTTP()
		metrics.WritePrometheus(w, false)
	})
	log.Infof("prometheus metrics ready at: %s", path)
}

// Address return the current network address used by the HTTP router
func (r *HTTProuter) Address() net.Addr {
	return r.address
}

// AddNamespace creates a new namespace handled by the RouterNamespace implementation.
func (r *HTTProuter) AddNamespace(id string, rns RouterNamespace) {
	log.Infof("added namespace %s", id)
	r.namespacesLock.Lock()
	defer r.namespacesLock.Unlock()
	r.namespaces[id] = rns
}

func (r *HTTProuter) getNamespace(id string) (RouterNamespace, bool) {
	r.namespacesLock.RLock()
	defer r.namespacesLock.RUnlock()
	rns, ok := r.namespaces[id]
	return rns, ok
}

// AddAdminHandler adds a handler function for the namespace, pattern and HTTPmethod.
// The Admin requests are usually protected by some authorization mechanism.
func (r *HTTProuter) AddAdminHandler(namespaceID,
	pattern, HTTPmethod string, handler RouterHandlerFn) {
	log.Infof("added admin handler for namespace %s with pattern %s", namespaceID, pattern)
	r.Mux.MethodFunc(HTTPmethod, pattern, r.routerHandler(namespaceID, AccessTypeAdmin, handler))
}

// AddQuotaHandler adds a handler function for the namespace, pattern and HTTPmethod.
// The Quota requests are rate-limited per bearer token
func (r *HTTProuter) AddQuotaHandler(namespaceID,
	pattern, HTTPmethod string, handler RouterHandlerFn) {
	log.Infow("added handler", "type", "quota", "namespace", namespaceID, "pattern", pattern)
	r.Mux.MethodFunc(HTTPmethod, pattern, r.routerHandler(namespaceID, AccessTypeQuota, handler))
}

// AddPrivateHandler adds a handler function for the namespace, pattern and HTTPmethod.
// The Private requests are usually protected by some authorization mechanism, such as signature verification,
// which must be handled by the namespace implementation.
func (r *HTTProuter) AddPrivateHandler(namespaceID,
	pattern, HTTPmethod string, handler RouterHandlerFn) {
	log.Infow("added handler", "type", "private", "namespace", namespaceID, "pattern", pattern)
	r.Mux.MethodFunc(HTTPmethod, pattern, r.routerHandler(namespaceID, AccessTypePrivate, handler))
}

// AddPublicHandler adds a handled function for the namespace, patter and HTTPmethod.
// The public requests are not protected so all requests are allowed.
func (r *HTTProuter) AddPublicHandler(namespaceID,
	pattern, HTTPmethod string, handler RouterHandlerFn) {
	log.Infow("added handler", "type", "public", "namespace", namespaceID, "pattern", pattern)
	r.Mux.MethodFunc(HTTPmethod, pattern, r.routerHandler(namespaceID, AccessTypePublic, handler))
}

// AddRawHTTPHandler adds a standard net/http handled function to the router.
// The public requests are not protected so all requests are allowed.
func (r *HTTProuter) AddRawHTTPHandler(pattern, HTTPmethod string, handler http.HandlerFunc) {
	log.Infow("added handler", "type", "raw", "pattern", pattern)
	r.Mux.MethodFunc(HTTPmethod, pattern, handler)
}

func (r *HTTProuter) routerHandler(namespaceID string, accessType AuthAccessType,
	handlerFunc RouterHandlerFn) func(w http.ResponseWriter, req *http.Request) {
	return func(w http.ResponseWriter, req *http.Request) {
		defer req.Body.Close()

		nsProcessor, ok := r.getNamespace(namespaceID)
		if !ok {
			log.Errorf("namespace %s is not defined", namespaceID)
			return
		}
		data, err := nsProcessor.ProcessData(req)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		if ok, err := nsProcessor.AuthorizeRequest(data, accessType); !ok {
			http.Error(w, err.Error(), http.StatusUnauthorized)
			return
		}

		hc := &HTTPContext{Request: req, Writer: w, sent: make(chan struct{})}
		msg := Message{
			Data:      data,
			TimeStamp: time.Now(),
			Context:   hc,
			Path:      strings.Split(req.URL.Path, "/")[1:],
		}
		go handlerFunc(msg)

		// The contract is that every handled request must send a
		// response, even when they fail or time out.
		<-hc.sent
	}
}

// generateTLScert generates a TLS certificated
func (r *HTTProuter) generateTLScert(host string, port int) (*http.Server, *autocert.Manager) {
	m := autocert.Manager{
		Prompt:     autocert.AcceptTOS,
		HostPolicy: autocert.HostWhitelist(r.TLSdomain),
		Cache:      autocert.DirCache(r.TLSdirCert),
	}
	if r.TLSconfig == nil {
		r.TLSconfig = &tls.Config{
			MinVersion: tls.VersionTLS13, // secure version of TLS
			MaxVersion: 0,                // setting MaxVersion to 0 means that the highest version available in the package will be used
		}
	}
	r.TLSconfig.GetCertificate = m.GetCertificate
	serverConfig := &http.Server{
		Addr:              net.JoinHostPort(host, fmt.Sprintf("%d", port)),
		TLSConfig:         r.TLSconfig,
		ReadTimeout:       10 * time.Second,
		IdleTimeout:       10 * time.Second,
		ReadHeaderTimeout: 3 * time.Second,
	}
	serverConfig.TLSConfig.NextProtos = append(serverConfig.TLSConfig.NextProtos, acme.ALPNProto)

	return serverConfig, &m
}

func (r *HTTProuter) getCertificates(m *autocert.Manager) ([][]byte, error) {
	hello := &tls.ClientHelloInfo{
		ServerName:   r.TLSdomain,
		CipherSuites: []uint16{tls.TLS_ECDHE_RSA_WITH_CHACHA20_POLY1305},
	}
	hello.CipherSuites = append(hello.CipherSuites, tls.TLS_ECDHE_ECDSA_WITH_CHACHA20_POLY1305)

	cert, err := m.GetCertificate(hello)
	if err != nil {
		return nil, err
	}
	return cert.Certificate, nil
}

func somaxconn() int {
	content, err := os.ReadFile("/proc/sys/net/core/somaxconn")
	if err != nil {
		return syscall.SOMAXCONN
	}
	n, err := strconv.Atoi(strings.Trim(string(content), "\n"))
	if err != nil {
		return syscall.SOMAXCONN
	}
	return n
}

type stdLogger struct {
	log *zerolog.Logger
}

func (l stdLogger) Print(v ...any) { l.log.Debug().Msg(fmt.Sprint(v...)) }

package endpoint

import (
	"crypto/tls"
	"fmt"
	"time"

	"go.vocdoni.io/dvote/log"
	"go.vocdoni.io/dvote/multirpc/metrics"
	"go.vocdoni.io/dvote/multirpc/transports"
	"go.vocdoni.io/dvote/multirpc/transports/mhttp"
)

const (
	OptionListenHost      = "listenHost"
	OptionListenPort      = "listenPort"
	OptionTLSdomain       = "tlsDomain"
	OptionTLSdirCert      = "tlsDirCert"
	OptionTLSconfig       = "tlsConfig"
	OptionMetricsInterval = "metricsInterval"
	OptionSetMode         = "setMode"

	ModeHTTPWS   = 0
	ModeHTTPonly = 1
	ModeWSonly   = 2
)

type HTTPWSconfig struct {
	ListenHost string
	ListenPort int32
	TLSdomain  string
	TLSdirCert string
	Mode       int8 // Modes available: 0:HTTP+WS, 1:HTTP, 2:WS
	TLSconfig  *tls.Config
	Metrics    *metrics.Metrics
}

// HTTPWSendPoint handles an HTTP + Websocket connection (client chooses).
type HTTPWSendPoint struct {
	Proxy        *mhttp.Proxy
	MetricsAgent *metrics.Agent
	transport    transports.Transport
	id           string
	config       HTTPWSconfig
}

// ID returns the name of the transport implemented on the endpoint
func (e *HTTPWSendPoint) ID() string {
	return e.id
}

// Transport returns the transport used for this endpoint
func (e *HTTPWSendPoint) Transport() transports.Transport {
	return e.transport
}

// SetOption configures a endpoint option, valid options are:
// listenHost:string, listenPort:int32, tlsDomain:string, tlsDirCert:string, metricsInterval:int
func (e *HTTPWSendPoint) SetOption(name string, value interface{}) error {
	switch name {
	case OptionListenHost:
		if fmt.Sprintf("%T", value) != "string" {
			return fmt.Errorf("listenHost must be a valid string")
		}
		e.config.ListenHost = value.(string)
	case OptionListenPort:
		if fmt.Sprintf("%T", value) != "int32" {
			return fmt.Errorf("listenPort must be a valid int32")
		}
		e.config.ListenPort = value.(int32)
	case OptionTLSdomain:
		if fmt.Sprintf("%T", value) != "string" {
			return fmt.Errorf("tlsDomain must be a valid string")
		}
		e.config.TLSdomain = value.(string)
	case OptionTLSdirCert:
		if fmt.Sprintf("%T", value) != "string" {
			return fmt.Errorf("tlsDirCert must be a valid string")
		}
		e.config.TLSdirCert = value.(string)
	case OptionSetMode:
		if fmt.Sprintf("%T", value) != "int" {
			return fmt.Errorf("setMode must be a valid int")
		}
		e.config.Mode = int8(value.(int))
	case OptionTLSconfig:
		if tc, ok := value.(*tls.Config); !ok {
			return fmt.Errorf("tlsConfig must be of type *tls.Config")
		} else {
			e.config.TLSconfig = tc
		}
	case OptionMetricsInterval:
		if fmt.Sprintf("%T", value) != "int" {
			return fmt.Errorf("metricsInterval must be a valid int")
		}
		if e.config.Metrics == nil {
			e.config.Metrics = &metrics.Metrics{}
		}
		e.config.Metrics.Enabled = true
		e.config.Metrics.RefreshInterval = value.(int)
	}
	return nil
}

// Init creates a new websockets/http mixed endpoint
func (e *HTTPWSendPoint) Init(listener chan transports.Message) error {
	log.Infof("creating API service")

	// Create a HTTP Proxy service
	pxy, err := proxy(e.config.ListenHost, e.config.ListenPort, e.config.TLSconfig, e.config.TLSdomain, e.config.TLSdirCert)
	if err != nil {
		return err
	}

	// Create a HTTP+Websocket transport and attach the proxy
	var ts transports.Transport
	switch e.config.Mode {
	case 0:
		ts = new(mhttp.HttpWsHandler)
	case 1:
		ts = new(mhttp.HttpHandler)
	case 2:
		ts = new(mhttp.WebsocketHandle)
	default:
		return fmt.Errorf("mode %d not supported", e.config.Mode)
	}

	ts.Init(new(transports.Connection))

	switch e.config.Mode {
	case 0:
		ts.(*mhttp.HttpWsHandler).SetProxy(pxy)
	case 1:
		ts.(*mhttp.HttpHandler).SetProxy(pxy)
	case 2:
		ts.(*mhttp.WebsocketHandle).SetProxy(pxy)
	}

	go ts.Listen(listener)

	// Attach the metrics agent (Prometheus)
	var ma *metrics.Agent
	if e.config.Metrics != nil && e.config.Metrics.Enabled {
		ma = metrics.NewAgent("/metrics", time.Second*time.Duration(e.config.Metrics.RefreshInterval), pxy)
	}
	e.id = "httpws"
	e.Proxy = pxy
	e.MetricsAgent = ma
	e.transport = ts
	return nil
}

// proxy creates a new service for routing HTTP connections using go-chi server
// if tlsDomain is specified, it will use letsencrypt to fetch a valid TLS certificate
func proxy(host string, port int32, tlsConfig *tls.Config, tlsDomain, tlsDir string) (*mhttp.Proxy, error) {
	pxy := mhttp.NewProxy()
	pxy.Conn.TLSdomain = tlsDomain
	pxy.Conn.TLScertDir = tlsDir
	pxy.Conn.Address = host
	pxy.Conn.Port = port
	pxy.TLSConfig = tlsConfig
	log.Infof("creating proxy service, listening on %s:%d", host, port)
	if pxy.Conn.TLSdomain != "" {
		log.Infof("configuring proxy with TLS certificate for domain %s", tlsDomain)
	}
	return pxy, pxy.Init()
}

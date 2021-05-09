package mhttp

import (
	"fmt"

	"go.vocdoni.io/dvote/multirpc/transports"
)

// HttpWsHandler is a Mixed handler websockets/http
type HttpWsHandler struct {
	Proxy       *Proxy                 // proxy where the ws will be associated
	Connection  *transports.Connection // the ws connection
	WsReadLimit int64

	internalReceiver chan transports.Message
}

func (h *HttpWsHandler) Init(c *transports.Connection) error {
	if h.WsReadLimit == 0 {
		h.WsReadLimit = 32768 // default
	}

	h.internalReceiver = make(chan transports.Message, 1)
	return nil
}

func (h *HttpWsHandler) SetProxy(p *Proxy) {
	h.Proxy = p
}

// AddProxyHandler adds the current websocket handler into the Proxy
func (h *HttpWsHandler) AddProxyHandler(path string) {
	h.Proxy.AddMixedHandler(path,
		getHTTPhandler(path, h.internalReceiver),
		getWsHandler(path, h.internalReceiver),
		h.WsReadLimit)
}

func (h *HttpWsHandler) ConnectionType() string {
	return "HTTPWS"
}

func (h *HttpWsHandler) Listen(receiver chan<- transports.Message) {
	go func() {
		for {
			msg := <-h.internalReceiver
			receiver <- msg
		}
	}()
}

func (h *HttpWsHandler) SendUnicast(address string, msg transports.Message) error {
	// WebSocket is not p2p so sendUnicast makes the same of Send()
	return h.Send(msg)
}

func (h *HttpWsHandler) Send(msg transports.Message) error {
	// TODO(mvdan): this extra abstraction layer is probably useless
	return msg.Context.(*HttpContext).Send(msg)
}

func (h *HttpWsHandler) SetBootnodes(bootnodes []string) {
	// No bootnodes on websockets handler
}

func (h *HttpWsHandler) AddPeer(peer string) error {
	// No peers on websockets handler
	return nil
}

// AddNamespace adds a new namespace to the transport
func (h *HttpWsHandler) AddNamespace(namespace string) error {
	if len(namespace) == 0 || namespace[0] != '/' {
		return fmt.Errorf("namespace on httpws must start with /")
	}
	h.AddProxyHandler(namespace)
	return nil
}

func (h *HttpWsHandler) Address() string {
	return h.String()
}

func (h *HttpWsHandler) String() string {
	return h.Proxy.Addr.String()
}
func NewHttpWsHandleWithWsReadLimit(readLimit int64) *HttpWsHandler {
	return &HttpWsHandler{
		WsReadLimit: readLimit,
	}
}

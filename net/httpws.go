package net

import (
	"go.vocdoni.io/dvote/types"
)

// HttpWsHandler is a Mixed handler websockets/http
type HttpWsHandler struct {
	Proxy            *Proxy            // proxy where the ws will be associated
	Connection       *types.Connection // the ws connection
	internalReceiver chan types.Message
	WsReadLimit      int64
}

func NewHttpWsHandleWithWsReadLimit(readLimit int64) *HttpWsHandler {
	return &HttpWsHandler{
		WsReadLimit: readLimit,
	}
}

func (h *HttpWsHandler) Init(c *types.Connection) error {
	if h.WsReadLimit == 0 {
		h.WsReadLimit = 32768 // default
	}
	h.internalReceiver = make(chan types.Message, 1)
	return nil
}

func (h *HttpWsHandler) SetProxy(p *Proxy) {
	h.Proxy = p
}

// AddProxyHandler adds the current websocket handler into the Proxy
func (h *HttpWsHandler) AddProxyHandler(path string) {
	h.Proxy.AddMixedHandler(path, getHTTPhandler(path, h.internalReceiver), getWsHandler(path, h.internalReceiver), h.WsReadLimit)
}

func (h *HttpWsHandler) ConnectionType() string {
	return "HTTPWS"
}

func (h *HttpWsHandler) Listen(receiver chan<- types.Message) {
	go func() {
		for {
			msg := <-h.internalReceiver
			receiver <- msg
		}
	}()
}

func (h *HttpWsHandler) SendUnicast(address string, msg types.Message) {
	// WebSocket is not p2p so sendUnicast makes the same of Send()
	h.Send(msg)
}

func (h *HttpWsHandler) Send(msg types.Message) {
	// TODO(mvdan): this extra abstraction layer is probably useless
	msg.Context.(*HttpContext).Send(msg)
}

func (h *HttpWsHandler) SetBootnodes(bootnodes []string) {
	// No bootnodes on websockets handler
}

func (h *HttpWsHandler) AddPeer(peer string) error {
	// No peers on websockets handler
	return nil
}

// AddNamespace adds a new namespace to the transport
func (h *HttpWsHandler) AddNamespace(namespace string) {
	h.AddProxyHandler(namespace)
}

func (h *HttpWsHandler) Address() string {
	return h.String()
}

func (h *HttpWsHandler) String() string {
	return h.Proxy.Addr.String()
}

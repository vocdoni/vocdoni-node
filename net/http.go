package net

import (
	"fmt"
	"io/ioutil"
	"net/http"
	"time"

	"go.vocdoni.io/dvote/log"
	"go.vocdoni.io/dvote/types"
)

type HttpHandler struct {
	Proxy            *Proxy // proxy where the ws will be associated
	internalReceiver chan types.Message
}

type HttpContext struct {
	Writer  http.ResponseWriter
	Request *http.Request

	sent chan struct{}
}

func (h *HttpHandler) Init(c *types.Connection) error {
	h.internalReceiver = make(chan types.Message, 1)
	return nil
}

func (h *HttpHandler) SetProxy(p *Proxy) {
	h.Proxy = p
}

func getHTTPhandler(path string, receiver chan types.Message) func(w http.ResponseWriter, r *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		defer r.Body.Close()
		respBody, err := ioutil.ReadAll(r.Body)
		if err != nil {
			log.Warnf("HTTP connection closed: (%s)", err)
			return
		}
		hc := &HttpContext{Request: r, Writer: w, sent: make(chan struct{})}
		msg := types.Message{
			Data:      respBody,
			TimeStamp: int32(time.Now().Unix()),
			Context:   hc,
			Namespace: path,
		}
		receiver <- msg

		// Don't return this func until a response is sent, because the
		// connection is closed when the handler returns.
		// The contract is that every handled request must send a
		// response, even when they fail or time out.
		<-hc.sent
	}
}

// AddProxyHandler adds the current websocket handler into the Proxy
func (h *HttpHandler) AddProxyHandler(path string) {
	h.Proxy.AddHandler(path, getHTTPhandler(path, h.internalReceiver))
}

func (h *HttpContext) ConnectionType() string {
	return "HTTP"
}

func (h *HttpContext) Send(msg types.Message) {
	defer func() {
		if r := recover(); r != nil {
			log.Warnf("recovered http send panic: %v", r)
		}
	}()
	defer close(h.sent)
	if h.Request.Context().Err() != nil {
		// The connection was closed, so don't try to write to it.
		return
	}

	h.Writer.Header().Set("Content-Length", fmt.Sprintf("%d", len(msg.Data)+1))
	h.Writer.Header().Set("Content-Type", "application/json")
	if _, err := h.Writer.Write(msg.Data); err != nil {
		log.Warn(err)
	}
	// Ensure we end the response with a newline, to be nice.
	h.Writer.Write([]byte("\n"))
}

func (h *HttpHandler) ConnectionType() string {
	return "HTTP"
}

func (h *HttpHandler) Listen(receiver chan<- types.Message) {
	go func() {
		for {
			msg := <-h.internalReceiver
			receiver <- msg
		}
	}()
}

func (h *HttpHandler) SendUnicast(address string, msg types.Message) {
	// HTTP is not p2p, so sendUnicast makes the same of Send()
	h.Send(msg)
}

func (h *HttpHandler) Send(msg types.Message) {
	// TODO(mvdan): this extra abstraction layer is probably useless
	msg.Context.(*HttpContext).Send(msg)
}

func (h *HttpHandler) SetBootnodes(bootnodes []string) {
	// No bootnodes on websockets handler
}

func (h *HttpHandler) AddPeer(peer string) error {
	// No peers on websockets handler
	return nil
}

// AddNamespace adds a new namespace to the transport
func (h *HttpHandler) AddNamespace(namespace string) {
	h.AddProxyHandler(namespace)
}

func (h *HttpHandler) Address() string {
	return h.String()
}

func (h *HttpHandler) String() string {
	return h.Proxy.Addr.String()
}

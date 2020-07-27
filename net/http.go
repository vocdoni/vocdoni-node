package net

import (
	"fmt"
	"io/ioutil"
	"net/http"
	"time"

	"gitlab.com/vocdoni/go-dvote/log"
	"gitlab.com/vocdoni/go-dvote/types"
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

// AddProxyHandler adds the current websocket handler into the Proxy
func (h *HttpHandler) AddProxyHandler(path string) {
	h.Proxy.AddHandler(path, func(w http.ResponseWriter, r *http.Request) {
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
		h.internalReceiver <- msg

		// Don't return this func until a response is sent, because the
		// connection is closed when the handler returns.
		select {
		case <-hc.sent:
			// a response was sent.
		case <-r.Context().Done():
			// we hit chi's timeout.
		}
	})
}

func (h *HttpContext) ConnectionType() string {
	return "HTTP"
}

func (h *HttpHandler) ConnectionType() string {
	return "HTTP"
}

func (h *HttpHandler) Listen(receiver chan<- types.Message) {
	for {
		msg := <-h.internalReceiver
		receiver <- msg
	}
}

func (h *HttpHandler) SendUnicast(address string, msg types.Message) {
	// WebSocket is not p2p so sendUnicast makes the same of Send()
	h.Send(msg)
}

func (h *HttpHandler) Send(msg types.Message) {
	ctx := msg.Context.(*HttpContext)
	defer close(ctx.sent)

	ctx.Writer.Header().Set("Content-Length", fmt.Sprintf("%d", len(msg.Data)+1))
	ctx.Writer.Header().Set("Content-Type", "application/json")
	if _, err := ctx.Writer.Write(msg.Data); err != nil {
		log.Warn(err)
	}
	// Ensure we end the response with a newline, to be nice.
	ctx.Writer.Write([]byte("\n"))
}

func (h *HttpHandler) SetBootnodes(bootnodes []string) {
	// No bootnodes on websockets handler
}

func (h *HttpHandler) AddPeer(peer string) error {
	// No peers on websockets handler
	return nil
}

// Listen will listen the websockets handler and write the received data into the channel
func (h *HttpHandler) AddNamespace(namespace string) {
	h.AddProxyHandler(namespace)
}

func (h *HttpHandler) Address() string {
	return h.String()
}

func (h *HttpHandler) String() string {
	return h.Proxy.Addr.String()
}

package mhttp

import (
	"context"
	"fmt"
	"io/ioutil"
	"net/http"
	"strconv"
	"strings"
	"syscall"
	"time"

	"go.vocdoni.io/dvote/log"
	"go.vocdoni.io/dvote/multirpc/transports"
	"nhooyr.io/websocket"
)

// WebsocketHandle handles the websockets connection on the go-dvote proxy
type WebsocketHandle struct {
	Connection *transports.Connection // the ws connection
	WsProxy    *Proxy                 // proxy where the ws will be associated
	ReadLimit  int64

	internalReceiver chan transports.Message
}

type WebsocketContext struct {
	Conn *websocket.Conn
}

func (c WebsocketContext) ConnectionType() string {
	return "Websocket"
}

func (c *WebsocketContext) Send(msg transports.Message) error {
	tctx, cancel := context.WithTimeout(context.Background(), 1*time.Minute)
	defer cancel()
	return c.Conn.Write(tctx, websocket.MessageBinary, msg.Data)
}

// SetProxy sets the proxy for the ws
func (w *WebsocketHandle) SetProxy(p *Proxy) {
	w.WsProxy = p
}

func NewWebSocketHandleWithReadLimit(readLimit int64) *WebsocketHandle {
	return &WebsocketHandle{
		ReadLimit: readLimit,
	}
}

// Init initializes the websockets handler and the internal channel to communicate with other go-dvote components
func (w *WebsocketHandle) Init(c *transports.Connection) error {
	if w.ReadLimit == 0 {
		w.ReadLimit = 32768 // default by ws client
	}

	w.internalReceiver = make(chan transports.Message, 1)
	return nil
}

func getWsHandler(path string, receiver chan transports.Message) func(conn *websocket.Conn) {
	return func(conn *websocket.Conn) {
		// Read websocket messages until the connection is closed. HTTP
		// handlers are run in new goroutines, so we don't need to spawn
		// another goroutine.
		for {
			_, payload, err := conn.Read(context.TODO())
			if err != nil {
				conn.Close(websocket.StatusAbnormalClosure, "ws closed by client")
				break
			}
			msg := transports.Message{
				Data:      payload,
				TimeStamp: int32(time.Now().Unix()),
				Context:   &WebsocketContext{Conn: conn},
				Namespace: path,
			}
			receiver <- msg
		}
	}
}

// AddProxyHandler adds the current websocket handler into the Proxy
func (w *WebsocketHandle) AddProxyHandler(path string) {
	w.WsProxy.AddWsHandler(path, getWsHandler(path, w.internalReceiver), w.ReadLimit)
}

// ConnectionType returns a string identifying the transport connection type
func (w *WebsocketHandle) ConnectionType() string {
	return "Websocket"
}

// Listen will listen the websockets handler and write the received data into the channel
func (w *WebsocketHandle) Listen(receiver chan<- transports.Message) {
	go func() {
		for {
			msg := <-w.internalReceiver
			receiver <- msg
		}
	}()
}

// Listen will listen the websockets handler and write the received data into the channel
func (w *WebsocketHandle) AddNamespace(namespace string) error {
	if len(namespace) == 0 || namespace[0] != '/' {
		return fmt.Errorf("namespace on ws must start with /")
	}
	w.AddProxyHandler(namespace)
	return nil
}

// Send sends the response given a message
func (w *WebsocketHandle) Send(msg transports.Message) error {
	// TODO(mvdan): this extra abstraction layer is probably useless
	return msg.Context.(*WebsocketContext).Send(msg)
}

func (w *WebsocketHandle) SendUnicast(address string, msg transports.Message) error {
	// WebSocket is not p2p so sendUnicast makes the same of Send()
	return w.Send(msg)
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

func wshandler(w http.ResponseWriter, r *http.Request, ph ProxyWsHandler, readLimit int64) {
	conn, err := websocket.Accept(w, r, &websocket.AcceptOptions{OriginPatterns: []string{"*"}})
	if err != nil {
		log.Errorf("failed to set websocket upgrade: %s", err)
		return
	}
	conn.SetReadLimit(readLimit)
	ph(conn)
}

func somaxconn() int {
	content, err := ioutil.ReadFile("/proc/sys/net/core/somaxconn")
	if err != nil {
		return syscall.SOMAXCONN
	}
	n, err := strconv.Atoi(strings.Trim(string(content), "\n"))
	if err != nil {
		return syscall.SOMAXCONN
	}
	return n
}

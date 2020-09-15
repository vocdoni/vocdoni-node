package net

import (
	"context"
	"fmt"
	"io/ioutil"
	"net/http"
	"strconv"
	"strings"
	"syscall"
	"time"

	"gitlab.com/vocdoni/go-dvote/log"
	"gitlab.com/vocdoni/go-dvote/types"
	"nhooyr.io/websocket"
)

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

func (c *WebsocketContext) Send(msg types.Message) {
	c.Conn.Write(context.TODO(), websocket.MessageBinary, msg.Data)
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

func getWsHandler(path string, receiver chan types.Message) func(conn *websocket.Conn) {
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
			msg := types.Message{
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
	w.WsProxy.AddWsHandler(path, getWsHandler(path, w.internalReceiver))
}

// ConnectionType returns a string identifying the transport connection type
func (w *WebsocketHandle) ConnectionType() string {
	return "Websocket"
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
	// TODO(mvdan): this extra abstraction layer is probably useless
	msg.Context.(*WebsocketContext).Send(msg)
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

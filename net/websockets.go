package net

import (
	"net/http"
	"strconv"
	"time"

	"github.com/gorilla/websocket"

	"gitlab.com/vocdoni/go-dvote/log"
	"gitlab.com/vocdoni/go-dvote/types"
)

// WebsocketHandle represents the information required to work with ws in go-dvote
type WebsocketHandle struct {
	Connection *types.Connection // the ws connection
	WsProxy    *Proxy            // proxy where the ws will be associated
	Upgrader   *websocket.Upgrader

	internalReceiver chan types.Message
}

// SetProxy sets the proxy for the ws
func (w *WebsocketHandle) SetProxy(p *Proxy) {
	w.WsProxy = p
}

func (w *WebsocketHandle) Init(c *types.Connection) error {
	w.Upgrader = &websocket.Upgrader{
		ReadBufferSize:  4096,
		WriteBufferSize: 4096,
		CheckOrigin:     func(r *http.Request) bool { return true },
	}
	w.internalReceiver = make(chan types.Message, 1)
	return nil
}

func (w *WebsocketHandle) Listen(receiver chan<- types.Message) {
	// TODO(mvdan): this extra channel and goroutine is a tad unnecessary.
	// Consider refactoring the interface.
	for {
		msg := <-w.internalReceiver
		receiver <- msg
	}
}

// AddProxyHandler adds a handler on the proxy and upgrades the connection
// a ws connection is activated with a normal http request with Connection: upgrade
func (w *WebsocketHandle) AddProxyHandler(path string) {
	ssl := w.WsProxy.C.SSLDomain != ""
	serveWs := func(resp http.ResponseWriter, req *http.Request) {
		// Upgrade connection
		conn, err := w.Upgrader.Upgrade(resp, req, nil)
		if err != nil {
			log.Errorf("cannot upgrade http connection to ws: %s", err)
			return
		}

		resp.Header().Set("Access-Control-Allow-Origin", "*")
		resp.Header().Set("Access-Control-Allow-Methods", "POST, GET")
		resp.Header().Set("Access-Control-Allow-Headers", "Accept, Content-Type, Content-Length, Accept-Encoding, X-CSRF-Token")

		// Read websocket messages until the connection is closed. HTTP
		// handlers are run in new goroutines, so we don't need to spawn
		// another goroutine.
		for {
			_, payload, err := conn.ReadMessage()
			if err != nil {
				conn.Close()
				break
			}
			msg := types.Message{
				Data:      []byte(payload),
				TimeStamp: int32(time.Now().Unix()),
				Context: &WebsocketContext{
					Conn: conn,
				},
			}
			w.internalReceiver <- msg
		}
	}
	w.WsProxy.AddHandler(path, serveWs)

	if !ssl {
		log.Infof("ws initialized on ws://" + w.WsProxy.C.Address + ":" + strconv.Itoa(w.WsProxy.C.Port))
	} else {
		log.Infof("wss initialized on wss://" + w.WsProxy.C.SSLDomain + ":" + strconv.Itoa(w.WsProxy.C.Port))
	}
}

// Send sends the response given a message
func (w *WebsocketHandle) Send(msg types.Message) {
	clientConn := msg.Context.(*WebsocketContext).Conn
	clientConn.WriteMessage(websocket.BinaryMessage, msg.Data)
}

type WebsocketContext struct {
	Conn *websocket.Conn
}

func (c WebsocketContext) ConnectionType() string {
	return "Websocket"
}

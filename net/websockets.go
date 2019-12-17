package net

import (
	"net/http"
	"strconv"
	"syscall"
	"time"

	"github.com/gorilla/websocket"

	"gitlab.com/vocdoni/go-dvote/log"
	"gitlab.com/vocdoni/go-dvote/types"
)

type Client struct {
	ID   string
	Conn *websocket.Conn
	Pool *clientPool
}

type clientPool struct {
	Clients    map[*Client]bool
	Register   chan *Client
	Unregister chan *Client
}

func newClientPool() *clientPool {
	return &clientPool{
		Clients:    make(map[*Client]bool),
		Register:   make(chan *Client),
		Unregister: make(chan *Client),
	}
}

func (p *clientPool) Start() {
	for {
		select {
		case client := <-p.Register:
			p.Clients[client] = true
			log.Infof("client with id: %s connected", client.ID)
			log.Debugf("client pool size: %d", len(p.Clients))
			break
		case client := <-p.Unregister:
			delete(p.Clients, client)
			log.Infof("client with id: %s disconnected", client.ID)
			log.Debugf("client pool size: %d", len(p.Clients))
			break
		}
	}
}

// WebsocketHandle represents the information required to work with ws in go-dvote
type WebsocketHandle struct {
	Connection *types.Connection // the ws connection
	WsProxy    *Proxy            // proxy where the ws will be associated
	Upgrader   *websocket.Upgrader
	Pool       *clientPool
}

// SetProxy sets the proxy for the ws
func (w *WebsocketHandle) SetProxy(p *Proxy) {
	w.WsProxy = p
}

// Init increases the sys limitations regarding to the number of files opened
// to handle the connections and creates the epoll
func (w *WebsocketHandle) Init(c *types.Connection) error {
	// Increase resources limitations
	var rLimit syscall.Rlimit
	if err := syscall.Getrlimit(syscall.RLIMIT_NOFILE, &rLimit); err != nil {
		return err
	}
	rLimit.Cur = rLimit.Max
	if err := syscall.Setrlimit(syscall.RLIMIT_NOFILE, &rLimit); err != nil {
		return err
	}

	w.Upgrader = &websocket.Upgrader{
		ReadBufferSize:  4096,
		WriteBufferSize: 4096,
		CheckOrigin:     func(r *http.Request) bool { return true },
	}

	w.Pool = newClientPool()
	go w.Pool.Start()

	return nil
}

func (w *WebsocketHandle) Upgrade(responseWriter http.ResponseWriter, r *http.Request) (*websocket.Conn, error) {
	conn, err := w.Upgrader.Upgrade(responseWriter, r, nil)
	if err != nil {
		log.Infof("cannot upgrade http connection to ws: %s", err)
		return nil, err
	}

	return conn, nil
}

// AddProxyHandler adds a handler on the proxy and upgrades the connection
// a ws connection is activated with a normal http request with Connection: upgrade
func (w *WebsocketHandle) AddProxyHandler(path string) {
	ssl := w.WsProxy.C.SSLDomain != ""
	serveWs := func(writer http.ResponseWriter, reader *http.Request) {
		// Upgrade connection
		conn, err := w.Upgrade(writer, reader)
		if err != nil {
			return
		}

		client := &Client{
			Conn: conn,
			Pool: w.Pool,
			ID:   time.Now().String(),
		}

		w.Pool.Register <- client

		writer.Header().Set("Access-Control-Allow-Origin", "*")
		writer.Header().Set("Access-Control-Allow-Methods", "POST, GET")
		writer.Header().Set("Access-Control-Allow-Headers", "Accept, Content-Type, Content-Length, Accept-Encoding, X-CSRF-Token")
	}
	w.WsProxy.AddHandler(path, serveWs)

	if !ssl {
		log.Infof("ws initialized on ws://" + w.WsProxy.C.Address + ":" + strconv.Itoa(w.WsProxy.C.Port))
	} else {
		log.Infof("wss initialized on wss://" + w.WsProxy.C.SSLDomain + ":" + strconv.Itoa(w.WsProxy.C.Port))
	}
}

// Listen listens for incoming data
func (w *WebsocketHandle) Listen(reciever chan<- types.Message) {
	var msg types.Message

	for {
		for client, added := range w.Pool.Clients {
			if added == false {
				break
			}
			_, payload, err := client.Conn.ReadMessage()
			if err != nil {
				w.Pool.Unregister <- client
				client.Conn.Close()
				continue
			}
			msg.Data = []byte(payload)
			msg.TimeStamp = int32(time.Now().Unix())
			ctx := new(WebsocketContext)
			ctx.Conn = client.Conn
			msg.Context = ctx
			reciever <- msg
		}
	}
}

// Send sends the response given a message
func (w *WebsocketHandle) Send(msg types.Message) {
	clientConn := *msg.Context.(*WebsocketContext).Conn
	clientConn.WriteMessage(websocket.BinaryMessage, msg.Data)
}

type WebsocketContext struct {
	Conn *websocket.Conn
}

func (c WebsocketContext) ConnectionType() string {
	return "Websocket"
}

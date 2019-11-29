package net

import (
	"net/http"
	"strconv"
	"syscall"
	"time"

	"gitlab.com/vocdoni/go-dvote/log"
	"gitlab.com/vocdoni/go-dvote/net/epoll"
	"gitlab.com/vocdoni/go-dvote/types"

	"github.com/gobwas/ws"
	"github.com/gobwas/ws/wsutil"
)

// WebsocketHandle represents the information required to work with ws in go-dvote
type WebsocketHandle struct {
	Connection *types.Connection // the ws connection
	Epoll      *epoll.Epoll      // epoll for the ws implementation
	WsProxy    *Proxy            // proxy where the ws will be associated
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

	// Start epoll
	var err error
	w.Epoll, err = epoll.MkEpoll()
	if err != nil {
		return err
	}

	return nil
}

// AddProxyHandler adds a handler on the proxy and upgrades the connection
// a ws connection is activated with a normal http request with Connection: upgrade
func (w *WebsocketHandle) AddProxyHandler(path string) {
	ssl := w.WsProxy.C.SSLDomain != ""
	upgradeConn := func(writer http.ResponseWriter, reader *http.Request) {
		// Upgrade connection
		conn, _, _, err := ws.UpgradeHTTP(reader, writer)
		if err != nil {
			return
		}
		if err := w.Epoll.Add(conn, ssl); err != nil {
			log.Warnf("failed to add connection %v", err)
			conn.Close()
		}
		writer.Header().Set("Access-Control-Allow-Origin", "*")
		writer.Header().Set("Access-Control-Allow-Methods", "POST, GET")
		writer.Header().Set("Access-Control-Allow-Headers", "Accept, Content-Type, Content-Length, Accept-Encoding, X-CSRF-Token")
	}
	w.WsProxy.AddHandler(path, upgradeConn)

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
		connections, err := w.Epoll.Wait()
		if err != nil {
			log.Warn(err)
			continue
		}
		for _, conn := range connections {
			if conn == nil {
				break
			}
			payload, _, err := wsutil.ReadClientData(conn)
			if err != nil {
				ssl := w.WsProxy.C.SSLDomain != ""
				if err := w.Epoll.Remove(conn, ssl); err != nil {
					log.Warn(err)
				}
				conn.Close()
				continue
			}
			msg.Data = []byte(payload)
			msg.TimeStamp = int32(time.Now().Unix())
			ctx := new(types.WebsocketContext)
			ctx.Conn = &conn
			msg.Context = ctx
			reciever <- msg
		}
	}
}

// Send sends the response given a message
func (w *WebsocketHandle) Send(msg types.Message) {
	wsutil.WriteServerMessage(*msg.Context.(*types.WebsocketContext).Conn, ws.OpBinary, msg.Data)
}

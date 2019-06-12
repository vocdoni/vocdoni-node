package net

import (
	"log"
	"net/http"
	"strconv"
	"syscall"
	"time"

	"github.com/vocdoni/go-dvote/net/epoll"
	"github.com/vocdoni/go-dvote/types"

	"github.com/gobwas/ws"
	"github.com/gobwas/ws/wsutil"
)

// WebsocketHandle represents the information required to work with ws in go-dvote
type WebsocketHandle struct {
	c *types.Connection // the ws connection
	e *epoll.Epoll      // epoll for the ws implementation
	p *Proxy            // proxy where the ws will be associated
}

// SetProxy sets the proxy for the ws
func (w *WebsocketHandle) SetProxy(p *Proxy) {
	w.p = p
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
	w.e, err = epoll.MkEpoll()
	if err != nil {
		return err
	}

	return nil
}

// AddProxyHandler adds a handler on the proxy and upgrades the connection
// a ws connection is activated with a normal http request with Connection: upgrade
func (w *WebsocketHandle) AddProxyHandler(path string) {
	upgradeConn := func(writer http.ResponseWriter, reader *http.Request) {
		// Upgrade connection
		conn, _, _, err := ws.UpgradeHTTP(reader, writer)
		if err != nil {
			return
		}
		if w.p.SSLDomain == "" {
			if err := w.e.Add(conn, false); err != nil {
				log.Printf("Failed to add connection %v", err)
				conn.Close()
			}
		} else {
			if err := w.e.Add(conn, true); err != nil {
				log.Printf("Failed to add connection %v", err)
				conn.Close()
			}
		}
	}
	w.p.AddHandler(path, upgradeConn)

	if w.p.SSLDomain == "" {
		log.Printf("ws initialized on ws://" + w.p.Address + ":" + strconv.Itoa(w.p.Port) + path)
	} else {
		log.Printf("wss initialized on wss://" + w.p.SSLDomain + ":" + strconv.Itoa(w.p.Port) + path)
	}

}

// Listen listens for incoming data
func (w *WebsocketHandle) Listen(reciever chan<- types.Message) {
	var msg types.Message
	for {
		connections, err := w.e.Wait()
		if err != nil {
			log.Printf("WS recieve error: %s", err)
			continue
		}
		for _, conn := range connections {
			if conn == nil {
				break
			}
			if payload, _, err := wsutil.ReadClientData(conn); err != nil {
				if w.p.SSLDomain == "" {
					if err := w.e.Remove(conn, false); err != nil {
						log.Printf("WS recieve error: %s", err)
					}
				} else {
					if err := w.e.Remove(conn, true); err != nil {
						log.Printf("WS recieve error: %s", err)
					}
				}

				conn.Close()
			} else {
				msg.Data = []byte(payload)
				msg.TimeStamp = time.Now()
				ctx := new(types.WebsocketContext)
				ctx.Conn = &conn
				msg.Context = ctx
				reciever <- msg
			}
		}
	}
}

// Send sends the response given a message
func (w *WebsocketHandle) Send(msg types.Message) {
	wsutil.WriteServerMessage(*msg.Context.(*types.WebsocketContext).Conn, ws.OpBinary, msg.Data)
}

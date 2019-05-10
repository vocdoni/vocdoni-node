package net

import (
	"log"
	"net/http"
	"syscall"

	"github.com/vocdoni/go-dvote/net/epoll"
	"github.com/vocdoni/go-dvote/types"

	"github.com/gobwas/ws"
	"github.com/gobwas/ws/wsutil"
)

type WebsocketHandle struct {
	c *types.Connection
	e *epoll.Epoll
}

func (w *WebsocketHandle) upgrader(writer http.ResponseWriter, reader *http.Request) {
	// Upgrade connection
	conn, _, _, err := ws.UpgradeHTTP(reader, writer)
	if err != nil {
		return
	}
	if err := w.e.Add(conn); err != nil {
		log.Printf("Failed to add connection %v", err)
		conn.Close()
	}
}

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

	//go w.Listen()

	http.HandleFunc(c.Path, w.upgrader)
	if err := http.ListenAndServe(c.Address + ":" + c.Port, nil); err != nil {
		return err
	}
	return nil
}

func (w *WebsocketHandle) Listen(reciever chan<- types.Message, errorReciever chan<- error) {
	var msg types.Message
	for {
		connections, err := w.e.Wait()
		if err != nil {
			errorReciever <- err
			continue
		}
		for _, conn := range connections {
			if conn == nil {
				break
			}
			if payload, _, err := wsutil.ReadClientData(conn); err != nil {
				if err := w.e.Remove(conn); err != nil {
					errorReciever <- err
				}
				conn.Close()
			} else {
				//contruct message
				msg.Data = []byte(payload)
				msg.Conn = conn
				reciever <- msg
				//send to channel
				//log.Printf("Payload: %s", string(payload))
			}
		}
	}
}


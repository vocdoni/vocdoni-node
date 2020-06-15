package net

import (
	"context"
	"fmt"
	"io"
	"sync"
	"time"

	"gitlab.com/vocdoni/go-dvote/log"
	"nhooyr.io/websocket"
)

//                                  ___ [ws server 1] <active connection>
// client --- [local websocket] ---|___ [ws server 2] <disconnected>
//                                 |___ [ws server 3] <disconnected>
//
// wsPool is a websockets proxy which servers a single, stable WS local connection and manages the connection to multiple remote servers.
// If a remote server fails for some reason, the wsPool will automatically try the next one.
// Only if all remote server fail on a Dial round the wsPool will fail.
type wsPool struct {
	wsc        *websocket.Conn
	index      int
	servers    []string
	readChan   chan (wsReader)
	readCancel context.CancelFunc // TODO(mvdan): probably racy
	lock       sync.RWMutex       // TODO: what is this protecting?
	readWait   sync.WaitGroup     // TODO(mvdan): probably racy too; we mix Add/Done and Wait
}

type wsReader struct {
	msgType websocket.MessageType
	reader  io.Reader
}

func newWsPoll() *wsPool {
	return &wsPool{wsc: new(websocket.Conn), readChan: make(chan wsReader)}
}

// read is a blocking routine that will continuously wait for websocket input messages and will put it into the readChan channel.
// If error while reading, read will automatically dial again with the next available websocket server of the pool.
func (w *wsPool) read() {
	var ctx context.Context
	for {
		// In case dial() is happening, wait until done
		w.readWait.Wait()
		ctx, w.readCancel = context.WithCancel(context.Background())
		msgType, msg, err := w.wsc.Reader(ctx)
		if err == nil {
			w.readChan <- wsReader{msgType: msgType, reader: msg}
		} else {
			// If error, try to dial again. If dial fails, launch a Fatal error.
			if err := w.dial(); err != nil {
				log.Fatal(err)
				return
			}
		}
		// Use a sleep to avoid busy looping.
		time.Sleep(time.Millisecond * 200)
	}
}

func (w *wsPool) addServer(url string) {
	w.lock.Lock()
	defer w.lock.Unlock()
	w.servers = append(w.servers, url)
	w.index++
}

// dial will try to create a connection with the first available websocket server from the pool.
// If no server works, an error will be returned.
func (w *wsPool) dial() (err error) {
	w.lock.Lock()
	defer w.lock.Unlock()
	initialIndex := w.index
	// If a read routine is running, let's make it wait until the dial is done
	w.readWait.Add(1)
	defer w.readWait.Done()
	for {
		w.index++
		if w.index >= len(w.servers) {
			w.index = 0
		}
		if w.readCancel != nil {
			w.readCancel()
		}
		ctx, cancel := context.WithTimeout(context.Background(), time.Second*5)
		w.wsc, _, err = websocket.Dial(ctx, w.servers[w.index], nil)
		cancel()
		w.wsc.SetReadLimit(1024 * 1024) // 1MByte should be enough
		if err == nil {
			break
		}
		if initialIndex == w.index {
			err = fmt.Errorf("no more servers in pool")
			break
		}
	}
	return err
}

// write will send a message to the websockets server.
// If the connection is lost for some reason, try to dial and write again.
// If the second write fails, returns an error.
func (w *wsPool) write(mt websocket.MessageType, rb []byte) error {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*5)
	defer cancel()
	if err := w.wsc.Write(ctx, mt, rb); err == nil {
		return nil
	}
	log.Warnf("websocket connection lost, retrying...")
	if err := w.dial(); err != nil {
		return err
	}
	ctx, cancel = context.WithTimeout(context.Background(), time.Second*5)
	defer cancel()
	if err := w.wsc.Write(ctx, mt, rb); err == nil {
		return nil
	}
	return fmt.Errorf("websocket connection cannot be recovered")
}

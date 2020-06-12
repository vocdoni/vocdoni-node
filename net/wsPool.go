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

func (w *wsPool) read() {
	var ctx context.Context
	for {
		w.readWait.Wait()
		ctx, w.readCancel = context.WithCancel(context.Background())
		msgType, msg, err := w.wsc.Reader(ctx)
		if err == nil {
			w.readChan <- wsReader{msgType: msgType, reader: msg}
		} else {
			// TODO: this needs to be documented. both the reader
			// and writer can dial if they encounter an error?
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

func (w *wsPool) dial() (err error) {
	w.lock.Lock()
	defer w.lock.Unlock()
	initialIndex := w.index
	w.readWait.Add(1)
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
		w.wsc.SetReadLimit(1024 * 1024)
		if err == nil {
			break
		}
		if initialIndex == w.index {
			err = fmt.Errorf("no more servers in pool")
			break
		}
	}
	w.readWait.Done()
	return err
}

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

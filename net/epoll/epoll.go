package epoll

import (
	"net"
	"reflect"
	"sync"
	"syscall"

	"golang.org/x/sys/unix"

	"gitlab.com/vocdoni/go-dvote/log"
)

type Epoll struct {
	fd          int              // file descriptor
	connections map[int]net.Conn // connections map
	lock        *sync.RWMutex    // lock for the fd
}

// MkEpoll creates a linux epoll
func MkEpoll() (*Epoll, error) {
	fd, err := unix.EpollCreate1(0)
	if err != nil {
		return nil, err
	}
	return &Epoll{
		fd:          fd,
		lock:        &sync.RWMutex{},
		connections: make(map[int]net.Conn),
	}, nil
}

// Add adds a connection
func (e *Epoll) Add(conn net.Conn, ssl bool) error {
	// Extract file descriptor associated with the connection
	fd := websocketFD(conn, ssl)
	err := unix.EpollCtl(e.fd, syscall.EPOLL_CTL_ADD, fd, &unix.EpollEvent{Events: unix.POLLIN | unix.POLLHUP, Fd: int32(fd)})
	if err != nil {
		return err
	}
	e.lock.Lock()
	defer e.lock.Unlock()
	e.connections[fd] = conn
	log.Debugf("total number of connections: %v", len(e.connections))
	return nil
}

// Remove removes a connection
func (e *Epoll) Remove(conn net.Conn, ssl bool) error {
	fd := websocketFD(conn, ssl)
	err := unix.EpollCtl(e.fd, syscall.EPOLL_CTL_DEL, fd, nil)
	if err != nil {
		return err
	}
	e.lock.Lock()
	defer e.lock.Unlock()
	delete(e.connections, fd)
	if len(e.connections)%100 == 0 {
		log.Debugf("total number of connections: %v", len(e.connections))
	}
	return nil
}

// Wait given an added connection, listens for new events
func (e *Epoll) Wait() ([]net.Conn, error) {
	events := make([]unix.EpollEvent, 100)
	n, err := unix.EpollWait(e.fd, events, 100)
	if err != nil {
		return nil, err
	}
	e.lock.RLock()
	defer e.lock.RUnlock()
	var connections []net.Conn
	for i := 0; i < n; i++ {
		conn := e.connections[int(events[i].Fd)]
		connections = append(connections, conn)
	}
	return connections, nil
}

func websocketFD(conn net.Conn, ssl bool) int {
	var tcpConn reflect.Value
	if ssl {
		tcpConn = reflect.Indirect(reflect.Indirect(reflect.ValueOf(conn)).FieldByName("conn").Elem())
	} else {
		tcpConn = reflect.Indirect(reflect.ValueOf(conn)).FieldByName("conn")
	}

	fdVal := tcpConn.FieldByName("fd")
	pfdVal := reflect.Indirect(fdVal).FieldByName("pfd")

	return int(pfdVal.FieldByName("Sysfd").Int())
}

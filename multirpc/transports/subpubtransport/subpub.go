package subpubtransport

import (
	"context"
	"fmt"
	"time"

	"go.vocdoni.io/dvote/crypto/ethereum"
	"go.vocdoni.io/dvote/log"
	"go.vocdoni.io/dvote/multirpc/subpub"
	"go.vocdoni.io/dvote/multirpc/transports"
)

type SubPubContext struct {
	Sp     *SubPubHandle
	PeerID string
}

func (sc *SubPubContext) ConnectionType() string {
	return "subpub"
}
func (sc *SubPubContext) Send(msg transports.Message) (err error) {
	return sc.Sp.SendUnicast(sc.PeerID, msg)
}

type SubPubHandle struct {
	Conn      *transports.Connection
	SubPub    *subpub.SubPub
	BootNodes []string
}

func (p *SubPubHandle) Init(c *transports.Connection) error {
	p.Conn = c
	s := ethereum.NewSignKeys()
	if err := s.AddHexKey(p.Conn.Key); err != nil {
		return fmt.Errorf("cannot import privkey %s: %s", p.Conn.Key, err)
	}
	if len(p.Conn.TransportKey) == 0 {
		return fmt.Errorf("groupkey topic not specified")
	}
	if p.Conn.Port == 0 {
		p.Conn.Port = 45678
	}
	private := p.Conn.Encryption == "private"
	sp := subpub.NewSubPub(s.Private, []byte(p.Conn.TransportKey), int32(p.Conn.Port), private)
	c.Address = sp.PubKey
	p.SubPub = sp
	return nil
}

func (s *SubPubHandle) Listen(receiver chan<- transports.Message) {
	s.SubPub.Start(context.Background())
	go s.SubPub.Subscribe(context.Background())
	go func() {
		for {
			var msg transports.Message
			spmsg := <-s.SubPub.Reader
			msg.Data = spmsg.Data
			msg.TimeStamp = int32(time.Now().Unix())
			msg.Context = &SubPubContext{PeerID: spmsg.Peer, Sp: s}
			log.Debugf("received %d bytes from %s", len(msg.Data), spmsg.Peer)
			receiver <- msg
		}
	}()
}

func (s *SubPubHandle) Address() string {
	return s.SubPub.NodeID
}

func (s *SubPubHandle) SetBootnodes(bootnodes []string) {
	s.SubPub.BootNodes = bootnodes
}

func (s *SubPubHandle) AddPeer(peer string) error {
	return s.SubPub.TransportConnectPeer(peer)
}

func (s *SubPubHandle) String() string {
	return s.SubPub.String()
}

func (s *SubPubHandle) ConnectionType() string {
	return "SubPub"
}

func (s *SubPubHandle) Send(msg transports.Message) error {
	log.Debugf("sending %d bytes to broadcast channel", len(msg.Data))

	// Use a fallback timeout of five minutes, to prevent blocking forever
	// or leaking goroutines.
	// TODO(mvdan): turn this fallback timeout into a ctx parameter
	ctx, cancel := context.WithTimeout(context.TODO(), 5*time.Minute)
	defer cancel()

	select {
	case s.SubPub.BroadcastWriter <- msg.Data:
	case <-ctx.Done():
		return ctx.Err()
	}
	return nil
}

func (s *SubPubHandle) AddNamespace(namespace string) error {
	// TBD (could subscrive to a specific topic)
	return nil
}

func (s *SubPubHandle) SendUnicast(address string, msg transports.Message) error {
	log.Debugf("sending %d bytes to %s", len(msg.Data), address)
	if err := s.SubPub.PeerStreamWrite(address, msg.Data); err != nil {
		return fmt.Errorf("cannot send message to %s: (%w)", address, err)
	}
	return nil
}

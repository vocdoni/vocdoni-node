package types

import (
	"net"

	"gitlab.com/vocdoni/go-dvote/config"
)

type MessageContext interface {
	ConnectionType() string
}

type PssContext struct {
	Topic       string
	PeerAddress string //this is the address of the other side
}

func (c PssContext) ConnectionType() string {
	return "PSS"
}

/*
func (c *PssContext) Topic() string {
	return c.Topic
}

func (c *PssContext) PeerAddress() string {
	return c.PeerAddress
}
*/
type PubSubContext struct {
	Topic       string
	PeerAddress string
}

func (c PubSubContext) ConnectionType() string {
	return "PubSub"
}

/*
func (c *PubSubContext) Topic() string {
	return c.Topic
}
*/
type WebsocketContext struct {
	Conn *net.Conn
}

func (c WebsocketContext) ConnectionType() string {
	return "Websocket"
}

/*
func (c *WebsocketContext) Conn() *net.Conn {
	return c.Conn
}
*/

//Message is a wrapper for messages from various net transport modules
type Message struct {
	Data      []byte
	TimeStamp int32
	Context   MessageContext
}

//Connection describes the settings for any of the transports defined in the net module, note that not all
//fields are used for all transport types.
type Connection struct {
	Topic      string //channel/topic for topic based messaging such as PSS, PubSub
	Encryption string //what type of encryption to use
	Key        string //this node's key
	Address    string //this node's address
	Path       string //specific path on which a transport should listen
	SSLDomain  string //ssl domain
	SSLCertDir string //ssl certificates directory
	Port       int    //specific port on which a transport should listen
}

type DataStore struct {
	Datadir    string
	ClusterCfg config.ClusterCfg
}

//Ballot represents the choices of one user in one voting process
type Ballot struct {
	Type      string
	PID       string
	Nullifier []byte
	Vote      []byte
	Franchise []byte
}

//Envelope contains a Ballot, and additional metadata for processing
type Envelope struct {
	Type      string
	Nonce     uint64
	KeyProof  []byte
	Ballot    []byte
	Timestamp int32
}

//Batch contains a number of Ballots, ready to be counted
type Batch struct {
	Type       string
	Nullifiers []string
	URL        string
	TXID       string
	Nonce      []byte
	Signature  string
}


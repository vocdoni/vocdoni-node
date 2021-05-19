package transports

type Transport interface {
	// Init initializes the transport layer. Takes a struct of options. Not all options must have effect.
	Init(c *Connection) error
	// ConnectionType returns the human readable name for the transport layer.
	ConnectionType() string
	// Listen starts and keeps listening for new messages, takes a channel where new messages will be written.
	Listen(receiver chan<- Message)
	// Send outputs a new message to the connection of MessageContext.
	// The package implementing this interface must also implement its own transports.MessageContext.
	Send(msg Message) error
	// SendUnicast outputs a new message to a specific connection identified by address.
	SendUnicast(address string, msg Message) error
	// AddNamespace creates a new namespace for writing handlers. On HTTP namespace is usually the URL route.
	AddNamespace(namespace string) error
	// Address returns an string containing the transport own address identifier.
	Address() string
	// SetBootnodes takes a list of bootnodes and connects to them for boostraping the network protocol.
	// This function only makes sense on p2p transports.
	SetBootnodes(bootnodes []string)
	// AddPeer opens a new connection with the specified peer.
	// This function only makes sense on p2p transports.
	AddPeer(peer string) error
	// String returns a human readable string representation of the transport state
	String() string
}

type MessageContext interface {
	ConnectionType() string
	Send(Message) error
}

type MessageAPI interface {
	GetID() string
	SetID(string)
	SetTimestamp(int32)
	SetError(string)
	GetMethod() string
}

// Message is a wrapper for messages from various net transport modules
type Message struct {
	Data      []byte
	TimeStamp int32
	Namespace string

	Context MessageContext
}

// Connection describes the settings for any of the transports defined in the net module, note that not all
// fields are used for all transport types.
type Connection struct {
	Topic        string // channel/topic for topic based messaging such as PubSub
	Encryption   string // what type of encryption to use
	TransportKey string // transport layer key for encrypting messages
	Key          string // this node's key
	Address      string // this node's address
	TLSdomain    string // tls domain
	TLScertDir   string // tls certificates directory
	Port         int32  // specific port on which a transport should listen
}

# transport module

Wraps multiple messaging/communication protocols in a common interface intended to facilitate flexible stream processing.

```golang
type Transport interface {
	
  // Init initializes the transport layer. Takes a transports.Connection type with a set of common options. 
  // Not all options must have effect.
	Init(c *Connection) error
  
	// ConnectionType returns the human readable name for the transport layer.
  ConnectionType() string
  
	// Listen starts and keeps listening for new messages, takes a channel where new messages will be written.
  Listen(reciever chan<- Message)
  
	// Send outputs a new message to the connection of MessageContext.
	// The package implementing this interface must also implement its own transports.MessageContext.
  Send(msg Message) error
  
	// SendUnicast outputs a new message to a specific connection identified by address.
  // This method only makes sense on p2p transports.
  SendUnicast(address string, msg Message) error
  
	// AddNamespace creates a new namespace for writing handlers. On HTTP namespace is usually the URL route.
  AddNamespace(namespace string) error
  
	// Address returns an string containing the transport own address identifier.
  Address() string
  
	// SetBootnodes takes a list of bootnodes and connects to them for boostraping the network protocol.
	// This method only makes sense on p2p transports.
  SetBootnodes(bootnodes []string)
  
	// AddPeer opens a new connection with the specified peer.
	// This method only makes sense on p2p transports.
  AddPeer(peer string) error
  
	// String returns a human readable string representation of the transport state
  String() string
}
```

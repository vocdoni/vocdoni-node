# go-dvote net module
Wraps multiple messaging/communication protocols in a common interface intended to facilitate flexible stream processing

## Generic Interface
The primary interface presented by the net module is the Transport interface. It represents a bidrectional connection to a particular p2p messaging channel, or websocket connection pool.

The various underlying protocols are identified by a TransportID:
```
transportType := net.TransportTypeFromString("PSS")
```

An active default transport is initalized by a call to the module's public InitDefault method:
```
PSSTransport, err :=  net.InitDefault(transportType)
```

Transports expect to relay what they recieve to a Message channel, and to have an error channel for signaling:
```
listenerOutput := make(chan types.Message, 10)
listenerErrors := make(chan error)
```

To concurrently ingest a stream:
```
go transport.Listen(listenerOutput, listenerErrors)
```

The Transport Send method expect a slice of bytes, and a channel to which it can send any errors:
```
sendErrors := make(chan error)
exampleMessage := []byte{'d', 'v', 'o', 't', 'e'}
transport.Send(exampleMessage, sendErrors)
```

## Supported protocols
Currently supported are PSS and PubSub. Websocket support is planned in the near future.

### PSS
The relevant Connection specifier fields for PSS are Topic, Kind, Key, and Address:
```
exampleConnection := new(types.Connection)
exampleConnection.Topic = "exampleTopic"
exampleConnection.Encryption = "sym" //options are "sym" or "asym"
exampleConnection.Key = "exampleKey" //symmetric key if "sym", recipient pubkey if "asym"
exampleTransport, err := net.Init(transportType, exampleConnection)
//use as above
```


### Pubsub
The relevant Connection specifier field for PubSub is the Topic:
```
exampleConnection := new(types.Connection)
exampleConnection.Topic = "exampleTopic"
exampleTransport, err := net.Init(transportType, exampleConnection)
//use as above
```

### Websockets
Work in progress

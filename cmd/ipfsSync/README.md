# ipfsSync

A simple and easy to use multinode ipfs pin synchronization tool.

Uses a Kademlia DHT network (provided by libp2p) to exange pining information with other nodes.

Only those nodes of the DHT using the same symetric key will be able to synchronize with you.

No external ipfs daemon is required, ipfsSync will start and manage its own ipfs daemon.

Internally ipfsSync uses a MerkleTree to keep track of the current pining state. Each node announces his root merkletree. If a new root is announced on the network, the node will send a direct message to the publisher to get the list of pins. At the end all nodes will end up with the same list of pins and the same merkle root.

Pins can only be added but not deleted. So once a new pin enters into the network it will persist forever (WIP delete function).

#### Usage and example

```
go run cmd/ipfsSync/ipfsSync.go --help

      --dataDir string    directory for storing data (default "/home/user/.ipfs")
      --helloTime int     period in seconds for sending hello messages (default 40)
      --key string        symetric key of the sync ipfs cluster
      --logLevel string   log level (default "info")
      --port int16        port for the sync network (default 4171)
      --updateTime int    period in seconds for sending update messages (default 20)
```
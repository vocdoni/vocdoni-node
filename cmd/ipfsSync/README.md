# ipfsSync

A simple and easy to use multinode ipfs pin synchronization tool.

Uses a Kademlia DHT network (provided by Swarm/PSS) to exange pining information with other nodes.

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

```
go run cmd/ipfsSync/ipfsSync.go --dataDir /tmp/ipfs --key vocdoni

ipfsSync/ipfsSync.go:26 logger construction succeeded at level info and output stdout
ipfs/ipfs.go:32 checking if daemon is running
ipfs/ipfsInit.go:52     initializing IPFS node at /tmp/ipfs
generating 2048-bit RSA keypair...done
peer identity: QmeKJTUYiRDVexkfzDcq6oWqNykCgKvB86h8EGBBeE5NHt
ipfs/ipfsInit.go:76     IPFS configuration file initialized
ipfs/ipfs.go:55 attempting to start IPFS node
ipfs/ipfs.go:56 config root: /tmp/ipfs
data/ipfs.go:71 IPFS Peer ID: QmeKJTUYiRDVexkfzDcq6oWqNykCgKvB86h8EGBBeE5NHt
ipfssync/ipfssync.go:277        initializing new pin storage
API server listening on /ip4/0.0.0.0/tcp/5001
ipfssync/ipfssync.go:282        current hash 0x0000000000000000000000000000000000000000000000000000000000000000
swarm/swarm.go:250      add bootnode enode://d6e2a7a90ca736b1651974ca47feb2bc93a9bbc136c91140256c654b50d7de8c52d993fed56737bfabdf210b6892132471e8da499ce7a4b95c917d70935c3af2@109.201.1.151:4171
swarm/swarm.go:288      my PSS pubkey is 0x04bd7852ed303e82d12afa32f47424d5eca64094295d186fab950ac1630c216527c57a2cca27a03aeb160e2c4fc3261208a9b1272422430daf8a9cc4d49ef230a9
swarm/swarm.go:289      my PSS address is 170450c01a02724b14c10a21ed47dc131ea307e896c4a4f0ee796d8395cb42bf
swarm/swarm.go:290      my PSS enode is enode://bd7852ed303e82d12afa32f47424d5eca64094295d186fab950ac1630c216527c57a2cca27a03aeb160e2c4fc3261208a9b1272422430daf8a9cc4d49ef230a9@92.121.211.15:31000
swarm/swarm.go:351      Pss subscribed to sym, topic 30783733366263663531
swarm/swarm.go:191      PeerCount: 0, Neighborhood: 21195280
ipfssync/ipfssync.go:305        my multiaddress: /ip4/92.121.211.15/tcp/4001/ipfs/QmeKJTUYiRDVexkfzDcq6oWqNykCgKvB86h8EGBBeE5NHt
swarm/swarm.go:191      PeerCount: 0, Neighborhood: 21195280
ipfssync/ipfssync.go:154        connecting to peer /ip4/112.202.154.521/tcp/4001/ipfs/QmShHZ7XvdSX13rPbYzTRHCRfoWD5PU3YWP7wF6Zhea4na
ipfssync/ipfssync.go:169        found new hash 0x23a6a7ff8890a33c4ee8e5eac1f3aa5f2ff3cd3eab6ab72157d3660774b09aa6 from f9f6666f9f2a82d7e4ccf255323aebf04a62221cde2fcd7c9bd0710e7a25a6f8
ipfssync/ipfssync.go:179        got new pin list 0x23a6a7ff8890a33c4ee8e5eac1f3aa5f2ff3cd3eab6ab72157d3660774b09aa6 from f9f6666f9f2a82d7e4ccf255323aebf04a62221cde2fcd7c9bd0710e7a25a6f8
swarm/swarm.go:191      PeerCount: 1, Neighborhood: 21195280
ipfssync/ipfssync.go:76 pinning /ipld/Qmb8E1FnG2RcYYPCC8wfDZ68LFRZWckSVrpfnLXkwvRcxc
ipfssync/ipfssync.go:76 pinning /ipld/QmUbGbpMmewbmV66ev1b54Rj5FonrUHAGSVjFJU4gDXwyh
ipfssync/ipfssync.go:76 pinning /ipld/QmZMYm2QZg7rVRoijLwGz7crdiZtTpYt2Fgyh7sb8i9hHw
...
```

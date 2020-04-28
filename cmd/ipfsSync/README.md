# ipfsSync

A simple and easy to use multinode ipfs pin synchronization tool. It does not require an existing ipfs daemon running nor installed in the operating system, ipfsSync includes the required ipfs functionalities and exposes the same API.

+ **libp2p**: uses the libp2p DHT network to exange pining information with other nodes and to make direct peer connections
+ **automatic**: no need of a custom bootstrap node, ipfsSync uses the default IPFS bootnodes for DHT bootstraping
+ **private**: only those nodes sharing the same secret symetric key will be able to synchronize
+ **easy**: no external ipfs daemon is required, ipfsSync will start and manage its own ipfs daemon

Internally ipfsSync uses a MerkleTree to keep track of the current pining state. Each node announces his root merkletree. If a new root is announced on the network, the node will send a direct message to the publisher to get the list of pins. At the end all nodes will end up with the same list of pins and the same merkle root.

Pins can only be added but not deleted. So once a new pin enters into the network it will persist forever (TBD: delete function).

### Usage

Available options:

```
      --bootnode                act as a bootstrap node (will not try to connect with other bootnodes)
      --bootnodes stringArray   list of bootnodes (multiaddress separated by commas)
      --dataDir string          directory for storing data (default "/home/p4u/.ipfs")
      --helloTime int           period in seconds for sending hello messages (default 40)
      --key string              secret shared group key for the sync cluster (default "vocdoni")
      --logLevel string         log level (default "info")
      --nodeKey string          custom private hexadeciaml 256 bit key for p2p identity
      --peers stringArray       custom list of peers to connect (multiaddress separated by commas)
      --port int16              port for the sync network (default 4171)
      --private                 if enabled a private libp2p network will be created (using the secret key at transport layer)
      --updateTime int          period in seconds for sending update messages (default 20)
```

### Example

On the first computer (A) start ipfsSync with the following options.

```
ipfsSync --key mySecretKey --dataDir=/tmp/example

    logger construction succeeded at level info and output stdout
    checking if daemon is running
    initializing IPFS node at /tmp/example
    IPFS configuration file initialized
    attempting to start IPFS node
    config root: /tmp/example
    IPFS peerID: QmUYqEU3YLwPUpYRqfRjCT6rXjBEhGHcTaS6iP6nP3w7nW
    generating new node private key
    initializing new pin storage
    current hash 0x0000000000000000000000000000000000000000000000000000000000000000
    public address: 0x02b1361be81eb46a3156a7f1a4d9c67d686d818f586fa60274153eee1a180ba53c
    private key: 58076ef2a9cf37aa6488e0a6d14e23fdd82c0ec7d866611de5aeff43fd3ac461
    my multiaddress: /ip4/14.16.8.2/tcp/4001/p2p/QmUYqEU3YLwPUpYRqfRjCT6rXjBEhGHcTaS6iP6nP3w7nW
    my subpub multiaddress /ip4/14.16.8.2/tcp/4171/p2p/QmPymFbKAuPu87KzD7vZe8rQSi92fwASyFGayN9Wy5nK72
    bootstrapping the DHT
    connecting to bootstrap nodes...
    connection established with bootstrap node: {QmbLHAnMoJPWSCR5Zhtx6BHJX9KiKNN6tpvbUcqanj75Nb: [/dnsaddr/bootstrap.libp2p.io]}
    connection established with bootstrap node: {QmQCU2EcMqAqQPR2i9bChDtGNJchTbq5TbXJJ16u19uLTa: [/dnsaddr/bootstrap.libp2p.io]}
    advertising topic c42af2afb92623e0287c83335d29d4f44945bd5b1172637d496e94aba4d194a4
    advertising topic 0x02b1361be81eb46a3156a7f1a4d9c67d686d818f586fa60274153eee1a180ba53c
    [subPub info] dhtPeers:17 dhtKnown:291 clusterPeers:0
    [subPub info] dhtPeers:20 dhtKnown:434 clusterPeers:0
```

Lets do the same (using the same secret key) on the second computer (B). 
Once the bootstrap is done, the computers should find each other and print a message such as:

```
  connected to peer QmYESWwhd2EWyhkMBTDwX8UEtGJVYHnh39eeizUoKoEE6a
  [subPub info] dhtPeers:39 dhtKnown:994 clusterPeers:1
```

Then they start to exchange some messages, the most common ones will be **hello** and **update**.

The hello message is used to let the other peers know about yourself, the multiaddress is announced in order to allow the peers create a direct IPFS connection.

```
sending message: {"type":"hello","address":"QmdvQZAG4KqRRJXFf93ftwRzkeqsjHde5tfmj9Z7WWM9Np","mAddress":"/ip4/18.10.12.11/tcp/4001/p2p/QmPkG4yUMm49v7VrubjoRYf9q11C59TxVYapJxdDW8bmc8","timestamp":1588016502}
```

The update message is used to announce the current local hash of the pining merkle tree. In the current stage we do not have yet any file so the hash will be the empty hash (0x000...).

```
sending messages: {"type":"update","address":"QmPymFbKAuPu87KzD7vZe8rQSi92fwASyFGayN9Wy5nK72","hash":""0x0000000000000000000000000000000000000000000000000000000000000000,"timestamp":1588016495}
```

Let's add a new file to the computer A using the standard `ìpfs` tool.

```
ipfs --api=/ip4/127.0.0.1/tcp/5001 add file.png

    added QmZe5QrhFhuUkMmvBuLrTBfHcr7LeA1azGRzH2y7z5Mhdi file.png
    66.09 KiB / 66.09 KiB [==========================================] 100.00%
```

The log output of computer A will show the new pin and Merkle Root hash. The next periodic update message will then let known the other peers its hash has changed.

```
 [ipfsSync info] pins:1 hash:0x2ee216bbb3d3dbd12f173f3dba584d0e055e8697e6c3214a1ab9082c13845701

 sending messages: {"type":"update","address":"QmPymFbKAuPu87KzD7vZe8rQSi92fwASyFGayN9Wy5nK72","hash":"0x2ee216bbb3d3dbd12f173f3dba584d0e055e8697e6c3214a1ab9082c13845701","timestamp":1588016495}
```

When computer B receive the new update, a `fetch` message will be send in order to receive the new list of files to pin. As B attach its current hash to the query, A is able to send only the difference between both Merkle Trees.

```
sending message: {"type":"fetch","address":"QmdvQZAG4KqRRJXFf93ftwRzkeqsjHde5tfmj9Z7WWM9Np","hash":"0x0000000000000000000000000000000000000000000000000000000000000000","timestamp":1588016496}

message received: {"type":"fetchReply","address":"QmPymFbKAuPu87KzD7vZe8rQSi92fwASyFGayN9Wy5nK72","hash":"0x2ee216bbb3d3dbd12f173f3dba584d0e055e8697e6c3214a1ab9082c13845701","pinList":["L2lwbGQvUW1iQkhNcGJCSzRHQ3BxYk1ieXU1eWVVWFhNQ040MlplVVRrd0FyOVJCZTgzbQ=="],"timestamp":1588016496}
```
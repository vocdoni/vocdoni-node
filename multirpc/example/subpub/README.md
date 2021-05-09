# multirpc: subpub example

This example uses almost the same code of the HTTP+WS example but using a different transport: a libp2p custom protocol named subpub.

In the server side we just need to change the endpoint implementation `ep := endpoint.SubPubEndpoint{}`.

In the client side we'll use directly the subpub handle: `sp := subpubtransport.SubPubHandle{}` and a local HTTP server that will forward the POST body to the p2p network and write back the reply (if any).

```
   [ http client (curl) ]
             |
[ client localhost http server ] 
             |
       ( p2p network ) 
             |
         [ server ]
```

Let's start with the p2p server:

```bash
go run ./server --logLevel=debug
```

We'll see something like this (among other messages).

```log
DEBUG   router/router.go:65     adding new handler hello for namespace 
DEBUG   router/router.go:65     adding new handler addkey for namespace 
DEBUG   router/router.go:65     adding new handler getsecret for namespace 
DEBUG   subpub/discovery.go:15  searching for SubPub group identity b935f02bc28165df3076ccd945fb70b0eed26b2902961c8222fcb4b9dd555aa8
INFO    subpub/discovery.go:95  advertising topic b935f02bc28165df3076ccd945fb70b0eed26b2902961c8222fcb4b9dd555aa8
```

Subpub is doing the p2p bootstraping and discovery using a shared secret between client and server (`sharedSecret123`), all data transfered will be encrypted using this key thus only nodes knowing the key will be able to join the p2p subnetwork.

Same as the HTTP+WS example, we add three handlers to the server, `hello`, `addkey` and `getsecret` which will be transparently handled by the router (no matter which transport we use).

Now let's start the p2p client which will listen to `http://localhost:8080/p2p` in order to send and recive messages using any HTTP client such as `curl`.

```bash
go run ./client --logLevel=debug
```

The p2p discovery process between client and server can take some seconds (or minutes), but they should find each other and create a direct connection.

```log
DEBUG   subpub/discovery.go:41  found peer: Qmct2CptU82Xs64cousGNmr8gfPq27CZhJL64Af8WqgEXW
INFO    subpub/stream.go:34     connected to peer Qmct2CptU82Xs64cousGNmr8gfPq27CZhJL64Af8WqgEXW:/ip4/192.168.1.112/tcp/7788
```

Now we can use curl transparently with the client http local server.

```bash
 $ curl -X POST -d '{"method":"hello", "request":"1234"}' 127.0.0.1:8080/p2p
```

```json
{"request":{"reply":"hello! got your message with ID 272","request":"272","timestamp":1611842897},"id":"272","signature":"676df15d26840ed665d074d545b9b3ec74ab47b5259a59dceac9a44d5b8cae2810a89e1228a93b178b270d240f7810fed8f9590699847c1c4cffc26b458ddae001"}
```
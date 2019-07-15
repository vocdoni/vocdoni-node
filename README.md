# go-dvote

This repository contains a set of libraries and tools for the Vocdoni's backend infrastrucutre, as described [in the documentation](http://vocdoni.io/docs/#/).

The list of components that are implemented by `go-dvote` are

+ Voting relay
+ Gateway
+ Bootnode
+ Census service

## Gateway

Gateways provide an entry point to the P2P networks. 

They allow clients to reach decentralized services (census, relays, blockchain, etc.) through a HTTP/WebSockets API interface.

#### Status

- [x] Unified WebSockets JSON API
- [x] Ethereum blockchain(s) support
- [x] Letsencrypt SSL support
- [x] Swarm PSS integration
- [x] Nice logs
- [x] Docker support
- [x] ECDSA signature integration
- [x] Census Merkle Tree implementation
- [ ] BootNode automatic discovery
- [ ] Native IPFS support
- [ ] IPFS cluster support
- [ ] Linkable Ring Signature integration
- [ ] ZK-snark integration
- [ ] Tendermint/Vochain implementation

#### Compile and run

Compile from source in a golang environment:

```
git clone https://gitlab.com/vocdoni/go-dvote.git
cd go-dvote
unset GOPATH
go build cmd/gatewat/gateway.go
./gateway --help
```

Or with docker (configuration options in file `dockerfiles/gateway/env`):

```
bash dockerfiles/gateway/dockerlaunch.sh
```


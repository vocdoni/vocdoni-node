# go-dvote

This repository contains a set of libraries and tools for the Vocdoni's backend infrastrucutre, as described [in the documentation](http://vocdoni.io/docs/#/).

The list of main components implemented in `go-dvote` are:

+ Gateway
+ Bootnode
+ Census Manager
+ Tendermint Vochain

Find the package lib reference [here](https://godoc.org/gitlab.com/vocdoni/go-dvote)

## Gateway

Gateways provide an entry point to the P2P networks. 

They allow clients to reach decentralized services (census, relays, blockchain, etc.) through a HTTP/WebSockets API interface.

#### Status

- [x] Unified WebSockets JSON API
- [x] Ethereum blockchain(s) support
- [x] Letsencrypt SSL support
- [ ] Swarm PSS integration
- [x] Nice logs
- [x] Docker support
- [x] ECDSA signature integration
- [x] Census Merkle Tree implementation
- [ ] BootNode automatic discovery
- [x] Native IPFS support
- [x] IPFS cluster support
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

All data will be stored in the shared directory `run/`.

## Census Manager

The Census Manager provides a service for both, organizations and users. Its purpose is to store and manage one or multiple census. A census is basically a list of public keys stored as a Merkle Tree.

The organizer can:
+ Create new census (it might be one per election process)
+ Store claims (public keys)
+ Export the claim list
+ Recover in any moment the Merkle Root

The user can:
+ Recover in any moment the Merkle Root
+ Given the content of a claim, get the Merkle Proof (which demostrates his public key is in the Census)
+ Check if a Merkle Proof is valid for a specific Merkle Root

The interaction with the Census Manager is handled trough a JSON API, which can be fetched via HTTP(s) or any other transport mechanism (right now only HTTP(s) support is implemented).

More information [here](https://vocdoni.io/docs/#/architecture/components/census-service)

#### Compile and run

In a GO ready environment:

```
git clone https://gitlab.com/vocdoni/go-dvote.git
cd go-dvote
unset GOPATH
go build -o censushttp gitlab.com/vocdoni/go-dvote/cmd/censushttp
```

Or with docker (configuration options in file `dockerfiles/census/env`):

```
bash dockerfiles/census/dockerlaunch.sh
```

All data will be stored in the shared directory `run/`.

## Usage

The rootKey is a ECDSA public key which allows the creation of new Census. 

`./censushttp --port 1500 --logLevel debug --rootKey 347f650ea2adee1affe2fe81ee8e11c637d506da98dc16e74fc64ecb31e1bb2c1`

See some examples examples [here](https://gitlab.com/vocdoni/go-dvote/tree/master/cmd/censushttp)

---

[![Contributor Covenant](https://img.shields.io/badge/Contributor%20Covenant-v1.4%20adopted-ff69b4.svg)](code-of-conduct.md)


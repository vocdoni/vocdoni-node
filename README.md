# go-dvote

[![GoDoc](https://godoc.org/go.vocdoni.io/dvote?status.svg)](https://godoc.org/go.vocdoni.io/dvote)
[![Go Report Card](https://goreportcard.com/badge/go.vocdoni.io/dvote)](https://goreportcard.com/report/go.vocdoni.io/dvote)

[![Join Discord](https://img.shields.io/badge/discord-join%20chat-blue.svg)](https://discord.gg/4hKeArDaU2)
[![Twitter Follow](https://img.shields.io/twitter/follow/vocdoni.svg?style=social&label=Follow)](https://twitter.com/vocdoni)

This repository contains a set of libraries and tools for the **Vocdoni** decentralized backend infrastrucutre, as described [in the documentation](http://vocdoni.io/docs/#/).

A good summary of the whole Vocdoni architecture can be found in the [blog post technical overview v1](https://blog.vocdoni.io/vocdoni-technical-overview-v1/).

## Vocdoni

Vocdoni is a universally verifiable, censorship-resistant, and anonymous self sovereign governance system, designed with the scalability and ease-of-use to support either small/private and big/national elections.

Our main aim is a trustless voting system, where anyone can speak their voice and where everything can be audited. We are engineering building blocks for a permissionless, private and censorship resistant democracy.

We intend the algorithms, systems, and software that we build to be a useful contribution toward making violence in these cryptonetworks impossible by protecting users privacy with cryptography. In particular, our aim is to provide the necessary tooling for the political will of network participants to translate outwardly into real political capital, without sacrificing privacy.

![vocdoni go-dvote team](https://assets.gitlab-static.net/uploads/-/system/project/avatar/12677379/go-dvote.png)

## dvotenode

The dvotenode is the main tool of go-dvote, it contains all the required features for making the decentralized Vocdoni backend possible.

Currently dvotenode can operate in three modes:

- **gateway** mode provides an entry point to the P2P networks for the clients (APP or Web), it uses most of the components from go-dvote. Detailed information can be found [here](https://vocdoni.io/docs/#/architecture/components/gateway)

- **miner** mode provides a block validation node (full node) of the Vochain (Tendermint based blockchain for voting). Defailed information can be found [here](https://vocdoni.io/docs/#/architecture/components/vochain)
- **oracle** mode provdes a bridge between Ethereum and the Vochain

One of the design primitives of go-dvote is to run everything as a single daemon in order to have complete control over the components and avoid local RPC or IPC connections. So unlike other projects, go-dvote uses go-ethereum, go-ipfs and tendermint as GoLang libraries.

In addition, go-dvote is currently pure GoLang code, so generating a static and reproducible binary that works on most of the Linux hosts (and probably MacOS) without any dependence is possible.

For running dvotenode in gateway mode, 8GB of ram is recommended (4GB works but it is risky).

#### Status

- [x] Unified WebSockets JSON API for client connection
- [x] Letsencrypt automatic TLS support
- [x] Ethereum blockchain(s) support
- [x] Ethereum event subscription to the Vocdoni Process smart contract
- [x] ENS (ethereum name service) support
- [x] Libp2p pubsub like protocol for short encrypted messages
- [x] Nice logs
- [x] Docker support
- [x] Prometheus support (for metrics)
- [x] secp256k1 and ed25519 signature and encryption
- [x] Census Merkle Tree implementation
- [x] Native IPFS support
- [x] IPFS cluster support (custom implementation named ipfsSync)
- [x] Tendermint voting blockchain implementation
- [x] Vote Scrutinizer
- [x] BabyJubJub signature and hashing (ZK-snark friendly)
- [ ] ZK-snark integration
- [ ] BootNode automatic discovery

#### Compile and run

Compile from source in a golang environment (Go>1.16 required):

```
git clone https://go.vocdoni.io/dvote.git
cd go-dvote
go build ./cmd/dvotenode
./dvotenode --help
```

#### Docker

You can run go-dvote as a standalone container with a docker script (configuration options can be changed in file `dockerfiles/dvotenode/env`):

```
dockerfiles/gateway/dockerlaunch.sh
```

All data will be stored in the shared volume `run`.

#### Docker compose

Standalone components can be built and run as the examples:

- Gateway (default)

```
cd dockerfiles/dvotenode
docker-compose build
docker-compose up -d
```

- Miner

```
cd dockerfiles/dvotenode
docker-compose build
docker-compose -f docker-compose.miner.yml up -d
```

The test suite is an all-in-one compose file to bootstrap a minimal testing testing environment. To do a voting process test, follow the example:

```
cd dockerfiles/testsuite
docker-compose build
docker-compose up -d
go run ../../cmd/vochaintest/vochaintest.go --oracleKey 6aae1d165dd9776c580b8fdaf8622e39c5f943c715e20690080bbfce2c760223 --electionSize=1000
```

---

[![Contributor Covenant](https://img.shields.io/badge/Contributor%20Covenant-v1.4%20adopted-ff69b4.svg)](code-of-conduct.md) [![License: AGPL v3](https://img.shields.io/badge/License-AGPL%20v3-blue.svg)](https://www.gnu.org/licenses/agpl-3.0)

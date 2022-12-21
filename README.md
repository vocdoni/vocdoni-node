# go-dvote

[![GoDoc](https://godoc.org/go.vocdoni.io/dvote?status.svg)](https://godoc.org/go.vocdoni.io/dvote)
[![Go Report Card](https://goreportcard.com/badge/go.vocdoni.io/dvote)](https://goreportcard.com/report/go.vocdoni.io/dvote)

[![Join Discord](https://img.shields.io/badge/discord-join%20chat-blue.svg)](https://discord.gg/4hKeArDaU2)
[![Twitter Follow](https://img.shields.io/twitter/follow/vocdoni.svg?style=social&label=Follow)](https://twitter.com/vocdoni)

This repository contains a set of libraries and tools for the **Vocdoni** decentralized backend infrastructure, as described [in the documentation](https://docs.vocdoni.io/).

If you want to build on top of the Vocdoni protocol, you can visit the [developer portal](https://developer.vocdoni.io)

A good summary of the whole Vocdoni architecture can be found in [this paper](https://law.mit.edu/pub/remotevotingintheageofcryptography)

## Vocdoni

Vocdoni is a universally verifiable, censorship-resistant, and anonymous self sovereign governance system, designed with the scalability and ease-of-use to support either small/private and big/national elections.

Our main aim is a trustless voting system, where anyone can speak their voice and where everything can be audited. We are engineering building blocks for a permissionless, private and censorship resistant democracy.

We intend the algorithms, systems, and software that we build to be a useful contribution toward making violence in these cryptonetworks impossible by protecting users privacy with cryptography. In particular, our aim is to provide the necessary tooling for the political will of network participants to translate outwardly into real political capital, without sacrificing privacy.

![vocdoni go-dvote team](https://assets.gitlab-static.net/uploads/-/system/project/avatar/12677379/go-dvote.png)

## vocdoni node

The vocdoni node contains all the required features for running the decentralized Vocdoni Protocol blockchain node.

Currently the node can operate in three modes:

- **gateway** provides a full block validation node in addition to an entry point for the API and other services.

- **miner** provides a block validation node (full node), without providing any external service but capable of proposing new blocks.

- **oracle** mode provides special capabilities such as computing election results and storing encryption keys.

The most common mode is the `gateway`, that's probably what you are looking for.

One of the design primitives of vocdoni-node is to run everything as a single process in order to have complete control over the components and avoid local RPC or IPC connections. So unlike other projects, vocdoni node uses go-ethereum, go-ipfs and tendermint as GoLang libraries.

vocdoni-node is currently pure GoLang code, so generating a static and reproducible binary that works on most of the Linux and MacOS hosts without any dependence, is possible.

For running vocdoni-node in gateway mode, 8 GiB of ram memory is recommended (4 GiB works too).

#### Compile and run

Compile from source in a golang environment (Go>1.18 required):

```bash
git clone https://github.com/vocdoni/vocdoni-node.git
cd vocdoni-node
go build ./cmd/node
./node --help
./node --mode=gateway --chain=dev --logLevel=info
```

#### Docker

You can run vocdoni node as a standalone container with docker-compose (recommended).
It is recommended to also start `watchtower` to automatically update the container when a new version is released.

```bash
cd dockerfiles/vocdoninode
cp env.example env
docker-compose -f docker-compose.yml -f docker-compose.watchtower.yml up -d 
```

All data will be stored in the shared volume `run` and the API will be available at `http://127.0.0.1:9090`.

If the computer has the port 443 available and mapped to a public IP, you might want to enable TLS support (HTTPS) using letsencrypt by setting the environment variable `VOCDONI_TLS_DOMAIN=your.domain.io` in the file `dockerfiles/vocdoninode/env`.

To stop the container: 

```bash
docker-compose -f docker-compose.yml -f docker-compose.watchtower.yml down
```

#### Connecting

Once the node has finished the blockchain fast sync process, you can connect through the API:

```
$ curl http://127.0.0.1:9090/v2/chain/info

{
  "chainId": "vocdoni-development-71",
  "blockTime": [
    10000,
    11320,
    0,
    0,
    0
  ],
  "height": 43544,
  "blockTimestamp": 1669205270
}
```

API methods, SDK and documentation can be found at [the developer portal](https://developer.vocdoni.io)

#### Testing

The test suite is an all-in-one compose file to bootstrap a minimal testing testing environment. To do a voting process test, follow the examples mentioned in the included README:

```bash
cd dockerfiles/testsuite
cat README.md
```

---

[![Contributor Covenant](https://img.shields.io/badge/Contributor%20Covenant-v1.4%20adopted-ff69b4.svg)](code-of-conduct.md) [![License: AGPL v3](https://img.shields.io/badge/License-AGPL%20v3-blue.svg)](https://www.gnu.org/licenses/agpl-3.0)

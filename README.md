<p align="center" width="100%">
    <img src="https://developer.vocdoni.io/img/vocdoni_logotype_full_white.svg" />
</p>

<p align="center" width="100%">
    <a href="https://github.com/vocdoni/vocdoni-node/commits/main/"><img src="https://img.shields.io/github/commit-activity/m/vocdoni/vocdoni-node" /></a>
    <a href="https://github.com/vocdoni/vocdoni-node/issues"><img src="https://img.shields.io/github/issues/vocdoni/vocdoni-node" /></a>
    <a href="https://github.com/vocdoni/vocdoni-node/actions/workflows/main.yml/"><img src="https://github.com/vocdoni/vocdoni-node/actions/workflows/main.yml/badge.svg" /></a>
    <a href="https://godoc.org/go.vocdoni.io/dvote"><img src="https://godoc.org/go.vocdoni.io/dvote?status.svg"></a>
    <a href="https://goreportcard.com/report/go.vocdoni.io/dvote"><img src="https://goreportcard.com/badge/go.vocdoni.io/dvote"></a>
    <a href="https://coveralls.io/github/vocdoni/vocdoni-node"><img src="https://coveralls.io/repos/github/vocdoni/vocdoni-node/badge.svg"></a>
    <a href="https://discord.gg/xFTh8Np2ga"><img src="https://img.shields.io/badge/discord-join%20chat-blue.svg" /></a>
    <a href="https://twitter.com/vocdoni"><img src="https://img.shields.io/twitter/follow/vocdoni.svg?style=social&label=Follow" /></a>
</p>


  <div align="center">
    Vocdoni is the first universally verifiable, censorship-resistant, anonymous, and self-sovereign governance protocol. <br />
    Our main aim is a trustless voting system where anyone can speak their voice and where everything is auditable. <br />
    We are engineering building blocks for a permissionless, private and censorship resistant democracy.
    <br />
    <a href="https://developer.vocdoni.io/"><strong>Explore the developer portal »</strong></a>
    <br />
    <h3>More About Us</h3>
    <a href="https://vocdoni.io">Vocdoni Website</a>
    |
    <a href="https://vocdoni.app">Web Application</a>
    |
    <a href="https://explorer.vote/">Blockchain Explorer</a>
    |
    <a href="https://law.mit.edu/pub/remotevotingintheageofcryptography/release/1">MIT Law Publication</a>
    |
    <a href="https://chat.vocdoni.io">Contact Us</a>
    <br />
    <h3>Key Repositories</h3>
    <a href="https://github.com/vocdoni/vocdoni-node">Vocdoni Node</a>
    |
    <a href="https://github.com/vocdoni/vocdoni-sdk/">Vocdoni SDK</a>
    |
    <a href="https://github.com/vocdoni/ui-components">UI Components</a>
    |
    <a href="https://github.com/vocdoni/ui-scaffold">Application UI</a>
    |
    <a href="https://github.com/vocdoni/census3">Census3</a>
  </div>

# vocdoni-node

This repository contains a set of libraries and tools for the **Vocdoni** decentralized protocol, as described [in the documentation](https://developer.vocdoni.io/protocol).
It is written in golang and comprises the core of Vocdoni's distributed node architecture. Vocdoni-node is uniquely designed to run all components within a single process, giving full control and eliminating the need for local RPC or IPC connections. This is in contrast to other projects, as Vocdoni-node incorporates go-ethereum, go-ipfs, and tendermint directly as GoLang libraries.

The latest release is available at https://github.com/vocdoni/vocdoni-node/tree/v1.10.1

### Table of Contents
- [Getting Started](#getting-started)
- [Reference](#reference)
- [Examples](#examples)
- [Contributing](#contributing)
- [License](#license)


## Getting Started

There are two operational modes available for the node:

- Gateway: This mode runs a full block validation node and serves as an access point for the API and other services.

- Miner: In this mode, the node can validate blocks or run as a full node. It does not offer external services but can propose and validate new blocks.

### Gateway mode

The gateway mode is the most frequently used and likely the one you need.

To run a Vocdoni-node as a gateway, it's recommended to have at least 4 GiB of RAM and 40 GiB of disk space.

#### Compile and run

Compile from source in a golang environment (Go>1.23 required):

```bash
git clone https://github.com/vocdoni/vocdoni-node.git -b release-lts-1
cd vocdoni-node
go build ./cmd/node
./node --help
./node --mode=gateway --logLevel=info
```

#### Docker

You can run vocdoni node as a standalone container with docker compose (recommended).
It is recommended to also start `watchtower` to automatically update the container when a new version is released.

```bash
git clone https://github.com/vocdoni/vocdoni-node.git -b release-lts-1
cd vocdoni-node/dockerfiles/vocdoninode
cp env.example env # see env file for config options
COMPOSE_PROFILES=watchtower docker compose up -d
```

All data will be stored in the shared volume `run` and the API will be available at `http://127.0.0.1:9090/v2`. Once the node has finished the blockchain sync process, you can connect query the API at this address.

If the computer has the port 443 available and mapped to a public IP, you might want to enable TLS support (HTTPS) using letsencrypt by setting the environment variable `VOCDONI_TLS_DOMAIN=your.domain.io` in the `env` file.

To stop the container: 

```bash
docker compose down
```

### Miner mode

Miners, also known as validators, play a crucial role in proposing and validating new blocks on the blockchain, ensuring the network operates correctly.

The process of becoming a validator is selective to mitigate the risk of malicious activity. Although the role is not currently compensated, those interested in supporting the open-source protocol are encouraged to apply.

To become a validator, there is a manual verification process in place. If you're interested, please follow these steps and reach out to our team via Discord or email for further instructions.

Generate a private key: `hexdump -n 32 -e '4/4 "%08x" 1 ""' /dev/urandom`

Clone and prepare environment:
```bash
git clone https://github.com/vocdoni/vocdoni-node.git -b release-lts-1
cd vocdoni-node/dockerfiles/vocdoninode
echo "VOCDONI_NODE_TAG=release-lts-1" > .env
```

Create the config `env` file.

```bash
VOCDONI_DATADIR=/app/run
VOCDONI_MODE=miner
VOCDONI_CHAIN=lts
VOCDONI_LOGLEVEL=info
VOCDONI_VOCHAIN_LOGLEVEL=error
VOCDONI_DEV=True
VOCDONI_ENABLEAPI=False
VOCDONI_ENABLERPC=False
VOCDONI_LISTENHOST=0.0.0.0
VOCDONI_LISTENPORT=9090
VOCDONI_VOCHAIN_MINERKEY=<YOUR_HEX_PRIVATE_KEY>
VOCDONI_VOCHAIN_MEMPOOLSIZE=20000
VOCDONI_METRICS_ENABLED=True # if you want prometheus metrics enabled
VOCDONI_METRICS_REFRESHINTERVAL=5
```

Finally start the container: `COMPOSE_PROFILES=watchtower docker compose up -d`

You can monitor the log output: `docker compose logs -f vocdoninode`

#### Fetch the validator key

At this point **if you want your node to become validator**, you need to extract the public key from the logs: 

`docker compose logs vocdoninode | grep publicKey`.

Provide the public key and a name to the Vocdoni team so they can upgrade your node to validator.

## Reference

Vocdoni Node provides a core API with which anyone can interact with the Vocdoni protocol. This REST API is documented at our [developer portal](https://developer.vocdoni.io/vocdoni-api/vocdoni-api). You can also generate these API docs by following the instructions in https://github.com/vocdoni/vocdoni-node/blob/main/api/docs/README.md. 

This API is abstracted by the [Vocdoni SDK](https://developer.vocdoni.io/sdk), a typescript library that facilitates integration with vocdoni-node.

The overall design of the Vocdoni Protocol is [documented](https://developer.vocdoni.io/protocol) as well. This provides a high-level specification of much of the functionality of vocdoni-node. 

Additional lower-level documentation and guidance is provided in the README files for many of the libraries in this repository, especially for internal tools. An example of this is the [statedb](https://github.com/vocdoni/vocdoni-node/blob/main/statedb/README.md) package. 

## Examples

Vocdoni Node is an extensive library and protocol with many different elements. 

You can see example usage of many of the components in the [test](https://github.com/vocdoni/vocdoni-node/tree/main/test) folder. 

We have also created a standalone node configuration called [vocone](https://github.com/vocdoni/vocdoni-node/tree/main/vocone) to be used for testing. You can see how to build vocone and run a full end-to-end example in the corresponding dockerfile [folder](https://github.com/vocdoni/vocdoni-node/blob/main/dockerfiles/vocone/README.md). You can also just build and run vocone and use it to locally test the API or other vocdoni node functionality.

## Contributing 

While we welcome contributions from the community, we do not track all of our issues on Github and we may not have the resources to onboard developers and review complex pull requests. That being said, there are multiple ways you can get involved with the project. 

Please review our [development guidelines](https://developer.vocdoni.io/development-guidelines).

### Testing

The test suite for this repo is an all-in-one compose file to bootstrap a minimal testing environment. To do a voting process test, follow the examples mentioned in the included `dockerfiles/testsuite/README.md`:

```bash
cd dockerfiles/testsuite
cat README.md
bash start_test.sh # creates the environment and runs all tests
```

## License

This repository is licensed under the [GNU Affero General Public License v3.0.](./LICENSE)

    Vocdoni Node core library
    Copyright (C) 2024 Vocdoni Association

    This program is free software: you can redistribute it and/or modify
    it under the terms of the GNU Affero General Public License as published by
    the Free Software Foundation, either version 3 of the License, or
    (at your option) any later version.

    This program is distributed in the hope that it will be useful,
    but WITHOUT ANY WARRANTY; without even the implied warranty of
    MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
    GNU Affero General Public License for more details.

    You should have received a copy of the GNU Affero General Public License
    along with this program.  If not, see <https://www.gnu.org/licenses/>.


---

[![Contributor Covenant](https://img.shields.io/badge/Contributor%20Covenant-v1.4%20adopted-ff69b4.svg)](code-of-conduct.md) [![License: AGPL v3](https://img.shields.io/badge/License-AGPL%20v3-blue.svg)](https://www.gnu.org/licenses/agpl-3.0)

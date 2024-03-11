# vocdoni-node

[![GoDoc](https://godoc.org/go.vocdoni.io/dvote?status.svg)](https://godoc.org/go.vocdoni.io/dvote)
[![Go Report Card](https://goreportcard.com/badge/go.vocdoni.io/dvote)](https://goreportcard.com/report/go.vocdoni.io/dvote)
[![Coverage Status](https://coveralls.io/repos/github/vocdoni/vocdoni-node/badge.svg)](https://coveralls.io/github/vocdoni/vocdoni-node)

[![Join Discord](https://img.shields.io/badge/discord-join%20chat-blue.svg)](https://discord.gg/xFTh8Np2ga)
[![Twitter Follow](https://img.shields.io/twitter/follow/vocdoni.svg?style=social&label=Follow)](https://twitter.com/vocdoni)

This repository contains a set of libraries and tools for the **Vocdoni** decentralized protocol, as described [in the documentation](https://developer.vocdoni.io/protocol/overview).

If you want to build on top of the Vocdoni protocol, you can visit the [developer portal](https://developer.vocdoni.io)

A good summary of the whole Vocdoni architecture can be found in [this paper](https://law.mit.edu/pub/remotevotingintheageofcryptography)

## Vocdoni

Vocdoni is a universally verifiable, censorship-resistant, and anonymous self-sovereign governance protocol, designed with the scalability and ease-of-use to supply all kind of voting needs.

Our main aim is a trustless voting system, where anyone can speak their voice and where everything can be audited. We are engineering building blocks for a permissionless, private and censorship resistant democracy.

We intend the algorithms, systems, and software that we build to be a useful contribution toward making violence in these cryptonetworks impossible by protecting users privacy with cryptography. In particular, our aim is to provide the necessary tooling for the political will of network participants to translate outwardly into real political capital, without sacrificing privacy.

<img src="https://raw.githubusercontent.com/vocdoni/vocdoni-node/main/assets/vocdoni_logo.svg?sanitize=true&raw=true" />


## vocdoni node

The Vocdoni node is equipped with all the necessary components to operate a node on the decentralized Vocdoni protocol blockchain.

There are two operational modes available for the node:

- Gateway: This mode runs a full block validation node and serves as an access point for the API and other services.

- Miner: In this mode, the node can validate blocks or run as a full node. It does not offer external services but can propose and validate new blocks.


## Gateway mode

The gateway mode is the most frequently used and likely the one you need.

Vocdoni-node is uniquely designed to run all components within a single process, giving full control and eliminating the need for local RPC or IPC connections. This is in contrast to other projects, as Vocdoni-node incorporates go-ethereum, go-ipfs, and tendermint directly as GoLang libraries.

To run a Vocdoni-node as a gateway, it's recommended to have at least 4 GiB of RAM and 40 GiB of disk space.

#### Compile and run

Compile from source in a golang environment (Go>1.21 required):

```bash
git clone https://github.com/vocdoni/vocdoni-node.git -b release-lts-1
cd vocdoni-node
go build ./cmd/node
./node --help
./node --mode=gateway --chain=lts --logLevel=info
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

All data will be stored in the shared volume `run` and the API will be available at `http://127.0.0.1:9090/v2`.

If the computer has the port 443 available and mapped to a public IP, you might want to enable TLS support (HTTPS) using letsencrypt by setting the environment variable `VOCDONI_TLS_DOMAIN=your.domain.io` in the `env` file.

To stop the container: 

```bash
docker compose down
```

#### Connecting

Once the node has finished the blockchain sync process, you can connect query the API:

`$ curl http://127.0.0.1:9090/v2/chain/info`

```json
{
  "chainId": "test-chain-1",
  "blockTime": [
    12000,
    12000,
    11650,
    0,
    0
  ],
  "electionCount": 61,
  "organizationCount": 18,
  "genesisTime": "2023-02-28T22:40:43.668920539Z",
  "height": 1546,
  "syncing": false,
  "blockTimestamp": 1683056444,
  "transactionCount": 418935,
  "validatorCount": 4,
  "voteCount": 51201,
  "cicuitConfigurationTag": "dev",
  "maxCensusSize": 10000
}
```

API methods, SDK and documentation can be found at [the developer portal](https://developer.vocdoni.io)

## Miner mode

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

Provide the public key and a fancy name to the Vocdoni team so they can upgrade your node to validator.


## Testing

The test suite is an all-in-one compose file to bootstrap a minimal testing testing environment. To do a voting process test, follow the examples mentioned in the included README:

```bash
cd dockerfiles/testsuite
cat README.md
bash start_test.sh # creates the environment and runs all tests
```

---

[![Contributor Covenant](https://img.shields.io/badge/Contributor%20Covenant-v1.4%20adopted-ff69b4.svg)](code-of-conduct.md) [![License: AGPL v3](https://img.shields.io/badge/License-AGPL%20v3-blue.svg)](https://www.gnu.org/licenses/agpl-3.0)

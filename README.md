<p align="center" width="100%">
    <picture>
      <source media="(prefers-color-scheme: dark)" srcset="https://app-dev.vocdoni.io/assets/logo-classic-white.svg" />
      <source media="(prefers-color-scheme: light)" srcset="https://app-dev.vocdoni.io/assets/logo-classic.svg" />
      <img alt="Vocdoni logo" src="https://app-dev.vocdoni.io/assets/logo-classic.svg" />
  </picture>
</p>

<p align="center" width="100%">
    <a href="https://github.com/vocdoni/vocdoni-node/commits/main/"><img src="https://img.shields.io/github/commit-activity/m/vocdoni/vocdoni-node" /></a>
    <a href="https://github.com/vocdoni/vocdoni-node/issues"><img src="https://img.shields.io/github/issues/vocdoni/vocdoni-node" /></a>
    <a href="https://github.com/vocdoni/vocdoni-node/actions/workflows/main.yml/"><img src="https://github.com/vocdoni/vocdoni-node/actions/workflows/main.yml/badge.svg" /></a>
    <a href="https://godoc.org/go.vocdoni.io/dvote"><img src="https://godoc.org/go.vocdoni.io/dvote?status.svg"></a>
    <a href="https://goreportcard.com/report/go.vocdoni.io/dvote"><img src="https://goreportcard.com/badge/go.vocdoni.io/dvote"></a>
    <a href="https://coveralls.io/github/vocdoni/vocdoni-node"><img src="https://coveralls.io/repos/github/vocdoni/vocdoni-node/badge.svg"></a>
    <a href="https://chat.vocdoni.io"><img src="https://img.shields.io/badge/discord-join%20chat-blue.svg" /></a>
    <a href="https://twitter.com/vocdoni"><img src="https://img.shields.io/twitter/follow/vocdoni.svg?style=social&label=Follow" /></a>
</p>

  <div align="center">
    Vocdoni is the first universally verifiable, censorship-resistant, anonymous, and self-sovereign governance protocol. <br />
    Our main aim is a trustless voting system where anyone can speak their voice and where everything is auditable. <br />
    We are engineering building blocks for a permissionless, private and censorship resistant democracy.
    <br />
    <a href="https://vocdoni.io/developers"><strong>Explore the developer portal »</strong></a>
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
    <a href="https://vocdoni.io/contact">Contact Us</a>
    <br />
    <h3>Key Repositories</h3>
    <a href="https://github.com/vocdoni/vocdoni-app">Vocdoni App</a>
    |
    <a href="https://github.com/vocdoni/vocdoni-node">Vocdoni Node</a>
    |
    <a href="https://github.com/vocdoni/vocdoni-integrator-sdk">Vocdoni Integrator SDK</a>
  </div>

# vocdoni-node

This repository contains a set of libraries and tools for the **Vocdoni** decentralized protocol, as described [in the documentation](https://developer.vocdoni.io/protocol).
It is written in golang and runs the whole protocol in a single process: `voconed` produces the blocks, keeps the chain state and serves the API.

The latest release is available at https://github.com/vocdoni/vocdoni-node/tree/v1.10.1

### Table of Contents
- [Getting Started](#getting-started)
- [Reference](#reference)
- [Examples](#examples)
- [Contributing](#contributing)
- [License](#license)


## Getting Started

The node binary is `voconed` (see [vocone](./vocone)): a single node that produces the blocks,
keeps the chain state and serves the HTTP API.

### Compile and run

Compile from source in a golang environment (see `go.mod` for the required Go version):

```bash
git clone https://github.com/vocdoni/vocdoni-node.git
cd vocdoni-node
go build ./cmd/voconed
./voconed --help
./voconed --dir=$HOME/.voconed --chainID=mychain --enableFaucet=100
```

The chain is created on the first start, using the genesis options (`--chainID`,
`--fundedAccounts`, `--faucetBalance`), and persisted in `--dir`. Later starts resume the same
chain and ignore those options. The keykeeper key (`--keymanager`) and the faucet key
(`--faucetKey`) are generated on the first start if not provided, and saved in `voconed.yml`
inside `--dir`. Admin API endpoints are only enabled when `--adminToken` is set.

All flags can also be set with `VOCONED_` environment variables (e.g. `VOCONED_CHAINID`).

### Docker

The repository `Dockerfile` builds an image that runs `voconed`, storing its data in `/app/run`.
A docker compose deployment is available in [dockerfiles/vocone](./dockerfiles/vocone):

```bash
cd vocdoni-node/dockerfiles/vocone
# edit .env to set the options
docker compose up -d
```

The API will be available at `http://127.0.0.1:9090/v2`.

## Reference

Vocdoni Node provides a core API with which anyone can interact with the Vocdoni protocol. This REST API is documented at our [developer portal](https://developer.vocdoni.io/vocdoni-api/vocdoni-api). You can also generate these API docs by following the instructions in https://github.com/vocdoni/vocdoni-node/blob/main/api/docs/README.md. 

This API is abstracted by the [Vocdoni SDK](https://developer.vocdoni.io/sdk), a typescript library that facilitates integration with vocdoni-node.

The overall design of the Vocdoni Protocol is [documented](https://developer.vocdoni.io/protocol) as well. This provides a high-level specification of much of the functionality of vocdoni-node. 

Additional lower-level documentation and guidance is provided in the README files for many of the libraries in this repository, especially for internal tools. An example of this is the [statedb](https://github.com/vocdoni/vocdoni-node/blob/main/statedb/README.md) package. 

## Examples

Vocdoni Node is an extensive library and protocol with many different elements. 

You can see example usage of many of the components in the [test](https://github.com/vocdoni/vocdoni-node/tree/main/test) folder. 

You can see how to build vocone and run a full end-to-end example in the corresponding dockerfile [folder](https://github.com/vocdoni/vocdoni-node/blob/main/dockerfiles/vocone/README.md).

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

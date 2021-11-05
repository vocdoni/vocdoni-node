# dvotecli

This is a command line tool to help users interact with go-dvote API.
For now, only a small set of commands is available, but more commands will be added for the rest of the API calls.

### Build

```
go build ./cmd/dvotecli

```

### Usage

```
Usage:
  dvotecli [command]

Available Commands:
  genesis-gen Generate keys and genesis for vochain
  help        Help about any command
  json-client JSON command line client

Flags:
  -c, --color         colorize output (default true)
  -h, --help          help for dvotecli
      --host string   host to connect to (default "http://127.0.0.1:9090/dvote")
      --key string    private key for signature (leave blank for auto-generate)

Use "dvotecli [command] --help" for more information about a command.
```

- genesis-gen

This is a helper command to generate custom genesis data, along with the generation of the required private keys for seed nodes, miners and oracles.

```
 ./dvotecli genesis-gen --chainId examplechain --seeds 2 --miners 8 --oracles 2
```

- client

This command will open an interactive input where you can request raw JSON commands to the dvote API. Here are some examples:

```
./dvotecli json-client --host https://gw2.vocdoni.net/dvote
2020-12-23T10:46:55Z    INFO    commands/client.go:36   logger construction succeeded at level  and output stdout
2020-12-23T10:46:55Z    INFO    commands/client.go:47   connecting to https://gw2.vocdoni.net/dvote
{"method":"dump","censusId":"0x16c0feec71ab17f603bb8053802c745f77e75e65cd65e3b1bc92e8c6443be820"}
...
{"method":"genProof","censusId":"0x16c0feec71ab17f603bb8053802c745f77e75e65cd65e3b1bc92e8c6443be820", "digested":true,"claimData":"IutkMaMqFvU+VSNd6DRQs/SHoBxPqPjHZc90/rE/HDw="}
...
{"method": "getProcessList", "entityId":"F904848ea36c46817096E94f932A9901E377C8a5"}
...
```

You can also pipe commands like the following example:

```
echo  '{"method":"getProcListResults", "fromId":"" }' | ./dvotecli json-client --host https://gw2.vocdoni.net/dvote
```

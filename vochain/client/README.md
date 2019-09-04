### Vochain client

In order to make requests to the Vochain we are going to use the `abci-cli` package that tendermint provides.

To run the CLI:

```shell
go run ./cmd/vochain/vochain.go
```

The ABCI-CLi has this list of commands:

```shell
Usage:
  abci-cli [command]

Available Commands:
  batch       Run a batch of abci commands against an application
  check_tx    Validate a tx
  commit      Commit the application state and return the Merkle root hash
  console     Start an interactive abci console for multiple commands
  counter     ABCI demo example
  deliver_tx  Deliver a new tx to the application
  kvstore     ABCI demo example
  echo        Have the application echo a message
  help        Help about any command
  info        Get some info about the application
  query       Query the application state
  set_option  Set an options on the application

Flags:
      --abci string      socket or grpc (default "socket")
      --address string   address of application socket (default "tcp://127.0.0.1:26658")
  -h, --help             help for abci-cli
  -v, --verbose          print the command and results as if it were a console session

Use "abci-cli [command] --help" for more information about a command.
```


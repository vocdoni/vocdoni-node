# vocone

Vocone runs the Vocdoni protocol as a single node: it produces the blocks, keeps the state and
serves the API. The image built from the repository Dockerfile runs `voconed`.

Start with:
```
docker compose build
docker compose up -d
```

Ports 80 and 9090 are now serving the vocone API.

All data is stored in the `run` volume (`VOCONED_DIR`), including:

- `genesis.json`: the chainID and initial state. The genesis options (`VOCONED_CHAINID`,
  `VOCONED_FUNDEDACCOUNTS`, `VOCONED_FAUCETBALANCE`) are only applied when the chain is created;
  on restart the existing chain is resumed.
- `voconed.yml`: the configuration, including the keykeeper and faucet keys if they were generated.
  Back it up together with the chain data: losing the keykeeper key prevents revealing the keys of
  encrypted elections.

See `.env` for the available options, or run `voconed --help`.

For running a voting test:
```
go run ./cmd/end2endtest --host=http://localhost/v2 --logLevel=debug --votes=100 --faucet=http://localhost/v2/open/claim
```

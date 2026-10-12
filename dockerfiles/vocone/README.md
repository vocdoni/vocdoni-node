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

## Importing an existing chain

`voconed` can continue a chain produced by a cometbft node (`vocdoni-node`) instead of creating a
new one. Copy, from a stopped node, `<datadir>/vochain/vcstate` and `<datadir>/vochain/indexer` into
the vocone data directory (as `vcstate/` and `indexer/`), and pass the node genesis:

```
voconed --dir=/app/run --importGenesis=/app/run/import/genesis.json
```

On the first start with an empty vocone chain (no `genesis.json` in the data directory):

- no genesis is applied: the chainID, genesis time and initial height are taken from the imported
  genesis and persisted as `genesis.json`, so later starts resume the imported chain and ignore
  `--importGenesis`;
- block production continues at the state height plus one; that height is the `blockStoreBase`
  reported by `/chain/info`. Older blocks, transactions and votes are served from the imported
  indexer; the cometbft block store is not used;
- the validators of the imported state are replaced by the vocone keykeeper (key index 1). It handles
  the encrypted elections created afterwards; the keys of encrypted elections created on the
  imported chain can only be revealed by the keykeepers of that chain;
- the import fails if the indexer is ahead of the state (copied while the node was running).

No tokens are minted on import: `--fundedAccounts` and `--faucetBalance` only apply to new chains.
The faucet (`--enableFaucet`) requires `--faucetKey` with the key of an account funded in the
imported state. `--importFundedAccounts=address:amount,...` explicitly mints the given amounts
when the chain is imported, and is ignored afterwards.

For running a voting test:
```
go run ./cmd/end2endtest --host=http://localhost/v2 --logLevel=debug --votes=100 --faucet=http://localhost/v2/open/claim
```

# API route prune (parity with vochain-sequencer): design

Status: approved (2026-11-02; Fable audit: REWORK, 11 findings accepted)   Class: architectural
Owner decisions so far (2026-11-02):
- Prune in vocdoni-node every route that vochain-sequencer prunes (34 routes), ZK included.
- The 6 routes that still have live callers are pruned too. Their callers' fixes are explorer-ng #39 and saas-backend #778.
- Rows already in the wallet DB are purged on startup.
- ZK is pruned at the API level only. The transaction layer still accepts anonymous processes.
- This is a new PR, separate from #1501. #1501 keeps only the router `recover`.
- Reuse the vochain-sequencer work wherever it ports, instead of rewriting it.
- The zk branch of `GET /censuses/{id}/proof/{key}` is removed too: it only serves live zk censuses and there are none.
- The Go `apiclient` migrates its methods to the replacement routes (a breaking change for external consumers is accepted).
- Routes are removed, but the responses of the routes that stay are not changed: `sik` and `circuitVersion` stay.

## Problem

vocdoni-node, which still serves the LTS network, exposes API surface that vochain-sequencer already removed. Some of it is risky and the rest is dead weight:

- **Server-side key custody without authentication.**
  - `POST /wallet/add/{privateKey}` stores the private key encrypted with a key derived from the returned UUID (`api/wallet.go:155-183`).
  - Anyone holding that UUID can sign through `GET /wallet/bootstrap`, `GET /wallet/transfer/{dst}/{amount}` and `POST /wallet/election`. All four routes are public (`api/wallet.go:32-66`).
  - The unsalted `hash(privKey)` in `wph_` makes `/wallet/add` an oracle that tells whether a key is already stored (`:97-107`).
  - The key travels in the URL, and the chi request logger records it unredacted (`httprouter/httprouter.go:89-92`).
- **Admin endpoints with known defects and no callers.**
  - `censusIPFSExports` is reset at `api/censuses.go:217`, written from an untracked goroutine with no lock (`:991,1026`) and read by `/list` (`:1055`).
  - `/censuses/import/{ipfscid}` passes `maxSize=0` (`:1091`), so the fetch is only capped at the 1 GB default.
  - `/chain/export/indexer` loads the whole SQLite backup into memory (`api/chain.go:1656`).
- **ZK surface** for a feature with no use in 2026. The LTS audit found no election with `anonymous=true`.
- **Paginated, `/count` and `filter` aliases** that were `@Deprecated` in 2024 and have exact replacements.

## Goals

1. Each of the 34 routes in vochain-sequencer's `api/removed.go` answers **410 Gone** with code `4067` and a message that names the replacement.
2. `POST /censuses/zkweighted` answers 410/4067. The other census types are unchanged.
3. `POST /wallet/add/{privateKey}` becomes stateless. It returns `{token, address}` with a new random UUID and stores nothing. It sends `Deprecation` (RFC 9745) and `Warning: 299` headers and logs each caller.
4. No log sink writes the path segment that follows `/wallet/add/`: not the chi request log, not the apirest verbose auth log, and not the `runHandler` panic log (#1501).
5. On startup the node deletes every key under the wallet prefixes (`wpk_`, `wph_`) of the API DB and logs how many it deleted.
6. The responses of the routes that stay keep their shape. The only exceptions are `censusSiblings`, which only appeared for zk censuses (Design 3), and the request field `ElectionDescription.TempSIKs` of the removed `POST /wallet/election`. `sik` (account) and `circuitVersion` (chain info) stay: a missing field breaks callers silently, and these two expose nothing.
7. The code made dead by the pruning is deleted in **every package of the repo**, not only `api/`. In-repo clients and tests are migrated or deleted following Design 9 and 10.

## Non-goals

- Rejecting anonymous processes in the transaction layer (consensus change). The state and vochain code for SIKs and anonymous voting stays, as does `crypto/zk`.
- Removing cometbft or IPFS from the node. IPFS stays for metadata and census publishing.
- Touching routes that vochain-sequencer still serves beyond what is listed in Goals.
- Changing external clients (SDK, explorer, saas). Their fixes are separate PRs.
- Rate limiting the faucet or `/wallet/add`.

## Current state

- **Route registration** (main @ b5563810): `api/accounts.go:30-118`, `api/censuses.go:168-209`, `api/chain.go:38-278`, `api/elections.go:35-157`, `api/siks.go:20-42`, `api/vote.go:20-44`, `api/wallet.go:32-66`.
- **Handler groups:** `cmd/node/main.go:549-557` (gateway) and `:591-593` (census-only).
- **`SIKHandler` users:** `api/api.go:216`, `cmd/node/main.go:556`, `vocone/vocone.go:185`, `test/siks_test.go:33`, `benchmark/zk_census_benchmark_test.go:49`.
- **`CensusTypeZKWeighted` users:**
  - `api/censuses.go:31,230`
  - `api/census_test.go:275`
  - `test/siks_test.go:88`
  - `benchmark/zk_census_benchmark_test.go:61`
  - `cmd/end2endtest/helpers.go:306`
  - `cmd/end2endtest/zkweighted.go`
- **Wallet DB:** `NewAPI` opens `<dataDir>/db` with `metadb` and the prefix `api/` (`api/api.go:131-152`). Only `api/wallet.go` uses it (`:78,100,172`).
  - The purge has what it needs in `db.Reader.Iterate`, `WriteTx.Delete` and a single `Commit` (`db/interface.go`). Keys yielded by `Iterate` must be copied before use outside the callback (`:56-57`).
- **Census token:** `/censuses/*` only calls `uuid.Parse(msg.AuthToken)` (`api/censuses.go:234,272,434,482,616,653`), so a stateless `/wallet/add` keeps SDK 0.10.0 working.
- **Error codes:**
  - The highest 4xxx is 4066 (`api/errors.go:94`); 4067 and 4069 are free.
  - `apirest` has no `HTTPstatusGone` (`httprouter/apirest/apirest.go:36-41`).
- **ZK in the API:**
  - `SIK` (`api/api_types.go:468`, filled at `api/accounts.go:219`)
  - `CircuitVersion` (`api/api_types.go:453`, `api/chain.go:562`)
  - `ElectionDescription.TempSIKs` (`api/api_types.go:240`, request of `POST /wallet/election`)
  - `ErrSIKNotFound` 4054 and `ErrGettingSIK` 5031 (`api/errors.go:82,124`)
- **In-repo callers of pruned routes:**
  - `apiclient/sik.go:16,29,47`
  - `apiclient/account.go:279,426,446,465`
  - `apiclient/blockchain.go:122-133,151`
  - `apiclient/election.go:493,543`
  - `apiclient/client.go:95` (`circuitVersion`)
  - `cmd/end2endtest/helpers.go:224,379,394,726`
  - `test/api_test.go:263,278,338,339,471,479,492,500,512,590,676,1108`
  - `test/apierror_test.go:55,59,63,67,120,124,128`
- **Swagger:** CI generates it (`.github/workflows/swagger.yml:30`). The repo only commits `api/docs/descriptions/` and `models/`.
- **Reference in vochain-sequencer.** It shares history with vocdoni-node: it forks at `b5563810`, which is vocdoni-node's `main`. The relevant commits:
  - `1fb3211b`: redaction of `/wallet/add` and its test. It cherry-picks with one trivial conflict, in a swagger comment block of `api/wallet.go`.
  - `8ee736ff` (`drop routes no client calls`): creates `api/removed.go` and removes the handlers, apiclient methods and end2endtest steps. It does **not** cherry-pick (10 files in conflict) because it was written on top of `cae6c328`, which had already removed IPFS, ZK and the wallet DB, and of the module rename `638d0b12`.
  - `c3a2984b` (`answer 410 on deprecated paginated routes`): depends on `8ee736ff`.

## Design

**How it is ported.** In this order of preference:
- (a) Cherry-pick `1fb3211b`.
- (b) Copy whole files from the sequencer at `1fb3211b` and change only the import path `github.com/vocdoni/vochain-sequencer` → `go.vocdoni.io/dvote`. This applies to `api/removed.go`, `api/removed_test.go` and the body of `walletAddHandler`.
- (c) Make by hand the deletions that `git show 8ee736ff c3a2984b` describes, using those commits as the checklist of what goes (handlers, apiclient, end2endtest, indexer).
- (d) Write from scratch only what the sequencer does not have: the purge, the ZK fields as an API-only change, and the migration of `test/`.

1. **Tombstones.**
   - Port `api/removed.go` and its test with (b). `NewAPI` calls `enableRemovedRoutes()` unconditionally.
   - Add `ErrEndpointRemoved = apirest.APIerror{Code: 4067, HTTPstatus: apirest.HTTPstatusGone, Err: fmt.Errorf("endpoint removed")}` and `apirest.HTTPstatusGone = http.StatusGone`.
   - Exception to the sequencer's text: `reasonCensusIPFS` becomes `"the census IPFS routes are removed, use GET /censuses/export and POST /censuses/import"`, because vocdoni-node does use IPFS.
2. **Removing registrations and handlers.**
   - Delete the `RegisterMethod` calls of the 34 routes and their handlers, plus helpers and types used only by them, including the `censusIPFSExports` state (`api/censuses.go:217,991,1026,1055`).
   - Where a handler is shared with a live route, keep the shared code and remove only the alias.
   - The `SIKHandler` group goes: the constant, the case at `api/api.go:216`, `cmd/node/main.go:556`, `vocone/vocone.go:185`, and the files `test/siks_test.go` and `benchmark/zk_census_benchmark_test.go`, which only exercise removed routes.
   - Any indexer function left without users is deleted, as `8ee736ff` did (`ExportBackupAsBytes`, `CountTokenTransfersByAccount`). This is allowed only if `go vet` and `grep` confirm it has no other users.
3. **zkweighted.**
   - `POST /censuses/{type}` with `type == zkweighted` returns `ErrEndpointRemoved.With(...)` before any other validation (sequencer `api/censuses.go:206-208`).
   - `GET /censuses/{id}/proof/{key}` loses its zk branch (`api/censuses.go:857-889`): it no longer skips hashing for `ARBO_POSEIDON` and no longer returns `censusSiblings`. The handler and its swagger annotation are ported from the sequencer (`vochain-sequencer/api/censuses.go:792-848`). The route stays, because every Merkle-tree census uses it to vote.
   - `CensusSiblings` disappears from the `Census` type and `ErrCantGetCircomSiblings` disappears if it is left without users. `CensusTypeZKWeighted` stays as a constant only if the zkweighted tombstone uses it; otherwise it goes too.
   - Remove `zkweighted` from `Enums(weighted,zkweighted)` (`api/censuses.go:230`).
3b. **#1501 merges first, reduced to the router `recover`** (commit `httprouter: recover handler panics and reply 500`). Its wallet commit (nil checks and `api/wallet_test.go`) is dropped before the merge, because this PR deletes those handlers. This branch is based on `main` after the merge.
4. **Stateless `/wallet/add`.**
   - Port the sequencer's `walletAddHandler` with (b): value receiver, no `a.db`, `WalletToken{Token, Address}`, the `Deprecation: @1792627200` and `Warning` headers, and caller logging.
   - `ErrCantParsePrivateKey` uses code **4069**, as in the sequencer. 4068 stays unused, because the sequencer reserves it for `ErrRequestBodyTooLarge`.
   - Delete `walletFromToken`, `walletCheckKeyExists` and `privateKeyHashPrefix`.
   - The constants `walletDBprefixEncryptedPrivateKey` and `walletDBprefixPrivateKeyHash` move next to the purge code, which is their only remaining user.
5. **Redaction.**
   - Cherry-pick `1fb3211b` (a): `RedactRequestURI` and `redactingLogFormatter` in the chi `RequestLogger`, plus `httprouter/redact_test.go`.
   - Also route through `RedactRequestURI` the verbose auth log (`httprouter/apirest/apirest.go:216`) and the `runHandler` panic log (`hc.Request.URL.Path`).
6. **Purge.**
   - In `NewAPI`, after `api.db` is opened, iterate the `wpk_` and `wph_` prefixes, copy the keys, delete them in one `WriteTx` and `Commit`.
   - Log `purged N custodial wallet keys` at info level when N > 0.
   - If it fails, `NewAPI` returns the error and the node does not start.
   - It is idempotent.
7. **ZK fields.**
   - `SIK` in the account response and `CircuitVersion` in chain info stay, together with the code that fills them.
   - Remove `ElectionDescription.TempSIKs` but keep `ElectionDescription`, which `cmd/end2endtest` uses.
   - Remove `ErrSIKNotFound` and `ErrGettingSIK` if they are left without users.
8. **Swagger.**
   - Deleting the handlers removes their annotations.
   - Also delete the orphaned files under `api/docs/descriptions/` (at least `electionListByFilterHandler.md`) and fix the descriptions that name removed routes (`api.md`, `censusCreateHandler.md`).
   - The check is that the CI swagger job passes on the PR.
9. **In-repo tests.**
   - `test/siks_test.go` and `benchmark/zk_census_benchmark_test.go` are deleted (Design 2).
   - In `test/api_test.go` and `test/apierror_test.go`, each request to a pruned route is ported to its replacement when the assertion is about the data (`GET /accounts?page=`, `transfersCount`, `GET /chain/transactions?height=`, `transactionCount`, `GET /elections?page=`). It is deleted when the assertion was about the alias itself.
   - The fixtures in `apierror_test.go` that used removed routes to get a specific error code are moved to a live route that produces the same code. Where none exists, the case is deleted.
   - The zkweighted path is removed from `api/census_test.go`.
   - No test checks the old behaviour of a pruned route. AC1 is the only test of the tombstones.
10. **apiclient and end2endtest.**
    - Migrate the methods to their replacements, following the sequencer's `apiclient/account.go`, `blockchain.go` and `election.go` from `8ee736ff` and `c3a2984b`:
      - `CountAccounts`, `CountTransactions` and `ElectionVoteCount` read the count fields of `/accounts`, `/chain/info` and `/elections/{id}`.
      - The `page` methods move to the query-param routes.
      - `OrganizationsBySearchTermPaginated` and `ElectionFilterPaginated` move to `GET /chain/organizations` and `GET /elections` with query params.
    - Delete `apiclient/sik.go` and the SIK branch of `apiclient/vote.go`.
    - `apiclient/client.go` stops reading `circuitVersion` and stops loading zk artifacts in `New()`.
    - Delete the zkweighted modes of `cmd/end2endtest` (`zkweighted.go`) and the SIK branches of `helpers.go`. `helpers.go:726` uses the migrated vote-count method so the remaining modes keep working.

### Alternatives considered

| Option | Why not |
|---|---|
| Cherry-pick `8ee736ff` and `c3a2984b` | They do not apply: their context assumes the IPFS/ZK/wallet prune of `cae6c328` and the module rename. Resolving the 10 conflicts amounts to re-implementing them, so they serve as a checklist, not a patch. |
| Delete the routes outright, with no tombstones | The 410 with a message tells callers what replaces each route, and keeps the two nodes answering the same. |
| Keep the `@Deprecated` aliases as shims | The owner rejected it for the sequencer on 2026-10-22. |
| Leave the stored keys on disk | Owner decision: purge them. |
| Reject anonymous processes in the transaction | Consensus change. The owner left it out of scope. |

## Compatibility and rollout

- **Clients that break:**
  - SDK ≤ 0.8.x, through the paginated aliases.
  - explorer.vote, through `transactions/count`, `transfers/page`, `fees/page`, `fees/reference/.../page` and `transactions/{h}/{i}`. Fix: explorer-ng #39.
  - saas-backend `ElectionMemos`, through `votes/page`. Fix: saas-backend #778.
  - Anonymous-voting clients: `zk.ts` in the SDK and `@vocdoni/api-voting-zk`.
  - External Go consumers of `go.vocdoni.io/dvote/apiclient`, which get the method changes from Design 10 when they upgrade.
- **Rollout order:**
  1. #1501.
  2. explorer-ng #39 and saas-backend #778 deployed.
  3. This PR reaching the LTS gateways.
  The PR description must say so.
- **SDK 0.10.0 censuses:** keep working, because `/wallet/add` keeps returning `{token, address}` and `/censuses` only parses the UUID.
- **Changed responses:** none for live routes, apart from `censusSiblings`, which only zk censuses returned. `sik` and `circuitVersion` stay until the cutover to the sequencer.
- **Rollback:** go back to the previous image. The purged keys are not recovered. That is accepted: with the wallet routes gone nobody can use them, and SDK clients generate a new token on every census.

## Failure modes and security

- **Purge:** a failure stops the node instead of leaving keys half-deleted. No in-tree backend limits the size of the transaction, because `ErrTxnTooBig` is never returned.
- **Redaction:** covers `RequestURI` and `URL.Path`, with and without the `/v2` prefix and with a query string, in the three sinks of Goal 4.
- **`/wallet/add` stays public:** with nothing stored, it no longer grows the DB and no longer acts as an oracle.
- **Removed routes** answer through the tombstones, and no removed handler stays registered.

## Acceptance criteria

- [ ] AC1: A table-driven test calls each of the 34 routes and checks 410, code 4067 and that the message contains the path (ported `api/removed_test.go`).
- [ ] AC2: `POST /censuses/zkweighted` → 410/4067, and `POST /censuses/weighted` → 200 (test).
- [ ] AC3: `POST /wallet/add/{key}` returns `{token, address}` with a valid UUID, the right address and the `Deprecation` header, and leaves no keys under `wpk_`/`wph_`. An invalid key returns 4069 (test).
- [ ] AC4: A census created with the token from `/wallet/add` can be modified with that token (test).
- [ ] AC5:
  - `RedactRequestURI` removes the key in the `/wallet/add/<k>`, `/v2/wallet/add/<k>` and `?query` cases (cherry-picked unit test).
  - An HTTP-level test captures the log of a request to `/wallet/add/<k>` and checks that `<k>` does not appear.
  - Another test checks that the `runHandler` panic log does not contain it either.
- [ ] AC6: A DB with keys under `wpk_`/`wph_` and keys under other prefixes ends up without the first ones and keeps the others after `NewAPI`. A second `NewAPI` does nothing (test).
- [ ] AC7: The response of `GET /censuses/{id}/proof/{key}` does not contain `censusSiblings`, and `/accounts/{id}` and `/chain/info` still contain `sik` and `circuitVersion` (test). `ElectionDescription` has no `TempSIKs` (compiles).
- [ ] AC8: `go build ./...`, `go vet ./...` and `go test ./...` pass on z6 with `GOTOOLCHAIN=go1.26.5`, `gofmt -l` is empty, and the CI swagger job passes.
- [ ] AC9: Outside `removed.go`, nothing in the repo registers, calls or tests a pruned route. Check with `grep` over `api/ apiclient/ cmd/ test/ vocone/ benchmark/` for the pruned paths and for `SIKHandler`, `ErrSIKNotFound`, `ErrGettingSIK`, `walletFromToken`.

## Constraints

- Builds, tests and vet run only on z6 (`ssh z6.vocdoni.net`, `GOTOOLCHAIN=go1.26.5`), never on the laptop.
- Conventional commits `type(scope): summary`, with no AI attribution or `Co-Authored-By`. Commit with explicit paths. Never push without the owner's OK.
- Docs and comments describe what is and why; history goes in the commit message.
- Base: `main` after merging #1501. Branch `feat/api-route-prune`.

## Open questions

(none)

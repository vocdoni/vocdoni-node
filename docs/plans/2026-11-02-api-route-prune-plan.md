# API route prune: implementation plan

Spec: docs/specs/2026-11-02-api-route-prune-design.md
Class: architectural
Repos and branches: ~/src/vd/vocdoni-node → feat/api-route-prune (base ba9bd9d6, main 8a9b7b4b)

Worktree: `/home/user/src/vd/vocdoni-node/.claude/worktrees/trim-1501` (called `$W` below).
Sequencer reference: `refs/sequencer/main` (tip `1fb3211b`), commits `8ee736ff` and `c3a2984b`. These refs are present in this repo.

Intermediate commits do not have to compile or pass tests. Only the final branch must pass build, vet and test on z6 (Task 6). Each task does at most `gofmt -l` on its files, plus a targeted `go build`/`go vet` of the packages it touched when that is cheap. The new tests (AC1–AC7) are written in their tasks and first run in Task 6.

All code in this plan was compiled and the full suite run green on z6 in a scratch copy. The final tree has `go build ./...`, `go vet ./...`, `staticcheck ./...`, gofumpt, `go mod tidy`, `go generate ./...` and the CI `swag init` all clean, and `go test -count=1 ./...` passes (36 packages, about 33 s).

## Global Constraints
- Commits use conventional commits, `type(scope): lowercase summary`, in English.
- No AI attribution and no `Co-Authored-By` trailers, in any commit.
- Commit with explicit paths only: `git commit -m "…" -- <paths>`. Never `git add -A`. Never `git stash`. Never push.
- New files: `git add <path>` then `git commit -m "…" -- <paths>`. Deleted files: `git rm <path>`, then include the path in the commit.
- Never set git `user.email` in the repo or through `-c`. The global identity is the only one.
- Docs and code comments say what is and why. History (what it was, what changed or was removed, when) goes only in commit messages.
- Do not touch untracked files you did not create. The main checkout (`~/src/vd/vocdoni-node`) has untracked `.playwright-mcp/`, `cmd/census-fixture/` and `docs/`. They are irrelevant to this worktree; leave them alone.
- All builds, vets and tests run on z6 only, never on the laptop and never in /tmp:
  - `ssh z6.vocdoni.net`, with `export GOTOOLCHAIN=go1.26.5` in every remote shell.
- Sync recipe for committed work (run from the laptop):
  `git -C $W archive --format=tar HEAD | ssh z6.vocdoni.net "rm -rf ~/scratch/route-prune && mkdir -p ~/scratch/route-prune && tar -x -C ~/scratch/route-prune"`
- Sync recipe for uncommitted work (run from the laptop):
  `tar -C $W --exclude=.git -cf - . | ssh z6.vocdoni.net "rm -rf ~/scratch/route-prune && mkdir -p ~/scratch/route-prune && tar -x -C ~/scratch/route-prune"`
- For multi-line remote scripts, use `ssh z6.vocdoni.net 'bash -s' <<'EOF' … EOF`, so apostrophes in the script do not break the quoting.
- Behaviour that must stay unchanged:
  - Every kept route keeps its path, method, request and response shape.
  - `Account.sik` (`/accounts/{address}`) and `ChainInfo.circuitVersion` (`/chain/info`) stay.
  - `censusVerifyHandler` keeps its ARBO_POSEIDON branch. Only the proof handler drops the zk path.
  - `POST /censuses/import`, `GET /censuses/export` and `GET /censuses/list` keep working, synchronous, admin-only.
  - The indexer's `ExportBackupAsBytes` stays, because `service/indexer.go:60` uses it.
  - The tx-layer apiclient methods (`SetSIK`, `DelSIK`, `RegisterSIKForVote`, `AccountBootstrap(…, sik)`) stay.
  - apiclient methods keep their signatures; only the routes they call change.

## Review Focus
- `api/api.go:144-159` (final tree):
  - `enableRemovedRoutes` runs right after `apirest.NewAPI`, before any `EnableHandlers`.
  - The purge runs right after `api.db` is set. Any error aborts `NewAPI`, so a node never starts with keys half-deleted.
- `api/wallet_purge.go:19` `purgeWalletKeys`:
  - pebble's `Iterate` strips the prefix from the key it passes, so the key must be rebuilt as `prefix+key` and copied.
  - One `WriteTx`, `Discard` deferred.
  - Keys outside `api/` (the prefixed DB) are never touched.
- Redaction sinks:
  - The request logger, through `redactingLogFormatter` (`httprouter/httprouter.go:90`, `:386` `RedactRequestURI`, `:408` `NewLogEntry`).
  - The `runHandler` panic log (`httprouter/httprouter.go:310`).
  - The apirest verbose bearer log (`httprouter/apirest/apirest.go:218`).
  - Check: the key never reaches a log in the `/wallet/add/<k>`, `/v2/wallet/add/<k>` and `?query` cases, and the formatter does not mutate the request it is handed.
- `api/wallet.go:62` `walletAddHandler`:
  - It validates the key and derives the address.
  - It returns `{token, address}` with a new UUID and stores nothing.
  - It sets `Deprecation` and `Warning` on every reply, errors included.
  - Its log line never includes the key.
- `api/censuses.go:816` `censusProofHandler`:
  - It always hashes the key.
  - There is no `censusSiblings` and no zk import.
  - `censusCreateHandler` (`:206`) answers `ErrEndpointRemoved` for `zkweighted` before it parses the token.
- apiclient migration. These methods keep their signatures and must decode the replacement route's shape correctly:
  - `apiclient/account.go`: `:279` `ListTokenTransfers`, `:430` `ListAccounts`, `:444` `CountAccounts`, `:474` `CountTokenTransfers`.
  - `apiclient/blockchain.go`: `:105` `TransactionByHash`, `:123` `OrganizationsBySearchTermPaginated`, `:150` `TransactionCount`.
  - `apiclient/election.go`: `:493` `ElectionVoteCount`, `:520` `ElectionFilterPaginated`.
  - `apiclient/client.go:178` `RequestWithQuery`.
- `api/removed.go:27`: in the final tree, the 34 rows must equal the sequencer's `api/removed.go`. The only allowed difference is the `reasonCensusIPFS` text and the import path.

## Task order and parallelism
| Task | Depends on | Repo/files | May run in parallel with |
|---|---|---|---|
| 1 Request-log redaction | – | `httprouter/` | 2 |
| 2 Wallet, purge, tombstones and API surface | – | `api/` (wallet, removed, api.go, errors.go, api_types.go, api.md) | 1 |
| 3 Anonymous voting (ZK) prune | 2 | `api/` (censuses, chain, census tests), `apiclient/`, `cmd/`, `vocone/`, `test/`, `benchmark/`, `dockerfiles/` | – |
| 4 Server-side alias, IPFS and export route prune | 3 | `api/` handlers, `vochain/indexer/indexer.go` | – |
| 5 apiclient migration and in-repo tests | 4 | `apiclient/`, `test/`, `api/chain_organizations_test.go` | – |
| 6 Integrate and verify | 1–5 | whole repo | – |

Tasks 3–5 share files (`api/censuses.go`, `api/chain.go`, `apiclient/client.go`, `apiclient/election.go`), so they run one after another.

Models:
- Task 1: opus. It is a security sink.
- Task 2: opus. It covers the purge, the wallet handler and the tombstone wiring.
- Task 3: sonnet, careful.
- Task 4: sonnet.
- Task 5: sonnet, careful. The apiclient decoding must match the replacement routes.
- Task 6: opus.

## Acceptance criteria → tasks and tests
| AC | Task | Test / check |
|---|---|---|
| AC1 (34 tombstones: 410, 4067, path in message) | 2 (infra and all rows), 3–4 (handlers removed) | `api/removed_test.go` `TestRemovedRoutes` |
| AC2 (`zkweighted` → 410/4067, `weighted` → 200) | 3 | `api/anonymous_removed_test.go` `TestCensusCreateAnonymousRemoved` |
| AC3 (`/wallet/add` reply, Deprecation, nothing stored, 4069) | 2 | `api/wallet_test.go` `TestWalletAdd`, `TestWalletAddDeprecationHeaders`, `TestWalletAddInvalidKey` |
| AC4 (wallet token drives a census) | 2 | `api/wallet_test.go` `TestWalletAddTokenCreatesCensus` |
| AC5 (redaction) | 1 | `httprouter/redact_test.go` `TestRedactRequestURI`, `TestRequestLogRedactsPrivateKey`; `httprouter/apirest/redact_log_test.go` `TestRequestLogRedactsWalletKey`, `TestPanicLogRedactsWalletKey` |
| AC6 (purge, idempotent, others kept) | 2 | `api/wallet_purge_test.go` `TestNewAPIPurgesWalletKeys` |
| AC7 (no `censusSiblings`; `sik` and `circuitVersion` kept; no `TempSIKs`) | 2 (types), 3 (handler) | `api/anonymous_removed_test.go` `TestAnonymousResponseFields`; build |
| AC8 (build, vet, test, fmt, swagger) | 6 | Task 6 commands |
| AC9 (no live reference to a pruned route) | 6 | AC9 grep in Task 6 |

## Tasks

### Task 1: Request-log redaction
Model: opus   Repo: ~/src/vd/vocdoni-node   Files: `httprouter/httprouter.go`, `httprouter/apirest/apirest.go`, `httprouter/redact_test.go` (new), `httprouter/apirest/redact_log_test.go` (new)
Goal: AC5. A private key in `/wallet/add/{privateKey}` never reaches a log: not the request log, not the apirest verbose log, not the handler panic log.

Step 1: cherry-pick the sequencer's httprouter part. It applies cleanly with offsets.
```sh
cd $W && git show 1fb3211b -- httprouter/ | git apply
```
This adds:
- `RedactRequestURI`, `secretPathPrefixes` and `redactingLogFormatter`, wired into the `RequestLogger`;
- the redacted apirest verbose log;
- `HTTPstatusGone`;
- `httprouter/redact_test.go`.

Step 2: on top of Step 1, redact the `runHandler` panic log (`RedactRequestURI(hc.Request.URL.Path)`), which the cherry-pick does not cover, and add the HTTP-level test `httprouter/apirest/redact_log_test.go`. The full diff of Task 1 against base, Steps 1 and 2 together, is:
```diff
diff --git a/httprouter/apirest/apirest.go b/httprouter/apirest/apirest.go
index 83ab6e7..b41f1d6 100644
--- a/httprouter/apirest/apirest.go
+++ b/httprouter/apirest/apirest.go
@@ -38,6 +38,7 @@ const (
 	HTTPstatusBadRequest         = http.StatusBadRequest
 	HTTPstatusInternalErr        = http.StatusInternalServerError
 	HTTPstatusNotFound           = http.StatusNotFound
+	HTTPstatusGone               = http.StatusGone
 	HTTPstatusServiceUnavailable = http.StatusServiceUnavailable
 )
 
@@ -213,7 +214,8 @@ func (a *API) ProcessData(req *http.Request) (any, error) {
 	}
 	// If verbose logging is enabled, log the verbose authentication information
 	if a.verboseAuthLog && msg.AuthToken != "" {
-		fmt.Printf("[BearerAPI/%d/%s] %s {%s}\n", time.Now().Unix(), msg.AuthToken, req.URL.RequestURI(), reqBody)
+		fmt.Printf("[BearerAPI/%d/%s] %s {%s}\n", time.Now().Unix(), msg.AuthToken,
+			httprouter.RedactRequestURI(req.URL.RequestURI()), reqBody)
 	}
 	return msg, nil
 }
diff --git a/httprouter/apirest/redact_log_test.go b/httprouter/apirest/redact_log_test.go
new file mode 100644
index 0000000..722aa24
--- /dev/null
+++ b/httprouter/apirest/redact_log_test.go
@@ -0,0 +1,98 @@
+package apirest
+
+import (
+	"cmp"
+	"fmt"
+	"io"
+	"net/http"
+	"os"
+	"path/filepath"
+	"strings"
+	"testing"
+	"time"
+
+	qt "github.com/frankban/quicktest"
+	"go.vocdoni.io/dvote/httprouter"
+	"go.vocdoni.io/dvote/log"
+)
+
+const redactTestKey = "0b1e5e0b1e5e0b1e5e0b1e5e0b1e5e0b1e5e0b1e5e0b1e5e0b1e5e0b1e5e0b1e"
+
+// captureLog sends the global log, at debug level, to a file for the rest of
+// the test and returns a function that reads what was logged so far.
+func captureLog(t *testing.T) func() string {
+	t.Helper()
+	path := filepath.Join(t.TempDir(), "log.txt")
+	log.Init(log.LogLevelDebug, path, nil)
+	t.Cleanup(func() { log.Init(cmp.Or(os.Getenv("LOG_LEVEL"), log.LogLevelError), "stderr", nil) })
+	return func() string {
+		data, err := os.ReadFile(path)
+		qt.Assert(t, err, qt.IsNil)
+		return string(data)
+	}
+}
+
+// waitForLog polls the captured log until it contains want, since the request
+// log line is written after the response reaches the client.
+func waitForLog(t *testing.T, logged func() string, want string) string {
+	t.Helper()
+	deadline := time.Now().Add(5 * time.Second)
+	for {
+		got := logged()
+		if strings.Contains(got, want) {
+			return got
+		}
+		if time.Now().After(deadline) {
+			t.Fatalf("log never contained %q:\n%s", want, got)
+		}
+		time.Sleep(10 * time.Millisecond)
+	}
+}
+
+// TestRequestLogRedactsWalletKey checks that the request log of a real
+// POST /wallet/add/{privateKey}, with and without a query string, never
+// records the key.
+func TestRequestLogRedactsWalletKey(t *testing.T) {
+	logged := captureLog(t)
+	r := httprouter.HTTProuter{}
+	qt.Assert(t, r.Init("127.0.0.1", 0), qt.IsNil)
+	api, err := NewAPI(&r, "/v2")
+	qt.Assert(t, err, qt.IsNil)
+	qt.Assert(t, api.RegisterMethod("/wallet/add/{privateKey}", "POST", MethodAccessTypePublic,
+		func(_ *APIdata, ctx *httprouter.HTTPContext) error {
+			return ctx.Send([]byte("ok"), HTTPstatusOK)
+		}), qt.IsNil)
+
+	for _, suffix := range []string{"", "?x=1"} {
+		url := fmt.Sprintf("http://%s/v2/wallet/add/%s%s", r.Address(), redactTestKey, suffix)
+		resp, err := http.Post(url, "", nil)
+		qt.Assert(t, err, qt.IsNil)
+		_, err = io.Copy(io.Discard, resp.Body)
+		qt.Assert(t, err, qt.IsNil)
+		qt.Assert(t, resp.Body.Close(), qt.IsNil)
+		qt.Assert(t, resp.StatusCode, qt.Equals, HTTPstatusOK)
+		got := waitForLog(t, logged, "/v2/wallet/add/<redacted>"+suffix)
+		qt.Assert(t, got, qt.Not(qt.Contains), redactTestKey)
+	}
+}
+
+// TestPanicLogRedactsWalletKey checks that the log of a handler panic while
+// serving POST /wallet/add/{privateKey} does not record the key.
+func TestPanicLogRedactsWalletKey(t *testing.T) {
+	logged := captureLog(t)
+	r := httprouter.HTTProuter{}
+	qt.Assert(t, r.Init("127.0.0.1", 0), qt.IsNil)
+	api, err := NewAPI(&r, "/v2")
+	qt.Assert(t, err, qt.IsNil)
+	qt.Assert(t, api.RegisterMethod("/wallet/add/{privateKey}", "POST", MethodAccessTypePublic,
+		func(*APIdata, *httprouter.HTTPContext) error {
+			panic("handler bug")
+		}), qt.IsNil)
+
+	resp, err := http.Post(fmt.Sprintf("http://%s/v2/wallet/add/%s", r.Address(), redactTestKey), "", nil)
+	qt.Assert(t, err, qt.IsNil)
+	qt.Assert(t, resp.Body.Close(), qt.IsNil)
+	qt.Assert(t, resp.StatusCode, qt.Equals, HTTPstatusInternalErr)
+	got := waitForLog(t, logged, "panic serving POST /v2/wallet/add/<redacted>")
+	qt.Assert(t, got, qt.Not(qt.Contains), redactTestKey)
+}
diff --git a/httprouter/httprouter.go b/httprouter/httprouter.go
index 1f89abb..7558e82 100644
--- a/httprouter/httprouter.go
+++ b/httprouter/httprouter.go
@@ -87,10 +87,10 @@ func (r *HTTProuter) Init(host string, port int) error {
 
 	// If we want rich logging (e.g. with fields), we could implement our
 	// own version of DefaultLogFormatter.
-	r.Mux.Use(middleware.RequestLogger(&middleware.DefaultLogFormatter{
+	r.Mux.Use(middleware.RequestLogger(redactingLogFormatter{&middleware.DefaultLogFormatter{
 		Logger:  stdLogger{log.Logger()},
 		NoColor: true,
-	}))
+	}}))
 	r.Mux.Use(middleware.Recoverer)
 	r.Mux.Use(middleware.Heartbeat("/ping"))
 	r.Mux.Use(middleware.ThrottleBacklog(5000, 40000, 30*time.Second))
@@ -307,7 +307,7 @@ func runHandler(handlerFunc RouterHandlerFn, msg Message, hc *HTTPContext) {
 			return
 		}
 		log.Errorw(fmt.Errorf("%v", r), fmt.Sprintf("panic serving %s %s\n%s",
-			hc.Request.Method, hc.Request.URL.Path, debug.Stack()))
+			hc.Request.Method, RedactRequestURI(hc.Request.URL.Path), debug.Stack()))
 		select {
 		case <-hc.sent:
 		default:
@@ -376,3 +376,44 @@ type stdLogger struct {
 }
 
 func (l stdLogger) Print(v ...any) { l.log.Debug().Msg(fmt.Sprint(v...)) }
+
+// secretPathPrefixes are the path prefixes whose next segment is a secret:
+// POST /wallet/add/{privateKey} carries a private key in the URL.
+var secretPathPrefixes = []string{"/wallet/add/"}
+
+// RedactRequestURI returns uri with the path segment that follows any of
+// secretPathPrefixes replaced by "<redacted>", so request logs never record it.
+func RedactRequestURI(uri string) string {
+	for _, prefix := range secretPathPrefixes {
+		i := strings.Index(uri, prefix)
+		if i < 0 {
+			continue
+		}
+		start := i + len(prefix)
+		end := strings.IndexAny(uri[start:], "/?#")
+		if end < 0 {
+			end = len(uri) - start
+		}
+		uri = uri[:start] + "<redacted>" + uri[start+end:]
+	}
+	return uri
+}
+
+// redactingLogFormatter hands the request log formatter a copy of the request
+// whose URI went through RedactRequestURI.
+type redactingLogFormatter struct {
+	middleware.LogFormatter
+}
+
+func (f redactingLogFormatter) NewLogEntry(r *http.Request) middleware.LogEntry {
+	redacted := RedactRequestURI(r.RequestURI)
+	if redacted == r.RequestURI {
+		return f.LogFormatter.NewLogEntry(r)
+	}
+	rc := *r
+	rc.RequestURI = redacted
+	u := *r.URL
+	u.Path, u.RawPath = RedactRequestURI(r.URL.Path), ""
+	rc.URL = &u
+	return f.LogFormatter.NewLogEntry(&rc)
+}
diff --git a/httprouter/redact_test.go b/httprouter/redact_test.go
new file mode 100644
index 0000000..02fcf52
--- /dev/null
+++ b/httprouter/redact_test.go
@@ -0,0 +1,46 @@
+package httprouter
+
+import (
+	"bytes"
+	stdlog "log"
+	"net/http/httptest"
+	"testing"
+	"time"
+
+	qt "github.com/frankban/quicktest"
+	"github.com/go-chi/chi/v5/middleware"
+)
+
+const testKey = "0b1e5e0b1e5e0b1e5e0b1e5e0b1e5e0b1e5e0b1e5e0b1e5e0b1e5e0b1e5e0b1e"
+
+func TestRedactRequestURI(t *testing.T) {
+	for _, tc := range []struct{ in, want string }{
+		{"/v2/wallet/add/" + testKey, "/v2/wallet/add/<redacted>"},
+		{"/wallet/add/0x" + testKey, "/wallet/add/<redacted>"},
+		{"/v2/wallet/add/" + testKey + "?x=1", "/v2/wallet/add/<redacted>?x=1"},
+		{"/v2/wallet/add/" + testKey + "/extra", "/v2/wallet/add/<redacted>/extra"},
+		{"/v2/wallet/add/", "/v2/wallet/add/<redacted>"},
+		{"/v2/wallet/bootstrap", "/v2/wallet/bootstrap"},
+		{"/v2/censuses/weighted", "/v2/censuses/weighted"},
+	} {
+		qt.Check(t, RedactRequestURI(tc.in), qt.Equals, tc.want, qt.Commentf("in: %s", tc.in))
+	}
+}
+
+// TestRequestLogRedactsPrivateKey checks that the request log line of
+// POST /wallet/add/{privateKey} carries <redacted> instead of the key, and
+// that the formatter leaves the request it was given untouched.
+func TestRequestLogRedactsPrivateKey(t *testing.T) {
+	var buf bytes.Buffer
+	f := redactingLogFormatter{&middleware.DefaultLogFormatter{
+		Logger:  stdlog.New(&buf, "", 0),
+		NoColor: true,
+	}}
+	req := httptest.NewRequest("POST", "/v2/wallet/add/"+testKey, nil)
+	f.NewLogEntry(req).Write(200, 2, nil, time.Millisecond, nil)
+
+	qt.Assert(t, buf.String(), qt.Not(qt.Contains), testKey)
+	qt.Assert(t, buf.String(), qt.Contains, "/v2/wallet/add/<redacted>")
+	qt.Assert(t, req.RequestURI, qt.Equals, "/v2/wallet/add/"+testKey)
+	qt.Assert(t, req.URL.Path, qt.Equals, "/v2/wallet/add/"+testKey)
+}
```

Step 3: quick check.
Run: `git -C $W diff --name-only HEAD; tar -C $W --exclude=.git -cf - . | ssh z6.vocdoni.net "rm -rf ~/scratch/route-prune && mkdir -p ~/scratch/route-prune && tar -x -C ~/scratch/route-prune" && ssh z6.vocdoni.net 'cd ~/scratch/route-prune && export GOTOOLCHAIN=go1.26.5 && gofmt -l httprouter && go vet ./httprouter/...'`
Expected: no output from `gofmt`/`vet`. `httprouter` does not depend on the other tasks, so `go test -count=1 ./httprouter/...` may also be run here. The expected output is `ok` for `httprouter` and `httprouter/apirest`.

Step 4: commit.
`git add httprouter/redact_test.go httprouter/apirest/redact_log_test.go && git commit -m "fix(httprouter): redact the wallet private key from request and panic logs" -- httprouter/httprouter.go httprouter/apirest/apirest.go httprouter/redact_test.go httprouter/apirest/redact_log_test.go`

Interfaces provided:
- `httprouter.RedactRequestURI(uri string) string`
- `apirest.HTTPstatusGone`

### Task 2: Stateless wallet, key purge, tombstones and API surface
Model: opus   Repo: ~/src/vd/vocdoni-node
Files:
- `api/wallet.go` (replaced)
- `api/wallet_purge.go` (new)
- `api/wallet_purge_test.go` (new)
- `api/wallet_test.go` (new)
- `api/removed.go` (new)
- `api/removed_test.go` (new)
- `api/api.go`
- `api/errors.go`
- `api/api_types.go`
- `api/docs/descriptions/api.md`

Goal: AC3, AC4, AC6, and the AC1 infrastructure with all 34 tombstone rows. It also gives the final shape of `api.go`, `errors.go` and `api_types.go`, so later tasks do not edit them.

`api/removed.go` gets all 34 rows here. Until Tasks 3–4 remove the live registrations, chi silently lets the later `EnableHandlers` registration overwrite a tombstone with the same method and pattern. This is harmless: nothing is tested before Task 6, and in the final tree every pattern is registered only once.

Step 1: tests (they run in Task 6).

`api/removed_test.go`:
```go
package api

import (
	"encoding/json"
	"net/url"
	"regexp"
	"strings"
	"testing"

	qt "github.com/frankban/quicktest"
	"github.com/google/uuid"
	"go.vocdoni.io/dvote/api/censusdb"
	"go.vocdoni.io/dvote/data/ipfs"
	"go.vocdoni.io/dvote/db"
	"go.vocdoni.io/dvote/db/metadb"
	"go.vocdoni.io/dvote/httprouter"
	"go.vocdoni.io/dvote/httprouter/apirest"
	"go.vocdoni.io/dvote/test/testcommon/testutil"
	"go.vocdoni.io/dvote/vochain"
	"go.vocdoni.io/dvote/vochain/indexer"
	"go.vocdoni.io/dvote/vochain/vochaininfo"
)

// newFullTestAPI starts an API with every handler group a gateway enables and
// returns it with a client for its base URL.
func newFullTestAPI(t *testing.T) (*API, *testutil.TestHTTPclient) {
	t.Helper()
	router := httprouter.HTTProuter{}
	qt.Assert(t, router.Init("127.0.0.1", 0), qt.IsNil)
	addr, err := url.Parse("http://" + router.Address().String() + "/")
	qt.Assert(t, err, qt.IsNil)
	api, err := NewAPI(&router, "/", t.TempDir(), db.TypePebble)
	qt.Assert(t, err, qt.IsNil)

	app := vochain.TestBaseApplication(t)
	idx, err := indexer.New(app, indexer.Options{DataDir: t.TempDir()})
	qt.Assert(t, err, qt.IsNil)
	t.Cleanup(func() { _ = idx.Close() })
	kv, err := metadb.New(db.TypePebble, t.TempDir())
	qt.Assert(t, err, qt.IsNil)
	api.Attach(app, vochaininfo.NewVochainInfo(app), idx, ipfs.MockIPFS(t), censusdb.NewCensusDB(kv))
	qt.Assert(t, api.EnableHandlers(ElectionHandler, VoteHandler, ChainHandler,
		WalletHandler, AccountHandler, CensusHandler), qt.IsNil)

	token := uuid.New()
	return api, testutil.NewTestHTTPclient(t, addr, &token)
}

// assertAPIError checks that a response is the given API error.
func assertAPIError(t *testing.T, resp []byte, code int, want apirest.APIerror) {
	t.Helper()
	qt.Assert(t, code, qt.Equals, want.HTTPstatus, qt.Commentf("response: %s", resp))
	got := struct {
		Code int `json:"code"`
	}{}
	qt.Assert(t, json.Unmarshal(resp, &got), qt.IsNil, qt.Commentf("response: %s", resp))
	qt.Assert(t, got.Code, qt.Equals, want.Code, qt.Commentf("response: %s", resp))
}

// TestRemovedRoutes checks that every removed route answers ErrEndpointRemoved
// with its reason, even where a kept route with a URL parameter matches the
// same path (such as /accounts/{address} for /accounts/count).
func TestRemovedRoutes(t *testing.T) {
	_, cl := newFullTestAPI(t)
	param := regexp.MustCompile(`\{[^}]+\}`)
	for _, r := range removedRoutes {
		t.Run(r.method+" "+r.path, func(t *testing.T) {
			c := qt.New(t)
			path := param.ReplaceAllString(r.path, "0x0000000000000000000000000000000000000001")
			resp, code := cl.Request(r.method, nil, strings.TrimPrefix(path, "/"))
			assertAPIError(t, resp, code, ErrEndpointRemoved)
			var got struct {
				Error string `json:"error"`
			}
			c.Assert(json.Unmarshal(resp, &got), qt.IsNil)
			c.Assert(got.Error, qt.Contains, r.path)
			c.Assert(got.Error, qt.Contains, r.reason)
		})
	}
}
```

`api/wallet_test.go`:
```go
package api

import (
	"encoding/json"
	"fmt"
	"math/big"
	"net/http"
	"net/url"
	"testing"

	qt "github.com/frankban/quicktest"
	"github.com/google/uuid"
	"go.vocdoni.io/dvote/api/censusdb"
	"go.vocdoni.io/dvote/crypto/ethereum"
	"go.vocdoni.io/dvote/db"
	"go.vocdoni.io/dvote/db/metadb"
	"go.vocdoni.io/dvote/httprouter"
	"go.vocdoni.io/dvote/test/testcommon/testutil"
	"go.vocdoni.io/dvote/types"
	"go.vocdoni.io/dvote/vochain"
)

// newWalletTestAPI starts an API with the census and wallet handlers enabled
// and returns it with its base URL.
func newWalletTestAPI(t *testing.T) (*API, *url.URL) {
	router := httprouter.HTTProuter{}
	router.Init("127.0.0.1", 0)
	addr, err := url.Parse("http://" + router.Address().String() + "/")
	qt.Assert(t, err, qt.IsNil)

	api, err := NewAPI(&router, "/", t.TempDir(), db.TypePebble)
	qt.Assert(t, err, qt.IsNil)
	mdb, err := metadb.New(db.TypePebble, t.TempDir())
	qt.Assert(t, err, qt.IsNil)
	api.Attach(vochain.TestBaseApplication(t), nil, nil, nil, censusdb.NewCensusDB(mdb))
	qt.Assert(t, api.EnableHandlers(CensusHandler, WalletHandler), qt.IsNil)
	return api, addr
}

// walletAdd calls POST /wallet/add/{privateKey} and decodes the reply.
func walletAdd(t *testing.T, cl *testutil.TestHTTPclient, privKey string) WalletToken {
	t.Helper()
	resp, code := cl.Request("POST", nil, "wallet", "add", privKey)
	qt.Assert(t, code, qt.Equals, 200, qt.Commentf("response: %s", resp))
	var got WalletToken
	qt.Assert(t, json.Unmarshal(resp, &got), qt.IsNil)
	return got
}

// TestWalletAdd checks that POST /wallet/add/{privateKey} returns the address
// of the key and a new random token on each call, with or without 0x, and
// that it stores nothing in the API database.
func TestWalletAdd(t *testing.T) {
	c := qt.New(t)
	api, addr := newWalletTestAPI(t)
	cl := testutil.NewTestHTTPclient(t, addr, nil)

	signer := ethereum.NewSignKeys()
	c.Assert(signer.Generate(), qt.IsNil)
	_, privKey := signer.HexString()

	first := walletAdd(t, cl, privKey)
	c.Assert(first.Token, qt.Not(qt.Equals), uuid.Nil)
	c.Assert(first.Address, qt.DeepEquals, types.HexBytes(signer.Address().Bytes()))

	second := walletAdd(t, cl, "0x"+privKey)
	c.Assert(second.Address, qt.DeepEquals, first.Address)
	c.Assert(second.Token, qt.Not(qt.Equals), first.Token)

	// The reply carries the token as a UUID string, the way the SDK reads it.
	resp, _ := cl.Request("POST", nil, "wallet", "add", privKey)
	var raw map[string]string
	c.Assert(json.Unmarshal(resp, &raw), qt.IsNil)
	_, err := uuid.Parse(raw["token"])
	c.Assert(err, qt.IsNil)
	c.Assert(raw["address"], qt.Equals, fmt.Sprintf("%x", signer.Address().Bytes()))

	for _, prefix := range []string{walletDBprefixEncryptedPrivateKey, walletDBprefixPrivateKeyHash} {
		stored := 0
		c.Assert(api.db.Iterate([]byte(prefix), func(_, _ []byte) bool {
			stored++
			return true
		}), qt.IsNil)
		c.Assert(stored, qt.Equals, 0, qt.Commentf("keys under %s", prefix))
	}
}

// TestWalletAddTokenCreatesCensus follows the @vocdoni/sdk census flow: get a
// token from /wallet/add with a throwaway key, then use it as bearer to create
// a census and add a participant.
func TestWalletAddTokenCreatesCensus(t *testing.T) {
	c := qt.New(t)
	_, addr := newWalletTestAPI(t)

	signer := ethereum.NewSignKeys()
	c.Assert(signer.Generate(), qt.IsNil)
	_, privKey := signer.HexString()
	wallet := walletAdd(t, testutil.NewTestHTTPclient(t, addr, nil), privKey)

	cl := testutil.NewTestHTTPclient(t, addr, &wallet.Token)
	resp, code := cl.Request("POST", nil, "censuses", CensusTypeWeighted)
	c.Assert(code, qt.Equals, 200, qt.Commentf("response: %s", resp))
	census := &Census{}
	c.Assert(json.Unmarshal(resp, census), qt.IsNil)

	participants := CensusParticipants{Participants: []CensusParticipant{{
		Key:    signer.Address().Bytes(),
		Weight: (*types.BigInt)(big.NewInt(1)),
	}}}
	resp, code = cl.Request("POST", &participants, "censuses", census.CensusID.String(), "participants")
	c.Assert(code, qt.Equals, 200, qt.Commentf("response: %s", resp))

	resp, code = cl.Request("GET", nil, "censuses", census.CensusID.String(), "size")
	c.Assert(code, qt.Equals, 200, qt.Commentf("response: %s", resp))
	c.Assert(json.Unmarshal(resp, census), qt.IsNil)
	c.Assert(census.Size, qt.Equals, uint64(1))
}

// TestWalletAddDeprecationHeaders checks that every reply of
// POST /wallet/add/{privateKey}, accepted or rejected, carries the RFC 9745
// Deprecation header and a Warning saying the key is not stored.
func TestWalletAddDeprecationHeaders(t *testing.T) {
	c := qt.New(t)
	_, addr := newWalletTestAPI(t)

	signer := ethereum.NewSignKeys()
	c.Assert(signer.Generate(), qt.IsNil)
	_, privKey := signer.HexString()

	for _, tc := range []struct {
		key  string
		code int
	}{
		{privKey, http.StatusOK},
		{"zz", ErrCantParsePrivateKey.HTTPstatus},
	} {
		resp, err := http.Post(addr.JoinPath("wallet", "add", tc.key).String(), "", nil)
		c.Assert(err, qt.IsNil)
		c.Assert(resp.Body.Close(), qt.IsNil)
		c.Assert(resp.StatusCode, qt.Equals, tc.code)
		c.Assert(resp.Header.Get("Deprecation"), qt.Equals, walletAddDeprecation)
		c.Assert(resp.Header.Get("Warning"), qt.Equals, fmt.Sprintf("299 - %q", walletAddWarning))
		c.Assert(resp.Header.Get("Warning"), qt.Contains, "not stored")
	}
}

// TestWalletAddInvalidKey checks that a key that is not 32 hex encoded bytes
// is rejected with ErrCantParsePrivateKey.
func TestWalletAddInvalidKey(t *testing.T) {
	_, addr := newWalletTestAPI(t)
	cl := testutil.NewTestHTTPclient(t, addr, nil)
	for _, key := range []string{
		"zz",
		"0x1234",
		"00000000000000000000000000000000000000000000000000000000000000000000",
		"0000000000000000000000000000000000000000000000000000000000000000",
		"0xgggggggggggggggggggggggggggggggggggggggggggggggggggggggggggggggg",
	} {
		t.Run(key, func(t *testing.T) {
			resp, code := cl.Request("POST", nil, "wallet", "add", key)
			assertAPIError(t, resp, code, ErrCantParsePrivateKey)
		})
	}
}
```

`api/wallet_purge_test.go`:
```go
package api

import (
	"fmt"
	"path/filepath"
	"testing"

	qt "github.com/frankban/quicktest"
	"go.vocdoni.io/dvote/db"
	"go.vocdoni.io/dvote/db/metadb"
	"go.vocdoni.io/dvote/db/prefixeddb"
	"go.vocdoni.io/dvote/httprouter"
)

// TestNewAPIPurgesWalletKeys checks that NewAPI deletes every key under the
// custodial wallet prefixes of the API database, keeps the other keys, and
// that a second start deletes nothing.
func TestNewAPIPurgesWalletKeys(t *testing.T) {
	c := qt.New(t)
	dataDir := t.TempDir()

	walletKeys := [][]byte{}
	for i := range 3 {
		walletKeys = append(walletKeys,
			fmt.Appendf(nil, "%s%032x", walletDBprefixEncryptedPrivateKey, i),
			fmt.Appendf(nil, "%s%032x", walletDBprefixPrivateKeyHash, i))
	}
	otherKeys := [][]byte{[]byte("other"), []byte("wp_x"), []byte("wpkx")}

	// the database NewAPI opens, with the same prefix
	mdb, err := metadb.New(db.TypePebble, filepath.Join(dataDir, "db"))
	c.Assert(err, qt.IsNil)
	apiDB := prefixeddb.NewPrefixedDatabase(mdb, []byte("api/"))
	wtx := apiDB.WriteTx()
	for _, key := range append(append([][]byte{}, walletKeys...), otherKeys...) {
		c.Assert(wtx.Set(key, []byte("v")), qt.IsNil)
	}
	c.Assert(wtx.Commit(), qt.IsNil)
	// a key with a wallet prefix outside api/ belongs to someone else
	wtx = mdb.WriteTx()
	c.Assert(wtx.Set([]byte(walletDBprefixEncryptedPrivateKey+"outside"), []byte("v")), qt.IsNil)
	c.Assert(wtx.Commit(), qt.IsNil)
	c.Assert(mdb.Close(), qt.IsNil)

	newAPI := func() *API {
		router := httprouter.HTTProuter{}
		c.Assert(router.Init("127.0.0.1", 0), qt.IsNil)
		api, err := NewAPI(&router, "/", dataDir, db.TypePebble)
		c.Assert(err, qt.IsNil)
		return api
	}

	api := newAPI()
	for _, key := range walletKeys {
		_, err := api.db.Get(key)
		c.Assert(err, qt.ErrorIs, db.ErrKeyNotFound, qt.Commentf("key %s", key))
	}
	for _, key := range otherKeys {
		_, err := api.db.Get(key)
		c.Assert(err, qt.IsNil, qt.Commentf("key %s", key))
	}
	purged, err := purgeWalletKeys(api.db)
	c.Assert(err, qt.IsNil)
	c.Assert(purged, qt.Equals, 0)
	c.Assert(api.db.Close(), qt.IsNil)

	// a second start finds nothing to purge and keeps the rest
	api = newAPI()
	for _, key := range otherKeys {
		_, err := api.db.Get(key)
		c.Assert(err, qt.IsNil, qt.Commentf("key %s", key))
	}
	c.Assert(api.db.Close(), qt.IsNil)

	mdb, err = metadb.New(db.TypePebble, filepath.Join(dataDir, "db"))
	c.Assert(err, qt.IsNil)
	_, err = mdb.Get([]byte(walletDBprefixEncryptedPrivateKey + "outside"))
	c.Assert(err, qt.IsNil)
	c.Assert(mdb.Close(), qt.IsNil)
}
```

Step 2: implement.

`api/wallet.go` (whole file; the custodial wallet code is gone):
```go
package api

import (
	"encoding/json"
	"fmt"

	"github.com/google/uuid"
	"go.vocdoni.io/dvote/crypto/ethereum"
	"go.vocdoni.io/dvote/httprouter"
	"go.vocdoni.io/dvote/httprouter/apirest"
	"go.vocdoni.io/dvote/log"
)

const (
	WalletHandler = "wallet"

	// ParamPrivateKey is the URL param of POST /wallet/add/{privateKey}.
	ParamPrivateKey = "privateKey"
)

const (
	// walletAddDeprecation is the RFC 9745 Deprecation header value of
	// POST /wallet/add/{privateKey}: 2026-10-22T00:00:00Z as a Unix timestamp.
	walletAddDeprecation = "@1792627200"
	// walletAddWarning tells a client reading the headers that the key is not
	// kept and that the token only serves the census endpoints.
	walletAddWarning = "the private key is not stored: the token is only a bearer for the /censuses endpoints, " +
		reasonWallet
)

func (a *API) enableWalletHandlers() error {
	return a.Endpoint.RegisterMethod(
		"/wallet/add/{privateKey}",
		"POST",
		apirest.MethodAccessTypePublic,
		a.walletAddHandler,
	)
}

// walletAddHandler
//
// It exists so census clients get a bearer token: @vocdoni/sdk calls it with a
// throwaway key before it creates a census. The private key is only used to
// derive its address and it is not stored. The node's request logs redact it
// (httprouter.RedactRequestURI), but it travels in the URL, so proxies and
// access logs in front of the node may record it: call this route only with a
// freshly generated key that holds nothing. The token is a new random UUID,
// which is what the /censuses endpoints accept as bearer.
//
// Every reply carries Deprecation and Warning headers, and every call is
// logged with the caller's address and user agent, so the remaining callers
// can be found before the route is removed.
//
//	@Summary		Get a bearer token for the census endpoints
//	@Description	Validates the hex private key (with or without 0x), derives its address and returns it with a new random UUID token. The key is not stored, but it travels in the URL and proxies may log it: use only a freshly generated key that holds nothing. The token is the bearer the /censuses endpoints accept.
//	@Deprecated
//	@Tags		Censuses
//	@Produce	json
//	@Param		privateKey	path		string	true	"Hex encoded private key"
//	@Success	200			{object}	WalletToken
//	@Router		/wallet/add/{privateKey} [post]
func (*API) walletAddHandler(_ *apirest.APIdata, ctx *httprouter.HTTPContext) error {
	r := ctx.Request
	log.Infow("deprecated route called", "route", "POST /wallet/add/{privateKey}",
		"remoteAddr", r.RemoteAddr,
		"cfConnectingIP", r.Header.Get("CF-Connecting-IP"),
		"xForwardedFor", r.Header.Get("X-Forwarded-For"),
		"userAgent", r.UserAgent(),
		"origin", r.Header.Get("Origin"))
	ctx.SetHeader("Deprecation", walletAddDeprecation)
	ctx.SetHeader("Warning", fmt.Sprintf("299 - %q", walletAddWarning))

	signer := ethereum.NewSignKeys()
	if err := signer.AddHexKey(ctx.URLParam(ParamPrivateKey)); err != nil {
		return ErrCantParsePrivateKey
	}
	data, err := json.Marshal(WalletToken{
		Token:   uuid.New(),
		Address: signer.Address().Bytes(),
	})
	if err != nil {
		return ErrMarshalingServerJSONFailed.WithErr(err)
	}
	return ctx.Send(data, apirest.HTTPstatusOK)
}
```

`api/wallet_purge.go`:
```go
package api

import (
	"go.vocdoni.io/dvote/db"
)

// Prefixes of the API database keys where the custodial wallet of
// POST /wallet/add/{privateKey} kept, per token, the encrypted private key
// and, per private key, its hash. The node no longer stores private keys, so
// purgeWalletKeys deletes whatever is left under them.
const (
	walletDBprefixEncryptedPrivateKey = "wpk_"
	walletDBprefixPrivateKeyHash      = "wph_"
)

// purgeWalletKeys deletes every key under the custodial wallet prefixes of
// database in a single transaction and returns how many it deleted. Running it
// again deletes nothing.
func purgeWalletKeys(database db.Database) (int, error) {
	var keys [][]byte
	for _, prefix := range []string{walletDBprefixEncryptedPrivateKey, walletDBprefixPrivateKeyHash} {
		if err := database.Iterate([]byte(prefix), func(key, _ []byte) bool {
			// Iterate passes the key without the prefix and only valid until
			// the callback returns: append copies it after the prefix.
			keys = append(keys, append([]byte(prefix), key...))
			return true
		}); err != nil {
			return 0, err
		}
	}
	if len(keys) == 0 {
		return 0, nil
	}
	wtx := database.WriteTx()
	defer wtx.Discard()
	for _, key := range keys {
		if err := wtx.Delete(key); err != nil {
			return 0, err
		}
	}
	if err := wtx.Commit(); err != nil {
		return 0, err
	}
	return len(keys), nil
}
```

`api/removed.go`. This is the sequencer's file with the import path rewritten and `reasonCensusIPFS` reworded, because vocdoni-node does use IPFS (spec Design 3):
```go
package api

import (
	"fmt"

	"go.vocdoni.io/dvote/httprouter"
	"go.vocdoni.io/dvote/httprouter/apirest"
)

const (
	reasonCensusIPFS = "the census IPFS routes are removed, use GET /censuses/export and POST /censuses/import"
	reasonAnonymous  = "anonymous voting is not supported"
	reasonWallet     = "the wallet API is not available"
	reasonExport     = "exporting the indexer is not available"
)

// removedRoute is a vocdoni-node API route this node does not serve.
type removedRoute struct {
	method string
	path   string
	reason string
}

// removedRoutes stay registered so that a client calling one of them gets
// ErrEndpointRemoved with the reason, instead of an empty 405 or the error of
// another route whose URL parameter happens to match the path.
var removedRoutes = []removedRoute{
	{"GET", "/accounts/count", "use GET /accounts and its pagination.totalItems"},
	{"GET", "/accounts/page/{page}", "use GET /accounts?page={page}"},
	{"GET", "/accounts/{accountId}/fees/page/{page}", "use GET /chain/fees?accountId={accountId}&page={page}"},
	{"GET", "/accounts/{accountId}/transfers/count", "use transfersCount from GET /accounts/{accountId}"},
	{"GET", "/accounts/{accountId}/transfers/page/{page}", "use GET /chain/transfers?accountId={accountId}&page={page}"},
	{"GET", "/accounts/{organizationId}/elections/count", "use GET /elections?organizationId={organizationId} and its pagination.totalItems"},
	{"GET", "/accounts/{organizationId}/elections/page/{page}", "use GET /elections?organizationId={organizationId}&page={page}"},
	{"GET", "/accounts/{organizationId}/elections/status/{status}/page/{page}", "use GET /elections?organizationId={organizationId}&status={status}&page={page}"},
	{"GET", "/censuses/export/ipfs", reasonCensusIPFS},
	{"GET", "/censuses/export/ipfs/list", reasonCensusIPFS},
	{"GET", "/censuses/import/{ipfscid}", reasonCensusIPFS},
	{"GET", "/chain/blocks/{height}/transactions/page/{page}", "use GET /chain/transactions?height={height}&page={page}"},
	{"GET", "/chain/export/indexer", reasonExport},
	{"GET", "/chain/fees/page/{page}", "use GET /chain/fees?page={page}"},
	{"GET", "/chain/fees/reference/{reference}/page/{page}", "use GET /chain/fees?reference={reference}&page={page}"},
	{"GET", "/chain/fees/type/{type}/page/{page}", "use GET /chain/fees?type={type}&page={page}"},
	{"GET", "/chain/info/circuit", reasonAnonymous},
	{"GET", "/chain/organizations/count", "use organizationCount from GET /chain/info"},
	{"POST", "/chain/organizations/filter/page/{page}", "use GET /chain/organizations?organizationId={id}&page={page}"},
	{"GET", "/chain/organizations/page/{page}", "use GET /chain/organizations?page={page}"},
	{"GET", "/chain/transactions/count", "use transactionCount from GET /chain/info"},
	{"GET", "/chain/transactions/page/{page}", "use GET /chain/transactions?page={page}"},
	{"GET", "/chain/transactions/{height}/{index}", "use GET /chain/transactions?height={height}, whose items carry index and hash, then GET /chain/transactions/{hash}"},
	{"POST", "/elections/filter", "use GET /elections with query params"},
	{"POST", "/elections/filter/page/{page}", "use GET /elections with query params and page={page}"},
	{"GET", "/elections/page/{page}", "use GET /elections?page={page}"},
	{"GET", "/elections/{electionId}/votes/count", "use voteCount from GET /elections/{electionId}"},
	{"GET", "/elections/{electionId}/votes/page/{page}", "use GET /votes?electionId={electionId}&page={page}"},
	{"GET", "/siks/proof/{address}", reasonAnonymous},
	{"GET", "/siks/roots", reasonAnonymous},
	{"GET", "/siks/{address}", reasonAnonymous},
	{"GET", "/wallet/bootstrap", reasonWallet},
	{"POST", "/wallet/election", reasonWallet},
	{"GET", "/wallet/transfer/{dstAddress}/{amount}", reasonWallet},
}

// enableRemovedRoutes registers every removed route with a handler that
// returns ErrEndpointRemoved.
func (a *API) enableRemovedRoutes() error {
	for _, r := range removedRoutes {
		msg := fmt.Sprintf("%s %s: %s", r.method, r.path, r.reason)
		if err := a.Endpoint.RegisterMethod(r.path, r.method, apirest.MethodAccessTypePublic,
			func(*apirest.APIdata, *httprouter.HTTPContext) error {
				return ErrEndpointRemoved.With(msg)
			},
		); err != nil {
			return err
		}
	}
	return nil
}
```

`api/api.go`, `api/errors.go`, `api/api_types.go`, `api/docs/descriptions/api.md` (exact diff against base):
```diff
diff --git a/api/api.go b/api/api.go
index e7ec876..bd3223b 100644
--- a/api/api.go
+++ b/api/api.go
@@ -16,6 +16,7 @@ import (
 	"go.vocdoni.io/dvote/db/prefixeddb"
 	"go.vocdoni.io/dvote/httprouter"
 	"go.vocdoni.io/dvote/httprouter/apirest"
+	"go.vocdoni.io/dvote/log"
 	"go.vocdoni.io/dvote/vochain"
 	"go.vocdoni.io/dvote/vochain/indexer"
 	"go.vocdoni.io/dvote/vochain/vochaininfo"
@@ -36,10 +37,6 @@ import (
 //	@tag.description	Manage census: create, add, get, verify...
 //	@tag.name			Votes
 //	@tag.description	Submit, get, and verify votes
-//	@tag.name			Wallet
-//	@tag.description	Operations for the wallets managed backend side
-//	@tag.name			SIK
-//	@tag.description	The Secret Identity Key (SIK) operations used on anonymous voting
 
 //	@schemes	https
 //	@host		api-dev.vocdoni.net
@@ -48,7 +45,7 @@ import (
 //	@securityDefinitions.apikey	ApiKeyAuth
 //	@in							header
 //	@name						Authorization
-//	@description				Send as "Authorization: Bearer <token>". Census endpoints take the client-chosen UUID used when creating the census; wallet endpoints take the token returned by /wallet/add/{privateKey}; admin-only endpoints take the node admin token.
+//	@description				Send as "Authorization: Bearer <token>". Census endpoints take the client-chosen UUID used when creating the census, such as the token returned by /wallet/add/{privateKey}; admin-only endpoints take the node admin token.
 
 const (
 	// DefaultItemsPerPage defines how many items per page are returned by the paginated endpoints,
@@ -144,11 +141,21 @@ func NewAPI(router *httprouter.HTTProuter, baseRoute, dataDir, dbType string) (*
 	if err != nil {
 		return nil, err
 	}
+	if err := api.enableRemovedRoutes(); err != nil {
+		return nil, err
+	}
 	mdb, err := metadb.New(dbType, filepath.Join(dataDir, "db"))
 	if err != nil {
 		return nil, err
 	}
 	api.db = prefixeddb.NewPrefixedDatabase(mdb, []byte("api/"))
+	purged, err := purgeWalletKeys(api.db)
+	if err != nil {
+		return nil, fmt.Errorf("cannot purge custodial wallet keys: %w", err)
+	}
+	if purged > 0 {
+		log.Infof("purged %d custodial wallet keys", purged)
+	}
 	return &api, nil
 }
 
@@ -213,14 +220,6 @@ func (a *API) EnableHandlers(handlers ...string) error {
 			if a.censusdb == nil {
 				return fmt.Errorf("%w %s", ErrMissingModulesForHandler, h)
 			}
-		case SIKHandler:
-			if err := a.enableSIKHandlers(); err != nil {
-				return err
-			}
-			if a.censusdb == nil {
-				return fmt.Errorf("%w %s", ErrMissingModulesForHandler, h)
-			}
-
 		default:
 			return fmt.Errorf("%w: %s", ErrHandlerUnknown, h)
 		}
diff --git a/api/api_types.go b/api/api_types.go
index b51972d..ecaced4 100644
--- a/api/api_types.go
+++ b/api/api_types.go
@@ -92,11 +92,6 @@ type VoteParams struct {
 
 // ### Objects returned ###
 
-// CountResult wraps a count inside an object
-type CountResult struct {
-	Count uint64 `json:"count" example:"10"`
-}
-
 // Pagination contains all the values needed for the UI to easily organize the returned data
 type Pagination struct {
 	TotalItems   uint64  `json:"totalItems"`
@@ -237,7 +232,6 @@ type ElectionDescription struct {
 	ElectionType ElectionType          `json:"electionType"`
 	Questions    []Question            `json:"questions"`
 	Census       CensusTypeDescription `json:"census"`
-	TempSIKs     bool                  `json:"tempSIKs"`
 }
 
 type Key struct {
@@ -463,11 +457,17 @@ type Account struct {
 	TransfersCount uint64           `json:"transfersCount,omitempty"`
 	FeesCount      uint64           `json:"feesCount,omitempty"`
 	InfoURL        string           `json:"infoURL,omitempty"`
-	Token          *uuid.UUID       `json:"token,omitempty" swaggerignore:"true"`
 	Metadata       *AccountMetadata `json:"metadata,omitempty"`
 	SIK            types.HexBytes   `json:"sik"`
 }
 
+// WalletToken is the reply of POST /wallet/add/{privateKey}: the address of
+// the given key and a new bearer token for the /censuses endpoints.
+type WalletToken struct {
+	Token   uuid.UUID      `json:"token" swaggertype:"string" format:"uuid"`
+	Address types.HexBytes `json:"address" swaggertype:"string"`
+}
+
 type AccountsList struct {
 	Accounts   []*indexertypes.Account `json:"accounts"`
 	Pagination *Pagination             `json:"pagination"`
@@ -490,11 +490,10 @@ type Census struct {
 	Valid    bool           `json:"valid,omitempty"`
 	URI      string         `json:"uri,omitempty"`
 	// proof stuff
-	CensusRoot     types.HexBytes `json:"censusRoot,omitempty"`
-	CensusProof    types.HexBytes `json:"censusProof,omitempty"`
-	Key            types.HexBytes `json:"key,omitempty"`
-	Value          types.HexBytes `json:"value,omitempty"`
-	CensusSiblings []string       `json:"censusSiblings,omitempty"`
+	CensusRoot  types.HexBytes `json:"censusRoot,omitempty"`
+	CensusProof types.HexBytes `json:"censusProof,omitempty"`
+	Key         types.HexBytes `json:"key,omitempty"`
+	Value       types.HexBytes `json:"value,omitempty"`
 }
 
 type File struct {
diff --git a/api/docs/descriptions/api.md b/api/docs/descriptions/api.md
index 426bb2e..183dc81 100644
--- a/api/docs/descriptions/api.md
+++ b/api/docs/descriptions/api.md
@@ -14,8 +14,6 @@ The API contains the following endpoints:
 - [**Elections**](elections): The elections endpoint serves information related to elections such as basic election information, election keys, and submitted votes, as well as enabling users to create a new election and modify existing ones. There is a set of [options, specifications, and lifecycle states](https://developer.vocdoni.io/protocol#elections) that determine the behavior of an election and how votes are counted. 
 - [**Censuses**](censuses): The census is a key component of any voting process. It specifies the set of users (each identified by a public key or address) eligible to participate in an election. The various types of census are documented [here](https://developer.vocdoni.io/protocol/census). This endpoint provides census information like the Merkle root, type, total weight, and size of a census. It also allows you to import/export censuses and create new ones.
 - [**Votes**](votes): This endpoint serves all the information associated with any specific vote, including its validity. It is also how users can cast votes.
-- [**Wallet**](wallet): The wallet endpoint facilitates the creation of accounts on the Vochain. This endpoint fulfills requests relating to the token balance held by a given account. 
-- [**SIK**](sik): The Secret Identity Key is a user-generated piece of information that proves the user's identity without revealing it. It is the hash of the user's address, the signature of a public message, and an optional secret part. It is used to ensure anonymous voting. All registered accounts or anonymous voters must register a SIK, and these keys are all stored in a Merkle tree. The `/siks` endpoints helps to generate a proof of membership, get the current valid SIK roots, or check if an account has a valid SIK.
 
 
 ### Errors 
diff --git a/api/errors.go b/api/errors.go
index a6e8a2a..82bab3b 100644
--- a/api/errors.go
+++ b/api/errors.go
@@ -40,7 +40,6 @@ var (
 	ErrFileSizeTooBig                   = apirest.APIerror{Code: 4012, HTTPstatus: apirest.HTTPstatusBadRequest, Err: fmt.Errorf("file size exceeds the maximum allowed")}
 	ErrCantParseOrgID                   = apirest.APIerror{Code: 4013, HTTPstatus: apirest.HTTPstatusBadRequest, Err: fmt.Errorf("cannot parse organizationId")}
 	ErrCantParseAccountID               = apirest.APIerror{Code: 4014, HTTPstatus: apirest.HTTPstatusBadRequest, Err: fmt.Errorf("cannot parse accountId")}
-	ErrCantParseBearerToken             = apirest.APIerror{Code: 4015, HTTPstatus: apirest.HTTPstatusBadRequest, Err: fmt.Errorf("cannot parse bearer token")}
 	ErrCantParseDataAsJSON              = apirest.APIerror{Code: 4016, HTTPstatus: apirest.HTTPstatusBadRequest, Err: fmt.Errorf("cannot parse data as JSON")}
 	ErrCantParseElectionID              = apirest.APIerror{Code: 4017, HTTPstatus: apirest.HTTPstatusBadRequest, Err: fmt.Errorf("cannot parse electionId")}
 	ErrCantParseMetadataAsJSON          = apirest.APIerror{Code: 4018, HTTPstatus: apirest.HTTPstatusBadRequest, Err: fmt.Errorf("cannot parse metadata (invalid format)")}
@@ -66,8 +65,6 @@ var (
 	ErrParamToInvalid                   = apirest.APIerror{Code: 4038, HTTPstatus: apirest.HTTPstatusBadRequest, Err: fmt.Errorf("invalid address")}
 	ErrParticipantKeyMissing            = apirest.APIerror{Code: 4039, HTTPstatus: apirest.HTTPstatusBadRequest, Err: fmt.Errorf("missing participant key")}
 	ErrIndexedCensusCantUseWeight       = apirest.APIerror{Code: 4040, HTTPstatus: apirest.HTTPstatusBadRequest, Err: fmt.Errorf("indexed census cannot use weight")}
-	ErrWalletNotFound                   = apirest.APIerror{Code: 4041, HTTPstatus: apirest.HTTPstatusNotFound, Err: fmt.Errorf("wallet not found")}
-	ErrWalletPrivKeyAlreadyExists       = apirest.APIerror{Code: 4042, HTTPstatus: apirest.HTTPstatusBadRequest, Err: fmt.Errorf("wallet private key already exists")}
 	ErrElectionEndDateInThePast         = apirest.APIerror{Code: 4043, HTTPstatus: apirest.HTTPstatusBadRequest, Err: fmt.Errorf("election end date cannot be in the past")}
 	ErrElectionEndDateBeforeStart       = apirest.APIerror{Code: 4044, HTTPstatus: apirest.HTTPstatusBadRequest, Err: fmt.Errorf("election end date must be after start date")}
 	ErrElectionNotFound                 = apirest.APIerror{Code: 4045, HTTPstatus: apirest.HTTPstatusNotFound, Err: fmt.Errorf("election not found")}
@@ -79,7 +76,6 @@ var (
 	ErrInvalidCensusKeyLength           = apirest.APIerror{Code: 4051, HTTPstatus: apirest.HTTPstatusBadRequest, Err: fmt.Errorf("invalid census key length")}
 	ErrUnmarshalingServerProto          = apirest.APIerror{Code: 4052, HTTPstatus: apirest.HTTPstatusInternalErr, Err: fmt.Errorf("error unmarshaling protobuf data")}
 	ErrMarshalingServerProto            = apirest.APIerror{Code: 4053, HTTPstatus: apirest.HTTPstatusInternalErr, Err: fmt.Errorf("error marshaling protobuf data")}
-	ErrSIKNotFound                      = apirest.APIerror{Code: 4054, HTTPstatus: apirest.HTTPstatusNotFound, Err: fmt.Errorf("SIK not found")}
 	ErrCantParseBoolean                 = apirest.APIerror{Code: 4055, HTTPstatus: apirest.HTTPstatusBadRequest, Err: fmt.Errorf("cannot parse string into boolean")}
 	ErrCantParseHexString               = apirest.APIerror{Code: 4056, HTTPstatus: apirest.HTTPstatusBadRequest, Err: fmt.Errorf("cannot parse string into hex bytes")}
 	ErrPageNotFound                     = apirest.APIerror{Code: 4057, HTTPstatus: apirest.HTTPstatusNotFound, Err: fmt.Errorf("page not found")}
@@ -92,6 +88,8 @@ var (
 	ErrParamOrderInvalid                = apirest.APIerror{Code: 4064, HTTPstatus: apirest.HTTPstatusBadRequest, Err: fmt.Errorf("parameter (order) invalid, must be asc or desc")}
 	ErrMetadataHashNotMatchContent      = apirest.APIerror{Code: 4065, HTTPstatus: apirest.HTTPstatusBadRequest, Err: fmt.Errorf("metadata hash does not match metadata content")}
 	ErrNotSetMetadataTx                 = apirest.APIerror{Code: 4066, HTTPstatus: apirest.HTTPstatusBadRequest, Err: fmt.Errorf("transaction is not a SET_PROCESS_METADATA for this election")}
+	ErrEndpointRemoved                  = apirest.APIerror{Code: 4067, HTTPstatus: apirest.HTTPstatusGone, Err: fmt.Errorf("endpoint removed")}
+	ErrCantParsePrivateKey              = apirest.APIerror{Code: 4069, HTTPstatus: apirest.HTTPstatusBadRequest, Err: fmt.Errorf("cannot parse private key, must be 32 hex encoded bytes")}
 	ErrVochainEmptyReply                = apirest.APIerror{Code: 5000, HTTPstatus: apirest.HTTPstatusInternalErr, Err: fmt.Errorf("vochain returned an empty reply")}
 	ErrVochainSendTxFailed              = apirest.APIerror{Code: 5001, HTTPstatus: apirest.HTTPstatusInternalErr, Err: fmt.Errorf("vochain SendTx failed")}
 	ErrVochainGetTxFailed               = apirest.APIerror{Code: 5002, HTTPstatus: apirest.HTTPstatusInternalErr, Err: fmt.Errorf("vochain GetTx failed")}
@@ -117,9 +115,7 @@ var (
 	ErrElectionResultsNotYetAvailable   = apirest.APIerror{Code: 5024, HTTPstatus: apirest.HTTPstatusInternalErr, Err: fmt.Errorf("election results are not yet available")}
 	ErrElectionResultsIsNil             = apirest.APIerror{Code: 5025, HTTPstatus: apirest.HTTPstatusInternalErr, Err: fmt.Errorf("election results is nil")}
 	ErrElectionResultsMismatch          = apirest.APIerror{Code: 5026, HTTPstatus: apirest.HTTPstatusInternalErr, Err: fmt.Errorf("election results don't match reported ones")}
-	ErrCantGetCircomSiblings            = apirest.APIerror{Code: 5027, HTTPstatus: apirest.HTTPstatusInternalErr, Err: fmt.Errorf("cannot get circom siblings")}
 	ErrCensusProofVerificationFailed    = apirest.APIerror{Code: 5028, HTTPstatus: apirest.HTTPstatusInternalErr, Err: fmt.Errorf("census proof verification failed")}
-	ErrCantCountVotes                   = apirest.APIerror{Code: 5029, HTTPstatus: apirest.HTTPstatusInternalErr, Err: fmt.Errorf("cannot count votes")}
 	ErrVochainOverloaded                = apirest.APIerror{Code: 5030, HTTPstatus: apirest.HTTPstatusServiceUnavailable, Err: fmt.Errorf("vochain overloaded")}
 	ErrGettingSIK                       = apirest.APIerror{Code: 5031, HTTPstatus: apirest.HTTPstatusInternalErr, Err: fmt.Errorf("error getting SIK")}
 	ErrCensusBuild                      = apirest.APIerror{Code: 5032, HTTPstatus: apirest.HTTPstatusInternalErr, Err: fmt.Errorf("error building census")}
```

Notes on these four files:
- `errors.go` drops these codes, whose last users go in Tasks 2–4: 4015, 4041, 4042, 4054, 5027, 5029. It adds 4067 `ErrEndpointRemoved` (410) and 4069 `ErrCantParsePrivateKey`. `ErrGettingSIK` (5031) stays, because `accountHandler` uses it (`api/accounts.go:196` at base).
- `api_types.go` drops `CountResult`, `ElectionDescription.TempSIKs`, `Account.Token` and `Census.CensusSiblings`, and adds `WalletToken`.
- `api.go` drops the SIK `EnableHandlers` case and the Wallet and SIK swagger tags.

Step 3: quick check. Other packages do not compile until Tasks 3–5, so check formatting only.
Run (on the laptop, formatting only): `cd $W && gofmt -l api`
Expected: empty.

Step 4: commit.
`git add api/wallet_purge.go api/wallet_purge_test.go api/wallet_test.go api/removed.go api/removed_test.go && git commit -m "feat(api): stateless /wallet/add, purge stored wallet keys and answer 410 on removed routes" -- api/wallet.go api/wallet_purge.go api/wallet_purge_test.go api/wallet_test.go api/removed.go api/removed_test.go api/api.go api/errors.go api/api_types.go api/docs/descriptions/api.md`

Interfaces provided:
- `ErrEndpointRemoved`
- `reasonAnonymous`, `reasonCensusIPFS`, `reasonExport`, `reasonWallet`
- `removedRoutes`
- `newFullTestAPI(t) (*API, *testutil.TestHTTPclient)`
- `assertAPIError(t, resp, code, want)`
- `WalletToken`
- `WalletHandler` (stays a handler group; it registers only `POST /wallet/add/{privateKey}`)

### Task 3: Anonymous voting (ZK) prune
Model: sonnet (careful)   Repo: ~/src/vd/vocdoni-node
Files: listed in the steps below.
Goal: AC2 and AC7. Remove the SIK routes, `/chain/info/circuit`, zk census creation, proof siblings and the apiclient/end2endtest anonymous paths.

Step 1: delete whole files.
```sh
cd $W && git rm -q api/siks.go apiclient/sik.go benchmark/zk_census_benchmark_test.go cmd/end2endtest/zkweighted.go test/siks_test.go
```
`benchmark/zk_census_benchmark_test.go` is the only file in `benchmark/`.

Step 2: tests.
- Delete `func TestCensusZk` from `api/census_test.go` (base `api/census_test.go:250` through the closing `}` of that function, with its preceding blank line).
- Add `api/anonymous_removed_test.go`:
```go
package api

import (
	"encoding/hex"
	"encoding/json"
	"math/big"
	"testing"

	"github.com/ethereum/go-ethereum/common"
	qt "github.com/frankban/quicktest"
	"go.vocdoni.io/dvote/test/testcommon/testutil"
	"go.vocdoni.io/dvote/types"
)

// TestCensusCreateAnonymousRemoved checks that creating an anonymous census
// answers ErrEndpointRemoved while weighted censuses are still created.
func TestCensusCreateAnonymousRemoved(t *testing.T) {
	_, cl := newFullTestAPI(t)

	resp, code := cl.Request("POST", nil, "censuses", CensusTypeZKWeighted)
	assertAPIError(t, resp, code, ErrEndpointRemoved)
	qt.Assert(t, string(resp), qt.Contains, reasonAnonymous)

	resp, code = cl.Request("POST", nil, "censuses", CensusTypeWeighted)
	qt.Assert(t, code, qt.Equals, 200, qt.Commentf("response: %s", resp))
}

// TestAnonymousResponseFields checks that census proofs carry no siblings
// while the account sik and the chain circuitVersion fields stay, since
// clients still read them.
func TestAnonymousResponseFields(t *testing.T) {
	c := qt.New(t)
	api, cl := newFullTestAPI(t)

	resp, code := cl.Request("POST", nil, "censuses", CensusTypeWeighted)
	c.Assert(code, qt.Equals, 200, qt.Commentf("response: %s", resp))
	census := &Census{}
	c.Assert(json.Unmarshal(resp, census), qt.IsNil)
	censusID := census.CensusID.String()

	rnd := testutil.NewRandom(1)
	key := rnd.RandomBytes(20)
	participants := CensusParticipants{Participants: []CensusParticipant{
		{Key: key, Weight: (*types.BigInt)(big.NewInt(1))},
	}}
	resp, code = cl.Request("POST", &participants, "censuses", censusID, "participants")
	c.Assert(code, qt.Equals, 200, qt.Commentf("response: %s", resp))

	resp, code = cl.Request("GET", nil, "censuses", censusID, "proof", hex.EncodeToString(key))
	c.Assert(code, qt.Equals, 200, qt.Commentf("response: %s", resp))
	fields := map[string]json.RawMessage{}
	c.Assert(json.Unmarshal(resp, &fields), qt.IsNil)
	_, ok := fields["censusProof"]
	c.Assert(ok, qt.IsTrue, qt.Commentf("response: %s", resp))
	_, ok = fields["censusSiblings"]
	c.Assert(ok, qt.IsFalse, qt.Commentf("response: %s", resp))

	addr := common.HexToAddress("0x0000000000000000000000000000000000000001")
	c.Assert(api.vocapp.State.CreateAccount(addr, "", nil, 100), qt.IsNil)
	api.vocapp.AdvanceTestBlock()
	resp, code = cl.Request("GET", nil, "accounts", addr.Hex())
	c.Assert(code, qt.Equals, 200, qt.Commentf("response: %s", resp))
	fields = map[string]json.RawMessage{}
	c.Assert(json.Unmarshal(resp, &fields), qt.IsNil)
	_, ok = fields["sik"]
	c.Assert(ok, qt.IsTrue, qt.Commentf("response: %s", resp))

	resp, code = cl.Request("GET", nil, "chain", "info")
	c.Assert(code, qt.Equals, 200, qt.Commentf("response: %s", resp))
	fields = map[string]json.RawMessage{}
	c.Assert(json.Unmarshal(resp, &fields), qt.IsNil)
	_, ok = fields["circuitVersion"]
	c.Assert(ok, qt.IsTrue, qt.Commentf("response: %s", resp))
}
```

Step 3: implement. The exact diff for modified files:
```diff
diff --git a/api/census_helpers.go b/api/census_helpers.go
index 62bfda8..e568898 100644
--- a/api/census_helpers.go
+++ b/api/census_helpers.go
@@ -3,7 +3,6 @@ package api
 import (
 	"encoding/hex"
 
-	"github.com/ethereum/go-ethereum/common"
 	"go.vocdoni.io/dvote/util"
 	"go.vocdoni.io/proto/build/go/models"
 )
@@ -49,7 +48,3 @@ func censusKeyParse(key string) ([]byte, error) {
 	key = util.TrimHex(key)
 	return hex.DecodeString(key)
 }
-
-func addressParse(key string) common.Address {
-	return common.HexToAddress(util.TrimHex(key))
-}
diff --git a/api/censuses.go b/api/censuses.go
index 45be57f..2fcb175 100644
--- a/api/censuses.go
+++ b/api/censuses.go
@@ -13,7 +13,6 @@ import (
 	"github.com/google/uuid"
 	"go.vocdoni.io/dvote/api/censusdb"
 	"go.vocdoni.io/dvote/censustree"
-	"go.vocdoni.io/dvote/crypto/zk"
 	"go.vocdoni.io/dvote/crypto/zk/circuit"
 	"go.vocdoni.io/dvote/data/compressor"
 	"go.vocdoni.io/dvote/httprouter"
@@ -27,7 +26,9 @@ import (
 const (
 	CensusHandler = "censuses"
 
-	CensusTypeWeighted   = "weighted"
+	CensusTypeWeighted = "weighted"
+	// CensusTypeZKWeighted is the anonymous census type. Creating one answers
+	// ErrEndpointRemoved, and existing ones still report it as their type.
 	CensusTypeZKWeighted = "zkweighted"
 	CensusTypeCSP        = "csp"
 	// CensusTypeCSPv2 is CensusTypeCSP with the fixed salt derivation for the
@@ -227,10 +228,13 @@ func (a *API) enableCensusHandlers() error {
 //	@Accept					json
 //	@Produce				json
 //	@Security				ApiKeyAuth
-//	@Param					type	path		string	true	"Census type"	Enums(weighted,zkweighted)
+//	@Param					type	path		string	true	"Census type"	Enums(weighted)
 //	@Success				200		{object}	object{censusID=string}
 //	@Router					/censuses/{type} [post]
 func (a *API) censusCreateHandler(msg *apirest.APIdata, ctx *httprouter.HTTPContext) error {
+	if ctx.URLParam(ParamType) == CensusTypeZKWeighted {
+		return ErrEndpointRemoved.Withf("POST /censuses/%s: %s", CensusTypeZKWeighted, reasonAnonymous)
+	}
 	token, err := uuid.Parse(msg.AuthToken)
 	if err != nil {
 		return err
@@ -835,7 +839,7 @@ func (a *API) censusPublishCheckHandler(_ *apirest.APIdata, ctx *httprouter.HTTP
 //	@Produce				json
 //	@Param					censusId	path		string																										true	"Census id"
 //	@Param					key			path		string																										true	"Key to proof"
-//	@Success				200			{object}	object{type=string,weight=string,value=string,censusProof=string,censusRoot=string,censusSiblings=[]string}	"censusProof is the Merkle proof, value is the Merkle tree leaf value and censusSiblings is only returned for zkweighted censuses"
+//	@Success				200			{object}	object{type=string,weight=string,value=string,censusProof=string,censusRoot=string}	"censusProof is the Merkle proof and value is the Merkle tree leaf value"
 //	@Router					/censuses/{censusId}/proof/{key} [get]
 func (a *API) censusProofHandler(_ *apirest.APIdata, ctx *httprouter.HTTPContext) error {
 	censusID, err := censusIDparse(ctx.URLParam(ParamCensusId))
@@ -857,18 +861,10 @@ func (a *API) censusProofHandler(_ *apirest.APIdata, ctx *httprouter.HTTPContext
 	// Get the census type to return it into the response to prevent to perform
 	// two api calls.
 	censusType := encodeCensusType(models.Census_Type(ref.CensusType))
-	// If the census type is zkweighted (that means that it uses Poseidon hash
-	// with Arbo merkle tree), skip to perform a hash function over the census
-	// key. It is because the zk friendly key of any census leaf is the
-	// ZkAddress which follows a specific transformation process that must be
-	// implemented into the circom circuit also, and it is already hashed.
-	// Otherwhise, hash the key before get the proof.
-	leafKey := address
-	if ref.CensusType != int32(models.Census_ARBO_POSEIDON) {
-		leafKey, err = ref.Tree().Hash(address)
-		if err != nil {
-			return err
-		}
+	// hash the key before get the proof
+	leafKey, err := ref.Tree().Hash(address)
+	if err != nil {
+		return err
 	}
 	response := Census{Type: censusType}
 	response.Value, response.CensusProof, err = ref.Tree().GenProof(leafKey)
@@ -878,15 +874,6 @@ func (a *API) censusProofHandler(_ *apirest.APIdata, ctx *httprouter.HTTPContext
 	if response.CensusRoot, err = ref.Tree().Root(); err != nil {
 		return ErrCensusRootIsNil.WithErr(err)
 	}
-
-	// Get the leaf siblings from arbo based on the key received and include
-	// them into the response, only if it is zkweighted.
-	if ref.CensusType == int32(models.Census_ARBO_POSEIDON) {
-		response.CensusSiblings, err = zk.ProofToCircomSiblings(response.CensusProof)
-		if err != nil {
-			return ErrCantGetCircomSiblings.WithErr(err)
-		}
-	}
 	if len(response.Value) > 0 {
 		// return the string representation of the census value (weight)
 		// to make the client know his voting power for the census
diff --git a/api/chain.go b/api/chain.go
index 3357235..e20fbce 100644
--- a/api/chain.go
+++ b/api/chain.go
@@ -74,14 +74,6 @@ func (a *API) enableChainHandlers() error {
 	); err != nil {
 		return err
 	}
-	if err := a.Endpoint.RegisterMethod(
-		"/chain/info/circuit",
-		"GET",
-		apirest.MethodAccessTypePublic,
-		a.chainCircuitInfoHandler,
-	); err != nil {
-		return err
-	}
 	if err := a.Endpoint.RegisterMethod(
 		"/chain/info/electionPriceFactors",
 		"GET",
@@ -569,24 +561,6 @@ func (a *API) chainInfoHandler(_ *apirest.APIdata, ctx *httprouter.HTTPContext)
 	return ctx.Send(data, apirest.HTTPstatusOK)
 }
 
-// chainCircuitInfoHandler
-//
-//	@Summary		Circuit info
-//	@Description	Returns the circuit configuration according to the current circuit
-//	@Tags			Chain
-//	@Accept			json
-//	@Produce		json
-//	@Success		200	{object}	circuit.Config
-//	@Router			/chain/info/circuit [get]
-func (a *API) chainCircuitInfoHandler(_ *apirest.APIdata, ctx *httprouter.HTTPContext) error {
-	// Encode the current circuit configuration to JSON
-	data, err := json.Marshal(circuit.Global().Config)
-	if err != nil {
-		return err
-	}
-	return ctx.Send(data, apirest.HTTPstatusOK)
-}
-
 // chainInfoPriceFactors
 //
 //	@Summary				Price factors information
diff --git a/api/docs/descriptions/censusCreateHandler.md b/api/docs/descriptions/censusCreateHandler.md
index 49f9cc4..e144945 100644
--- a/api/docs/descriptions/censusCreateHandler.md
+++ b/api/docs/descriptions/censusCreateHandler.md
@@ -6,5 +6,6 @@ Returns a new random censusID (a random 32 byte hex string), which is used (alon
 
 To use a census for an election, it **must be published**.
 
-- Available types are: `weighted` and `zkindexed`
+- Available type: `weighted`
+- `zkweighted` (anonymous voting) is not supported and answers HTTP 410 with code 4067
 - Requires header Bearer token created user side
\ No newline at end of file
diff --git a/apiclient/census.go b/apiclient/census.go
index bd4aadf..052a8f6 100644
--- a/apiclient/census.go
+++ b/apiclient/census.go
@@ -25,13 +25,10 @@ type CensusProof struct {
 	LeafWeight *big.Int
 	// KeyType defines the type of the census tree key
 	KeyType models.ProofArbo_KeyType
-	// Siblings contains the decoded siblings keys
-	Siblings []string
 }
 
 // NewCensus creates a new census and returns its ID. The censusType can be
-// weighted (api.CensusTypeWeighted), zkweighted (api.CensusTypeZKWeighted) or
-// csp (api.CensusTypeCSP).
+// weighted (api.CensusTypeWeighted) or csp (api.CensusTypeCSP).
 func (c *HTTPclient) NewCensus(censusType string) (types.HexBytes, error) {
 	// create a new census
 	resp, code, err := c.Request(HTTPPOST, nil, "censuses", censusType)
@@ -131,7 +128,6 @@ func (c *HTTPclient) CensusGenProof(censusID, voterKey types.HexBytes) (*CensusP
 		Root:      censusData.CensusRoot,
 		Proof:     censusData.CensusProof,
 		LeafValue: censusData.Value,
-		Siblings:  censusData.CensusSiblings,
 	}
 	if censusData.Weight != nil {
 		cp.LeafWeight = censusData.Weight.MathBigInt()
diff --git a/apiclient/client.go b/apiclient/client.go
index fd10f9f..f6290db 100644
--- a/apiclient/client.go
+++ b/apiclient/client.go
@@ -14,7 +14,6 @@ import (
 	"github.com/google/uuid"
 	"go.vocdoni.io/dvote/api"
 	"go.vocdoni.io/dvote/crypto/ethereum"
-	"go.vocdoni.io/dvote/crypto/zk/circuit"
 	"go.vocdoni.io/dvote/httprouter/apirest"
 	"go.vocdoni.io/dvote/log"
 )
@@ -45,7 +44,6 @@ type HTTPclient struct {
 	addr    *url.URL
 	account *ethereum.SignKeys
 	chainID string
-	circuit *circuit.ZkCircuit
 	retries int
 }
 
@@ -91,11 +89,6 @@ func NewWithURLAndBearer(addr *url.URL, bearerToken *uuid.UUID) (*HTTPclient, er
 		return nil, fmt.Errorf("cannot get chain ID from API server")
 	}
 	c.chainID = info.ID
-
-	c.circuit, err = circuit.LoadVersion(info.CircuitVersion)
-	if err != nil {
-		return nil, fmt.Errorf("error loading circuit: %w", err)
-	}
 	return c, nil
 }
 
diff --git a/apiclient/election.go b/apiclient/election.go
index 8013c1c..3a0bb49 100644
--- a/apiclient/election.go
+++ b/apiclient/election.go
@@ -199,7 +199,6 @@ func (c *HTTPclient) NewElection(description *api.ElectionDescription, wait bool
 		Metadata:      &metadataURI,
 		MetadataHash:  api.MetadataHash(metadataBytes),
 		MaxCensusSize: description.Census.Size,
-		TempSIKs:      &description.TempSIKs,
 	}
 	log.Debugf("election transaction: %+v", log.FormatProto(process))
 
diff --git a/apiclient/vote.go b/apiclient/vote.go
index da0bf35..50090b8 100644
--- a/apiclient/vote.go
+++ b/apiclient/vote.go
@@ -9,9 +9,6 @@ import (
 	"go.vocdoni.io/dvote/api"
 	"go.vocdoni.io/dvote/crypto/ethereum"
 	"go.vocdoni.io/dvote/crypto/nacl"
-	"go.vocdoni.io/dvote/crypto/zk"
-	"go.vocdoni.io/dvote/crypto/zk/circuit"
-	"go.vocdoni.io/dvote/crypto/zk/prover"
 	"go.vocdoni.io/dvote/httprouter/apirest"
 	"go.vocdoni.io/dvote/log"
 	"go.vocdoni.io/dvote/types"
@@ -44,10 +41,9 @@ type VoteData struct {
 	// (UTF-8 text, JSON, anything) is the caller's choice.
 	Memo []byte
 
-	ProofMkTree  *CensusProof
-	ProofSIKTree *CensusProof
-	ProofCSP     types.HexBytes
-	Keys         []api.Key
+	ProofMkTree *CensusProof
+	ProofCSP    types.HexBytes
+	Keys        []api.Key
 
 	// if VoterAccount is set, it will be used to sign the vote
 	// instead of the keys found in HTTPclient.account
@@ -90,56 +86,7 @@ func (cl *HTTPclient) Vote(v *VoteData) (types.HexBytes, error) {
 	censusOriginWeighted := models.CensusOrigin_name[int32(models.CensusOrigin_OFF_CHAIN_TREE_WEIGHTED)]
 	switch {
 	case v.Election.VoteMode.Anonymous:
-		// support no vote weight provided
-		if v.VoteWeight == nil {
-			v.VoteWeight = v.ProofMkTree.LeafWeight
-		}
-		// generate circuit inputs with the election, census and voter
-		// information and encode it into a json
-		rawInputs, err := circuit.GenerateCircuitInput(circuit.CircuitInputsParameters{
-			Account:         c.account,
-			ElectionId:      v.Election.ElectionID,
-			CensusRoot:      v.Election.Census.CensusRoot,
-			SIKRoot:         v.ProofSIKTree.Root,
-			CensusSiblings:  v.ProofMkTree.Siblings,
-			SIKSiblings:     v.ProofSIKTree.Siblings,
-			VoteWeight:      v.VoteWeight,
-			AvailableWeight: v.ProofMkTree.LeafWeight,
-			VotePackage:     vote.VotePackage,
-		})
-		if err != nil {
-			return nil, fmt.Errorf("could not generate zk circuit inputs: %w", err)
-		}
-		inputs, err := json.Marshal(rawInputs)
-		if err != nil {
-			return nil, fmt.Errorf("error encoding inputs: %w", err)
-		}
-		// instance the prover with the circuit config loaded and generate the
-		// proof for the calculated inputs
-		proof, err := prover.Prove(c.circuit.ProvingKey, c.circuit.Wasm, inputs)
-		if err != nil {
-			return nil, fmt.Errorf("could not generate anonymous proof: %w", err)
-		}
-		// encode the proof into a protobuf
-		protoProof, err := zk.ProverProofToProtobufZKProof(proof, nil, nil, nil, nil, nil)
-		if err != nil {
-			return nil, err
-		}
-		// include vote nullifier and the encoded proof in a VoteEnvelope
-		vote.Nullifier, err = proof.Nullifier()
-		if err != nil {
-			return nil, err
-		}
-		vote.Proof = &models.Proof{
-			Payload: &models.Proof_ZkSnark{
-				ZkSnark: protoProof,
-			},
-		}
-		// prepare an unsigned vote transaction with the VoteEnvelope
-		voteAPI, err = c.prepareVoteTx(vote, false)
-		if err != nil {
-			return nil, fmt.Errorf("could not prepare vote transaction: %w", err)
-		}
+		return nil, fmt.Errorf("anonymous voting is not supported")
 	case v.Election.Census.CensusOrigin == censusOriginWeighted:
 		// support custom vote weight
 		var voteWeight []byte
diff --git a/cmd/end2endtest/dynamicensus.go b/cmd/end2endtest/dynamicensus.go
index f6db222..a5720ab 100644
--- a/cmd/end2endtest/dynamicensus.go
+++ b/cmd/end2endtest/dynamicensus.go
@@ -70,7 +70,7 @@ func (t *E2EDynamicensusElection) Run() error {
 	setupNewCensusAndVote := func(election e2eElection) (types.HexBytes, apiclient.VoteData, error) {
 		api := election.api
 
-		censusID, vAccts, err := election.setupCensus(vapi.CensusTypeWeighted, 1, false)
+		censusID, vAccts, err := election.setupCensus(vapi.CensusTypeWeighted, 1)
 		if err != nil {
 			return nil, apiclient.VoteData{}, err
 		}
diff --git a/cmd/end2endtest/helpers.go b/cmd/end2endtest/helpers.go
index d54652b..6eae20b 100644
--- a/cmd/end2endtest/helpers.go
+++ b/cmd/end2endtest/helpers.go
@@ -30,9 +30,8 @@ const (
 )
 
 type acctProof struct {
-	account  *ethereum.SignKeys
-	proof    *apiclient.CensusProof
-	proofSIK *apiclient.CensusProof
+	account *ethereum.SignKeys
+	proof   *apiclient.CensusProof
 }
 
 func newTestElectionDescription(numChoices int) *vapi.ElectionDescription {
@@ -193,7 +192,7 @@ func (t *e2eElection) generateProofs(csp *ethereum.SignKeys, voterAccts []*ether
 		wg     sync.WaitGroup
 		vcount int32
 	)
-	// Wait for the next block to assure the SIK root is updated
+	// Wait for the next block to assure the census is updated
 	if err := t.api.WaitUntilNextBlock(); err != nil {
 		return err
 	}
@@ -217,15 +216,6 @@ func (t *e2eElection) generateProofs(csp *ethereum.SignKeys, voterAccts []*ether
 				errorChan <- err
 			}
 			accP := acctProof{account: acc, proof: proof}
-
-			if t.election.VoteMode.Anonymous {
-				voterPrivKey := acc.PrivateKey()
-				voterApi := t.api.Clone(voterPrivKey.String())
-				accP.proofSIK, err = voterApi.GenSIKProof()
-				if err != nil {
-					errorChan <- err
-				}
-			}
 			t.voters.Store(acc.Public, accP)
 			atomic.AddInt32(&vcount, 1)
 		}
@@ -287,7 +277,7 @@ func (t *e2eElection) publishCheckCensus(censusID types.HexBytes, nAcct uint64)
 }
 
 // setupCensus create a new census that will have nAcct voterAccounts and participants
-func (t *e2eElection) setupCensus(censusType string, nAcct int, createAccounts bool) (types.HexBytes, []*ethereum.SignKeys, error) {
+func (t *e2eElection) setupCensus(censusType string, nAcct int) (types.HexBytes, []*ethereum.SignKeys, error) {
 	// Create a new census
 	censusID, err := t.api.NewCensus(censusType)
 	if err != nil {
@@ -302,13 +292,6 @@ func (t *e2eElection) setupCensus(censusType string, nAcct int, createAccounts b
 		return nil, nil, err
 	}
 
-	// Register the accounts in the vochain if is required
-	if censusType == vapi.CensusTypeZKWeighted && createAccounts {
-		if err := t.registerAnonAccts(voterAccounts); err != nil {
-			return nil, nil, err
-		}
-	}
-
 	return censusID, voterAccounts, nil
 }
 
@@ -317,7 +300,7 @@ func (t *e2eElection) setupElection(ed *vapi.ElectionDescription, nvAccts int, w
 		return err
 	}
 
-	censusID, voterAccts, err := t.setupCensus(ed.Census.Type, nvAccts, !ed.TempSIKs)
+	censusID, voterAccts, err := t.setupCensus(ed.Census.Type, nvAccts)
 	if err != nil {
 		return err
 	}
@@ -364,61 +347,6 @@ func (t *e2eElection) setupElection(ed *vapi.ElectionDescription, nvAccts int, w
 		}
 	}
 
-	if ed.TempSIKs {
-		errorChan := make(chan error)
-		wg := &sync.WaitGroup{}
-		halfAccts := len(voterAccts) / 2
-
-		// register SIK for 1/2 accounts and left the rest to vote with account registered
-		for i, acc := range voterAccts[:halfAccts] {
-			wg.Add(1)
-			go func(i int, acc *ethereum.SignKeys) {
-				defer wg.Done()
-				pKey := acc.PrivateKey()
-				accountApi := t.api.Clone(pKey.String())
-				hash, err := accountApi.RegisterSIKForVote(electionID, nil, nil)
-				if err != nil {
-					log.Errorf("could not register SIK for vote, address: %s, %v", acc.AddressString(), err)
-					errorChan <- err
-				}
-				log.Infow("sik registered for anonymous census uncreated account", "index", i, "address", acc.AddressString())
-				ctx, cancel := context.WithTimeout(context.Background(), t.config.timeout)
-				defer cancel()
-
-				if _, err := accountApi.WaitUntilTxIsMined(ctx, hash); err != nil {
-					log.Errorf("gave up waiting for tx %x to be mined: %s", hash, err)
-					errorChan <- err
-				}
-
-				// check if the current account has a valid SIK
-				validSik, err := accountApi.ValidSIK()
-				if err != nil {
-					errorChan <- fmt.Errorf("unexpected error in account %s, when validate SIK, %s", acc.Address(), err)
-				}
-				if !validSik {
-					errorChan <- fmt.Errorf("unexpected invalid SIK for account %x", acc.Address())
-				}
-				log.Infof("valid SIK for the account %x", acc.Address())
-			}(i, acc)
-		}
-
-		go func() { // avoid blocking the main goroutine
-			wg.Wait()
-			close(errorChan) // close the error channel after all goroutines have finished
-		}()
-
-		for err := range errorChan { // receive errors from the errorChan until it's closed.
-			if err != nil {
-				return err
-			}
-		}
-
-		// register the remaining accounts
-		if err := t.registerAnonAccts(voterAccts[halfAccts:]); err != nil {
-			return err
-		}
-
-	}
 	if err := t.generateProofs(nil, voterAccts); err != nil {
 		return err
 	}
@@ -459,7 +387,7 @@ func (t *e2eElection) setupElectionRaw(prc *models.Process) error {
 
 	default:
 		censusType := vapi.CensusTypeWeighted
-		censusID, vAccts, err := t.setupCensus(censusType, t.config.nvotes, false)
+		censusID, vAccts, err := t.setupCensus(censusType, t.config.nvotes)
 		if err != nil {
 			return err
 		}
diff --git a/cmd/node/main.go b/cmd/node/main.go
index 94ee390..1cde7be 100644
--- a/cmd/node/main.go
+++ b/cmd/node/main.go
@@ -553,7 +553,6 @@ func main() {
 			urlapi.WalletHandler,
 			urlapi.AccountHandler,
 			urlapi.CensusHandler,
-			urlapi.SIKHandler,
 		); err != nil {
 			log.Fatal(err)
 		}
diff --git a/dockerfiles/testsuite/start_test.sh b/dockerfiles/testsuite/start_test.sh
index ae287e2..4e8892a 100755
--- a/dockerfiles/testsuite/start_test.sh
+++ b/dockerfiles/testsuite/start_test.sh
@@ -4,7 +4,6 @@
 #  e2etest_plaintextelection_empty: run poll vote test, with no votes
 #  e2etest_plaintextelection: run poll vote test
 #  e2etest_encryptedelection: run encrypted vote test
-#  e2etest_anonelection: run anonymous vote test
 #  e2etest_tokentxs: run token transactions test (no end-user voting at all)
 #  e2etest_overwritelection: run overwrite test
 #  e2etest_censusizelection: run max census size test
@@ -24,7 +23,6 @@ COMPOSE_CMD_RUN="$COMPOSE_CMD run"
 [ -n "$NOTTY" ] && COMPOSE_CMD_RUN="$COMPOSE_CMD_RUN -T"
 
 ELECTION_SIZE=${TESTSUITE_ELECTION_SIZE:-30}
-ELECTION_SIZE_ANON=${TESTSUITE_ELECTION_SIZE_ANON:-8}
 BUILD=${BUILD:-1}
 CLEAN=${CLEAN:-1}
 LOGLEVEL=${LOGLEVEL:-debug}
@@ -52,7 +50,6 @@ tests_to_run=(
   	"e2etest_plaintextelection_empty"
   	"e2etest_plaintextelection"
   	"e2etest_encryptedelection"
-  	"e2etest_anonelection"
   	"e2etest_overwritelection"
   	"e2etest_censusizelection"
   	"e2etest_ballotelection"
@@ -73,7 +70,6 @@ tests_to_run=(
 	echo "env vars:"
 	echo "  CLEAN=1"
 	echo "  ELECTION_SIZE=30"
-	echo "  ELECTION_SIZE_ANON=8"
 	echo "  CONCURRENT=0"
 	echo "  LOGLEVEL=debug"
 	echo "  APIHOST=http://gateway0:9090/v2"
@@ -118,12 +114,6 @@ e2etest_encryptedelection() {
 	e2etest encryptedelection
 }
 
-e2etest_anonelection() {
-	e2etest anonelection
-	e2etest anonelectionTempSiks
-	e2etest anonelectionEncrypted
-}
-
 e2etest_hysteresis() {
 	e2etest hysteresis
 }
diff --git a/vocone/vocone.go b/vocone/vocone.go
index 6681c13..f389ae2 100644
--- a/vocone/vocone.go
+++ b/vocone/vocone.go
@@ -182,7 +182,6 @@ func (vc *Vocone) EnableAPI(host string, port int, urlPath string) (*api.API, er
 		api.WalletHandler,
 		api.AccountHandler,
 		api.CensusHandler,
-		api.SIKHandler,
 	)
 }
 
```

Notes:
- `chain.go` loses `chainCircuitInfoHandler` (base `api/chain.go:572-588`) and its registration. `chainInfoHandler` keeps `CircuitVersion: circuit.Version()`.
- `census_helpers.go` loses `addressParse`, whose only user was `siks.go`.
- `cmd/end2endtest/helpers.go`:
  - `setupCensus` loses its `withSIK` parameter, and the callers are updated.
  - The `TempSIKs` block goes, but the `generateProofs` call stays.
  - `registerAnonAccts` stays, because `cmd/end2endtest/account.go:420` uses it.
- `start_test.sh` drops the `e2etest_anonelection` suite. Its operations lived in the deleted `zkweighted.go`.

Step 4: quick check.
Run: `cd $W && gofmt -l api apiclient cmd vocone`
Expected: empty.

Step 5: commit.
`git add api/anonymous_removed_test.go && git commit -m "refactor(api): remove anonymous voting routes, census proof siblings and the apiclient zk paths" -- api/siks.go apiclient/sik.go benchmark/zk_census_benchmark_test.go cmd/end2endtest/zkweighted.go test/siks_test.go api/census_test.go api/anonymous_removed_test.go api/censuses.go api/chain.go api/census_helpers.go api/docs/descriptions/censusCreateHandler.md apiclient/client.go apiclient/census.go apiclient/election.go apiclient/vote.go cmd/end2endtest/helpers.go cmd/end2endtest/dynamicensus.go cmd/node/main.go vocone/vocone.go dockerfiles/testsuite/start_test.sh`

Interfaces provided:
- `apiclient` no longer exports `CensusProof.Siblings` or the SIK proof helpers.
- `(*HTTPclient).Vote` returns `anonymous voting is not supported` for anonymous elections.

### Task 4: Server-side alias, census IPFS and indexer export prune
Model: sonnet   Repo: ~/src/vd/vocdoni-node
Files:
- `api/accounts.go`
- `api/chain.go`
- `api/elections.go`
- `api/censuses.go`
- `api/helpers.go`
- `api/legacy.go` (deleted)
- `api/docs/descriptions/electionListByFilterHandler.md` (deleted)
- `vochain/indexer/indexer.go`

Goal: AC1. In the final tree, no removed route keeps a live registration. The removed handlers and the helpers only they used are gone.

Step 1: delete the registration block (`if err := a.Endpoint.RegisterMethod("<path>", "<method>", …); err != nil { return err }`) for each of these routes.
- `api/accounts.go` (base lines 53-116):
  - `/accounts/{organizationId}/elections/count`
  - `/accounts/{organizationId}/elections/status/{status}/page/{page}`
  - `/accounts/{organizationId}/elections/page/{page}`
  - `/accounts/{accountId}/transfers/page/{page}`
  - `/accounts/{accountId}/fees/page/{page}`
  - `/accounts/{accountId}/transfers/count`
  - `/accounts/count`
  - `/accounts/page/{page}`
- `api/chain.go`:
  - `/chain/organizations/page/{page}`
  - `/chain/organizations/count`
  - `/chain/blocks/{height}/transactions/page/{page}`
  - `/chain/transactions/{height}/{index}`
  - `/chain/transactions/page/{page}`
  - `POST /chain/organizations/filter/page/{page}`
  - `/chain/transactions/count`
  - `/chain/fees/page/{page}`
  - `/chain/fees/reference/{reference}/page/{page}`
  - `/chain/fees/type/{type}/page/{page}`
  - `/chain/export/indexer`
- `api/elections.go`:
  - `/elections/page/{page}`
  - `/elections/{electionId}/votes/count`
  - `/elections/{electionId}/votes/page/{page}`
  - `POST /elections/filter/page/{page}`
  - `POST /elections/filter`
- `api/censuses.go`:
  - `/censuses/export/ipfs`
  - `/censuses/export/ipfs/list`
  - `/censuses/import/{ipfscid}`

Step 2: delete these functions, each with its doc comment. Base line numbers are of the `func` line.
- `api/accounts.go`:
  - `accountCountHandler` :339
  - `accountElectionsListByPageHandler` :360
  - `accountElectionsListByStatusAndPageHandler` :399
  - `accountElectionsCountHandler` :437
  - `tokenTransfersListHandler` :470
  - `tokenFeesHandler` :507
  - `tokenTransfersCountHandler` :547
  - `accountListByPageHandler` :579
- `api/chain.go`:
  - `organizationListByPageHandler` :329
  - `organizationListByFilterAndPageHandler` :367
  - `organizationCountHandler` :453
  - `chainTxByHeightAndIndexHandler` :974
  - `chainTxListByPageHandler` :1087
  - `chainTxListByHeightAndPageHandler` :1126
  - `chainTxCountHandler` :1386
  - `chainFeesListByPageHandler` :1440
  - `chainFeesListByReferenceAndPageHandler` :1477
  - `chainFeesListByTypeAndPageHandler` :1518
  - `chainIndexerExportHandler` :1653
- `api/elections.go`:
  - `electionListByFilterAndPageHandler` :180
  - `electionListByFilterHandler` :222
  - `electionListByPageHandler` :253
  - `electionVotesCountHandler` :457
  - `electionVotesListByPageHandler` :617
- `api/censuses.go`: `censusExportIPFSListDBHandler`, and the `censusIPFSExports` variable with its comment.
- `api/helpers.go`: `parseHexString` :358.
- `vochain/indexer/indexer.go`: `CountTokenTransfersByAccount` :1054. The sqlc query in `vochain/indexer/db` stays, because it is generated code.
- Leave `ExportBackupAsBytes` in place: `service/indexer.go:60` uses it.

Then:
```sh
cd $W && git rm -q api/legacy.go api/docs/descriptions/electionListByFilterHandler.md
```
- All `empty*List` helpers in `legacy.go` lose their last users here.
- The `.md` file was used only by `electionListByFilterHandler`.

Step 3: the remaining non-deletion edits.
- `api/censuses.go`:
  - `censusExportDBHandler` and `censusImportDBHandler` become the sequencer's synchronous versions.
  - Drop the `censusIPFSExports` initialisation in `enableCensusHandlers` and the `censusRetrieveTimeout` const.
  - Exact diff after Task 3:
```diff
diff --git a/api/censuses.go b/api/censuses.go
index 2fcb175..5a125ec 100644
--- a/api/censuses.go
+++ b/api/censuses.go
@@ -38,8 +38,7 @@ const (
 
 	MaxCensusAddBatchSize = 8192
 
-	censusIDsize          = 32
-	censusRetrieveTimeout = 10 * time.Minute
+	censusIDsize = 32
 )
 
 func (a *API) enableCensusHandlers() error {
@@ -165,22 +164,6 @@ func (a *API) enableCensusHandlers() error {
 	}
 
 	// Admin-only endpoints
-	if err := a.Endpoint.RegisterMethod(
-		"/censuses/export/ipfs",
-		"GET",
-		apirest.MethodAccessTypeAdmin,
-		a.censusExportDBHandler,
-	); err != nil {
-		return err
-	}
-	if err := a.Endpoint.RegisterMethod(
-		"/censuses/export/ipfs/list",
-		"GET",
-		apirest.MethodAccessTypeAdmin,
-		a.censusExportIPFSListDBHandler,
-	); err != nil {
-		return err
-	}
 	if err := a.Endpoint.RegisterMethod(
 		"/censuses/export",
 		"GET",
@@ -189,14 +172,6 @@ func (a *API) enableCensusHandlers() error {
 	); err != nil {
 		return err
 	}
-	if err := a.Endpoint.RegisterMethod(
-		"/censuses/import/{ipfscid}",
-		"GET",
-		apirest.MethodAccessTypeAdmin,
-		a.censusImportDBHandler,
-	); err != nil {
-		return err
-	}
 	if err := a.Endpoint.RegisterMethod(
 		"/censuses/import",
 		"POST",
@@ -214,9 +189,6 @@ func (a *API) enableCensusHandlers() error {
 		return err
 	}
 
-	// Initialize the map to store the status of the async export to ipfs
-	censusIPFSExports = make(map[string]time.Time)
-
 	return nil
 }
 
@@ -837,8 +809,8 @@ func (a *API) censusPublishCheckHandler(_ *apirest.APIdata, ctx *httprouter.HTTP
 //	@Tags					Censuses
 //	@Accept					json
 //	@Produce				json
-//	@Param					censusId	path		string																										true	"Census id"
-//	@Param					key			path		string																										true	"Key to proof"
+//	@Param					censusId	path		string																				true	"Census id"
+//	@Param					key			path		string																				true	"Key to proof"
 //	@Success				200			{object}	object{type=string,weight=string,value=string,censusProof=string,censusRoot=string}	"censusProof is the Merkle proof and value is the Merkle tree leaf value"
 //	@Router					/censuses/{censusId}/proof/{key} [get]
 func (a *API) censusProofHandler(_ *apirest.APIdata, ctx *httprouter.HTTPContext) error {
@@ -974,116 +946,37 @@ func (a *API) censusListHandler(_ *apirest.APIdata, ctx *httprouter.HTTPContext)
 	return ctx.Send(data, apirest.HTTPstatusOK)
 }
 
-// censusIPFSExports is a map of ipfs uri to the time when the export was requested
-var censusIPFSExports = map[string]time.Time{}
-
 // censusExportDBHandler
 //
 //	@Summary		Export census database
 //	@Description	Export the whole census database to a JSON file. Requires Admin Bearer token.
-//	@Description	- `/censuses/export` returns the census database dump.
-//	@Description	- `/censuses/export/ipfs` schedules an async export to IPFS; check `/censuses/export/ipfs/list` for the resulting URI.
 //	@Tags			Censuses
 //	@Accept			json
 //	@Produce		json
 //	@Security		ApiKeyAuth
-//	@Success		200	{array}	censusdb.CensusDump	"census database dump (`/censuses/export`), or object{message=string} (`/censuses/export/ipfs`)"
-//	@Router			/censuses/export/ipfs [get]
+//	@Success		200	{array}	censusdb.CensusDump	"census database dump"
 //	@Router			/censuses/export [get]
 func (a *API) censusExportDBHandler(_ *apirest.APIdata, ctx *httprouter.HTTPContext) error {
-	isIPFSExport := strings.HasSuffix(ctx.Request.URL.Path, "ipfs")
 	buf := bytes.Buffer{}
-	var data []byte
-	if isIPFSExport {
-		go func() {
-			log.Infow("exporting census database to ipfs async")
-			startTime := time.Now()
-			if err := a.censusdb.ExportCensusDB(&buf); err != nil {
-				log.Errorw(err, "could not export census database")
-				return
-			}
-			log.Infow("census database exported", "duration (s)", time.Since(startTime).Seconds())
-			startTime = time.Now()
-			uri, err := a.storage.PublishReader(context.Background(), &buf)
-			if err != nil {
-				log.Errorw(err, "could not publish census database to ipfs")
-				return
-			}
-			log.Infow("census database published to ipfs", "uri", uri, "duration (s)", time.Since(startTime).Seconds())
-			censusIPFSExports[uri] = time.Now()
-		}()
-		var err error
-		data, err = json.Marshal(map[string]string{
-			"message": "scheduled, check /censuses/export/ipfs/list",
-		})
-		if err != nil {
-			log.Errorw(err, "could not marshal response")
-		}
-	} else {
-		if err := a.censusdb.ExportCensusDB(&buf); err != nil {
-			return err
-		}
-		data = buf.Bytes()
-	}
-	return ctx.Send(data, apirest.HTTPstatusOK)
-}
-
-// censusExportIPFSListDBHandler
-//
-//	@Summary		List export census database to IPFS
-//	@Description	List the IPFS URIs of the census database exports. Requires Admin Bearer token.
-//	@Tags			Censuses
-//	@Accept			json
-//	@Produce		json
-//	@Security		ApiKeyAuth
-//	@Success		200	{object}	map[string]string	"map of IPFS CID (usable with /censuses/import/{ipfscid}) to export completion time (RFC3339)"
-//	@Router			/censuses/export/ipfs/list [get]
-func (a *API) censusExportIPFSListDBHandler(_ *apirest.APIdata, ctx *httprouter.HTTPContext) error {
-	data, err := json.Marshal(censusIPFSExports)
-	if err != nil {
+	if err := a.censusdb.ExportCensusDB(&buf); err != nil {
 		return err
 	}
-	return ctx.Send(data, apirest.HTTPstatusOK)
+	return ctx.Send(buf.Bytes(), apirest.HTTPstatusOK)
 }
 
-// censusImportHandler
+// censusImportDBHandler
 //
 //	@Summary		Import census database
-//	@Description	Import the whole census database from a JSON file. Requires Admin Bearer token.
-//	@Description	- `GET /censuses/import/{ipfscid}` imports asynchronously from IPFS.
-//	@Description	- `POST /censuses/import` imports synchronously from the request body (the JSON returned by `/censuses/export`).
+//	@Description	Import the whole census database from a JSON file (the JSON returned by `/censuses/export`). Requires Admin Bearer token.
 //	@Tags			Censuses
 //	@Accept			json
 //	@Produce		json
 //	@Security		ApiKeyAuth
-//	@Param			ipfscid	path	string	true	"IPFS CID of a database export (see /censuses/export/ipfs/list)"
-//	@Success		200		"(empty body)"
-//	@Router			/censuses/import/{ipfscid} [get]
+//	@Success		200	"(empty body)"
 //	@Router			/censuses/import [post]
 func (a *API) censusImportDBHandler(msg *apirest.APIdata, ctx *httprouter.HTTPContext) error {
-	ipfscid := ctx.URLParam("ipfscid")
-	if ipfscid == "" {
-		// if no ipfs cid, import from the request body
-		if err := a.censusdb.ImportCensusDB(bytes.NewReader(msg.Data)); err != nil {
-			return err
-		}
-		return ctx.Send(nil, apirest.HTTPstatusOK)
-	}
-	log.Infow("importing census database from ipfs async", "ipfscid", ipfscid)
-	go func() {
-		ctx, cancel := context.WithTimeout(context.Background(), censusRetrieveTimeout)
-		defer cancel()
-		// TODO: Retrieve should return a reader
-		startTime := time.Now()
-		data, err := a.storage.Retrieve(ctx, "/ipfs/"+ipfscid, 0)
-		if err != nil {
-			log.Errorw(err, "could not retrieve census database from ipfs")
-			return
-		}
-		log.Infow("census database retrieved from ipfs", "ipfscid", ipfscid, "duration", time.Since(startTime))
-		if err := a.censusdb.ImportCensusDB(bytes.NewReader(data)); err != nil {
-			log.Errorw(err, "could not import census database from ipfs")
-		}
-	}()
+	if err := a.censusdb.ImportCensusDB(bytes.NewReader(msg.Data)); err != nil {
+		return err
+	}
 	return ctx.Send(nil, apirest.HTTPstatusOK)
 }
```
- `api/chain.go`: rewrite the `organizationList` doc comment, which mentioned the removed POST filter, and drop the now unused `"fmt"` import:
```diff
 // organizationList produces a filtered, sorted, paginated OrganizationsList.
 //
-// The ordering is validated here rather than in parseOrganizationParams so that
-// the deprecated POST filter endpoint, which unmarshals its OrganizationParams
-// straight from the request body, is covered as well.
-//
 // Errors returned are always of type APIerror.
```
- `api/elections.go`: drop the now unused `"go.vocdoni.io/dvote/statedb"` import.
- `api/helpers.go`: drop the now unused `"go.vocdoni.io/dvote/util"` import.
- Run `gofmt -w` on the touched files to collapse the blank lines that the deleted blocks leave behind.

Step 4: quick check (api compiles again, but the apiclient callers are only fixed in Task 5).
Run: `tar -C $W --exclude=.git -cf - . | ssh z6.vocdoni.net "rm -rf ~/scratch/route-prune && mkdir -p ~/scratch/route-prune && tar -x -C ~/scratch/route-prune" && ssh z6.vocdoni.net 'cd ~/scratch/route-prune && export GOTOOLCHAIN=go1.26.5 && gofmt -l api vochain && go build ./api/ ./vochain/... && go vet ./api/'`
Expected: no output.

Step 5: commit.
`git commit -m "refactor(api): remove deprecated paginated, count, census IPFS and indexer export routes" -- api/accounts.go api/chain.go api/elections.go api/censuses.go api/helpers.go api/legacy.go api/docs/descriptions/electionListByFilterHandler.md vochain/indexer/indexer.go`

### Task 5: apiclient migration and in-repo tests
Model: sonnet (careful)   Repo: ~/src/vd/vocdoni-node
Files:
- `apiclient/client.go`
- `apiclient/account.go`
- `apiclient/blockchain.go`
- `apiclient/election.go`
- `test/api_test.go`
- `test/createaccounts_test.go`
- `test/apierror_test.go`
- `api/chain_organizations_test.go`

Goal: AC1 and AC9. The apiclient methods call the replacement routes with unchanged signatures, and no in-repo test calls a pruned route.

Migration map:
- `RequestWithQuery(method, body, url.Values, path...)` is new; `Request` delegates to it (from `c3a2984b`).
- `ListTokenTransfers` → `GET /chain/transfers?accountId=&page=`.
- `ListAccounts` → `GET /accounts?page=`. The sequencer still calls `/accounts/page/{page}` here, so do not copy it.
- `CountAccounts` → `GET /accounts` `pagination.totalItems`.
- `CountTokenTransfers` → `transfersCount` of `GET /accounts/{address}`.
- `TransactionByHash` → `GET /chain/transactions/{hash}`, decoding the `tx` field of `GenericTransactionWithInfo` (from `c3a2984b`).
- `TransactionCount` → `transactionCount` of `GET /chain/info`. The sequencer still calls `/chain/transactions/count`, so do not copy it.
- `OrganizationsBySearchTermPaginated` → `GET /chain/organizations?organizationId=&page=`, returning the IDs.
- `ElectionVoteCount` → `voteCount` of `GET /elections/{id}`.
- `ElectionFilterPaginated` → `GET /elections` with `organizationId`, `electionId`, `status`, `withResults` and `page`. Empty and zero values are left out.

The tests move to the same replacement routes:
- The duplicate `page/{n}` comparisons are dropped.
- The account and transfer counts are read from pagination and `transfersCount`.
- The per-block list uses `?height=`.
- `waitUntilHeight` no longer cross-checks `/chain/transactions/count`.
- The `POST /elections/filter` comparison becomes a `withResults` total check.
- `test/apierror_test.go` drops the 8 fixtures on removed routes. Their error codes stay covered by `TestAPIerrorWithQuery`.
- `api/chain_organizations_test.go` drops the legacy by-page and POST filter section. `TestRemovedRoutes` covers those routes now.

Step 1–2: exact diff after Task 4:
```diff
diff --git a/api/chain_organizations_test.go b/api/chain_organizations_test.go
index ec1ba60..b41d611 100644
--- a/api/chain_organizations_test.go
+++ b/api/chain_organizations_test.go
@@ -124,47 +124,4 @@ func TestOrganizationListSorting(t *testing.T) {
 		c.Assert(json.Unmarshal(resp, apiErr), qt.IsNil)
 		c.Assert(apiErr.Code, qt.Equals, wantErr.Code, qt.Commentf("query %q: %s", query, resp))
 	}
-
-	// The deprecated by-page endpoint keeps working, with the default ordering.
-	resp, code := cl.Request("GET", nil, "organizations", "page", "0")
-	c.Assert(code, qt.Equals, apirest.HTTPstatusOK)
-	legacy := &OrganizationsList{}
-	c.Assert(json.Unmarshal(resp, legacy), qt.IsNil)
-	c.Assert(legacy.Organizations, qt.HasLen, 3)
-
-	// The deprecated POST filter endpoint takes its OrganizationParams straight
-	// from the request body, so it reaches the ordering without going through
-	// parseOrganizationParams. It must sort, and reject, just the same.
-	postList := func(body *OrganizationParams) []string {
-		resp, code := cl.Request("POST", body, "organizations", "filter", "page", "0")
-		c.Assert(code, qt.Equals, apirest.HTTPstatusOK, qt.Commentf("body %+v: %s", body, resp))
-		result := &OrganizationsList{}
-		c.Assert(json.Unmarshal(resp, result), qt.IsNil)
-		byID := map[string]string{}
-		for name, eid := range orgs {
-			byID[hex.EncodeToString(eid)] = name
-		}
-		names := []string{}
-		for _, org := range result.Organizations {
-			names = append(names, byID[org.OrganizationID.String()])
-		}
-		return names
-	}
-	c.Assert(postList(&OrganizationParams{SortBy: "electionCount", Order: "desc"}),
-		qt.DeepEquals, []string{"abacus", "zenith", "quorum"})
-	c.Assert(postList(&OrganizationParams{}), qt.DeepEquals, []string{"zenith", "abacus", "quorum"})
-
-	for _, tc := range []struct {
-		body    *OrganizationParams
-		wantErr apirest.APIerror
-	}{
-		{&OrganizationParams{SortBy: "votes"}, ErrParamSortByInvalid},
-		{&OrganizationParams{Order: "sideways"}, ErrParamOrderInvalid},
-	} {
-		resp, code := cl.Request("POST", tc.body, "organizations", "filter", "page", "0")
-		c.Assert(code, qt.Equals, tc.wantErr.HTTPstatus, qt.Commentf("body %+v: %s", tc.body, resp))
-		apiErr := &apirest.APIerror{}
-		c.Assert(json.Unmarshal(resp, apiErr), qt.IsNil)
-		c.Assert(apiErr.Code, qt.Equals, tc.wantErr.Code, qt.Commentf("body %+v: %s", tc.body, resp))
-	}
 }
diff --git a/apiclient/account.go b/apiclient/account.go
index 4946ab9..8a2d5ec 100644
--- a/apiclient/account.go
+++ b/apiclient/account.go
@@ -3,6 +3,7 @@ package apiclient
 import (
 	"encoding/json"
 	"fmt"
+	"net/url"
 	"strconv"
 
 	"github.com/ethereum/go-ethereum/common"
@@ -276,7 +277,11 @@ func (c *HTTPclient) AccountSetMetadata(metadata *api.AccountMetadata) (types.He
 
 // ListTokenTransfers returns the list of sent and received token transfers associated with an account
 func (c *HTTPclient) ListTokenTransfers(account common.Address, page int) (*api.TransfersList, error) {
-	resp, code, err := c.Request(HTTPGET, nil, "accounts", account.Hex(), "transfers", "page", strconv.Itoa(page))
+	query := url.Values{
+		api.ParamAccountId: {account.Hex()},
+		api.ParamPage:      {strconv.Itoa(page)},
+	}
+	resp, code, err := c.RequestWithQuery(HTTPGET, nil, query, "chain", "transfers")
 	if err != nil {
 		return nil, err
 	}
@@ -423,58 +428,53 @@ func (c *HTTPclient) RegisterSIKForVote(electionId types.HexBytes, proof *Census
 
 // ListAccounts return the account list information (address, balance and nonce)
 func (c *HTTPclient) ListAccounts(page int) ([]indexertypes.Account, error) {
-	resp, code, err := c.Request(HTTPGET, nil, "accounts", "page", strconv.Itoa(page))
+	accts, err := c.accountsList(page)
 	if err != nil {
 		return nil, err
 	}
-	if code != apirest.HTTPstatusOK {
-		return nil, fmt.Errorf("%s: %d (%s)", errCodeNot200, code, resp)
-	}
-	accts := new(struct {
-		Accounts []indexertypes.Account `json:"accounts"`
-	})
-
-	if err := json.Unmarshal(resp, &accts); err != nil {
-		return nil, err
+	list := make([]indexertypes.Account, 0, len(accts.Accounts))
+	for _, acc := range accts.Accounts {
+		list = append(list, *acc)
 	}
-
-	return accts.Accounts, nil
+	return list, nil
 }
 
-// CountAccounts returns the total count of exiting accounts
+// CountAccounts returns the total count of existing accounts, read from the
+// pagination of GET /accounts.
 func (c *HTTPclient) CountAccounts() (uint64, error) {
-	resp, code, err := c.Request(HTTPGET, nil, "accounts", "count")
+	accts, err := c.accountsList(0)
 	if err != nil {
 		return 0, err
 	}
-	if code != apirest.HTTPstatusOK {
-		return 0, fmt.Errorf("%s: %d (%s)", errCodeNot200, code, resp)
+	if accts.Pagination == nil {
+		return 0, fmt.Errorf("accounts list has no pagination")
 	}
-	accts := new(struct {
-		Count uint64 `json:"count"`
-	})
-
-	if err := json.Unmarshal(resp, accts); err != nil {
-		return 0, err
-	}
-	return accts.Count, nil
+	return accts.Pagination.TotalItems, nil
 }
 
-// CountTokenTransfers returns the total count of transfers sent and received for an account
-func (c *HTTPclient) CountTokenTransfers(accountID common.Address) (uint64, error) {
-	resp, code, err := c.Request(HTTPGET, nil, "accounts", accountID.Hex(), "transfers", "count")
+// accountsList fetches a page of GET /accounts.
+func (c *HTTPclient) accountsList(page int) (*api.AccountsList, error) {
+	query := url.Values{api.ParamPage: {strconv.Itoa(page)}}
+	resp, code, err := c.RequestWithQuery(HTTPGET, nil, query, "accounts")
 	if err != nil {
-		return 0, err
+		return nil, err
 	}
 	if code != apirest.HTTPstatusOK {
-		return 0, fmt.Errorf("%s: %d (%s)", errCodeNot200, code, resp)
+		return nil, fmt.Errorf("%s: %d (%s)", errCodeNot200, code, resp)
 	}
-	tokentxs := new(struct {
-		Count uint64 `json:"count"`
-	})
+	accts := &api.AccountsList{}
+	if err := json.Unmarshal(resp, accts); err != nil {
+		return nil, err
+	}
+	return accts, nil
+}
 
-	if err := json.Unmarshal(resp, tokentxs); err != nil {
+// CountTokenTransfers returns the total count of transfers sent and received
+// for an account, read from transfersCount of GET /accounts/{address}.
+func (c *HTTPclient) CountTokenTransfers(accountID common.Address) (uint64, error) {
+	acc, err := c.Account(accountID.Hex())
+	if err != nil {
 		return 0, err
 	}
-	return tokentxs.Count, nil
+	return acc.TransfersCount, nil
 }
diff --git a/apiclient/blockchain.go b/apiclient/blockchain.go
index 565c380..12187e5 100644
--- a/apiclient/blockchain.go
+++ b/apiclient/blockchain.go
@@ -3,6 +3,8 @@ package apiclient
 import (
 	"encoding/json"
 	"fmt"
+	"net/url"
+	"strconv"
 
 	"go.vocdoni.io/dvote/api"
 	"go.vocdoni.io/dvote/httprouter/apirest"
@@ -101,20 +103,19 @@ func (c *HTTPclient) TransactionReference(txHash types.HexBytes) (*api.Transacti
 // For querying if a transaction is included in a block, it is recommended to
 // use TransactionReference which is much faster.
 func (c *HTTPclient) TransactionByHash(txHash types.HexBytes) (*models.Tx, error) {
-	ref, err := c.TransactionReference(txHash)
-	if err != nil {
-		return nil, err
-	}
-	resp, code, err := c.Request(HTTPGET, nil,
-		"chain", "transactions", fmt.Sprintf("%d", ref.Height), fmt.Sprintf("%d", ref.Index))
+	resp, code, err := c.Request(HTTPGET, nil, "chain", "transactions", txHash.String())
 	if err != nil {
 		return nil, err
 	}
 	if code != apirest.HTTPstatusOK {
 		return nil, ErrTransactionDoesNotExist
 	}
+	txInfo := &api.GenericTransactionWithInfo{}
+	if err := json.Unmarshal(resp, txInfo); err != nil {
+		return nil, err
+	}
 	tx := &models.Tx{}
-	return tx, protojson.Unmarshal(resp, tx)
+	return tx, protojson.Unmarshal(txInfo.TxContent, tx)
 }
 
 // OrganizationsBySearchTermPaginated returns a paginated list of organizations
@@ -122,42 +123,34 @@ func (c *HTTPclient) TransactionByHash(txHash types.HexBytes) (*models.Tx, error
 func (c *HTTPclient) OrganizationsBySearchTermPaginated(
 	organizationID types.HexBytes, page int,
 ) ([]types.HexBytes, error) {
-	// make a post request to /chain/organizations/filter/page/<page> with the organizationID
-	// as the body and page as the url parameter
-	resp, code, err := c.Request(HTTPPOST,
-		organizationID,
-		"chain",
-		"organizations",
-		"filter",
-		"page",
-		fmt.Sprintf("%d", page))
+	query := url.Values{
+		api.ParamOrganizationId: {organizationID.String()},
+		api.ParamPage:           {strconv.Itoa(page)},
+	}
+	resp, code, err := c.RequestWithQuery(HTTPGET, nil, query, "chain", "organizations")
 	if err != nil {
 		return nil, err
 	}
 	if code != apirest.HTTPstatusOK {
 		return nil, fmt.Errorf("%s: %d (%s)", errCodeNot200, code, resp)
 	}
-	orgs := new(struct {
-		Organizations []types.HexBytes `json:"organizations"`
-	})
+	orgs := &api.OrganizationsList{}
 	if err := json.Unmarshal(resp, orgs); err != nil {
 		return nil, err
 	}
-	return orgs.Organizations, nil
+	ids := make([]types.HexBytes, 0, len(orgs.Organizations))
+	for _, org := range orgs.Organizations {
+		ids = append(ids, org.OrganizationID)
+	}
+	return ids, nil
 }
 
-// TransactionCount returns the count of transactions
+// TransactionCount returns the count of transactions, read from
+// transactionCount of GET /chain/info.
 func (c *HTTPclient) TransactionCount() (uint64, error) {
-	resp, code, err := c.Request(HTTPGET, nil, "chain", "transactions", "count")
+	info, err := c.ChainInfo()
 	if err != nil {
 		return 0, err
 	}
-	if code != apirest.HTTPstatusOK {
-		return 0, fmt.Errorf("%s: %d (%s)", errCodeNot200, code, resp)
-	}
-	txsCount := new(struct {
-		Count uint64 `json:"count"`
-	})
-
-	return txsCount.Count, json.Unmarshal(resp, txsCount)
+	return info.TransactionCount, nil
 }
diff --git a/apiclient/client.go b/apiclient/client.go
index f6290db..e1b29bc 100644
--- a/apiclient/client.go
+++ b/apiclient/client.go
@@ -171,6 +171,11 @@ func (c *HTTPclient) SetTimeout(d time.Duration) {
 // Method is either GET or POST. If POST, a JSON struct should be attached.  Returns the response,
 // the status code and an error.
 func (c *HTTPclient) Request(method string, jsonBody any, urlPath ...string) ([]byte, int, error) {
+	return c.RequestWithQuery(method, jsonBody, nil, urlPath...)
+}
+
+// RequestWithQuery is Request with the query string built from query.
+func (c *HTTPclient) RequestWithQuery(method string, jsonBody any, query url.Values, urlPath ...string) ([]byte, int, error) {
 	body, err := json.Marshal(jsonBody)
 	if err != nil {
 		return nil, 0, err
@@ -180,6 +185,7 @@ func (c *HTTPclient) Request(method string, jsonBody any, urlPath ...string) ([]
 		return nil, 0, err
 	}
 	u.Path = path.Join(u.Path, path.Join(urlPath...))
+	u.RawQuery = query.Encode()
 	headers := http.Header{}
 	if c.token != nil {
 		headers = http.Header{
diff --git a/apiclient/election.go b/apiclient/election.go
index 3a0bb49..44a01b4 100644
--- a/apiclient/election.go
+++ b/apiclient/election.go
@@ -4,6 +4,7 @@ import (
 	"context"
 	"encoding/json"
 	"fmt"
+	"net/url"
 	"strconv"
 	"time"
 
@@ -487,23 +488,14 @@ func (c *HTTPclient) SetElectionCensus(electionID types.HexBytes, census api.Ele
 	return hash, err
 }
 
-// ElectionVoteCount returns the number of registered votes for a given election.
+// ElectionVoteCount returns the number of registered votes for a given
+// election, read from voteCount of GET /elections/{electionId}.
 func (c *HTTPclient) ElectionVoteCount(electionID types.HexBytes) (uint32, error) {
-	resp, code, err := c.Request(HTTPGET, nil, "elections", electionID.String(), "votes", "count")
+	election, err := c.Election(electionID)
 	if err != nil {
 		return 0, err
 	}
-	if code != apirest.HTTPstatusOK {
-		return 0, fmt.Errorf("%s: %d (%s)", errCodeNot200, code, resp)
-	}
-	votes := new(struct {
-		Count uint32 `json:"count"`
-	})
-
-	if err := json.Unmarshal(resp, votes); err != nil {
-		return 0, err
-	}
-	return votes.Count, nil
+	return uint32(election.VoteCount), nil
 }
 
 // ElectionResults returns the election results given its ID.
@@ -522,34 +514,40 @@ func (c *HTTPclient) ElectionResults(electionID types.HexBytes) (*api.ElectionRe
 	return electionResults, nil
 }
 
-// ElectionFilterPaginated returns a list of elections filtered by the given parameters.
-// POST /elections/filter/page/<page>
-// Returns a list of elections filtered by the given parameters.
+// ElectionFilterPaginated returns a page of GET /elections filtered by the
+// given parameters. Empty IDs, status PROCESS_UNKNOWN and withResults false
+// leave that filter out.
 func (c *HTTPclient) ElectionFilterPaginated(organizationID types.HexBytes, electionID types.HexBytes,
 	status models.ProcessStatus, withResults bool, page int,
 ) (*[]api.ElectionSummary, error) {
-	body := struct {
-		OrganizationID types.HexBytes `json:"organizationId,omitempty"`
-		ElectionID     types.HexBytes `json:"electionId,omitempty"`
-		WithResults    bool           `json:"withResults,omitempty"`
-		Status         string         `json:"status,omitempty"`
-	}{
-		OrganizationID: organizationID,
-		ElectionID:     electionID,
-		WithResults:    withResults,
-		Status:         status.String(),
+	query := url.Values{api.ParamPage: {strconv.Itoa(page)}}
+	if len(organizationID) > 0 {
+		query.Set(api.ParamOrganizationId, organizationID.String())
+	}
+	if len(electionID) > 0 {
+		query.Set(api.ParamElectionId, electionID.String())
 	}
-	resp, code, err := c.Request(HTTPPOST, body, "elections", "filter", "page", strconv.Itoa(page))
+	if status != models.ProcessStatus_PROCESS_UNKNOWN {
+		query.Set(api.ParamStatus, status.String())
+	}
+	if withResults {
+		query.Set(api.ParamWithResults, "true")
+	}
+	resp, code, err := c.RequestWithQuery(HTTPGET, nil, query, "elections")
 	if err != nil {
 		return nil, err
 	}
 	if code != apirest.HTTPstatusOK {
 		return nil, fmt.Errorf("%s: %d (%s)", errCodeNot200, code, resp)
 	}
-	var elections []api.ElectionSummary
-	if err := json.Unmarshal(resp, &elections); err != nil {
+	list := &api.ElectionsList{}
+	if err := json.Unmarshal(resp, list); err != nil {
 		return nil, err
 	}
+	elections := make([]api.ElectionSummary, 0, len(list.Elections))
+	for _, e := range list.Elections {
+		elections = append(elections, *e)
+	}
 	return &elections, nil
 }
 
diff --git a/test/api_test.go b/test/api_test.go
index 9e82ddd..c7a7522 100644
--- a/test/api_test.go
+++ b/test/api_test.go
@@ -259,31 +259,17 @@ func TestAPIaccount(t *testing.T) {
 	resp, code := c.Request("GET", nil, "accounts", signer.Address().String())
 	qt.Assert(t, code, qt.Equals, 200, qt.Commentf("response: %s", resp))
 
-	// get accounts count
-	resp, code = c.Request("GET", nil, "accounts", "count")
+	// get the accounts info
+	resp, code = c.RequestWithQuery("GET", nil, "page=0", "accounts")
 	qt.Assert(t, code, qt.Equals, 200, qt.Commentf("response: %s", resp))
 
-	countAccts := struct {
-		Count uint64 `json:"count"`
-	}{}
-
-	err := json.Unmarshal(resp, &countAccts)
+	accts := api.AccountsList{}
+	err := json.Unmarshal(resp, &accts)
 	qt.Assert(t, err, qt.IsNil)
 
 	// 3 accounts must exist: the previously new created account plus burn + faucet
 	// account used to transfer to the new account
-	qt.Assert(t, countAccts.Count, qt.Equals, uint64(3))
-
-	// get the accounts info
-	resp, code = c.Request("GET", nil, "accounts", "page", "0")
-	qt.Assert(t, code, qt.Equals, 200, qt.Commentf("response: %s", resp))
-
-	accts := struct {
-		Accounts []indexertypes.Account `json:"accounts"`
-	}{}
-
-	err = json.Unmarshal(resp, &accts)
-	qt.Assert(t, err, qt.IsNil)
+	qt.Assert(t, accts.Pagination.TotalItems, qt.Equals, uint64(3))
 
 	// the second account must be the account created in the test
 	// due 'ORDER BY balance DESC' in the sql query
@@ -335,18 +321,12 @@ func TestAPIAccountsList(t *testing.T) {
 	el := make(map[string]api.AccountsList)
 	el["0"] = fetchAL("GET", nil, "", "accounts")
 	el["1"] = fetchAL("GET", nil, "page=1", "accounts")
-	el["p0"] = fetchAL("GET", nil, "", "accounts", "page", "0")
-	el["p1"] = fetchAL("GET", nil, "", "accounts", "page", "1")
 
 	qt.Assert(t, el["0"], qt.Not(qt.DeepEquals), el["1"])
-	qt.Assert(t, el["0"], qt.DeepEquals, el["p0"])
-	qt.Assert(t, el["1"], qt.DeepEquals, el["p1"])
 
 	// 2 accounts pre-exist: the faucet account, and the burn address
 	qt.Assert(t, el["0"].Pagination.TotalItems, qt.Equals, uint64(2+20))
 	qt.Assert(t, el["1"].Pagination.TotalItems, qt.Equals, el["0"].Pagination.TotalItems)
-	qt.Assert(t, el["p0"].Pagination.TotalItems, qt.Equals, el["0"].Pagination.TotalItems)
-	qt.Assert(t, el["p1"].Pagination.TotalItems, qt.Equals, el["0"].Pagination.TotalItems)
 
 	for _, item := range el {
 		qt.Assert(t, len(item.Accounts), qt.Equals, api.DefaultItemsPerPage)
@@ -468,7 +448,7 @@ func TestAPIAccountTokentxs(t *testing.T) {
 	waitUntilHeight(t, c, 5)
 
 	// get the token transfers received and sent for account 1
-	resp, code = c.Request("GET", nil, "accounts", signer.Address().Hex(), "transfers", "page", "0")
+	resp, code = c.RequestWithQuery("GET", nil, "accountId="+signer.Address().Hex()+"&page=0", "chain", "transfers")
 	qt.Assert(t, code, qt.Equals, 200, qt.Commentf("response: %s", resp))
 
 	tokenTxs := &api.TransfersList{}
@@ -476,20 +456,18 @@ func TestAPIAccountTokentxs(t *testing.T) {
 	qt.Assert(t, err, qt.IsNil)
 
 	// get the total token transfers count for account 1
-	resp, code = c.Request("GET", nil, "accounts", signer.Address().Hex(), "transfers", "count")
+	resp, code = c.Request("GET", nil, "accounts", signer.Address().Hex())
 	qt.Assert(t, code, qt.Equals, 200, qt.Commentf("response: %s", resp))
 
-	countTnsAcc := struct {
-		Count uint64 `json:"count"`
-	}{}
-	err = json.Unmarshal(resp, &countTnsAcc)
+	acct := &api.Account{}
+	err = json.Unmarshal(resp, acct)
 	qt.Assert(t, err, qt.IsNil)
 
 	// compare count of total token transfers for the account 1 using the two response
-	qt.Assert(t, tokenTxs.Pagination.TotalItems, qt.Equals, countTnsAcc.Count)
+	qt.Assert(t, tokenTxs.Pagination.TotalItems, qt.Equals, acct.TransfersCount)
 
 	// get the token transfers received and sent for account 2
-	resp, code = c.Request("GET", nil, "accounts", signer2.Address().Hex(), "transfers", "page", "0")
+	resp, code = c.RequestWithQuery("GET", nil, "accountId="+signer2.Address().Hex()+"&page=0", "chain", "transfers")
 	qt.Assert(t, code, qt.Equals, 200, qt.Commentf("response: %s", resp))
 
 	tokenTxs2 := &api.TransfersList{}
@@ -497,24 +475,20 @@ func TestAPIAccountTokentxs(t *testing.T) {
 	qt.Assert(t, err, qt.IsNil)
 
 	// get the total token transfers count for account 2
-	resp, code = c.Request("GET", nil, "accounts", signer2.Address().Hex(), "transfers", "count")
+	resp, code = c.Request("GET", nil, "accounts", signer2.Address().Hex())
 	qt.Assert(t, code, qt.Equals, 200, qt.Commentf("response: %s", resp))
 
-	countTnsAcc2 := struct {
-		Count uint64 `json:"count"`
-	}{}
-	err = json.Unmarshal(resp, &countTnsAcc2)
+	acct2 := &api.Account{}
+	err = json.Unmarshal(resp, acct2)
 	qt.Assert(t, err, qt.IsNil)
 
 	// compare count of total token transfers for the account 2 using the two response
-	qt.Assert(t, tokenTxs2.Pagination.TotalItems, qt.Equals, countTnsAcc2.Count)
+	qt.Assert(t, tokenTxs2.Pagination.TotalItems, qt.Equals, acct2.TransfersCount)
 
-	resp, code = c.Request("GET", nil, "accounts", "page", "0")
+	resp, code = c.RequestWithQuery("GET", nil, "page=0", "accounts")
 	qt.Assert(t, code, qt.Equals, 200, qt.Commentf("response: %s", resp))
 
-	accts := struct {
-		Accounts []indexertypes.Account `json:"accounts"`
-	}{}
+	accts := api.AccountsList{}
 
 	err = json.Unmarshal(resp, &accts)
 	qt.Assert(t, err, qt.IsNil)
@@ -582,12 +556,9 @@ func TestAPIBlocks(t *testing.T) {
 	server.VochainAPP.AdvanceTestBlock()
 	waitUntilHeight(t, c, 3)
 
-	// each block must be listed with its own transactions by the per-block endpoint
-	// (regression: the height was passed into the hash filter slot, so the endpoint
-	// filtered by a hash substring with no height filter at all, listing every
-	// transaction whose hex hash contained the height as a substring)
+	// filtering by height must list only the transactions of that block
 	blockTxs := func(height uint32) []*indexertypes.TransactionMetadata {
-		resp, code := c.Request("GET", nil, "chain", "blocks", fmt.Sprint(height), "transactions", "page", "0")
+		resp, code := c.RequestWithQuery("GET", nil, fmt.Sprintf("height=%d&page=0", height), "chain", "transactions")
 		qt.Assert(t, code, qt.Equals, 200, qt.Commentf("response: %s", resp))
 		txList := api.TransactionsList{}
 		qt.Assert(t, json.Unmarshal(resp, &txList), qt.IsNil)
@@ -672,15 +643,6 @@ func waitUntilHeight(t testing.TB, c *testutil.TestHTTPclient, h uint32) {
 		chainInfo := api.ChainInfo{}
 		err := json.Unmarshal(resp, &chainInfo)
 		qt.Assert(t, err, qt.IsNil)
-		// check transaction count
-		resp, code = c.Request("GET", nil, "chain", "transactions", "count")
-		qt.Assert(t, code, qt.Equals, 200)
-		txsCount := new(struct {
-			Count uint64 `json:"count"`
-		})
-		err = json.Unmarshal(resp, txsCount)
-		qt.Assert(t, err, qt.IsNil)
-		qt.Assert(t, txsCount.Count, qt.Equals, chainInfo.TransactionCount)
 		if chainInfo.Height >= h {
 			break
 		}
@@ -1105,32 +1067,19 @@ func TestAPIElectionsList(t *testing.T) {
 	el := make(map[string]api.ElectionsList)
 	el["0"] = fetchEL("GET", nil, "", "elections")
 	el["1"] = fetchEL("GET", nil, "page=1", "elections")
-	el["p0"] = fetchEL("GET", nil, "", "elections", "page", "0")
-	el["p1"] = fetchEL("GET", nil, "", "elections", "page", "1")
 
 	qt.Assert(t, el["0"], qt.Not(qt.DeepEquals), el["1"])
-	qt.Assert(t, el["0"], qt.DeepEquals, el["p0"])
-	qt.Assert(t, el["1"], qt.DeepEquals, el["p1"])
 
 	qt.Assert(t, el["0"].Pagination.TotalItems, qt.Equals, uint64(20+1))
 	qt.Assert(t, el["1"].Pagination.TotalItems, qt.Equals, el["0"].Pagination.TotalItems)
-	qt.Assert(t, el["p0"].Pagination.TotalItems, qt.Equals, el["0"].Pagination.TotalItems)
-	qt.Assert(t, el["p1"].Pagination.TotalItems, qt.Equals, el["0"].Pagination.TotalItems)
 
 	for _, item := range el {
 		qt.Assert(t, len(item.Elections), qt.Equals, api.DefaultItemsPerPage)
 	}
 
-	boolPtr := func(b bool) *bool { return &b }
-
 	// this will exclude the 1 ongoing EncryptedVotes election
-	el["post_wr"] = fetchEL("POST", api.ElectionParams{
-		WithResults:      boolPtr(true),
-		PaginationParams: api.PaginationParams{Limit: 100},
-	}, "", "elections", "filter")
-	qt.Assert(t, el["post_wr"].Pagination.TotalItems, qt.Equals, el["0"].Pagination.TotalItems-1)
 	el["get_wr"] = fetchEL("GET", nil, "withResults=true&limit=100", "elections")
-	qt.Assert(t, el["post_wr"], qt.DeepEquals, el["get_wr"])
+	qt.Assert(t, el["get_wr"].Pagination.TotalItems, qt.Equals, el["0"].Pagination.TotalItems-1)
 
 	// this should return all elections, since no election has FinalResults or was ManuallyEnded
 	el["no_fr_me"] = fetchEL("GET", nil, "limit=100", "elections")
diff --git a/test/apierror_test.go b/test/apierror_test.go
index 659f8f4..e28ae09 100644
--- a/test/apierror_test.go
+++ b/test/apierror_test.go
@@ -51,22 +51,6 @@ func TestAPIerror(t *testing.T) {
 			args: args{"GET", nil, []string{"accounts", "0123456789012345678901234567890123456789"}},
 			want: api.ErrAccountNotFound,
 		},
-		{
-			args: args{"GET", nil, []string{"accounts", "0123456789012345678901234567890123456789", "elections", "count"}},
-			want: api.ErrOrgNotFound,
-		},
-		{
-			args: args{"GET", nil, []string{"accounts", "0123456789012345678901234567890123456789", "elections", "page", "0"}},
-			want: api.ErrOrgNotFound,
-		},
-		{
-			args: args{"GET", nil, []string{"accounts", "0123456789012345678901234567890123456789", "transfers", "page", "0"}},
-			want: api.ErrAccountNotFound,
-		},
-		{
-			args: args{"GET", nil, []string{"accounts", "0123456789012345678901234567890123456789", "fees", "page", "0"}},
-			want: api.ErrAccountNotFound,
-		},
 		{
 			args: args{"GET", nil, []string{"chain", "blocks", "1234"}},
 			want: api.ErrBlockNotFound,
@@ -107,27 +91,6 @@ func TestAPIerror(t *testing.T) {
 			}},
 			want: api.ErrCantParseElectionID,
 		},
-		{
-			args: args{"GET", nil, []string{
-				"accounts", "0123456789012345678901234567890123456789",
-				"elections",
-				"status", "ready",
-				"page", "-1",
-			}},
-			want: api.ErrPageNotFound,
-		},
-		{
-			args: args{"GET", nil, []string{"elections", "page", "thisIsTotallyNotAnInt"}},
-			want: api.ErrCantParseNumber,
-		},
-		{
-			args: args{"GET", nil, []string{"elections", "page", "-1"}},
-			want: api.ErrPageNotFound,
-		},
-		{
-			args: args{"GET", nil, []string{"elections", "0123456789012345678901234567890123456789", "votes", "page", "0"}},
-			want: api.ErrElectionNotFound,
-		},
 	}
 	for _, tt := range tests {
 		t.Run(tt.want.Error(), func(t *testing.T) {
diff --git a/test/createaccounts_test.go b/test/createaccounts_test.go
index 1fc94d8..376e5bf 100644
--- a/test/createaccounts_test.go
+++ b/test/createaccounts_test.go
@@ -30,17 +30,14 @@ func BenchmarkAPICreateNAccounts(b *testing.B) {
 
 	countAccts := func() uint64 {
 		// get accounts count
-		resp, code := c.Request("GET", nil, "accounts", "count")
+		resp, code := c.Request("GET", nil, "accounts")
 		qt.Assert(b, code, qt.Equals, 200, qt.Commentf("response: %s", resp))
 
-		countAccts := struct {
-			Count uint64 `json:"count"`
-		}{}
-
-		err := json.Unmarshal(resp, &countAccts)
+		accts := api.AccountsList{}
+		err := json.Unmarshal(resp, &accts)
 		qt.Assert(b, err, qt.IsNil)
 
-		return countAccts.Count
+		return accts.Pagination.TotalItems
 	}
 
 	// create a new account
```

Step 3: quick check.
Run: `tar -C $W --exclude=.git -cf - . | ssh z6.vocdoni.net "rm -rf ~/scratch/route-prune && mkdir -p ~/scratch/route-prune && tar -x -C ~/scratch/route-prune" && ssh z6.vocdoni.net 'cd ~/scratch/route-prune && export GOTOOLCHAIN=go1.26.5 && gofmt -l . && go build ./... && go vet ./apiclient/ ./test/ ./api/ ./cmd/... ./vocone/'`
Expected: no output.

Step 4: commit.
`git commit -m "refactor(apiclient): call the query-param and count-field routes that replace the removed ones" -- apiclient/client.go apiclient/account.go apiclient/blockchain.go apiclient/election.go test/api_test.go test/createaccounts_test.go test/apierror_test.go api/chain_organizations_test.go`

### Task 6: Integrate and verify
Model: opus   Repo: ~/src/vd/vocdoni-node   Files: whatever the checks below require
Goal: AC8 and AC9, plus the gate. Fix anything the checks find, and commit the fixes with a scoped message.

Step 1: sync the committed branch and set up a git index in the scratch, so `go generate` changes can be diffed.
```sh
git -C $W archive --format=tar HEAD | ssh z6.vocdoni.net "rm -rf ~/scratch/route-prune && mkdir -p ~/scratch/route-prune && tar -x -C ~/scratch/route-prune"
ssh z6.vocdoni.net 'cd ~/scratch/route-prune && git init -q && git add -A && git write-tree'
```

Step 2: CI-equivalent checks.
```sh
ssh z6.vocdoni.net 'bash -s' <<'EOF'
cd ~/scratch/route-prune && export GOTOOLCHAIN=go1.26.5
go mod tidy && git diff --exit-code go.mod go.sum && echo TIDY_OK
diff -u <(echo -n) <(go run mvdan.cc/gofumpt@v0.9.2 -d .) && echo GOFUMPT_OK
go generate ./... && git diff --exit-code && echo GENERATE_OK
go build ./... && go vet ./... && echo VET_OK
go run honnef.co/go/tools/cmd/staticcheck@2025.1 ./... && echo STATICCHECK_OK
go test -count=1 ./... 2>&1 | grep -Ev '^(ok|\?) ' ; echo "test exit=${PIPESTATUS[0]}"
EOF
```
Expected:
- `TIDY_OK`, `GOFUMPT_OK`, `GENERATE_OK`, `VET_OK`, `STATICCHECK_OK`.
- No non-`ok` test lines, and `test exit=0`: 36 `ok` packages, about 35 s.
- `go generate` (swag fmt v1.16.3) realigns the annotation columns of `walletAddHandler` and `censusProofHandler`. The `api/wallet.go` above is already formatted. If `GENERATE_OK` is missing because of `api/censuses.go`, copy the formatted file back, `scp z6.vocdoni.net:scratch/route-prune/api/censuses.go $W/api/censuses.go`, and commit it: `git commit -m "style(api): swag fmt annotations" -- api/censuses.go`.

Step 3: swagger job (CI `swagger.yml`, swag v1.16.6).
```sh
ssh z6.vocdoni.net 'bash -s' <<'EOF'
cd ~/scratch/route-prune && export GOTOOLCHAIN=go1.26.5
go run github.com/swaggo/swag/cmd/swag@v1.16.6 init --parseDependency --parseDepth 1 --parseInternal --md api/docs/descriptions --overridesFile api/docs/.swaggo -g api.go -d api/,api/docs/models/ -o api/docs --ot yaml >/dev/null 2>swag.err; echo "swag exit=$?"
grep -cE '/siks|/wallet/(bootstrap|transfer|election)|/page/\{page\}|export/ipfs|export/indexer|votes/count' api/docs/swagger.yaml
EOF
```
Expected:
- `swag exit=0`. `swag.err` only has the known mongo-driver and x/exp const warnings.
- The grep count is `0`. Removed routes are tombstones registered at runtime, not annotated handlers.

Step 4: AC9 grep, run on the laptop in `$W` (read-only).
```sh
cd $W && grep -rnE '"(count|circuit)"\)|"page", *(strconv|fmt|"[0-9])|"filter"|export/ipfs|import/\{ipfscid\}|export/indexer|"siks"|/siks|SIKHandler|walletFromToken|(votes|transactions|organizations|accounts)/count|/page/\{page\}|"wallet", *"(bootstrap|election|transfer)"|(^|[^.])\bErrSIKNotFound\b' api apiclient cmd test vocone benchmark 2>/dev/null | grep -vE '^api/removed(_test)?\.go:'; echo "exit=$?"
```
Expected: no matches and `exit=1`. The same grep finds 107 matches at base.
- `ErrGettingSIK` is deliberately not in the pattern: it stays (see Spec discrepancies).
- `state.ErrSIKNotFound` is excluded by the `[^.]` guard.

Step 5: tombstone parity with the sequencer.
```sh
cd $W && diff <(git show refs/sequencer/main:api/removed.go | sed 's#github.com/vocdoni/vochain-sequencer#go.vocdoni.io/dvote#') api/removed.go
```
Expected: exactly one changed line, the `reasonCensusIPFS` text:
```
< 	reasonCensusIPFS = "this node does not use IPFS, use GET /censuses/export and POST /censuses/import"
---
> 	reasonCensusIPFS = "the census IPFS routes are removed, use GET /censuses/export and POST /censuses/import"
```

Step 6: clean up `~/scratch/route-prune` on z6 when the gate passes: `ssh z6.vocdoni.net 'rm -rf ~/scratch/route-prune'`.

## Spec discrepancies
1. AC9 lists `ErrGettingSIK` and `ErrSIKNotFound` as names that must not appear, but both legitimately stay.
   - `api.ErrGettingSIK` (`api/errors.go`, code 5031) is used by `accountHandler` (`api/accounts.go:196` at base) to report failures reading the `sik` that Goal 6 keeps.
   - `state.ErrSIKNotFound` (`api/accounts.go:194`) is the state-layer sentinel.
   - Resolution: remove only `api.ErrSIKNotFound` (4054). Use the Task 6 AC9 grep, which drops `ErrGettingSIK` and excludes the qualified `state.ErrSIKNotFound`.
2. Design 3 lists `ExportBackupAsBytes` among the indexer functions to delete, but `service/indexer.go:60` uses it for the indexer backup. Design 3's own rule ("only if `go vet` and `grep` confirm it has no other users") keeps it. Resolution: keep it. Only `CountTokenTransfersByAccount` goes.
3. `dockerfiles/testsuite/start_test.sh:7,27,55,76,121-125` runs the `anonelection*` end2endtest operations. These live in `cmd/end2endtest/zkweighted.go`, which the spec deletes, and the spec does not mention the script, so the CI compose job would fail. Resolution: Task 3 removes the suite and `ELECTION_SIZE_ANON`.
4. The spec's apiclient inventory (Design 10, `apiclient/account.go:279,426,446,465`, `blockchain.go:122-133,151`) is incomplete:
   - It omits `TransactionByHash` (`apiclient/blockchain.go:103-117`), which calls the removed `/chain/transactions/{height}/{index}`.
   - It calls `TransactionCount` "CountTransactions".
   - It does not name `CountTokenTransfers` (`apiclient/account.go:465`).
   - The sequencer's own `ListAccounts` and `TransactionCount` still call tombstoned routes, so "follow the sequencer" cannot be literal there.
   - Resolution: Task 5 migrates all of them, keeping their signatures.
5. Design 9's test inventory misses these callers of pruned routes:
   - `test/createaccounts_test.go:33` (`/accounts/count`)
   - `test/api_test.go:1130` (`POST /elections/filter`), plus `:263`, `:278`, `:338-339`, `:471-512`, `:590`, `:676` and `:1108-1109`
   - `api/chain_organizations_test.go:127-170` (legacy by-page and POST filter)
   - `test/apierror_test.go:54-69,109-130`

   Design 6 also misses that `ElectionDescription.TempSIKs` is read by `apiclient/election.go:202`. Resolution: Tasks 3 and 5 update them all.
6. The sequencer's `api/removed_test.go` depends on `newMetadataTestEnv`/`assertAPIError`, which do not exist here. Resolution: Task 2 adds `newFullTestAPI` (every gateway handler group on a test app and indexer) and `assertAPIError`.
7. The sequencer folds `/wallet/add` into `CensusHandler`. Here `WalletHandler` stays as its own group, because `cmd/node/main.go`, `vocone` and tests enable it by name. It registers only `POST /wallet/add/{privateKey}`.
8. `api/docs/descriptions/electionListByFilterHandler.md` is orphaned once `electionListByFilterHandler` goes, and the spec does not list it. Resolution: Task 4 deletes it.

## Merge and release
1. Full suite on z6: Task 6 Steps 1–5 against the final branch head. Expected: all `*_OK` markers, `test exit=0` with 36 `ok` packages, `swag exit=0`, AC9 `exit=1`, and the one-line `removed.go` diff.
2. **Owner OK required:** `git -C $W push -u origin feat/api-route-prune`.
3. **Owner OK required:** open the PR against `main` (a normal PR, not stacked): `gh pr create --base main --head feat/api-route-prune --title "refactor(api): prune removed routes to vochain-sequencer parity" --body-file <body>`. The body is about 15 lines. It must state the rollout order from the spec:
   - #1501 (merged as `8a9b7b4b`);
   - then explorer-ng #39 and saas-backend #778 deployed;
   - then this PR on the LTS gateways.
4. Release: none. No tag, no version bump.
5. Post-merge checks, on a gateway running the merged image:
   - `curl -s -o /dev/null -w '%{http_code}' -X POST https://<gw>/v2/wallet/add/<fresh-key>` → `200` with a `Deprecation` header.
   - `GET /v2/accounts/count` → `410`.
   - The node log contains `purged N custodial wallet keys` once, or nothing on a node that never stored any.

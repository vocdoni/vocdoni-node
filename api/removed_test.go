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

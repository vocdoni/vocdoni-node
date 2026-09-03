package api

import (
	"encoding/hex"
	"encoding/json"
	"net/url"
	"path"
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
	"go.vocdoni.io/dvote/util"
	"go.vocdoni.io/dvote/vochain"
	"go.vocdoni.io/dvote/vochain/indexer"
	"go.vocdoni.io/dvote/vochain/state"
	"go.vocdoni.io/proto/build/go/models"
)

// TestOrganizationListSorting exercises the sortBy/order params of
// GET /chain/organizations end to end, over a small indexed fixture.
func TestOrganizationListSorting(t *testing.T) {
	c := qt.New(t)

	router := httprouter.HTTProuter{}
	router.Init("127.0.0.1", 0)
	addr, err := url.Parse("http://" + path.Join(router.Address().String(), "chain"))
	c.Assert(err, qt.IsNil)

	api, err := NewAPI(&router, "/", t.TempDir(), db.TypePebble)
	c.Assert(err, qt.IsNil)
	kv, err := metadb.New(db.TypePebble, t.TempDir())
	c.Assert(err, qt.IsNil)
	app := vochain.TestBaseApplication(t)
	idx, err := indexer.New(app, indexer.Options{DataDir: t.TempDir()})
	c.Assert(err, qt.IsNil)
	api.Attach(app, nil, idx, ipfs.MockIPFS(t), censusdb.NewCensusDB(kv))
	c.Assert(api.EnableHandlers(ChainHandler), qt.IsNil)

	// Three organizations with a distinct number of elections each, created
	// oldest to newest. A process is indexed with the timestamp of the block
	// before the one committing it, hence two blocks per organization.
	orgs := map[string][]byte{
		"quorum": util.RandomBytes(20),
		"abacus": util.RandomBytes(20),
		"zenith": util.RandomBytes(20),
	}
	electionCounts := map[string]int{"quorum": 1, "abacus": 3, "zenith": 2}
	for _, name := range []string{"quorum", "abacus", "zenith"} {
		eid := orgs[name]
		for i := 0; i < electionCounts[name]; i++ {
			c.Assert(app.State.AddProcess(&models.Process{
				ProcessId:     util.RandomBytes(32),
				EntityId:      eid,
				EnvelopeType:  &models.EnvelopeType{},
				Status:        models.ProcessStatus_READY,
				Mode:          &models.ProcessMode{AutoStart: true},
				BlockCount:    100,
				MaxCensusSize: 10,
				VoteOptions:   &models.ProcessVoteOptions{MaxCount: 1, MaxValue: 1},
			}), qt.IsNil)
		}
		idx.OnSetAccount(eid, &state.Account{})
		app.AdvanceTestBlock()
		app.AdvanceTestBlock()
		c.Assert(idx.SetAccountMetadata(eid, name, ""), qt.IsNil)
	}

	token := uuid.New()
	cl := testutil.NewTestHTTPclient(t, addr, &token)

	list := func(query string) []string {
		resp, code := cl.RequestWithQuery("GET", nil, query, "organizations")
		c.Assert(code, qt.Equals, apirest.HTTPstatusOK, qt.Commentf("query %q: %s", query, resp))
		result := &OrganizationsList{}
		c.Assert(json.Unmarshal(resp, result), qt.IsNil)
		byID := map[string]string{}
		for name, eid := range orgs {
			byID[hex.EncodeToString(eid)] = name
		}
		names := []string{}
		for _, org := range result.Organizations {
			names = append(names, byID[org.OrganizationID.String()])
		}
		return names
	}

	// Ranking by election count, which is what a "top organizations" view needs:
	// one request, no client-side sweep of every page.
	c.Assert(list("sortBy=electionCount&order=desc"), qt.DeepEquals, []string{"abacus", "zenith", "quorum"})
	c.Assert(list("sortBy=electionCount&order=asc"), qt.DeepEquals, []string{"quorum", "zenith", "abacus"})
	// order defaults to the natural direction of each sortBy.
	c.Assert(list("sortBy=electionCount"), qt.DeepEquals, []string{"abacus", "zenith", "quorum"})
	c.Assert(list("sortBy=name"), qt.DeepEquals, []string{"abacus", "quorum", "zenith"})
	c.Assert(list("sortBy=name&order=desc"), qt.DeepEquals, []string{"zenith", "quorum", "abacus"})
	c.Assert(list("sortBy=createdAt"), qt.DeepEquals, []string{"zenith", "abacus", "quorum"})
	c.Assert(list("sortBy=createdAt&order=asc"), qt.DeepEquals, []string{"quorum", "abacus", "zenith"})
	// No sortBy at all keeps the ordering the endpoint had before it took one.
	c.Assert(list(""), qt.DeepEquals, []string{"zenith", "abacus", "quorum"})

	// Sorting composes with paging and with the existing filters.
	c.Assert(list("sortBy=electionCount&order=desc&limit=1"), qt.DeepEquals, []string{"abacus"})
	c.Assert(list("sortBy=electionCount&order=desc&limit=1&page=1"), qt.DeepEquals, []string{"zenith"})
	c.Assert(list("sortBy=electionCount&order=desc&limit=1&page=2"), qt.DeepEquals, []string{"quorum"})
	c.Assert(list("sortBy=name&order=asc&name=quo"), qt.DeepEquals, []string{"quorum"})

	// Unsupported values are a 400, not silently ignored.
	for query, wantErr := range map[string]apirest.APIerror{
		"sortBy=electioncount":              ErrParamSortByInvalid,
		"sortBy=votes":                      ErrParamSortByInvalid,
		"order=sideways":                    ErrParamOrderInvalid,
		"sortBy=electionCount&order=DESC":   ErrParamOrderInvalid,
		"sortBy=electionCount&order=lowest": ErrParamOrderInvalid,
	} {
		resp, code := cl.RequestWithQuery("GET", nil, query, "organizations")
		c.Assert(code, qt.Equals, wantErr.HTTPstatus, qt.Commentf("query %q: %s", query, resp))
		apiErr := &apirest.APIerror{}
		c.Assert(json.Unmarshal(resp, apiErr), qt.IsNil)
		c.Assert(apiErr.Code, qt.Equals, wantErr.Code, qt.Commentf("query %q: %s", query, resp))
	}

	// The deprecated by-page endpoint keeps working, with the default ordering.
	resp, code := cl.Request("GET", nil, "organizations", "page", "0")
	c.Assert(code, qt.Equals, apirest.HTTPstatusOK)
	legacy := &OrganizationsList{}
	c.Assert(json.Unmarshal(resp, legacy), qt.IsNil)
	c.Assert(legacy.Organizations, qt.HasLen, 3)

	// The deprecated POST filter endpoint takes its OrganizationParams straight
	// from the request body, so it reaches the ordering without going through
	// parseOrganizationParams. It must sort, and reject, just the same.
	postList := func(body *OrganizationParams) []string {
		resp, code := cl.Request("POST", body, "organizations", "filter", "page", "0")
		c.Assert(code, qt.Equals, apirest.HTTPstatusOK, qt.Commentf("body %+v: %s", body, resp))
		result := &OrganizationsList{}
		c.Assert(json.Unmarshal(resp, result), qt.IsNil)
		byID := map[string]string{}
		for name, eid := range orgs {
			byID[hex.EncodeToString(eid)] = name
		}
		names := []string{}
		for _, org := range result.Organizations {
			names = append(names, byID[org.OrganizationID.String()])
		}
		return names
	}
	c.Assert(postList(&OrganizationParams{SortBy: "electionCount", Order: "desc"}),
		qt.DeepEquals, []string{"abacus", "zenith", "quorum"})
	c.Assert(postList(&OrganizationParams{}), qt.DeepEquals, []string{"zenith", "abacus", "quorum"})

	for _, tc := range []struct {
		body    *OrganizationParams
		wantErr apirest.APIerror
	}{
		{&OrganizationParams{SortBy: "votes"}, ErrParamSortByInvalid},
		{&OrganizationParams{Order: "sideways"}, ErrParamOrderInvalid},
	} {
		resp, code := cl.Request("POST", tc.body, "organizations", "filter", "page", "0")
		c.Assert(code, qt.Equals, tc.wantErr.HTTPstatus, qt.Commentf("body %+v: %s", tc.body, resp))
		apiErr := &apirest.APIerror{}
		c.Assert(json.Unmarshal(resp, apiErr), qt.IsNil)
		c.Assert(apiErr.Code, qt.Equals, tc.wantErr.Code, qt.Commentf("body %+v: %s", tc.body, resp))
	}
}

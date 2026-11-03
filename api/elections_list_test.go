package api

import (
	"encoding/hex"
	"encoding/json"
	"net/url"
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
	"go.vocdoni.io/dvote/vochain/vochaininfo"
	"go.vocdoni.io/proto/build/go/models"
)

// TestElectionListSorting exercises the title filter and the sortBy/order
// params of GET /elections end to end, over a small indexed fixture.
func TestElectionListSorting(t *testing.T) {
	c := qt.New(t)

	router := httprouter.HTTProuter{}
	router.Init("127.0.0.1", 0)
	// The trailing slash matters: the test client joins request paths onto this
	// URL's path, and an empty one would yield a request line without a leading
	// slash, which the server drops.
	addr, err := url.Parse("http://" + router.Address().String() + "/")
	c.Assert(err, qt.IsNil)

	api, err := NewAPI(&router, "/", t.TempDir(), db.TypePebble)
	c.Assert(err, qt.IsNil)
	kv, err := metadb.New(db.TypePebble, t.TempDir())
	c.Assert(err, qt.IsNil)
	app := vochain.TestBaseApplication(t)
	idx, err := indexer.New(app, indexer.Options{DataDir: t.TempDir()})
	c.Assert(err, qt.IsNil)
	api.Attach(app, vochaininfo.NewVochainInfo(app), idx, ipfs.MockIPFS(t), censusdb.NewCensusDB(kv))
	c.Assert(api.EnableHandlers(ElectionHandler), qt.IsNil)

	// Three elections created oldest to newest (two blocks apart, as a process
	// is indexed with the timestamp of the block before the one committing it),
	// with start dates, vote counts and titles in three different orders.
	eid := util.RandomBytes(20)
	type fixture struct {
		name      string
		startTime uint32
		votes     int
		title     string
	}
	fixtures := []fixture{
		{"old", 3000, 1, "Budget 2026"},
		{"mid", 1000, 4, "board election"},
		{"new", 2000, 0, "Annual budget review"},
	}
	names := map[string]string{}
	pids := map[string][]byte{}
	for _, f := range fixtures {
		pid := util.RandomBytes(32)
		c.Assert(app.State.AddProcess(&models.Process{
			ProcessId:     pid,
			EntityId:      eid,
			StartTime:     f.startTime,
			Duration:      1000,
			EnvelopeType:  &models.EnvelopeType{},
			Status:        models.ProcessStatus_READY,
			Mode:          &models.ProcessMode{AutoStart: true},
			BlockCount:    100,
			MaxCensusSize: 10,
			VoteOptions:   &models.ProcessVoteOptions{MaxCount: 1, MaxValue: 1},
		}), qt.IsNil)
		idx.OnSetAccount(eid, &state.Account{})
		names[hex.EncodeToString(pid)] = f.name
		pids[f.name] = pid
		app.AdvanceTestBlock()
		app.AdvanceTestBlock()
	}
	vp, err := state.NewVotePackage([]int{1}).Encode()
	c.Assert(err, qt.IsNil)
	for _, f := range fixtures {
		for range f.votes {
			v := &state.Vote{ProcessID: pids[f.name], VotePackage: vp, Nullifier: util.RandomBytes(32)}
			c.Assert(app.State.AddVote(v), qt.IsNil)
		}
	}
	app.AdvanceTestBlock()
	for _, f := range fixtures {
		c.Assert(idx.SetProcessMetadataTitle(pids[f.name], f.title), qt.IsNil)
	}

	token := uuid.New()
	cl := testutil.NewTestHTTPclient(t, addr, &token)

	namesOf := func(resp []byte) []string {
		result := &ElectionsList{}
		c.Assert(json.Unmarshal(resp, result), qt.IsNil)
		out := []string{}
		for _, e := range result.Elections {
			out = append(out, names[e.ElectionID.String()])
		}
		return out
	}
	list := func(query string) []string {
		resp, code := cl.RequestWithQuery("GET", nil, query, "elections")
		c.Assert(code, qt.Equals, apirest.HTTPstatusOK, qt.Commentf("query %q: %s", query, resp))
		return namesOf(resp)
	}

	// No sortBy at all keeps the ordering the endpoint had before it took one.
	c.Assert(list(""), qt.DeepEquals, []string{"new", "mid", "old"})
	c.Assert(list("sortBy=createdAt&order=asc"), qt.DeepEquals, []string{"old", "mid", "new"})
	c.Assert(list("sortBy=startDate"), qt.DeepEquals, []string{"old", "new", "mid"})
	c.Assert(list("sortBy=voteCount"), qt.DeepEquals, []string{"mid", "old", "new"})
	c.Assert(list("sortBy=title"), qt.DeepEquals, []string{"new", "mid", "old"}) // case-insensitive
	c.Assert(list("sortBy=title&order=desc"), qt.DeepEquals, []string{"old", "mid", "new"})

	// The title filter is a case-insensitive substring, and composes with the
	// ordering and with paging.
	c.Assert(list("title=BUDGET&sortBy=voteCount"), qt.DeepEquals, []string{"old", "new"})
	c.Assert(list("title=budget&sortBy=voteCount&limit=1&page=1"), qt.DeepEquals, []string{"new"})
	c.Assert(list("title=nothing-matches"), qt.HasLen, 0)

	// Unsupported values are a 400, not silently ignored.
	for query, wantErr := range map[string]apirest.APIerror{
		"sortBy=votes":                  ErrParamSortByInvalid,
		"sortBy=votecount":              ErrParamSortByInvalid,
		"order=sideways":                ErrParamOrderInvalid,
		"sortBy=voteCount&order=DESC":   ErrParamOrderInvalid,
		"sortBy=__unsupported__&page=0": ErrParamSortByInvalid,
	} {
		resp, code := cl.RequestWithQuery("GET", nil, query, "elections")
		c.Assert(code, qt.Equals, wantErr.HTTPstatus, qt.Commentf("query %q: %s", query, resp))
		apiErr := &apirest.APIerror{}
		c.Assert(json.Unmarshal(resp, apiErr), qt.IsNil)
		c.Assert(apiErr.Code, qt.Equals, wantErr.Code, qt.Commentf("query %q: %s", query, resp))
	}

	// The deprecated POST filter endpoint takes its ElectionParams straight from
	// the request body, so it reaches the ordering without going through
	// electionParams. It must filter, sort and reject just the same.
	resp, code := cl.Request("POST", &ElectionParams{Title: "budget", SortBy: "voteCount"}, "elections", "filter", "page", "0")
	c.Assert(code, qt.Equals, apirest.HTTPstatusOK, qt.Commentf("%s", resp))
	c.Assert(namesOf(resp), qt.DeepEquals, []string{"old", "new"})
	resp, code = cl.Request("POST", &ElectionParams{SortBy: "votes"}, "elections", "filter", "page", "0")
	c.Assert(code, qt.Equals, ErrParamSortByInvalid.HTTPstatus, qt.Commentf("%s", resp))
}

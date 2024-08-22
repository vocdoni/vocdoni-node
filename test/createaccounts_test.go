package test

import (
	"encoding/json"
	"testing"

	qt "github.com/frankban/quicktest"
	"github.com/google/uuid"
	"go.vocdoni.io/dvote/api"
	"go.vocdoni.io/dvote/test/testcommon"
	"go.vocdoni.io/dvote/test/testcommon/testutil"
)

func BenchmarkAPICreateNAccounts(b *testing.B) {
	server := testcommon.APIserver{}
	server.Start(b,
		api.ChainHandler,
		api.CensusHandler,
		api.VoteHandler,
		api.AccountHandler,
		api.ElectionHandler,
		api.WalletHandler,
	)
	token1 := uuid.New()
	c := testutil.NewTestHTTPclient(b, server.ListenAddr, &token1)

	// Block 1
	server.VochainAPP.AdvanceTestBlock()
	waitUntilHeight(b, c, 1)

	countAccts := func() uint64 {
		// get accounts count
		resp, code := c.Request("GET", nil, "accounts", "count")
		qt.Assert(b, code, qt.Equals, 200, qt.Commentf("response: %s", resp))

		countAccts := struct {
			Count uint64 `json:"count"`
		}{}

		err := json.Unmarshal(resp, &countAccts)
		qt.Assert(b, err, qt.IsNil)

		return countAccts.Count
	}

	// create a new account
	initBalance := uint64(80)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = createAccount(b, c, server, initBalance)
	}
	// Block 2
	server.VochainAPP.AdvanceTestBlock()
	waitUntilHeight(b, c, 2)

	if count := countAccts(); count < uint64(b.N) {
		qt.Assert(b, count, qt.Equals, b.N)
	}
}

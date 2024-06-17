package test

import (
	"bytes"
	"encoding/json"
	"testing"

	qt "github.com/frankban/quicktest"
	"github.com/google/uuid"
	"go.vocdoni.io/dvote/api"
	"go.vocdoni.io/dvote/httprouter/apirest"
	"go.vocdoni.io/dvote/test/testcommon"
	"go.vocdoni.io/dvote/test/testcommon/testutil"
)

func TestAPIerror(t *testing.T) {
	server := testcommon.APIserver{}
	server.Start(t,
		api.ChainHandler,
		api.CensusHandler,
		api.VoteHandler,
		api.AccountHandler,
		api.ElectionHandler,
		api.WalletHandler,
	)
	// Block 1
	server.VochainAPP.AdvanceTestBlock()

	token1 := uuid.New()
	c := testutil.NewTestHTTPclient(t, server.ListenAddr, &token1)

	hugeFile := &apirest.APIdata{
		Data: bytes.Repeat([]byte("0"), api.MaxOffchainFileSize+1),
	}

	type args struct {
		method   string
		jsonBody any
		urlPath  []string
	}
	tests := []struct {
		name string
		args args
		want apirest.APIerror
	}{
		{
			args: args{"GET", nil, []string{"accounts", "0123456789"}},
			want: api.ErrAddressMalformed,
		},
		{
			args: args{"GET", nil, []string{"accounts", "0123456789012345678901234567890123456789"}},
			want: api.ErrAccountNotFound,
		},
		{
			args: args{"GET", nil, []string{"accounts", "0123456789012345678901234567890123456789", "elections", "count"}},
			want: api.ErrOrgNotFound,
		},
		{
			args: args{"GET", nil, []string{"chain", "blocks", "1234"}},
			want: api.ErrBlockNotFound,
		},
		{
			args: args{"POST", hugeFile, []string{"files", "cid"}},
			want: api.ErrFileSizeTooBig,
		},
		{
			args: args{"GET", nil, []string{"accounts", "totallyWrong!@#$", "elections", "status", "ready", "page", "0"}},
			want: api.ErrCantParseOrgID,
		},
		{
			args: args{"GET", nil, []string{"accounts", "totallyWrong!@#$", "transfers", "page", "0"}},
			want: api.ErrCantParseAccountID,
		},
		{
			args: args{"GET", nil, []string{
				"votes", "verify",
				"0123456789012345678901234567890123456789012345678901234567890123",
				"000",
			}},
			want: api.ErrCantParseVoteID,
		},
		{
			args: args{"GET", nil, []string{
				"votes", "verify",
				"0123456789012345678901234567890123456789012345678901234567890123",
				"0000",
			}},
			want: api.ErrVoteIDMalformed,
		},
		{
			args: args{"GET", nil, []string{
				"votes", "verify",
				"0123456789012345678901234567890123456789012345678901234567890123",
				"0123456789012345678901234567890123456789012345678901234567890123",
			}},
			want: api.ErrVoteNotFound,
		},
		{
			args: args{"GET", nil, []string{
				"votes", "verify",
				"bbbbb",
				"0123456789012345678901234567890123456789012345678901234567890123",
			}},
			want: api.ErrCantParseElectionID,
		},
		{
			args: args{"GET", nil, []string{
				"accounts", "0123456789012345678901234567890123456789",
				"elections",
				"status", "ready",
				"page", "-1",
			}},
			want: api.ErrCantFetchElectionList,
		},
		{
			args: args{"GET", nil, []string{"elections", "page", "thisIsTotallyNotAnInt"}},
			want: api.ErrCantParsePageNumber,
		},
	}
	for _, tt := range tests {
		t.Run(tt.want.Error(), func(t *testing.T) {
			resp, code := c.Request(tt.args.method, tt.args.jsonBody, tt.args.urlPath...)
			t.Logf("httpstatus=%d body=%s", code, resp)
			qt.Assert(t, code, qt.Equals, tt.want.HTTPstatus)
			apierr := &apirest.APIerror{}
			qt.Assert(t, json.Unmarshal(resp, apierr), qt.IsNil)
			qt.Assert(t, apierr.Code, qt.Equals, tt.want.Code)
		})
	}
}

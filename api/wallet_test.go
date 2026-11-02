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

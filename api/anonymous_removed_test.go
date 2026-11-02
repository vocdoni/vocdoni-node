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

package test

/*
This test starts the following services

1. Starts the Proxy
2. Starts the IPFS storage
3. Starts the Dvote API router
4. Starts the Census Manager

Then it creates two pairs of signing keys

sign1: as the signer for the API server
sign2: as the signer for the API client

Sign2 address is added as "allowedAddress" for the API router.

A WebSockets client is created to make the API calls.

Then the following census operations are tested:

1. addCensus, getRoot, addClaim (to check basic operation)
2. addClaimBulk to add 100 claims to the census merkle tree
3. publish to export and publish the census to IPFS
4. importRemote to import the IPFS exported census to a new census
5. check that the new census has the same rootHash of the original one

Run it executing `go test -v test/census_test.go`
*/

import (
	"encoding/hex"
	"flag"
	"math/rand"
	"testing"
	"time"

	ethcommon "github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
	qt "github.com/frankban/quicktest"

	"go.vocdoni.io/dvote/api"
	"go.vocdoni.io/dvote/client"
	"go.vocdoni.io/dvote/crypto/ethereum"
	models "go.vocdoni.io/proto/build/go/models"

	"go.vocdoni.io/dvote/test/testcommon"
)

var censusSize = flag.Int("censusSize", 100, "number of claims to add in the census")

func init() { rand.Seed(time.Now().UnixNano()) }

func TestCensus(t *testing.T) {
	t.Parallel()

	var server testcommon.DvoteAPIServer
	server.Start(t, "file", "census")

	signer1 := ethereum.NewSignKeys()
	signer1.Authorized = make(map[ethcommon.Address]bool)
	signer1.Generate()
	signer2 := ethereum.NewSignKeys()
	signer2.Authorized = make(map[ethcommon.Address]bool)
	signer2.Generate()
	server.Signer.AddAuthKey(signer2.Address())
	t.Logf("added authorized address %s", signer2.AddressString())

	// Create websocket client
	t.Logf("connecting to %s", server.ListenAddr)
	cl, err := client.New(server.ListenAddr)
	qt.Assert(t, err, qt.IsNil)

	// Send the API requets
	var req api.APIrequest
	doRequest := cl.ForTest(t, &req)

	// Create census
	req.CensusID = "test"
	req.CensusType = models.Census_ARBO_BLAKE2B
	resp := doRequest("addCensus", signer2)
	qt.Assert(t, err, qt.IsNil)
	censusID := resp.CensusID

	// addClaim
	req.CensusID = censusID
	req.CensusKey = []byte("hello")
	req.Digested = true
	resp = doRequest("addClaim", signer2)
	if !resp.Ok {
		t.Logf("%s", resp.Message)
	}
	qt.Assert(t, resp.Ok, qt.IsTrue)

	// addClaim not authorized; use Request directly
	req.CensusID = censusID
	req.Method = "addClaim"
	req.CensusKey = []byte("hello2")
	resp, err = cl.Request(req, signer1)
	qt.Assert(t, err, qt.IsNil)
	qt.Assert(t, resp.Ok, qt.IsFalse)

	// GenProof valid
	req.CensusID = censusID
	req.CensusKey = []byte("hello")
	resp = doRequest("genProof", nil)
	if !resp.Ok {
		t.Logf("%s", resp.Message)
	}
	qt.Assert(t, resp.Ok, qt.IsTrue)

	// GenProof not valid
	req.CensusID = censusID
	req.CensusKey = []byte("hello3")
	resp = doRequest("genProof", nil)
	qt.Assert(t, resp.Siblings, qt.HasLen, 0)

	// getRoot
	resp = doRequest("getRoot", nil)
	root := resp.Root
	qt.Assert(t, root, qt.Not(qt.HasLen), 0)

	// Create census2
	req.CensusID = "test2"
	resp = doRequest("addCensus", signer2)
	qt.Assert(t, resp.Ok, qt.IsTrue)
	censusID = resp.CensusID
	req.CensusID = censusID

	// addClaimBulk
	var claims [][]byte
	req.CensusKey = []byte{}
	req.Digested = false
	keys := testcommon.CreateEthRandomKeysBatch(t, *censusSize)
	for _, key := range keys {
		claim := crypto.FromECDSAPub(&key.Public)
		qt.Assert(t, claim, qt.Not(qt.HasLen), 0)
		claims = append(claims, claim)
	}
	req.CensusKeys = claims
	resp = doRequest("addClaimBulk", signer2)
	qt.Assert(t, resp.Ok, qt.IsTrue)

	// GenProof valid
	req.RootHash = nil
	req.CensusKey = claims[1]
	resp = doRequest("genProof", nil)
	siblings := resp.Siblings
	qt.Assert(t, siblings, qt.Not(qt.HasLen), 0)

	// CheckProof valid
	req.ProofData = siblings
	resp = doRequest("checkProof", nil)
	qt.Assert(t, *resp.ValidProof, qt.IsTrue)

	// CheckProof invalid (old root)
	req.ProofData = siblings
	req.RootHash = root
	resp = doRequest("checkProof", nil)
	qt.Assert(t, resp.Ok, qt.IsTrue)
	qt.Assert(t, *resp.ValidProof, qt.IsFalse)
	req.RootHash = nil

	// publish
	req.CensusKeys = [][]byte{}
	resp = doRequest("publish", signer2)
	qt.Assert(t, resp.Ok, qt.IsTrue)
	uri := resp.URI

	// getRoot
	resp = doRequest("getRoot", nil)
	root = resp.Root
	qt.Assert(t, root, qt.Not(qt.HasLen), 0)

	// getRoot from published census and check censusID=root
	req.CensusID = hex.EncodeToString(root)
	resp = doRequest("getRoot", nil)
	qt.Assert(t, resp.Ok, qt.IsTrue)
	qt.Assert(t, resp.Root, qt.DeepEquals, root)

	// add second census
	req.CensusID = "importTest"
	resp = doRequest("addCensus", signer2)
	qt.Assert(t, resp.Ok, qt.IsTrue)

	// importRemote
	req.CensusID = resp.CensusID
	req.URI = uri
	resp = doRequest("importRemote", signer2)
	qt.Assert(t, resp.Ok, qt.IsTrue)

	// getRoot
	resp = doRequest("getRoot", nil)
	qt.Assert(t, resp.Root, qt.DeepEquals, root)

	// getSize
	req.RootHash = nil
	resp = doRequest("getSize", nil)
	qt.Assert(t, *resp.Size, qt.Equals, int64(*censusSize))

	// get census list
	resp = doRequest("getCensusList", signer2)
	qt.Assert(t, resp.CensusList, qt.HasLen, 4)
}

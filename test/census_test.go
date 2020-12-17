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
	"bytes"
	"flag"
	"math/rand"
	"testing"
	"time"

	ethcommon "github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"

	"gitlab.com/vocdoni/go-dvote/client"
	"gitlab.com/vocdoni/go-dvote/crypto/ethereum"
	"gitlab.com/vocdoni/go-dvote/crypto/snarks"
	"gitlab.com/vocdoni/go-dvote/types"

	"gitlab.com/vocdoni/go-dvote/test/testcommon"
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
	t.Logf("connecting to %s", server.PxyAddr)
	cl, err := client.New(server.PxyAddr)
	if err != nil {
		t.Fatal(err)
	}

	// Send the API requets
	var req types.MetaRequest
	doRequest := cl.ForTest(t, &req)

	// Create census
	req.CensusID = "test"
	resp := doRequest("addCensus", signer2)
	if !resp.Ok {
		t.Fatalf("%s failed", req.Method)
	}
	censusID := resp.CensusID

	// addClaim
	req.CensusID = censusID
	req.ClaimData = []byte("hello")
	req.Digested = true
	resp = doRequest("addClaim", signer2)
	if !resp.Ok {
		t.Fatalf("%s failed", req.Method)
	}

	// addClaim not authorized; use Request directly
	req.CensusID = censusID
	req.Method = "addClaim"
	req.ClaimData = []byte("hello2")
	resp, err = cl.Request(req, signer1)
	if err != nil {
		t.Fatal(err)
	}
	if resp.Ok {
		t.Fatalf("%s didn't fail", req.Method)
	}

	// GenProof valid
	req.CensusID = censusID
	req.ClaimData = []byte("hello")
	resp = doRequest("genProof", nil)
	if !resp.Ok {
		t.Fatalf("%s failed", req.Method)
	}

	// GenProof not valid
	req.CensusID = censusID
	req.ClaimData = []byte("hello3")
	resp = doRequest("genProof", nil)
	if len(resp.Siblings) > 1 {
		t.Fatalf("proof should not exist!")
	}

	// getRoot
	resp = doRequest("getRoot", nil)
	root := resp.Root
	if len(root) < 1 {
		t.Fatalf("got invalid root")
	}

	// Create census2
	req.CensusID = "test2"
	resp = doRequest("addCensus", signer2)
	if !resp.Ok {
		t.Fatalf("%s failed", req.Method)
	}
	censusID = resp.CensusID
	req.CensusID = censusID

	// addClaimBulk
	var claims [][]byte
	req.ClaimData = []byte{}
	keys := testcommon.CreateEthRandomKeysBatch(t, *censusSize)
	for _, key := range keys {
		hash := snarks.Poseidon.Hash(crypto.FromECDSAPub(&key.Public))
		if len(hash) == 0 {
			t.Fatalf("cannot create poseidon hash of public key: %#v", key.Public)
		}
		claims = append(claims, hash)
	}
	req.ClaimsData = claims
	resp = doRequest("addClaimBulk", signer2)
	if !resp.Ok {
		t.Fatalf("%s failed", req.Method)
	}

	// dumpPlain
	req.ClaimData = []byte{}
	req.ClaimsData = [][]byte{}
	resp = doRequest("dumpPlain", signer2)
	if !resp.Ok {
		t.Fatalf("%s failed", req.Method)
	}
	var found bool
	for _, c := range claims {
		found = false
		for _, c2 := range resp.ClaimsData {
			if bytes.Equal(c, c2) {
				found = true
				break
			}
		}
		if !found {
			t.Fatalf("claim not found: %s", c)
		}
	}

	// GenProof valid
	req.RootHash = ""
	req.ClaimData = claims[1]
	resp = doRequest("genProof", nil)
	siblings := resp.Siblings
	if len(siblings) == 0 {
		t.Fatalf("proof not generated while it should be generated correctly")
	}

	// CheckProof valid
	req.ProofData = siblings
	resp = doRequest("checkProof", nil)
	if !*resp.ValidProof {
		t.Fatal("proof is invalid but it should be valid")
	}

	// CheckProof invalid (old root)
	req.ProofData = siblings
	req.RootHash = root
	resp = doRequest("checkProof", nil)
	if !resp.Ok {
		t.Fatalf("%s failed", req.Method)
	}
	if *resp.ValidProof {
		t.Fatalf("proof must be invalid for hash %s but it is valid!", root)
	}
	req.RootHash = ""

	// publish
	req.ClaimsData = [][]byte{}
	resp = doRequest("publish", signer2)
	if !resp.Ok {
		t.Fatalf("%s failed", req.Method)
	}
	uri := resp.URI

	// getRoot
	resp = doRequest("getRoot", nil)
	root = resp.Root
	if len(root) < 1 {
		t.Fatalf("got invalid root")
	}

	// getRoot from published census and check censusID=root
	req.CensusID = root
	resp = doRequest("getRoot", nil)
	if !resp.Ok {
		t.Fatalf("%s failed", req.Method)
	}
	if root != resp.Root {
		t.Fatalf("got invalid root from published census")
	}

	// add second census
	req.CensusID = "importTest"
	resp = doRequest("addCensus", signer2)
	if !resp.Ok {
		t.Fatalf("%s failed", req.Method)
	}

	// importRemote
	req.CensusID = resp.CensusID
	req.URI = uri
	resp = doRequest("importRemote", signer2)
	if !resp.Ok {
		t.Fatalf("%s failed", req.Method)
	}

	// getRoot
	resp = doRequest("getRoot", nil)
	if root != resp.Root {
		t.Fatalf("root is different after importing! %s != %s", root, resp.Root)
	}

	// getSize
	req.RootHash = ""
	resp = doRequest("getSize", nil)
	if exp, got := int64(*censusSize), *resp.Size; exp != got {
		t.Fatalf("expected size %v, got %v", exp, got)
	}

	// get census list
	resp = doRequest("getCensusList", signer2)
	if len(resp.CensusList) != 4 {
		t.Fatalf("census list size does not match")
	}
	t.Logf("census list: %v", resp.CensusList)
}

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
	"encoding/base64"
	"flag"
	"math/rand"
	"testing"
	"time"

	"gitlab.com/vocdoni/go-dvote/crypto/signature"
	"gitlab.com/vocdoni/go-dvote/log"
	"gitlab.com/vocdoni/go-dvote/types"

	common "gitlab.com/vocdoni/go-dvote/test/testcommon"
)

var censusSize = flag.Int("censusSize", 100, "number of claims to add in the census")

func init() { rand.Seed(time.Now().UnixNano()) }

func TestCensus(t *testing.T) {
	t.Parallel()

	var server common.DvoteAPIServer
	server.Start(t, "file", "census")

	signer1 := new(signature.SignKeys)
	signer1.Generate()
	signer2 := new(signature.SignKeys)
	signer2.Generate()
	if err := server.Signer.AddAuthKey(signer2.EthAddrString()); err != nil {
		t.Fatalf("cannot add authorized address %s", err)
	}
	t.Logf("added authorized address %s", signer2.EthAddrString())

	// Create websocket client
	t.Logf("connecting to %s", server.PxyAddr)
	c := common.NewAPIConnection(t, server.PxyAddr)

	// Send the API requets
	var req types.MetaRequest

	// Create census
	req.Method = "addCensus"
	req.CensusID = "test"
	resp := c.Request(req, signer2)
	if !resp.Ok {
		t.Fatalf("%s failed", req.Method)
	}
	censusID := resp.CensusID

	// addClaim
	req.CensusID = censusID
	req.Method = "addClaim"
	req.ClaimData = base64.StdEncoding.EncodeToString([]byte("hello"))
	req.Digested = true
	resp = c.Request(req, signer2)
	if !resp.Ok {
		t.Fatalf("%s failed", req.Method)
	}

	// addClaim not authorized
	req.CensusID = censusID
	req.Method = "addClaim"
	req.ClaimData = base64.StdEncoding.EncodeToString([]byte("hello2"))
	resp = c.Request(req, signer1)
	if resp.Ok {
		t.Fatalf("%s failed", req.Method)
	}

	// GenProof valid
	req.CensusID = censusID
	req.Method = "genProof"
	req.ClaimData = base64.StdEncoding.EncodeToString([]byte("hello"))
	resp = c.Request(req, nil)
	if !resp.Ok {
		t.Fatalf("%s failed", req.Method)
	}

	// GenProof not valid
	req.CensusID = censusID
	req.Method = "genProof"
	req.ClaimData = base64.StdEncoding.EncodeToString([]byte("hello3"))
	resp = c.Request(req, nil)
	if len(resp.Siblings) > 1 {
		t.Fatalf("proof should not exist!")
	}

	// getRoot
	req.Method = "getRoot"
	resp = c.Request(req, nil)
	root := resp.Root
	if len(root) < 1 {
		t.Fatalf("got invalid root")
	}

	// addClaimBulk
	var claims []string
	req.Method = "addClaimBulk"
	req.ClaimData = ""
	keys, err := signature.CreateEthRandomKeysBatch(*censusSize)
	if err != nil {
		t.Fatal(err)
	}
	for _, k := range keys {
		pub, _ := k.HexString()
		pubDesc, err := signature.DecompressPubKey(pub)
		if err != nil {
			t.Fatal(err)
		}
		claims = append(claims, base64.StdEncoding.EncodeToString(signature.HashPoseidon(pubDesc)))
	}
	req.ClaimsData = claims
	resp = c.Request(req, signer2)
	if !resp.Ok {
		t.Fatalf("%s failed", req.Method)
	}

	// dumpPlain
	req.Method = "dumpPlain"
	req.ClaimData = ""
	req.ClaimsData = []string{}
	resp = c.Request(req, signer2)
	if !resp.Ok {
		t.Fatalf("%s failed", req.Method)
	}

	// GenProof valid
	req.Method = "genProof"
	req.RootHash = ""
	req.ClaimData = claims[1]
	resp = c.Request(req, nil)
	siblings := resp.Siblings
	if len(siblings) == 0 {
		t.Fatalf("proof not generated while it should be generated correctly")
	}

	// CheckProof valid
	req.Method = "checkProof"
	req.ProofData = siblings
	resp = c.Request(req, nil)
	if !resp.ValidProof {
		t.Fatal("proof is invalid but it should be valid")
	}

	// CheckProof invalid (old root)
	req.ProofData = siblings
	req.Method = "checkProof"
	req.RootHash = root
	resp = c.Request(req, nil)
	if !resp.Ok {
		t.Fatalf("%s failed", req.Method)
	}
	if resp.ValidProof {
		t.Fatalf("proof must be invalid for hash %s but it is valid!", root)
	}
	req.RootHash = ""

	// publish
	req.Method = "publish"
	req.ClaimsData = []string{}
	resp = c.Request(req, signer2)
	if !resp.Ok {
		t.Fatalf("%s failed", req.Method)
	}
	uri := resp.URI

	// getRoot
	req.Method = "getRoot"
	resp = c.Request(req, nil)
	root = resp.Root
	if len(root) < 1 {
		t.Fatalf("got invalid root")
	}

	// getRoot from published census and check censusID=root
	req.Method = "getRoot"
	req.CensusID = root
	resp = c.Request(req, nil)
	if !resp.Ok {
		t.Fatalf("%s failed", req.Method)
	}
	if root != resp.Root {
		t.Fatalf("got invalid root from published census")
	}

	// add second census
	req.Method = "addCensus"
	req.CensusID = "importTest"
	resp = c.Request(req, signer2)
	if !resp.Ok {
		t.Fatalf("%s failed", req.Method)
	}

	// importRemote
	req.Method = "importRemote"
	req.CensusID = resp.CensusID
	req.URI = uri
	resp = c.Request(req, signer2)
	if !resp.Ok {
		t.Fatalf("%s failed", req.Method)
	}

	// getRoot
	req.Method = "getRoot"
	resp = c.Request(req, nil)
	if root != resp.Root {
		t.Fatalf("root is different after importing! %s != %s", root, resp.Root)
	}

	// getSize
	req.Method = "getSize"
	req.RootHash = ""
	resp = c.Request(req, nil)
	if exp, got := int64(*censusSize+1), resp.Size; exp != got {
		t.Fatalf("expected size %v, got %v", exp, got)
	}

	// get census list
	req.Method = "getCensusList"
	resp = c.Request(req, signer2)
	if len(resp.CensusList) != 3 {
		t.Fatalf("census list size does not match")
	}
	log.Infof("census list: %v", resp.CensusList)
}

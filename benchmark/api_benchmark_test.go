package test

import (
	"flag"
	"fmt"
	"math/rand"
	"testing"
	"time"

	"go.vocdoni.io/dvote/api"
	"go.vocdoni.io/dvote/client"
	"go.vocdoni.io/dvote/crypto/ethereum"
	"go.vocdoni.io/dvote/log"
	"go.vocdoni.io/dvote/types"
	"go.vocdoni.io/dvote/util"

	"go.vocdoni.io/dvote/test/testcommon"
)

// The init function and flags are shared with the other benchmark files.
// These globals can only be read, not modified.
func init() { rand.Seed(time.Now().UnixNano()) }

var (
	hostFlag   = flag.String("host", "", "alternative host to run against, e.g. ws[s]://<HOST>[:9090]/dvote)")
	censusSize = flag.Int("censusSize", 100, "number of census entries to add (minimum 100)")
	onlyCensus = flag.Bool("onlyCreateCensus", false, "only perform create census operations")
	// NOTE: censusBackend is not used
	censusBackend = flag.String("censusBackend", "graviton", "supported backends are: graviton")
)

// go test -v -run=- -bench=Census -benchmem -benchtime=10s . -censusSize=10000
func BenchmarkCensus(b *testing.B) {
	b.ReportAllocs()

	host := *hostFlag
	if host == "" {
		var server testcommon.DvoteAPIServer
		server.CensusBackend = *censusBackend // NOTE: CensusBackend is not used
		server.Start(b, "file", "census")
		host = server.ListenAddr
	}

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		cl, err := client.New(host)
		if err != nil {
			b.Fatal(err)
		}
		// Create websocket client
		for pb.Next() {
			censusBench(b, cl, *censusSize)
		}
	})
}

func censusBench(b *testing.B, cl *client.Client, size int) {
	// Create client signer
	signer := ethereum.NewSignKeys()
	if err := signer.Generate(); err != nil {
		b.Fatal(err)
	}

	// check required components

	req := &api.APIrequest{}
	zeroReq := &api.APIrequest{}
	reset := func(r *api.APIrequest) {
		*r = *zeroReq
	}
	doRequest := cl.ForTest(b, req)

	// Create census
	log.Infof("Create census")
	req.CensusID = fmt.Sprintf("test%x", util.RandomBytes(16))
	resp := doRequest("addCensus", signer)
	if len(resp.CensusID) == 0 {
		b.Fatalf("cannot create census")
	}
	// Set correct censusID for commint requests
	censusId := resp.CensusID

	// addClaimBulk
	log.Infof("Add bulk claims (size %d)", size)
	var keys [][]byte
	var weights []*types.BigInt
	for i := 0; i < size; i++ {
		keys = append(keys, util.RandomBytes(32))
		weights = append(weights, new(types.BigInt).SetUint64(uint64(util.RandomInt(1, 100))))
	}
	i := 0
	for i < size-200 {
		reset(req)
		req.CensusID = censusId
		req.Digested = false
		req.CensusKeys = keys[i : i+200]
		req.Weights = weights[i : i+200]
		doRequest("addClaimBulk", signer)
		i += 200
	}
	// Add remaining claims (if size%200 != 0)
	if i < size {
		reset(req)
		req.CensusID = censusId
		req.Digested = false
		req.CensusKeys = keys[i:]
		req.Weights = weights[i:]
		doRequest("addClaimBulk", signer)
	}

	// getSize
	log.Infof("Get size")
	reset(req)
	req.CensusID = censusId
	resp = doRequest("getSize", nil)
	if got := *resp.Size; int64(size) != got {
		b.Fatalf("expected size %v, got %v", size, got)
	}

	// publish
	log.Infof("Publish census")
	reset(req)
	req.CensusID = censusId
	resp = doRequest("publish", signer)
	if !resp.Ok {
		b.Fatalf("cannot publish census: %s", resp.Message)
	}
	root := fmt.Sprintf("%x", resp.Root)
	if len(root) < 1 {
		b.Fatal("got invalid root")
	}
	reset(req)
	req.CensusID = root
	resp = doRequest("getRoot", nil)
	if !resp.Ok {
		b.Fatalf("cannot publish census: %s", resp.Message)
	}
	if fmt.Sprintf("%x", resp.Root) != root {
		b.Fatal("published root mismatch")
	}

	if *onlyCensus {
		return
	}

	// GenProof valid
	log.Infof("Generating proofs")
	var siblings [][]byte
	var values [][]byte
	for _, cl := range keys {
		reset(req)
		req.CensusID = root
		req.Digested = false
		req.CensusKey = cl
		resp = doRequest("genProof", nil)
		if len(resp.Siblings) == 0 {
			b.Fatalf("proof not generated while it should be generated correctly")
		}
		siblings = append(siblings, resp.Siblings)
		values = append(values, resp.CensusValue)
	}

	// CheckProof valid
	log.Infof("Checking proofs")
	for i, sibl := range siblings {
		req.CensusID = root
		req.Digested = false
		req.ProofData = sibl
		req.CensusKey = keys[i]
		req.CensusValue = values[i]
		resp = doRequest("checkProof", nil)
		if resp.ValidProof != nil && !*resp.ValidProof {
			b.Fatalf("proof is invalid but it should be valid")
		}
	}
}

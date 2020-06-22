package test

import (
	"encoding/base64"
	"flag"
	"fmt"
	"math/rand"
	"testing"
	"time"

	"gitlab.com/vocdoni/go-dvote/client"
	"gitlab.com/vocdoni/go-dvote/crypto/ethereum"
	"gitlab.com/vocdoni/go-dvote/log"
	"gitlab.com/vocdoni/go-dvote/types"

	"gitlab.com/vocdoni/go-dvote/test/testcommon"
)

// The init function and flags are shared with the other benchmark files.
// These globals can only be read, not modified.

func init() { rand.Seed(time.Now().UnixNano()) }

var (
	hostFlag   = flag.String("host", "", "alternative host to run against, e.g. ws[s]://<HOST>[:9090]/dvote)")
	censusSize = flag.Int("censusSize", 100, "number of census entries to add (minimum 100)")
	onlyCensus = flag.Bool("onlyCreateCensus", false, "perform only create census operations")
)

func BenchmarkCensus(b *testing.B) {
	b.ReportAllocs()

	host := *hostFlag
	if host == "" {
		var server testcommon.DvoteAPIServer
		server.Start(b, "file", "census")
		host = server.PxyAddr
	}

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		// Create websocket client
		cl, err := client.New(host)
		if err != nil {
			b.Fatal(err)
		}

		for pb.Next() {
			censusBench(b, cl)
		}
	})
}

func censusBench(b *testing.B, cl *client.Client) {
	// API requets
	var req types.MetaRequest

	// Create client signer
	signer := new(ethereum.SignKeys)
	signer.Generate()

	doRequest := cl.ForTest(b, &req)

	// getInfo
	rint := rand.Int()
	log.Infof("[%d] get info", rint)
	resp := doRequest("getGatewayInfo", nil)
	log.Infof("apis available: %v", resp.APIList)

	// Create census
	log.Infof("[%d] Create census", rint)
	req.CensusID = fmt.Sprintf("test%d", rint)
	resp = doRequest("addCensus", signer)

	// Set correct censusID for commint requests
	req.CensusID = resp.CensusID

	// addClaimBulk
	log.Infof("[%d] add bulk claims (size %d)", rint, *censusSize)
	var claims []string
	req.ClaimData = ""
	req.Digested = false
	currentSize := *censusSize
	i := 0
	rint2 := 0
	for currentSize > 0 {
		iclaims := []string{}
		rint2 = rand.Int()
		for j := 0; j < 100; j++ {
			if currentSize < 1 {
				break
			}
			data := fmt.Sprintf("%d0123456789abcdef0123456789%d%d%d", rint, i, j, rint2)
			iclaims = append(iclaims, base64.StdEncoding.EncodeToString([]byte(data)))
			currentSize--
		}
		claims = append(claims, iclaims...)
		req.ClaimsData = iclaims
		resp = doRequest("addClaimBulk", signer)
		i++
		log.Infof("census creation progress: %d%%", int((i*100*100)/(*censusSize)))
	}

	// getSize
	log.Infof("[%d] get size", rint)
	req.RootHash = ""
	resp = doRequest("getSize", nil)
	if got := *resp.Size; int64(*censusSize) != got {
		b.Fatalf("expected size %v, got %v", *censusSize, got)
	}

	if *onlyCensus {
		return
	}

	// dumpPlain
	log.Infof("[%d] dump claims", rint)
	req.ClaimData = ""
	req.ClaimsData = []string{}
	resp = doRequest("dumpPlain", signer)
	if len(resp.ClaimsData) != len(claims) {
		b.Fatalf("missing claims on dumpPlain, %d != %d", len(req.ClaimsData), len(claims))
	}

	// GenProof valid
	log.Infof("[%d] generating proofs", rint)
	req.RootHash = ""
	var siblings []string

	for _, cl := range claims {
		req.ClaimData = cl
		resp = doRequest("genProof", nil)
		if len(resp.Siblings) == 0 {
			b.Fatalf("proof not generated while it should be generated correctly")
		}
		siblings = append(siblings, resp.Siblings)
	}

	// CheckProof valid
	log.Infof("[%d] checking proofs", rint)
	for i, s := range siblings {
		req.ProofData = s
		req.ClaimData = claims[i]
		resp = doRequest("checkProof", nil)
		if resp.ValidProof != nil && !*resp.ValidProof {
			b.Fatalf("proof is invalid but it should be valid")
		}
	}
	req.ProofData = ""

	// publish
	log.Infof("[%d] publish census", rint)
	req.ClaimsData = []string{}
	resp = doRequest("publish", signer)

	// getRoot
	log.Infof("[%d] get root", rint)
	resp = doRequest("getRoot", nil)
	root := resp.Root
	if len(root) < 1 {
		b.Fatalf("got invalid root")
	}

	// addFile
	log.Infof("[%d] add files", rint)
	req.Type = "ipfs"
	var uris []string
	for i := 0; i < 100; i++ {
		req.Name = fmt.Sprintf("%d_%d", rint, i)
		req.Content = base64.StdEncoding.EncodeToString([]byte(
			fmt.Sprintf("%d0123456789abcdef0123456789abc%d", rint, i)))
		resp = doRequest("addFile", nil)
		if len(resp.URI) < 1 {
			b.Fatalf("%s wrong URI received", req.Method)
		}
		uris = append(uris, resp.URI)
	}
	req.Type = ""

	// fetchFile
	log.Infof("[%d] fetching files", rint)
	for _, u := range uris {
		req.URI = u
		resp = doRequest("fetchFile", nil)
		if len(resp.Content) < 32 {
			b.Fatalf("%s wrong content received", req.Method)
		}
	}
	log.Infof("[%d] finish", rint)
}

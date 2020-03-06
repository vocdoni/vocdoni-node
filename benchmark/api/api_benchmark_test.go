package test

import (
	"encoding/base64"
	"flag"
	"fmt"
	"math/rand"
	"os"
	"testing"
	"time"

	sig "gitlab.com/vocdoni/go-dvote/crypto/signature"
	"gitlab.com/vocdoni/go-dvote/log"
	"gitlab.com/vocdoni/go-dvote/types"

	common "gitlab.com/vocdoni/go-dvote/test/testcommon"
)

var (
	logLevel   = flag.String("logLevel", "error", "logging level (debug, info, warning, error)")
	host       = flag.String("host", "", "alternative host to run against, e.g. ws[s]://<HOST>[:9090]/dvote)")
	censusSize = flag.Int("censusSize", 100, "number of census entries to add")
)

func BenchmarkCensus(b *testing.B) {
	rand.Seed(time.Now().UnixNano())
	flag.Parse()
	log.InitLogger(*logLevel, "stdout")
	if *host == "" {
		var server common.DvoteAPIServer
		if err := server.Start("file", "census"); err != nil {
			b.Fatal(err)
		}
		// TODO(mvdan): use b.Cleanup once Go 1.14 is out, so that we
		// can support many benchmark iterations.
		defer os.RemoveAll(server.IpfsDir)
		defer os.RemoveAll(server.CensusDir)
		host = &server.PxyAddr
	}

	b.RunParallel(func(pb *testing.PB) {
		// Create websocket client
		var c common.APIConnection
		if err := c.Connect(*host); err != nil {
			b.Fatalf("dial: %s", err)
		}
		defer c.Conn.Close()

		for pb.Next() {
			censusBench(b, c)
		}
	})
}

func censusBench(b *testing.B, c common.APIConnection) {
	// API requets
	var req types.MetaRequest

	// Create client signer
	signer := new(sig.SignKeys)
	signer.Generate()

	// getInfo
	rint := rand.Int()
	log.Infof("[%d] get info", rint)
	req.Method = "getGatewayInfo"
	resp, err := c.Request(req, nil)
	if err != nil {
		b.Fatal(err)
	}
	if !resp.Ok {
		b.Fatalf("%s failed: %s", req.Method, resp.Message)
	}
	log.Infof("apis available: %v", resp.APIList)

	// Create census
	log.Infof("[%d] Create census", rint)
	req.Method = "addCensus"
	req.CensusID = fmt.Sprintf("test%d", rint)
	resp, err = c.Request(req, signer)
	if err != nil {
		b.Fatal(err)
	}
	if !resp.Ok {
		b.Fatalf("%s failed: %s", req.Method, resp.Message)
	}

	// Set correct censusID for commint requests
	req.CensusID = resp.CensusID

	// addClaimBulk
	log.Infof("[%d] add bulk claims (size %d)", rint, *censusSize)
	var claims []string
	req.Method = "addClaimBulk"
	req.ClaimData = ""
	for i := 0; i < *censusSize; i++ {
		log.Infof("%d0123456789abcdef0123456789%d", rint, i)
		claims = append(claims, base64.StdEncoding.EncodeToString([]byte(
			fmt.Sprintf("%d0123456789abcdef%d", rint, i))))
	}
	req.ClaimsData = claims
	resp, err = c.Request(req, signer)
	if err != nil {
		b.Fatal(err)
	}
	if !resp.Ok {
		b.Fatalf("%s failed: %s", req.Method, resp.Message)
	}

	// dumpPlain
	log.Infof("[%d] dump claims", rint)
	req.Method = "dumpPlain"
	req.ClaimData = ""
	req.ClaimsData = []string{}
	resp, err = c.Request(req, signer)
	if err != nil {
		b.Fatal(err)
	}
	if !resp.Ok {
		b.Fatalf("%s failed: %s", req.Method, resp.Message)
	}

	// GenProof valid
	log.Infof("[%d] generating proofs", rint)
	req.Method = "genProof"
	req.RootHash = ""
	var siblings []string
	claims = []string{}

	for i := 0; i < *censusSize; i++ {
		req.ClaimData = base64.StdEncoding.EncodeToString([]byte(
			fmt.Sprintf("%d0123456789abcdef%d", rint, i)))
		resp, err = c.Request(req, nil)
		if err != nil {
			b.Fatal(err)
		}
		if len(resp.Siblings) == 0 {
			b.Fatalf("proof not generated while it should be generated correctly")
		}
		siblings = append(siblings, resp.Siblings)
		claims = append(claims, req.ClaimData)
	}

	// CheckProof valid
	log.Infof("[%d] checking proofs", rint)
	req.Method = "checkProof"
	for i, s := range siblings {
		req.ProofData = s
		req.ClaimData = claims[i]
		resp, err = c.Request(req, nil)
		if err != nil {
			b.Fatal(err)
		}
		if !resp.ValidProof {
			b.Fatalf("proof is invalid but it should be valid")
		}
	}
	req.ProofData = ""

	// publish
	log.Infof("[%d] publish census", rint)
	req.Method = "publish"
	req.ClaimsData = []string{}
	resp, err = c.Request(req, signer)
	if err != nil {
		b.Fatal(err)
	}
	if !resp.Ok {
		b.Fatalf("%s failed: %s", req.Method, resp.Message)
	}

	// getRoot
	log.Infof("[%d] get root", rint)
	req.Method = "getRoot"
	resp, err = c.Request(req, nil)
	if err != nil {
		b.Fatal(err)
	}
	root := resp.Root
	if len(root) < 1 {
		b.Fatalf("got invalid root")
	}

	// getSize
	log.Infof("[%d] get size", rint)
	req.Method = "getSize"
	req.RootHash = ""
	resp, err = c.Request(req, nil)
	if err != nil {
		b.Fatal(err)
	}
	if got := resp.Size; int64(*censusSize) != got {
		b.Fatalf("expected size %v, got %v", *censusSize, got)
	}

	// addFile
	log.Infof("[%d] add files", rint)
	req.Method = "addFile"
	req.Type = "ipfs"
	var uris []string
	for i := 0; i < 100; i++ {
		req.Name = fmt.Sprintf("%d_%d", rint, i)
		req.Content = base64.StdEncoding.EncodeToString([]byte(
			fmt.Sprintf("%d0123456789abcdef0123456789abc%d", rint, i)))
		resp, err = c.Request(req, nil)
		if err != nil {
			b.Fatal(err)
		}
		if !resp.Ok {
			b.Fatalf("%s failed: %s", req.Method, resp.Message)
		}
		if len(resp.URI) < 1 {
			b.Fatalf("%s wrong URI received", req.Method)
		}
		uris = append(uris, resp.URI)
	}
	req.Type = ""

	// fetchFile
	log.Infof("[%d] fetching files", rint)
	req.Method = "fetchFile"
	for _, u := range uris {
		req.URI = u
		resp, err = c.Request(req, nil)
		if err != nil {
			b.Fatal(err)
		}
		if !resp.Ok {
			b.Fatalf("%s failed: %s", req.Method, resp.Message)
		}
		if len(resp.Content) < 32 {
			b.Fatalf("%s wrong content received", req.Method)
		}
	}
	log.Infof("[%d] finish", rint)
}

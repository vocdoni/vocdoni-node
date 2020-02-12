// +build racy

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

	common "gitlab.com/vocdoni/go-dvote/test/test_common"
)

var logLevel = flag.String("logLevel", "error", "logging level")
var host = flag.String("host", "", "alternative host to launch the tests (i.e ws://192.168.1.33:9090/dvote)")
var routines = flag.Int("routines", 5, "number of routines to launch in paralel")

func init() { rand.Seed(time.Now().UnixNano()) }

func TestBenchmark(t *testing.T) {
	log.InitLogger(*logLevel, "stdout")

	if *host == "" {
		var server common.DvoteApiServer
		err := server.Start(*logLevel)
		if err != nil {
			t.Fatal(err)
		}
		defer os.RemoveAll(server.IpfsDir)
		defer os.RemoveAll(server.CensusDir)
		host = &server.PxyAddr
	}

	errch := make(chan error, 1)
	for i := 0; i < *routines; i++ {
		go func() { errch <- censusTest(*host) }()
	}
	for i := 0; i < *routines; i++ {
		if err := <-errch; err != nil {
			t.Errorf("subtest failed: %v", err)
		} else {
			t.Logf("subtest finished")
		}
	}
}

func censusTest(addr string) error {
	var err error
	rint := rand.Int()

	// Create websocket client
	log.Infof("connecting to %s", addr)
	var c common.ApiConnection
	err = c.Connect(addr)
	if err != nil {
		return fmt.Errorf("dial: %s", err)
	}
	defer c.Conn.Close()

	// API requets
	var req types.MetaRequest

	// Create client signer
	signer := new(sig.SignKeys)
	signer.Generate()

	// Create census
	log.Infof("[%d] Create census", rint)
	req.Method = "addCensus"
	req.CensusID = fmt.Sprintf("test%d", rint)
	resp, err := c.Request(req, signer)
	if err != nil {
		return err
	}
	if !resp.Ok {
		return fmt.Errorf("%s failed", req.Method)
	}

	// Set correct censusID for commint requests
	req.CensusID = resp.CensusID

	// addClaimBulk
	log.Infof("[%d] Add bulk claims", rint)
	var claims []string
	req.Method = "addClaimBulk"
	req.ClaimData = ""
	for i := 0; i < 100; i++ {
		claims = append(claims, base64.StdEncoding.EncodeToString([]byte(
			fmt.Sprintf("%d0123456789abcdef0123456789abc%d", rint, i))))
	}
	req.ClaimsData = claims
	resp, err = c.Request(req, signer)
	if err != nil {
		return err
	}
	if !resp.Ok {
		return fmt.Errorf("%s failed", req.Method)
	}

	// dumpPlain
	log.Infof("[%d] dump claims", rint)
	req.Method = "dumpPlain"
	req.ClaimData = ""
	req.ClaimsData = []string{}
	resp, err = c.Request(req, signer)
	if err != nil {
		return err
	}
	if !resp.Ok {
		return fmt.Errorf("%s failed", req.Method)
	}

	// GenProof valid
	log.Infof("[%d] get proof", rint)
	req.Method = "genProof"
	req.RootHash = ""
	req.ClaimData = base64.StdEncoding.EncodeToString([]byte(
		fmt.Sprintf("%d0123456789abcdef0123456789abc0", rint)))
	resp, err = c.Request(req, nil)
	if err != nil {
		return err
	}
	siblings := resp.Siblings
	if len(siblings) == 0 {
		return fmt.Errorf("proof not generated while it should be generated correctly")
	}

	// CheckProof valid
	log.Infof("[%d] check proof", rint)
	req.Method = "checkProof"
	req.ProofData = siblings
	resp, err = c.Request(req, nil)
	if err != nil {
		return err
	}
	if !resp.ValidProof {
		return fmt.Errorf("proof is invalid but it should be valid")
	}

	// publish
	log.Infof("[%d] publish census", rint)
	req.Method = "publish"
	req.ClaimsData = []string{}
	resp, err = c.Request(req, signer)
	if err != nil {
		return err
	}
	if !resp.Ok {
		return fmt.Errorf("%s failed", req.Method)
	}

	// getRoot
	log.Infof("[%d] get root", rint)
	req.Method = "getRoot"
	resp, err = c.Request(req, nil)
	if err != nil {
		return err
	}
	root := resp.Root
	if len(root) < 1 {
		return fmt.Errorf("got invalid root")
	}

	// getSize
	log.Infof("[%d] get size", rint)
	req.Method = "getSize"
	req.RootHash = ""
	resp, err = c.Request(req, nil)
	if err != nil {
		return err
	}
	if exp, got := int64(100), resp.Size; exp != got {
		return fmt.Errorf("expected size %v, got %v", exp, got)
	}

	log.Infof("[%d] finish", rint)
	return nil
}

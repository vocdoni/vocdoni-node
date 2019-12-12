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
	"encoding/json"
	"flag"
	"fmt"
	"io/ioutil"
	"math/rand"
	"os"
	"testing"
	"time"

	"github.com/gorilla/websocket"

	"gitlab.com/vocdoni/go-dvote/census"
	sig "gitlab.com/vocdoni/go-dvote/crypto/signature"
	"gitlab.com/vocdoni/go-dvote/data"
	"gitlab.com/vocdoni/go-dvote/log"
	"gitlab.com/vocdoni/go-dvote/net"
	"gitlab.com/vocdoni/go-dvote/router"
	"gitlab.com/vocdoni/go-dvote/types"
)

var level = flag.String("level", "error", "logging level")

func init() { rand.Seed(time.Now().UnixNano()) }

func TestCensus(t *testing.T) {
	log.InitLogger(*level, "stdout")
	// create the proxy to handle HTTP queries
	pxy := net.NewProxy()
	pxy.C.Address = "127.0.0.1"
	pxy.C.Port = 0
	addr, err := pxy.Init()
	if err != nil {
		t.Fatal(err)
	}

	// the server
	signer1 := new(sig.SignKeys)
	signer1.Generate()

	// the client
	signer2 := new(sig.SignKeys)
	signer2.Generate()
	if err := signer1.AddAuthKey(signer2.EthAddrString()); err != nil {
		t.Fatalf("cannot add authorized address %s", err)
	}
	t.Logf("added authorized address %s", signer2.EthAddrString())

	// Create WebSocket endpoint
	ws := new(net.WebsocketHandle)
	ws.Init(new(types.Connection))
	ws.SetProxy(pxy)

	// Create the listener for routing messages
	listenerOutput := make(chan types.Message)
	go ws.Listen(listenerOutput)

	// Create the API router
	ipfsDir, err := ioutil.TempDir("", "ipfs")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(ipfsDir)
	ipfsStore := data.IPFSNewConfig(ipfsDir)
	storage, err := data.Init(data.StorageIDFromString("IPFS"), ipfsStore)
	if err != nil {
		t.Fatalf("cannot start IPFS %s", err)
	}
	routerAPI := router.InitRouter(listenerOutput, storage, ws, signer1)

	// Create the Census Manager and enable it trough the router
	var cm census.CensusManager
	censusDir, err := ioutil.TempDir("", "census")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(censusDir)
	pub, _ := signer2.HexString()
	if err := cm.Init(censusDir, pub); err != nil {
		t.Fatal(err)
	}
	routerAPI.EnableCensusAPI(&cm)

	go routerAPI.Route()
	ws.AddProxyHandler("/dvote")

	// Create websocket client
	u := fmt.Sprintf("ws://%s/dvote", addr)
	t.Logf("connecting to %s", u)
	c, _, err := websocket.DefaultDialer.Dial(u, nil)
	if err != nil {
		t.Fatalf("dial: %s", err)
	}
	defer c.Close()

	// Send the API requets
	var req types.MetaRequest
	req.Payload = new(types.VoteTx)

	// Create census
	req.Method = "addCensus"
	req.CensusID = "test"
	resp := sendCensusReq(t, c, req, signer2)
	if !*resp.Ok {
		t.Fatalf("%s failed", req.Method)
	}
	censusID := resp.CensusID

	// addClaim
	req.CensusID = censusID
	req.Method = "addClaim"
	req.ClaimData = base64.StdEncoding.EncodeToString([]byte("hello"))
	resp = sendCensusReq(t, c, req, signer2)
	if !*resp.Ok {
		t.Fatalf("%s failed", req.Method)
	}

	// addClaim not authorized
	req.CensusID = censusID
	req.Method = "addClaim"
	req.ClaimData = base64.StdEncoding.EncodeToString([]byte("hello2"))
	resp = sendCensusReq(t, c, req, signer1)
	if resp.Ok != nil && *resp.Ok {
		t.Fatalf("%s failed", req.Method)
	}

	// GenProof valid
	req.CensusID = censusID
	req.Method = "genProof"
	req.ClaimData = base64.StdEncoding.EncodeToString([]byte("hello"))
	resp = sendCensusReq(t, c, req, nil)
	if !*resp.Ok {
		t.Fatalf("%s failed", req.Method)
	}

	// GenProof not valid
	req.CensusID = censusID
	req.Method = "genProof"
	req.ClaimData = base64.StdEncoding.EncodeToString([]byte("hello3"))
	resp = sendCensusReq(t, c, req, nil)
	if len(resp.Siblings) > 1 {
		t.Fatalf("proof should not exist!")
	}

	// getRoot
	req.Method = "getRoot"
	resp = sendCensusReq(t, c, req, nil)
	root := resp.Root
	if len(root) < 1 {
		t.Fatalf("got invalid root")
	}

	// addClaimBulk
	var claims []string
	req.Method = "addClaimBulk"
	req.ClaimData = ""
	for i := 0; i < 100; i++ {
		claims = append(claims, base64.StdEncoding.EncodeToString([]byte(
			fmt.Sprintf("0123456789abcdef0123456789abc%d", i))))
	}
	req.ClaimsData = claims
	resp = sendCensusReq(t, c, req, signer2)
	if !*resp.Ok {
		t.Fatalf("%s failed", req.Method)
	}

	// dumpPlain
	req.Method = "dumpPlain"
	req.ClaimData = ""
	req.ClaimsData = []string{}
	resp = sendCensusReq(t, c, req, signer2)
	if !*resp.Ok {
		t.Fatalf("%s failed", req.Method)
	}

	// GenProof valid
	req.Method = "genProof"
	req.RootHash = ""
	req.ClaimData = base64.StdEncoding.EncodeToString([]byte("0123456789abcdef0123456789abc0"))
	resp = sendCensusReq(t, c, req, nil)
	siblings := resp.Siblings
	if len(siblings) == 0 {
		t.Fatalf("proof not generated while it should be generated correctly")
	}

	// CheckProof valid
	req.Method = "checkProof"
	req.Payload.Proof = siblings
	resp = sendCensusReq(t, c, req, nil)
	if !resp.ValidProof {
		t.Fatal("proof is invalid but it should be valid")
	}

	// CheckProof invalid (old root)
	req.Payload.Proof = siblings
	req.Method = "checkProof"
	req.RootHash = root
	resp = sendCensusReq(t, c, req, nil)
	if !*resp.Ok {
		t.Fatalf("%s failed", req.Method)
	}
	if resp.ValidProof {
		t.Fatalf("proof must be invalid for hash %s but it is valid!", root)
	}
	req.RootHash = ""

	// publish
	req.Method = "publish"
	req.ClaimsData = []string{}
	resp = sendCensusReq(t, c, req, signer2)
	if !*resp.Ok {
		t.Fatalf("%s failed", req.Method)
	}
	uri := resp.URI

	// getRoot
	req.Method = "getRoot"
	resp = sendCensusReq(t, c, req, nil)
	root = resp.Root
	if len(root) < 1 {
		t.Fatalf("got invalid root")
	}

	// getRoot from published census and check censusID=root
	req.Method = "getRoot"
	req.CensusID = root
	resp = sendCensusReq(t, c, req, nil)
	if !*resp.Ok {
		t.Fatalf("%s failed", req.Method)
	}
	if root != resp.Root {
		t.Fatalf("got invalid root from published census")
	}

	// add second census
	req.Method = "addCensus"
	req.CensusID = "importTest"
	resp = sendCensusReq(t, c, req, signer2)
	if !*resp.Ok {
		t.Fatalf("%s failed", req.Method)
	}

	// importRemote
	req.Method = "importRemote"
	req.CensusID = resp.CensusID
	req.URI = uri
	resp = sendCensusReq(t, c, req, signer2)
	if !*resp.Ok {
		t.Fatalf("%s failed", req.Method)
	}

	// getRoot
	req.Method = "getRoot"
	resp = sendCensusReq(t, c, req, nil)
	if root != resp.Root {
		t.Fatalf("root is different after importing! %s != %s", root, resp.Root)
	}

	// getSize
	req.Method = "getSize"
	req.RootHash = ""
	resp = sendCensusReq(t, c, req, nil)
	if exp, got := int64(101), resp.Size; exp != got {
		t.Fatalf("expected size %v, got %v", exp, got)
	}
}

func sendCensusReq(t *testing.T, c *websocket.Conn, req types.MetaRequest, signer *sig.SignKeys) types.MetaResponse {
	t.Helper()
	method := req.Method

	var cmReq types.RequestMessage
	cmReq.MetaRequest = req
	cmReq.ID = fmt.Sprintf("%d", rand.Intn(1000))
	cmReq.Timestamp = int32(time.Now().Unix())
	if signer != nil {
		var err error
		cmReq.Signature, err = signer.SignJSON(cmReq.MetaRequest)
		if err != nil {
			t.Fatalf("%s: %v", method, err)
		}
	}
	rawReq, err := json.Marshal(cmReq)
	if err != nil {
		t.Fatalf("%s: %v", method, err)
	}
	if err := c.WriteMessage(websocket.TextMessage, rawReq); err != nil {
		t.Fatalf("%s: %v", method, err)
	}
	_, message, err := c.ReadMessage()
	if err != nil {
		t.Fatalf("%s: %v", method, err)
	}
	var cmRes types.ResponseMessage
	if err := json.Unmarshal(message, &cmRes); err != nil {
		t.Fatalf("%s: %v", method, err)
	}
	if cmRes.ID != cmReq.ID {
		t.Fatalf("%s: %v", method, "request ID doesn't match")
	}
	return cmRes.MetaResponse
}

package test

/* This test starts the following services

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
	"errors"
	"fmt"
	"math/rand"
	"net/url"
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

var c *websocket.Conn

func TestCensus(t *testing.T) {
	log.InitLogger("debug", "stdout")
	// create the proxy to handle HTTP queries
	pxy := net.NewProxy()
	pxy.C.Address = "127.0.0.1"
	pxy.C.Port = 8788
	err := pxy.Init()
	if err != nil {
		t.Error(err.Error())
	}

	// the server
	var signer1 *sig.SignKeys
	signer1 = new(sig.SignKeys)
	signer1.Generate()

	// the client
	var signer2 *sig.SignKeys
	signer2 = new(sig.SignKeys)
	signer2.Generate()
	err = signer1.AddAuthKey(signer2.EthAddrString())
	if err != nil {
		t.Errorf("cannot add authorized address %s", err.Error())
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
	var storage data.Storage
	ipfsDir := fmt.Sprintf("/tmp/ipfs%d", rand.Intn(1000))
	ipfsStore := data.IPFSNewConfig(ipfsDir)
	defer os.RemoveAll(ipfsDir)
	storage, err = data.Init(data.StorageIDFromString("IPFS"), ipfsStore)
	if err != nil {
		t.Errorf("cannot start IPFS %s", err.Error())
	}
	routerAPI := router.InitRouter(listenerOutput, storage, ws, *signer1)

	// Create the Census Manager and enable it trough the router
	var cm census.CensusManager
	censusDir := fmt.Sprintf("/tmp/census%d", rand.Intn(1000))
	pub, _ := signer2.HexString()
	err = os.Mkdir(censusDir, 0755)
	if err != nil {
		t.Error(err.Error())
	}
	err = cm.Init(censusDir, pub)
	if err != nil {
		t.Error(err.Error())
	}
	defer os.RemoveAll(censusDir)
	routerAPI.EnableCensusAPI(&cm)

	go routerAPI.Route()
	ws.AddProxyHandler("/dvote")

	// Create websocket client
	time.Sleep(1 * time.Second)
	u := url.URL{Scheme: "ws", Host: "127.0.0.1:8788", Path: "/dvote"}
	t.Logf("connecting to %s", u.String())
	c, _, err = websocket.DefaultDialer.Dial(u.String(), nil)
	if err != nil {
		t.Errorf("dial: %s", err.Error())
	}
	defer c.Close()

	// Wait to let all get correctly initialitzed
	time.Sleep(5 * time.Second)

	// Send the API requets
	var req types.MetaRequest
	req.Payload = new(types.VoteTx)

	// Create census
	req.Method = "addCensus"
	req.CensusID = "test"
	resp, err := sendCensusReq(req, signer2, true)
	t.Logf("getRoot response %+v", resp)
	if !resp.Ok {
		t.Errorf("fail on getRoot")
	}
	censusID := resp.CensusID

	// addClaim
	req.CensusID = censusID
	req.Method = "addClaim"
	req.ClaimData = base64.StdEncoding.EncodeToString([]byte("hello"))
	resp, err = sendCensusReq(req, signer2, true)
	t.Logf("addClaim response %+v", resp)
	if !resp.Ok {
		t.Errorf("fail on addClaim")
	}

	// addClaim not authorized
	req.CensusID = censusID
	req.Method = "addClaim"
	req.ClaimData = base64.StdEncoding.EncodeToString([]byte("hello2"))
	resp, err = sendCensusReq(req, signer1, true)
	t.Logf("addClaim response %+v", resp)
	if resp.Ok {
		t.Errorf("client should not be authorized on addClaim")
	}

	// GenProof valid
	req.CensusID = censusID
	req.Method = "genProof"
	req.ClaimData = base64.StdEncoding.EncodeToString([]byte("hello"))
	resp, err = sendCensusReq(req, signer2, false)
	t.Logf("genProof response %+v", resp)
	if !resp.Ok {
		t.Errorf("proof cannot be obtained")
	}

	// GenProof not valid
	req.CensusID = censusID
	req.Method = "genProof"
	req.ClaimData = base64.StdEncoding.EncodeToString([]byte("hello3"))
	resp, err = sendCensusReq(req, signer2, false)
	t.Logf("genProof response %+v", resp)
	if len(resp.Siblings) > 1 {
		t.Errorf("proof should not exist!")
	}

	// getRoot
	req.Method = "getRoot"
	resp, err = sendCensusReq(req, signer2, false)
	t.Logf("getRoot response %+v", resp)
	if !resp.Ok {
		t.Errorf("fail on getRoot")
	}
	root := resp.Root
	if len(root) < 1 {
		t.Errorf("got invalid root")
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
	resp, err = sendCensusReq(req, signer2, true)
	t.Logf("addClaimBulk response %+v", resp)
	if !resp.Ok {
		t.Errorf("failed adding a bulk of claims")
	}

	// dumpPlain
	req.Method = "dumpPlain"
	req.ClaimData = ""
	req.ClaimsData = []string{}
	resp, err = sendCensusReq(req, signer2, true)
	t.Logf("dumpPlain response %+v", resp)
	if !resp.Ok {
		t.Errorf("failed dumping plain claims")
	}

	// GenProof valid
	req.Method = "genProof"
	req.RootHash = ""
	req.ClaimData = base64.StdEncoding.EncodeToString([]byte("0123456789abcdef0123456789abc0"))
	resp, err = sendCensusReq(req, signer2, false)
	t.Logf("genProof response %+v", resp)
	if !resp.Ok {
		t.Errorf("proof is invalid but it should be valid (%s)", resp.Error)
	}
	siblings := resp.Siblings
	if len(siblings) == 0 {
		t.Errorf("proof not generated while it should be generated correctly")
	}

	// CheckProof valid
	req.Method = "checkProof"
	req.Payload.Proof = siblings
	resp, err = sendCensusReq(req, signer2, false)
	t.Logf("checkProof response %+v", resp)
	if !resp.ValidProof {
		t.Error("proof is invalid but it should be valid")
	}
	if !resp.Ok {
		t.Errorf("proof cannot be checked (%s)", resp.Error)
	}

	// CheckProof invalid (old root)
	req.Payload.Proof = siblings
	req.Method = "checkProof"
	req.RootHash = root
	resp, err = sendCensusReq(req, signer2, false)
	t.Logf("checkProof response %+v", resp)
	if resp.ValidProof {
		t.Errorf("proof must be invalid for hash %s but it is valid!", root)
	}
	if !resp.Ok {
		t.Errorf("proof cannot be checked (%s)", resp.Error)
	}
	req.RootHash = ""

	// publish
	req.Method = "publish"
	req.ClaimsData = []string{}
	resp, err = sendCensusReq(req, signer2, true)
	t.Logf("publish response %+v", resp)
	if !resp.Ok {
		t.Errorf("failed publishing tree to remote storage")
	}
	uri := resp.URI

	// getRoot
	req.Method = "getRoot"
	resp, err = sendCensusReq(req, signer2, false)
	t.Logf("getRoot response %+v", resp)
	if !resp.Ok {
		t.Errorf("fail on getRoot")
	}
	root = resp.Root
	if len(root) < 1 {
		t.Errorf("got invalid root")
	}

	// getRoot from published census and check censusID=root
	req.Method = "getRoot"
	req.CensusID = root
	resp, err = sendCensusReq(req, signer2, false)
	t.Logf("getRoot response of published census %+v", resp)
	if !resp.Ok {
		t.Errorf("fail on getRoot of published census")
	}
	if root != resp.Root {
		t.Errorf("got invalid root from published census")
	}

	// add second census
	req.Method = "addCensus"
	req.CensusID = "importTest"
	resp, err = sendCensusReq(req, signer2, true)
	t.Logf("addCensus response %+v", resp)
	if !resp.Ok {
		t.Errorf("fail on addCensus")
	}

	// importRemote
	time.Sleep(1 * time.Second)
	req.Method = "importRemote"
	req.CensusID = resp.CensusID
	req.URI = uri
	resp, err = sendCensusReq(req, signer2, true)
	t.Logf("importRemote response %+v", resp)
	if !resp.Ok {
		t.Errorf("failed importing tree from remote storage")
	}

	// getRoot
	req.Method = "getRoot"
	resp, err = sendCensusReq(req, signer2, false)
	t.Logf("getRoot response %+v", resp)
	if !resp.Ok {
		t.Errorf("fail on getRoot")
	}
	if root != resp.Root {
		t.Errorf("root is different after importing! %s != %s", root, resp.Root)
	}

	// getSize
	req.Method = "getSize"
	req.RootHash = ""
	resp, err = sendCensusReq(req, signer2, false)
	t.Logf("getSize response %+v", resp)
	if !resp.Ok {
		t.Errorf("fail on getSize")
	}
	if 101 != resp.Size {
		t.Errorf("size is not correct: 102 != %d", resp.Size)
	}

	// Close connection
	c.WriteMessage(websocket.CloseMessage, websocket.FormatCloseMessage(websocket.CloseNormalClosure, ""))
	os.RemoveAll(ipfsDir)
	os.RemoveAll(censusDir)
}

func sendCensusReq(req types.MetaRequest, signer *sig.SignKeys, sign bool) (types.MetaResponse, error) {
	var cmRes types.ResponseMessage
	var resp types.MetaResponse
	var cmReq types.RequestMessage
	var err error
	cmReq.Request = req
	cmReq.ID = fmt.Sprintf("%d", rand.Intn(1000))
	cmReq.Request.Timestamp = int32(time.Now().Unix())
	if sign {
		cmReq.Signature, err = signer.SignJSON(cmReq.Request)
		if err != nil {
			return resp, err
		}
	}
	rawReq, err := json.Marshal(cmReq)
	if err != nil {
		return resp, err
	}
	err = c.WriteMessage(websocket.TextMessage, rawReq)
	if err != nil {
		return resp, err
	}
	time.Sleep(1 * time.Second)
	_, message, err := c.ReadMessage()

	err = json.Unmarshal(message, &cmRes)
	if err != nil {
		return resp, err
	}
	if cmRes.ID != cmReq.ID {
		return cmRes.Response, errors.New("Request ID does not match")
	}
	return cmRes.Response, nil
}

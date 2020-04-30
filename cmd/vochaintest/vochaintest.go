package main

import (
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"math/rand"
	"time"

	"github.com/gorilla/websocket"
	flag "github.com/spf13/pflag"
	"github.com/status-im/keycard-go/hexutils"

	"gitlab.com/vocdoni/go-dvote/crypto/signature"
	"gitlab.com/vocdoni/go-dvote/log"
	"gitlab.com/vocdoni/go-dvote/types"
)

func createEthRandomKeysBatch(n int) []*signature.SignKeys {
	s := make([]*signature.SignKeys, n)
	for i := 0; i < n; i++ {
		s[i] = new(signature.SignKeys)
		if err := s[i].Generate(); err != nil {
			log.Fatal(err)
		}
	}
	return s
}

// APIConnection holds an API websocket connection
type APIConnection struct {
	Conn *websocket.Conn
}

// NewAPIConnection starts a connection with the given endpoint address. The
// connection is closed automatically when the test or benchmark finishes.
func NewAPIConnection(addr string) *APIConnection {
	r := &APIConnection{}
	var err error
	r.Conn, _, err = websocket.DefaultDialer.Dial(addr, nil)
	if err != nil {
		log.Fatal(err)
	}
	return r
}

// Request makes a request to the previously connected endpoint
func (r *APIConnection) Request(req types.MetaRequest, signer *signature.SignKeys) *types.MetaResponse {
	method := req.Method
	var cmReq types.RequestMessage
	cmReq.MetaRequest = req
	cmReq.ID = fmt.Sprintf("%d", rand.Intn(1000))
	cmReq.Timestamp = int32(time.Now().Unix())
	if signer != nil {
		var err error
		cmReq.Signature, err = signer.SignJSON(cmReq.MetaRequest)
		if err != nil {
			log.Fatalf("%s: %v", method, err)
		}
	}
	rawReq, err := json.Marshal(cmReq)
	if err != nil {
		log.Fatalf("%s: %v", method, err)
	}
	log.Debugf("sending: %s", rawReq)
	if err := r.Conn.WriteMessage(websocket.TextMessage, rawReq); err != nil {
		log.Fatalf("%s: %v", method, err)
	}
	_, message, err := r.Conn.ReadMessage()
	log.Debugf("received: %s", message)
	if err != nil {
		log.Fatalf("%s: %v", method, err)
	}
	var cmRes types.ResponseMessage
	if err := json.Unmarshal(message, &cmRes); err != nil {
		log.Fatalf("%s: %v", method, err)
	}
	if cmRes.ID != cmReq.ID {
		log.Fatalf("%s: %v", method, "request ID doesn'tb match")
	}
	if cmRes.Signature == "" {
		log.Fatalf("%s: empty signature in response: %s", method, message)
	}
	return &cmRes.MetaResponse
}

func main() {
	loglevel := flag.String("logLevel", "info", "log level")
	oraclePrivKey := flag.String("oracleKey", "", "oracle private key (hex)")
	host := flag.String("gwHost", "ws://127.0.0.1:9090/dvote", "gateway websockets endpoint")
	electionSize := flag.Int("electionSize", 100, "election census size")
	flag.Parse()
	log.Init(*loglevel, "stdout")
	rand.Seed(time.Now().UnixNano())

	censusKeys := createEthRandomKeysBatch(*electionSize)
	entityKey := signature.SignKeys{}
	oracleKey := signature.SignKeys{}
	if err := oracleKey.AddHexKey(*oraclePrivKey); err != nil {
		log.Fatal(err)
	}

	if err := entityKey.Generate(); err != nil {
		log.Fatal(err)
	}

	// Create census
	log.Infof("connecting to %s", *host)
	c := NewAPIConnection(*host)
	defer c.Conn.Close()
	censusRoot, censusURI := createCensus(c, &entityKey, censusKeys)
	log.Infof("creaed census %s of size %d", censusRoot, len(censusKeys))

	// Create process
	pid := randomHex(32)

	start, err := createProcess(c, &oracleKey, entityKey.EthAddrString(), censusRoot, censusURI, pid, "poll-vote", 50)
	if err != nil {
		log.Fatal(err)
	}
	log.Infof("created process with ID: %s", pid)
	log.Infof("waiting for process to start...")
	for cb := getCurrentBlock(c); cb <= start; cb = getCurrentBlock(c) {
		time.Sleep(10 * time.Second)
		log.Infof("remaining blocks: %d", start-cb)
	}

	if err := sendVotes(c, pid, censusRoot, censusKeys); err != nil {
		log.Fatal(err)
	}
}

func getEnvelopeStatus(c *APIConnection, nullifier, pid string) (bool, error) {
	var req types.MetaRequest
	req.Method = "getEnvelopeStatus"
	req.ProcessID = pid
	req.Nullifier = nullifier
	resp := c.Request(req, nil)
	if !resp.Ok || resp.Registered == nil {
		return false, fmt.Errorf("cannot check envelope (%s)", resp.Message)
	}
	return *resp.Registered, nil
}

func getProof(c *APIConnection, pubkey, root string) (string, error) {
	var req types.MetaRequest
	req.Method = "genProof"
	req.CensusID = root
	req.Digested = true
	req.ClaimData = base64.StdEncoding.EncodeToString(signature.HashPoseidon(hexutils.HexToBytes(pubkey)))

	resp := c.Request(req, nil)
	if len(resp.Siblings) == 0 || !resp.Ok {
		return "", fmt.Errorf("cannot get merkle proof: (%s)", resp.Message)
	}

	return resp.Siblings, nil
}

func sendVotes(c *APIConnection, pid, root string, signers []*signature.SignKeys) error {
	var pub, proof string
	var err error
	vp := types.VotePackage{
		Votes: []int{1, 2, 3},
	}
	vpBytes, err := json.Marshal(vp)
	if err != nil {
		return err
	}

	proofs := []string{}
	log.Infof("generating proofs...")
	for i, s := range signers {
		pub, _ = s.HexString()
		if proof, err = getProof(c, pub, root); err != nil {
			return err
		}
		proofs = append(proofs, proof)
		log.Infof("proof generation progress: %d%%", int((i*100)/(len(signers))))
	}

	var req types.MetaRequest
	var nullifier string
	req.Method = "submitRawTx"
	start := time.Now()
	for i, s := range signers {
		log.Infof("sending vote %d", i)
		v := types.VoteTx{
			Nonce:       randomHex(32),
			ProcessID:   pid,
			Proof:       proofs[i],
			VotePackage: base64.StdEncoding.EncodeToString(vpBytes),
		}
		txBytes, err := json.Marshal(v)
		if err != nil {
			return err
		}
		if v.Signature, err = s.Sign(txBytes); err != nil {
			return err
		}
		v.Type = "vote"
		if txBytes, err = json.Marshal(v); err != nil {
			return err
		}
		req.RawTx = base64.StdEncoding.EncodeToString(txBytes)

		resp := c.Request(req, nil)
		if !resp.Ok {
			return fmt.Errorf("%s failed: %s", req.Method, resp.Message)
		}
		nullifier = resp.Payload
	}
	log.Infof("last nullifier %s", nullifier)

	for {
		time.Sleep(500 * time.Millisecond)
		es, _ := getEnvelopeStatus(c, nullifier, pid)
		if es {
			break
		}
	}
	log.Infof("the voting computation took %s", time.Since(start))
	return nil
}

func createProcess(c *APIConnection, oracle *signature.SignKeys, entityID, mkroot, mkuri, pid, ptype string, duration int) (int64, error) {
	var req types.MetaRequest
	req.Method = "submitRawTx"
	p := types.NewProcessTx{
		Type:           "newProcess",
		EntityID:       entityID,
		MkRoot:         mkroot,
		MkURI:          mkuri,
		NumberOfBlocks: int64(duration),
		ProcessID:      pid,
		ProcessType:    ptype,
		StartBlock:     getCurrentBlock(c) + 2,
	}
	txBytes, err := json.Marshal(p)
	if err != nil {
		return 0, err
	}
	if p.Signature, err = oracle.Sign(txBytes); err != nil {
		return 0, err
	}
	if txBytes, err = json.Marshal(p); err != nil {
		return 0, err
	}
	req.RawTx = base64.StdEncoding.EncodeToString(txBytes)

	resp := c.Request(req, nil)
	if !resp.Ok {
		log.Fatalf("%s failed: %s", req.Method, resp.Message)
	}
	return p.StartBlock, nil
}

func getCurrentBlock(c *APIConnection) int64 {
	var req types.MetaRequest
	req.Method = "getBlockHeight"
	resp := c.Request(req, nil)
	if !resp.Ok {
		log.Fatalf("%s failed: %s", req.Method, resp.Message)
	}
	if resp.Height == nil {
		log.Fatalf("height is nil!")
	}
	return *resp.Height

}

func createCensus(c *APIConnection, signer *signature.SignKeys, censusSigners []*signature.SignKeys) (root, uri string) {
	var req types.MetaRequest
	rint := rand.Int()
	censusSize := len(censusSigners)

	log.Infof("[%d] get info", rint)
	req.Method = "getGatewayInfo"
	resp := c.Request(req, nil)
	if !resp.Ok {
		log.Fatalf("%s failed: %s", req.Method, resp.Message)
	}
	log.Infof("apis available: %v", resp.APIList)

	// Create census
	log.Infof("[%d] Create census", rint)
	req.Method = "addCensus"
	req.CensusID = fmt.Sprintf("test%d", rint)
	resp = c.Request(req, signer)
	if !resp.Ok {
		log.Fatalf("%s failed: %s", req.Method, resp.Message)
	}

	// Set correct censusID for commint requests
	req.CensusID = resp.CensusID

	// addClaimBulk
	log.Infof("[%d] add bulk claims (size %d)", rint, censusSize)
	var claims []string
	req.Method = "addClaimBulk"
	req.ClaimData = ""
	req.Digested = true
	currentSize := censusSize
	i := 0
	var pub string
	var data string
	for currentSize > 0 {
		iclaims := []string{}
		for j := 0; j < 100; j++ {
			if currentSize < 1 {
				break
			}
			pub, _ = censusSigners[currentSize-1].HexString()
			data = base64.StdEncoding.EncodeToString(signature.HashPoseidon(hexutils.HexToBytes(pub)))
			iclaims = append(iclaims, data)
			currentSize--
		}
		claims = append(claims, iclaims...)
		req.ClaimsData = iclaims
		resp = c.Request(req, signer)
		if !resp.Ok {
			log.Fatalf("%s failed: %s", req.Method, resp.Message)
		}
		i++
		log.Infof("census creation progress: %d%%", int((i*100*100)/(censusSize)))
	}

	// getSize
	log.Infof("[%d] get size", rint)
	req.Method = "getSize"
	req.RootHash = ""
	resp = c.Request(req, nil)
	if got := *resp.Size; int64(censusSize) != got {
		log.Fatalf("expected size %v, got %v", censusSize, got)
	}

	// publish
	log.Infof("[%d] publish census", rint)
	req.Method = "publish"
	req.ClaimsData = []string{}
	resp = c.Request(req, signer)
	if !resp.Ok {
		log.Fatalf("%s failed: %s", req.Method, resp.Message)
	}
	uri = resp.URI
	if len(uri) < 40 {
		log.Fatalf("got invalid URI")
	}

	// getRoot
	log.Infof("[%d] get root", rint)
	req.Method = "getRoot"
	resp = c.Request(req, nil)
	root = resp.Root
	if len(root) < 1 {
		log.Fatalf("got invalid root")
	}

	return root, uri
}

func randomHex(n int) string {
	bytes := make([]byte, n)
	if _, err := rand.Read(bytes); err != nil {
		return ""
	}
	return hex.EncodeToString(bytes)
}

package main

import (
	"context"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"math/rand"
	"strings"
	"sync"
	"time"

	flag "github.com/spf13/pflag"
	"nhooyr.io/websocket"

	"gitlab.com/vocdoni/go-dvote/crypto/ethereum"
	"gitlab.com/vocdoni/go-dvote/crypto/nacl"
	"gitlab.com/vocdoni/go-dvote/crypto/snarks"
	"gitlab.com/vocdoni/go-dvote/log"
	"gitlab.com/vocdoni/go-dvote/types"
)

const retries = 10

// APIConnection holds an API websocket connection
type APIConnection struct {
	Conn *websocket.Conn
	Addr string
	ID   int
}

// NewAPIConnection starts a connection with the given endpoint address. The
// connection is closed automatically when the test or benchmark finishes.
func NewAPIConnection(addr string, id int) (*APIConnection, error) {
	r := &APIConnection{}
	var err error
	for i := 0; i < retries; i++ {
		r.Conn, _, err = websocket.Dial(context.TODO(), addr, nil)
		if err == nil {
			break
		}
		time.Sleep(2 * time.Second)
	}
	if err != nil {
		return nil, err
	}
	r.Addr = addr
	r.ID = id
	return r, nil
}

// Request makes a request to the previously connected endpoint
func (r *APIConnection) Request(req types.MetaRequest, signer *ethereum.SignKeys) *types.MetaResponse {
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
	if err := r.Conn.Write(context.TODO(), websocket.MessageText, rawReq); err != nil {
		log.Fatalf("%s: %v", method, err)
	}
	_, message, err := r.Conn.Read(context.TODO())
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
	// starting test
	loglevel := flag.String("logLevel", "info", "log level")
	oraclePrivKey := flag.String("oracleKey", "", "oracle private key (hex)")
	entityPrivKey := flag.String("entityKey", "", "entity private key (hex)")
	host := flag.String("gwHost", "ws://127.0.0.1:9090/dvote", "gateway websockets endpoint")
	electionType := flag.String("electionType", "encrypted-poll", "encrypted-poll or poll-vote")
	electionSize := flag.Int("electionSize", 100, "election census size")
	parallelCons := flag.Int("parallelCons", 1, "parallel API connections")
	procDuration := flag.Int("processDuration", 5, "voting process duration in blocks")
	gateways := flag.StringSlice("gwExtra", []string{}, "list of extra gateways to be used in addition to gwHost for sending votes")

	flag.Parse()
	log.Init(*loglevel, "stdout")
	rand.Seed(time.Now().UnixNano())

	censusKeys := createEthRandomKeysBatch(*electionSize)
	entityKey := ethereum.SignKeys{}
	oracleKey := ethereum.SignKeys{}
	if err := oracleKey.AddHexKey(*oraclePrivKey); err != nil {
		log.Fatal(err)
	}

	// create entity key
	if len(*entityPrivKey) > 0 {
		if err := entityKey.AddHexKey(*entityPrivKey); err != nil {
			log.Fatal(err)
		}
	} else {
		if err := entityKey.Generate(); err != nil {
			log.Fatal(err)
		}
	}

	// Create census
	log.Infof("connecting to main gateway %s", *host)

	var conns []*APIConnection

	// Add the first connection, this will be the main connection
	var mainCon *APIConnection
	var err error
	for tries := 13; tries > 0; tries-- {
		mainCon, err = NewAPIConnection(*host, 0)
		if err == nil {
			break
		}
		time.Sleep(1 * time.Second)
	}
	if err != nil {
		log.Fatal(err)
	}
	defer mainCon.Conn.Close(websocket.StatusNormalClosure, "")

	censusRoot, censusURI := createCensus(mainCon, &entityKey, censusKeys)
	log.Infof("creaed census %s of size %d", censusRoot, len(censusKeys))

	// Create process
	pid := randomHex(32)
	log.Infof("creating process with entityID: %s", entityKey.EthAddrString())
	start, err := createProcess(mainCon, &oracleKey, entityKey.EthAddrString(), censusRoot, censusURI, pid, *electionType, *procDuration)
	if err != nil {
		log.Fatal(err)
	}

	log.Infof("created process with ID: %s", pid)
	encrypted := types.ProcessIsEncrypted[*electionType]

	// Create the websockets connections for sending the votes
	gwList := append(*gateways, *host)

	for i := 0; i < *parallelCons; i++ {
		log.Infof("opening gateway connection to %s", gwList[i%len(gwList)])
		cgw, err := NewAPIConnection(gwList[i%len(gwList)], i+1)
		if err != nil {
			log.Warn(err)
			continue
		}
		defer cgw.Conn.Close(websocket.StatusNormalClosure, "")
		conns = append(conns, cgw)
	}

	// Make sure all gateways have the census
	gwsWithCensus := []string{mainCon.Addr}
	for _, con := range conns {
		found := false
		for _, gw := range gwsWithCensus {
			if gw == con.Addr {
				found = true
				break
			}
		}
		if !found {
			log.Infof("importing census to gateway %s", con.Addr)
			gwsWithCensus = append(gwsWithCensus, con.Addr)
			if root, err := importCensus(con, &entityKey, censusURI); err != nil {
				log.Fatal(err)
			} else {
				if root != censusRoot {
					log.Fatalf("imported census root does not match (%s != %s)", root, censusRoot)
				}
			}
		}
	}

	// Send votes
	i := 0
	p := len(censusKeys) / len(conns)
	var wg sync.WaitGroup
	votingTimes := make([]time.Duration, len(conns))
	for gw, con := range conns {
		signers := make([]*ethereum.SignKeys, p)
		copy(signers[:], censusKeys[i:i+p])
		log.Infof("voters from %d to %d will be sent to %s", i, i+p-1, con.Addr)
		gw, con := gw, con
		wg.Add(1)
		go func() {
			defer wg.Done()
			if votingTimes[gw], err = sendVotes(con, pid, entityKey.EthAddrString(), censusRoot, start, int64(*procDuration), signers, encrypted); err != nil {
				log.Fatal(err)
			}
			log.Infof("gateway %d has ended its job", con.ID)
		}()
		i += p
	}

	// Wait until all votes sent and check the results
	wg.Wait()
	/*	log.Infof("canceling process in order to fetch the results")
		if err := cancelProcess(mainCon, &oracleKey, pid); err != nil {
			log.Fatal(err)
		}
	*/
	maxVotingTime := time.Duration(0)
	for _, t := range votingTimes {
		if t > maxVotingTime {
			maxVotingTime = t
		}
	}
	log.Infof("the voting process took %s", maxVotingTime)
	log.Infof("checking results....")
	if r, err := results(mainCon, pid, len(censusKeys), start, int64(*procDuration)); err != nil {
		log.Fatal(err)
	} else {
		log.Infof("results: %+v", r)
	}
	log.Infof("all done!")
}

func createEthRandomKeysBatch(n int) []*ethereum.SignKeys {
	s := make([]*ethereum.SignKeys, n)
	for i := 0; i < n; i++ {
		s[i] = new(ethereum.SignKeys)
		if err := s[i].Generate(); err != nil {
			log.Fatal(err)
		}
	}
	return s
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

func getProof(c *APIConnection, hexpubkey, root string) (string, error) {
	var req types.MetaRequest
	req.Method = "genProof"
	req.CensusID = root
	req.Digested = true
	pubkey, err := hex.DecodeString(hexpubkey)
	if err != nil {
		return "", err
	}
	req.ClaimData = base64.StdEncoding.EncodeToString(snarks.Poseidon.Hash(pubkey))

	resp := c.Request(req, nil)
	if len(resp.Siblings) == 0 || !resp.Ok {
		return "", fmt.Errorf("cannot get merkle proof: (%s)", resp.Message)
	}

	return resp.Siblings, nil
}

func hexToBytes(s string) []byte {
	b := make([]byte, hex.DecodedLen(len(s)))
	_, err := hex.Decode(b, []byte(s))
	if err != nil {
		log.Fatal(err)
	}
	return b[:]
}

func getResults(c *APIConnection, pid string) ([][]uint32, error) {
	var req types.MetaRequest
	req.Method = "getResults"
	req.ProcessID = pid
	resp := c.Request(req, nil)
	if !resp.Ok {
		return nil, fmt.Errorf("cannot get results: (%s)", resp.Message)
	}
	return resp.Results, nil
}

func getEnvelopeHeight(c *APIConnection, pid string) (int64, error) {
	var req types.MetaRequest
	req.Method = "getEnvelopeHeight"
	req.ProcessID = pid
	resp := c.Request(req, nil)
	if !resp.Ok {
		return 0, fmt.Errorf(resp.Message)
	}
	return *resp.Height, nil
}

func importCensus(c *APIConnection, signer *ethereum.SignKeys, uri string) (string, error) {
	var req types.MetaRequest
	req.Method = "addCensus"
	req.CensusID = randomHex(16)
	resp := c.Request(req, signer)

	req.Method = "importRemote"
	req.CensusID = resp.CensusID
	req.URI = uri
	resp = c.Request(req, signer)
	if !resp.Ok {
		return "", fmt.Errorf(resp.Message)
	}

	req.Method = "publish"
	req.URI = ""
	resp = c.Request(req, signer)
	if !resp.Ok {
		return "", fmt.Errorf(resp.Message)
	}
	return resp.Root, nil
}

type pkeys struct {
	pub  []types.Key
	priv []types.Key
	comm []types.Key
	rev  []types.Key
}

func getKeys(c *APIConnection, pid, eid string) (*pkeys, error) {
	var req types.MetaRequest
	req.Method = "getProcessKeys"
	req.ProcessID = pid
	req.EntityId = eid
	resp := c.Request(req, nil)
	if !resp.Ok {
		return nil, fmt.Errorf("cannot get keys for process %s: (%s)", pid, resp.Message)
	}
	return &pkeys{
		pub:  resp.EncryptionPublicKeys,
		priv: resp.EncryptionPrivKeys,
		comm: resp.CommitmentKeys,
		rev:  resp.RevealKeys}, nil
}

func results(c *APIConnection, pid string, totalVotes int, startBlock, duration int64) (results [][]uint32, err error) {
	log.Infof("waiting for results...")
	waitUntilBlock(c, startBlock+duration+2)
	if results, err = getResults(c, pid); err != nil {
		return nil, err
	}
	total := uint32(totalVotes)
	if results[0][1] != total || results[1][2] != total || results[2][3] != total || results[3][4] != total {
		return nil, fmt.Errorf("invalid results")
	}
	return results, nil
}

func sendVotes(c *APIConnection, pid, eid, root string, startBlock, duration int64, signers []*ethereum.SignKeys, encrypted bool) (time.Duration, error) {
	var pub, proof string
	var err error
	var keys []string
	var proofs []string

	log.Infof("generating proofs...")
	for i, s := range signers {
		pub, _ = s.HexString()
		pub, _ = ethereum.DecompressPubKey(pub) // Temporary until everything is compressed
		if proof, err = getProof(c, pub, root); err != nil {
			return 0, err
		}
		proofs = append(proofs, proof)
		if (i+1)%100 == 0 {
			log.Infof("proof generation progress: %d%%", int(((i+1)*100)/(len(signers))))
		}
	}

	waitUntilBlock(c, startBlock)
	keyIndexes := []int{}
	if encrypted {
		if pk, err := getKeys(c, pid, eid); err != nil {
			return 0, fmt.Errorf("cannot get process keys: (%s)", err)
		} else {
			for _, k := range pk.pub {
				if len(k.Key) > 0 {
					keyIndexes = append(keyIndexes, k.Idx)
					keys = append(keys, k.Key)
				}
			}
		}
		if len(keys) == 0 {
			return 0, fmt.Errorf("process keys is empty")
		}
		log.Infof("got encryption keys!")
	}

	var req types.MetaRequest
	var nullifiers []string
	var vpb string
	req.Method = "submitRawTx"
	start := time.Now()
	log.Infof("sending votes")

	for i := 0; i < len(signers); i++ {
		s := signers[i]
		if vpb, err = genVote(encrypted, keys); err != nil {
			return 0, err
		}
		v := types.VoteTx{
			Nonce:                randomHex(16),
			ProcessID:            pid,
			Proof:                proofs[i],
			VotePackage:          vpb,
			EncryptionKeyIndexes: keyIndexes,
		}

		txBytes, err := json.Marshal(v)
		if err != nil {
			return 0, err
		}
		if v.Signature, err = s.Sign(txBytes); err != nil {
			return 0, err
		}
		v.Type = "vote"
		if txBytes, err = json.Marshal(v); err != nil {
			return 0, err
		}
		req.RawTx = base64.StdEncoding.EncodeToString(txBytes)
		resp := c.Request(req, nil)
		if !resp.Ok {
			if strings.Contains(resp.Message, "mempool is full") {
				log.Warnf("mempool is full, wait and retry")
				time.Sleep(1 * time.Second)
				i--
			} else {
				return 0, fmt.Errorf("%s failed: %s", req.Method, resp.Message)
			}
		}
		nullifiers = append(nullifiers, resp.Payload)
		if (i+1)%100 == 0 {
			log.Infof("voting progress: %d%%", int(((i+1)*100)/(len(signers))))
		}
	}
	log.Infof("votes submited! took %s", time.Since(start))
	checkStart := time.Now()
	registered := 0
	log.Infof("waiting for votes to be validated...")
	for {
		time.Sleep(time.Millisecond * 500)
		if h, err := getEnvelopeHeight(c, pid); err != nil {
			log.Warnf("error getting envelope height")
			continue
		} else {
			if h >= int64(len(signers)) {
				break
			}
		}
		if time.Since(checkStart) > time.Minute*10 {
			return 0, fmt.Errorf("wait for envelope height took more than 10 minutes, skipping")
		}
	}
	votingElapsedTime := time.Since(start)
	for {
		for i, n := range nullifiers {
			if n == "registered" {
				registered++
				continue
			}
			if es, _ := getEnvelopeStatus(c, n, pid); es {
				nullifiers[i] = "registered"
			}
		}
		if registered == len(nullifiers) {
			break
		}
		registered = 0
		if time.Since(checkStart) > time.Minute*10 {
			return 0, fmt.Errorf("check nullifier time took more than 10 minutes, skipping")
		}
	}
	return votingElapsedTime, nil
}

func genVote(encrypted bool, keys []string) (string, error) {
	vp := &types.VotePackage{
		Votes: []int{1, 2, 3, 4, 5, 6},
	}
	var vpBytes []byte
	var err error
	if encrypted {
		first := true
		for i, k := range keys {
			if len(k) > 0 {
				log.Debugf("encrypting with key %s", k)
				pub, err := nacl.DecodePublic(k)
				if err != nil {
					return "", fmt.Errorf("cannot decode encryption key with index %d: (%s)", i, err)
				}
				if first {
					vp.Nonce = randomHex(rand.Intn(16) + 16)
					vpBytes, err = json.Marshal(vp)
					if err != nil {
						return "", err
					}
					first = false
				}
				if vpBytes, err = nacl.Anonymous.Encrypt(vpBytes, pub); err != nil {
					return "", fmt.Errorf("cannot encrypt: (%s)", err)
				}
			}
		}
	} else {
		vpBytes, err = json.Marshal(vp)
		if err != nil {
			return "", err
		}

	}
	return base64.StdEncoding.EncodeToString(vpBytes), nil
}

func createProcess(c *APIConnection, oracle *ethereum.SignKeys, entityID, mkroot, mkuri, pid, ptype string, duration int) (int64, error) {
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
		StartBlock:     getCurrentBlock(c) + 4,
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

func cancelProcess(c *APIConnection, oracle *ethereum.SignKeys, pid string) error {
	var req types.MetaRequest
	req.Method = "submitRawTx"
	p := types.NewProcessTx{
		Type:      "cancelProcess",
		ProcessID: pid,
	}
	txBytes, err := json.Marshal(p)
	if err != nil {
		return err
	}
	if p.Signature, err = oracle.Sign(txBytes); err != nil {
		return err
	}
	if txBytes, err = json.Marshal(p); err != nil {
		return err
	}
	req.RawTx = base64.StdEncoding.EncodeToString(txBytes)

	resp := c.Request(req, nil)
	if !resp.Ok {
		log.Fatalf("%s failed: %s", req.Method, resp.Message)
	}
	return nil
}

func waitUntilBlock(c *APIConnection, block int64) {
	log.Infof("waiting for block %d...", block)
	for cb := getCurrentBlock(c); cb <= block; cb = getCurrentBlock(c) {
		time.Sleep(10 * time.Second)
		log.Infof("remaining blocks: %d", block-cb)
	}
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

func createCensus(c *APIConnection, signer *ethereum.SignKeys, censusSigners []*ethereum.SignKeys) (root, uri string) {
	var req types.MetaRequest
	rint := rand.Int()
	censusSize := len(censusSigners)
	resp := c.Request(req, nil)
	// Create census
	log.Infof("Create census")
	req.Method = "addCensus"
	req.CensusID = fmt.Sprintf("test%d", rint)
	resp = c.Request(req, signer)
	if !resp.Ok {
		log.Fatalf("%s failed: %s", req.Method, resp.Message)
	}
	// Set correct censusID for commint requests
	req.CensusID = resp.CensusID

	// addClaimBulk
	log.Infof("add bulk claims (size %d)", censusSize)
	req.Method = "addClaimBulk"
	req.ClaimData = ""
	req.Digested = true
	currentSize := censusSize
	i := 0
	var data string
	for currentSize > 0 {
		claims := []string{}
		for j := 0; j < 100; j++ {
			if currentSize < 1 {
				break
			}
			hexpub, _ := censusSigners[currentSize-1].HexString()
			hexpub, _ = ethereum.DecompressPubKey(hexpub) // Temporary until everything is compressed only
			pub, err := hex.DecodeString(hexpub)
			if err != nil {
				log.Fatal(err)
			}
			data = base64.StdEncoding.EncodeToString(snarks.Poseidon.Hash(pub))
			claims = append(claims, data)
			currentSize--
		}
		req.ClaimsData = claims
		resp = c.Request(req, signer)
		if !resp.Ok {
			log.Fatalf("%s failed: %s", req.Method, resp.Message)
		}
		i++
		log.Infof("census creation progress: %d%%", int((i*100*100)/(censusSize)))
	}

	// getSize
	log.Infof("get size")
	req.Method = "getSize"
	req.RootHash = ""
	resp = c.Request(req, nil)
	if got := *resp.Size; int64(censusSize) != got {
		log.Fatalf("expected size %v, got %v", censusSize, got)
	}

	// publish
	log.Infof("publish census")
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
	log.Infof("get root")
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

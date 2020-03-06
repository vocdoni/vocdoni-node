package test

import (
	"encoding/base64"
	"encoding/json"
	"flag"
	"fmt"
	"math/rand"
	"os"
	"strconv"
	"testing"
	"time"

	"gitlab.com/vocdoni/go-dvote/crypto/hashing"
	"gitlab.com/vocdoni/go-dvote/crypto/signature"
	"gitlab.com/vocdoni/go-dvote/log"
	"gitlab.com/vocdoni/go-dvote/test/testcommon"
	common "gitlab.com/vocdoni/go-dvote/test/testcommon"
	"gitlab.com/vocdoni/go-dvote/types"
	"gitlab.com/vocdoni/go-dvote/vochain"
)

// THIS BENCH DOES NOT PROVIDE ANY CONSENSUS GUARANTEES

const (
	numberOfBlocks = 1000
	processID      = "0xe9d5e8d791f51179e218c606f83f5967ab272292a6dbda887853d81f7a1d5105"
	processType    = "poll-vote"
)

var (
	logLevel   = flag.String("logLevel", "error", "logging level (debug, info, warning, error)")
	host       = flag.String("host", "", "alternative host to run against, e.g. ws[s]://<HOST>[:9090]/dvote)")
	censusSize = flag.Int("censusSize", 100, "number of census entries to add")
)

func BenchmarkVochain(b *testing.B) {
	rand.Seed(time.Now().UnixNano())
	flag.Parse()
	log.InitLogger(*logLevel, "stdout")
	var dvoteServer common.DvoteAPIServer
	rint := rand.Int()
	if *host == "" {
		if err := dvoteServer.Start("file", "census", "vote"); err != nil {
			b.Fatal(err)
		}
		defer os.RemoveAll(dvoteServer.IpfsDir)
		defer os.RemoveAll(dvoteServer.CensusDir)
		defer os.RemoveAll(dvoteServer.VochainCfg.DataDir)
		host = &dvoteServer.PxyAddr
	}

	// create random key batch
	keySet, err := signature.CreateEthRandomKeysBatch(*censusSize)
	if err != nil {
		b.Fatalf("cannot create keySet: %s", err)
	}

	// get public keys of signer set
	pubKeys := make([]string, len(keySet))
	for i := 0; i < len(keySet); i++ {
		pubKeys[i], _ = keySet[i].HexString()
		pubKeys[i], err = signature.DecompressPubKey(pubKeys[i])
		if err != nil {
			b.Fatalf("cannot decompress public key: %+v", pubKeys[i])
		}
	}
	log.Infof("generated %d keys", len(pubKeys))

	// get signer pubkey
	signerPub, _ := dvoteServer.Signer.HexString()

	// check required components
	var conn common.APIConnection
	if err := conn.Connect(*host); err != nil {
		b.Fatalf("dial: %s", err)
	}
	var req types.MetaRequest
	log.Info("get info")
	req.Method = "getGatewayInfo"
	resp, err := conn.Request(req, nil)
	if err != nil {
		b.Fatal(err)
	}
	if !resp.Ok {
		b.Fatalf("%s failed: %s", req.Method, resp.Message)
	}
	log.Infof("apis available: %v", resp.APIList)

	// create census
	log.Infof("creating census")
	req.Method = "addCensus"
	req.CensusID = fmt.Sprintf("test%d", rint)
	resp, err = conn.Request(req, dvoteServer.Signer)
	if err != nil {
		b.Fatal(err)
	}
	if !resp.Ok {
		b.Fatalf("%s failed: %s", req.Method, resp.Message)
	}

	// Set correct censusID for comming requests
	req.CensusID = resp.CensusID

	// census add claims
	poseidonHashes := make([]string, len(pubKeys))
	for count, key := range pubKeys {
		if poseidonHashes[count], err = hashing.PoseidonHash(key); err != nil {
			b.Fatalf("cannot create poseidon hash of public key: %+v", pubKeys[count])
		}
	}
	log.Debugf("poseidon hashes: %s", poseidonHashes)
	log.Debug("add bulk claims")
	var claims []string
	req.Method = "addClaimBulk"
	req.ClaimData = ""
	for i := 0; i < len(poseidonHashes); i++ {
		claims = append(claims, poseidonHashes[i])
	}
	req.ClaimsData = claims
	resp, err = conn.Request(req, dvoteServer.Signer)
	if err != nil {
		b.Fatal(err)
	}
	if !resp.Ok {
		b.Fatalf("%s failed: %s", req.Method, resp.Message)
	}

	// get census root
	log.Infof("get root")
	req.Method = "getRoot"
	resp, err = conn.Request(req, nil)
	if err != nil {
		b.Fatal(err)
	}
	mkRoot := resp.Root
	if len(mkRoot) < 1 {
		b.Fatalf("got invalid root")
	}

	log.Infof("check block height is not less than process start block")
	req.Method = "getBlockHeight"
	req.Timestamp = int32(time.Now().Unix())
	resp, err = conn.Request(req, nil)
	if err != nil {
		b.Fatal(err)
	}

	// create process
	process := &types.NewProcessTx{
		EncryptionPublicKeys: []string{""},
		EntityID:             signerPub,
		MkRoot:               mkRoot,
		NumberOfBlocks:       numberOfBlocks,
		ProcessID:            processID,
		ProcessType:          processType,
		StartBlock:           *resp.Height + 1,
		Type:                 "newProcess",
	}
	process.Signature, err = dvoteServer.Signer.SignJSON(process)
	if err != nil {
		b.Fatalf("cannot sign oracle tx: %s", err)
	}
	tx, err := json.Marshal(process)
	if err != nil {
		b.Fatalf("error marshaling process tx: %s", err)
	}
	res, err := dvoteServer.VochainRPCClient.BroadcastTxSync(tx)
	if err != nil {
		b.Fatalf("error broadcasting process tx: %s", err)
	} else {
		log.Infof("new transaction hash: %s", res.Hash)
	}

	// check if process is created
	log.Infof("check if process created")
	req.Method = "getProcessList"
	req.EntityId = process.EntityID
	req.Timestamp = int32(time.Now().Unix())

	for {
		resp, err = conn.Request(req, nil)
		if err != nil {
			b.Fatal(err)
		}
		if resp.ProcessList[0] == "0xe9d5e8d791f51179e218c606f83f5967ab272292a6dbda887853d81f7a1d5105" {
			break
		}
		time.Sleep(time.Second)
	}

	// send votes in parallel
	b.RunParallel(func(pb *testing.PB) {
		// Create websocket client
		var c common.APIConnection
		if err := c.Connect(*host); err != nil {
			b.Fatalf("dial: %s", err)
		}
		defer c.Conn.Close()

		count := 0
		for pb.Next() {
			vochainBench(b, c, keySet[count], poseidonHashes[count], mkRoot, process.ProcessID, req.CensusID)
			count++
		}
	})

	// scrutiny of the submited envelopes
	log.Infof("get results")
	req.Method = "getResults"
	req.ProcessID = process.ProcessID
	req.Timestamp = int32(time.Now().Unix())
	resp, err = conn.Request(req, nil)
	if err != nil {
		b.Fatal(err)
	}
	log.Infof("submited votes: %+v", resp.Results)
}

func vochainBench(b *testing.B, c testcommon.APIConnection, s *signature.SignKeys, poseidon, mkRoot, processID, censusID string) {
	rint := rand.Int()
	// API requests
	var req types.MetaRequest

	// create envelope
	log.Infof("adding vote using key [%s]", s.EthAddrString())

	pub, _ := s.HexString()
	// generate envelope proof
	log.Infof("generating proof for key %s with poseidon hash: %s", pub, poseidon)
	req.Method = "genProof"
	req.CensusID = censusID
	req.RootHash = mkRoot
	req.ClaimData = poseidon
	resp, err := c.Request(req, nil)
	if err != nil {
		b.Fatal(err)
	}
	if len(resp.Siblings) == 0 {
		b.Fatalf("proof not generated while it should be generated correctly")
	}

	req = types.MetaRequest{}
	req.Payload = new(types.VoteTx)
	req.Payload.Proof = resp.Siblings
	req.Method = "submitEnvelope"
	req.Timestamp = int32(time.Now().Unix())
	req.Payload.Nonce = strconv.Itoa(rint)
	req.Payload.ProcessID = processID

	// generate envelope vote-package
	votePkg := &types.VotePackageStruct{
		Nonce: req.Payload.Nonce,
		Votes: []int{1},
		Type:  "poll-vote",
	}
	voteBytes, err := json.Marshal(votePkg)
	if err != nil {
		b.Fatalf("cannot marshal vote: %s", err)
	}
	req.Payload.VotePackage = base64.StdEncoding.EncodeToString(voteBytes)
	// generate signature
	req.Payload.Signature, err = s.SignJSON(*req.Payload)
	if err != nil {
		b.Fatalf("cannot sign vote: %s", err)
	}

	// sending submitEnvelope request
	log.Info("vote payload: %+v,", req.Payload)
	log.Infof("request: %+v", req)
	resp, err = c.Request(req, nil)
	if err != nil {
		b.Fatal(err)
	}
	if !resp.Ok {
		b.Fatalf("%s failed: %s", req.Method, resp.Message)
	}
	log.Infof("response: %+v", resp)

	// check vote added
	req = types.MetaRequest{}
	req.Method = "getEnvelopeStatus"
	req.Timestamp = int32(time.Now().Unix())
	req.ProcessID = processID
	req.Nullifier, err = vochain.GenerateNullifier(s.EthAddrString(), processID)
	if err != nil {
		b.Fatal(err)
	}
	for {
		resp, err = c.Request(req, nil)
		if err != nil {
			b.Fatal(err)
		}
		if *resp.Registered {
			break
		}
		time.Sleep(time.Second)
	}
}

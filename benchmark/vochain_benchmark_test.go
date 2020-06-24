package test

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"math/rand"
	"strconv"
	"testing"
	"time"

	"github.com/ethereum/go-ethereum/crypto"
	"gitlab.com/vocdoni/go-dvote/client"
	"gitlab.com/vocdoni/go-dvote/crypto/ethereum"
	"gitlab.com/vocdoni/go-dvote/crypto/snarks"
	"gitlab.com/vocdoni/go-dvote/log"
	"gitlab.com/vocdoni/go-dvote/test/testcommon"
	"gitlab.com/vocdoni/go-dvote/types"
	"gitlab.com/vocdoni/go-dvote/vochain"
)

// THIS BENCH DOES NOT PROVIDE ANY CONSENSUS GUARANTEES

const (
	numberOfBlocks = 1000
	processID      = "0xe9d5e8d791f51179e218c606f83f5967ab272292a6dbda887853d81f7a1d5105"
)

func BenchmarkVochain(b *testing.B) {
	var dvoteServer testcommon.DvoteAPIServer
	rint := rand.Int()
	host := *hostFlag
	if host == "" {
		dvoteServer.Start(b, "file", "census", "vote")
		host = dvoteServer.PxyAddr
	}

	// create random key batch
	keySet := testcommon.CreateEthRandomKeysBatch(b, *censusSize)
	log.Infof("generated %d keys", len(keySet))

	// get signer pubkey
	signerPub, _ := dvoteServer.Signer.HexString()

	// check required components
	cl, err := client.New(host)
	if err != nil {
		b.Fatal(err)
	}

	var req types.MetaRequest
	doRequest := cl.ForTest(b, &req)

	log.Info("get info")
	resp := doRequest("getGatewayInfo", nil)
	log.Infof("apis available: %v", resp.APIList)

	// create census
	log.Infof("creating census")
	req.CensusID = fmt.Sprintf("test%d", rint)
	resp = doRequest("addCensus", dvoteServer.Signer)

	// Set correct censusID for comming requests
	req.CensusID = resp.CensusID

	// census add claims
	poseidonHashes := make([]string, len(keySet))
	for i, key := range keySet {
		hash := snarks.Poseidon.Hash(crypto.FromECDSAPub(&key.Public))
		if len(hash) == 0 {
			b.Fatalf("cannot create poseidon hash of public key: %#v", key.Public)
		}
		poseidonHashes[i] = base64.StdEncoding.EncodeToString(hash)
	}
	log.Debugf("poseidon hashes: %s", poseidonHashes)
	log.Debug("add bulk claims")
	var claims []string
	req.Digested = true
	req.ClaimData = ""
	for i := 0; i < len(poseidonHashes); i++ {
		claims = append(claims, poseidonHashes[i])
	}
	req.ClaimsData = claims
	doRequest("addClaimBulk", dvoteServer.Signer)

	// get census root
	log.Infof("get root")
	resp = doRequest("getRoot", nil)
	mkRoot := resp.Root
	if len(mkRoot) < 1 {
		b.Fatalf("got invalid root")
	}

	log.Infof("check block height is not less than process start block")
	resp = doRequest("getBlockHeight", nil)

	// create process
	process := &types.NewProcessTx{
		EntityID:       signerPub,
		MkRoot:         mkRoot,
		NumberOfBlocks: numberOfBlocks,
		ProcessID:      processID,
		ProcessType:    types.PollVote,
		StartBlock:     *resp.Height + 1,
		Type:           "newProcess",
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
	req.EntityId = process.EntityID

	for {
		resp = doRequest("getProcessList", nil)
		if resp.ProcessList[0] == "0xe9d5e8d791f51179e218c606f83f5967ab272292a6dbda887853d81f7a1d5105" {
			break
		}
		time.Sleep(time.Second)
	}

	// send votes in parallel
	b.RunParallel(func(pb *testing.PB) {
		// Create websocket client
		cl, err := client.New(host)
		if err != nil {
			b.Fatal(err)
		}

		count := 0
		for pb.Next() {
			vochainBench(b, cl, keySet[count], poseidonHashes[count], mkRoot, process.ProcessID, req.CensusID)
			count++
		}
	})

	// scrutiny of the submited envelopes
	log.Infof("get results")
	req.ProcessID = process.ProcessID
	resp = doRequest("getResults", nil)
	log.Infof("submited votes: %+v", resp.Results)

	// get entities that created at least ones process
	log.Infof("get entities")
	resp = doRequest("getScrutinizerEntities", nil)
	log.Infof("created entities: %+v", resp.EntityIDs)
}

func vochainBench(b *testing.B, cl *client.Client, s *ethereum.SignKeys, poseidon, mkRoot, processID, censusID string) {
	rint := rand.Int()
	// API requests
	var req types.MetaRequest
	doRequest := cl.ForTest(b, &req)

	// create envelope
	log.Infof("adding vote using key [%s]", s.EthAddrString())

	pub, _ := s.HexString()
	// generate envelope proof
	log.Infof("generating proof for key %s with poseidon hash: %s", pub, poseidon)
	req.CensusID = censusID
	req.RootHash = mkRoot
	req.ClaimData = poseidon
	resp := doRequest("genProof", nil)
	if len(resp.Siblings) == 0 {
		b.Fatalf("proof not generated while it should be generated correctly")
	}

	req = types.MetaRequest{}
	req.Payload = new(types.VoteTx)
	req.Payload.Proof = resp.Siblings
	req.Payload.Nonce = strconv.Itoa(rint)
	req.Payload.ProcessID = processID

	// generate envelope votePackage
	votePkg := &types.VotePackageStruct{
		Nonce: req.Payload.Nonce,
		Votes: []int{1},
		Type:  types.PollVote,
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
	resp = doRequest("submitEnvelope", nil)
	log.Infof("response: %+v", resp)

	// check vote added
	req = types.MetaRequest{}
	req.ProcessID = processID
	req.Nullifier, err = vochain.GenerateNullifier(s.EthAddrString(), processID)
	if err != nil {
		b.Fatal(err)
	}
	for {
		resp = doRequest("getEnvelopeStatus", nil)
		if *resp.Registered {
			break
		}
		time.Sleep(time.Second)
	}
}

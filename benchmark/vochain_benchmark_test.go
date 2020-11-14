package test

import (
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"math/rand"
	"strconv"
	"testing"
	"time"

	"github.com/ethereum/go-ethereum/crypto"
	models "github.com/vocdoni/dvote-protobuf/go-vocdonitypes"
	"gitlab.com/vocdoni/go-dvote/client"
	"gitlab.com/vocdoni/go-dvote/crypto/ethereum"
	"gitlab.com/vocdoni/go-dvote/crypto/snarks"
	"gitlab.com/vocdoni/go-dvote/log"
	"gitlab.com/vocdoni/go-dvote/test/testcommon"
	"gitlab.com/vocdoni/go-dvote/types"
	"gitlab.com/vocdoni/go-dvote/util"
	"gitlab.com/vocdoni/go-dvote/vochain"
	"google.golang.org/protobuf/proto"
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
	req.ClaimsData = nil
	req.Digested = false

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
	txEid, _ := hex.DecodeString(util.TrimHex(signerPub))
	txPid, _ := hex.DecodeString(util.TrimHex(processID))
	txMkRoot, _ := hex.DecodeString(util.TrimHex(mkRoot))
	processData := &models.Process{
		EntityId:     txEid,
		CensusMkRoot: txMkRoot,
		BlockCount:   numberOfBlocks,
		ProcessId:    txPid,
		ProcessType:  types.PollVote,
		StartBlock:   uint64(*resp.Height + 1),
	}
	process := &models.NewProcessTx{
		Txtype:  models.TxType_NEW_PROCESS,
		Nonce:   util.RandomHex(32),
		Process: processData,
	}

	txBytes, err := proto.Marshal(process)
	if err != nil {
		b.Fatal("cannot marshal process")
	}

	vtx := models.Tx{}
	signHex, err := dvoteServer.Signer.Sign(txBytes)
	if err != nil {
		b.Fatalf("cannot sign oracle tx: %s", err)
	}
	vtx.Signature, err = hex.DecodeString(signHex)
	if err != nil {
		b.Fatalf("cannot decode signature")
	}
	req.Payload, err = proto.Marshal(&vtx)
	if err != nil {
		b.Fatalf("error marshaling process tx: %v", err)
	}

	//	res, err := dvoteServer.VochainRPCClient.BroadcastTxSync(tx)
	//req.Method = "submitRawTx"
	resp = doRequest("submitRawTx", nil)

	if !resp.Ok {
		b.Fatalf("error broadcasting process tx: %s", resp.Message)
	} else {
		log.Info("process transaction sent")
	}

	// check if process is created
	log.Infof("check if process created")
	req.EntityId = signerPub

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
			vochainBench(b, cl, keySet[count], poseidonHashes[count], mkRoot, processID, req.CensusID)
			count++
		}
	})

	// scrutiny of the submited envelopes
	log.Infof("get results")
	req.ProcessID = processID
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
	log.Infof("adding vote using key [%s]", s.AddressString())

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

	// generate envelope votePackage
	votePkg := &types.VotePackageStruct{
		Nonce: util.RandomHex(32),
		Votes: []int{1},
		Type:  types.PollVote,
	}
	voteBytes, err := json.Marshal(votePkg)
	if err != nil {
		b.Fatalf("cannot marshal vote: %s", err)
	}
	// Generate VoteEnvelope package
	txPid, _ := hex.DecodeString(util.TrimHex(processID))
	siblings, _ := hex.DecodeString(util.TrimHex(resp.Siblings))
	tx := models.VoteEnvelope{
		Nonce:       strconv.Itoa(rint),
		ProcessId:   txPid,
		Proof:       &models.Proof{Proof: &models.Proof_Graviton{Graviton: &models.ProofGraviton{Siblings: siblings}}},
		VotePackage: voteBytes,
	}

	req = types.MetaRequest{}
	req.Payload, err = proto.Marshal(&tx)
	if err != nil {
		b.Fatal(err)
	}

	req.Signature, err = s.Sign(req.Payload)
	if err != nil {
		b.Fatal(err)
	}

	// sending submitEnvelope request
	log.Info("vote payload: %s", tx.String())
	log.Infof("request: %+v", req)
	resp = doRequest("submitEnvelope", nil)
	log.Infof("response: %+v", resp)

	// check vote added
	req = types.MetaRequest{}
	req.ProcessID = processID
	req.Nullifier = hex.EncodeToString(vochain.GenerateNullifier(s.Address(), util.Hex2byte(b, processID)))
	for {
		resp = doRequest("getEnvelopeStatus", nil)
		if *resp.Registered {
			break
		}
		time.Sleep(time.Second)
	}
}

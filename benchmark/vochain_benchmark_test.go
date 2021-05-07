package test

import (
	"encoding/json"
	"fmt"
	"sync/atomic"
	"testing"
	"time"

	"go.vocdoni.io/dvote/api"
	"go.vocdoni.io/dvote/client"
	"go.vocdoni.io/dvote/crypto/ethereum"
	"go.vocdoni.io/dvote/log"
	"go.vocdoni.io/dvote/test/testcommon"
	"go.vocdoni.io/dvote/test/testcommon/testutil"
	"go.vocdoni.io/dvote/types"
	"go.vocdoni.io/dvote/util"
	"go.vocdoni.io/dvote/vochain/scrutinizer/indexertypes"
	models "go.vocdoni.io/proto/build/go/models"
	"google.golang.org/protobuf/proto"
)

// THIS BENCH DOES NOT PROVIDE ANY CONSENSUS GUARANTEES

const (
	numberOfBlocks = 1000
	hexProcessID   = "0xe9d5e8d791f51179e218c606f83f5967ab272292a6dbda887853d81f7a1d5105"
)

func BenchmarkVochain(b *testing.B) {
	b.ReportAllocs()
	var dvoteServer testcommon.DvoteAPIServer
	rint := util.RandomInt(0, 8192)
	host := *hostFlag
	if host == "" {
		dvoteServer.Start(b, "file", "census", "vote")
		host = dvoteServer.PxyAddr
	}

	// create random key batch
	keySet := testcommon.CreateEthRandomKeysBatch(b, *censusSize)
	log.Infof("generated %d keys", len(keySet))

	// get signer pubkey
	signerPubHex, _ := dvoteServer.Signer.HexString()
	signerPub := testutil.Hex2byte(b, signerPubHex)

	// check required components
	cl, err := client.New(host)
	if err != nil {
		b.Fatal(err)
	}

	req := &api.MetaRequest{}
	zeroReq := &api.MetaRequest{}
	reset := func(r *api.MetaRequest) {
		*r = *zeroReq
	}
	doRequest := cl.ForTest(b, req)

	log.Info("get info")
	resp := doRequest("getInfo", nil)
	log.Infof("apis available: %v", resp.APIList)

	// create census
	log.Infof("creating census")
	reset(req)
	req.CensusID = fmt.Sprintf("test%d", rint)
	resp = doRequest("addCensus", dvoteServer.Signer)
	if !resp.Ok {
		b.Fatalf("error on addCensus response: %v", resp.Ok)
	}
	if len(resp.CensusID) < 32 {
		b.Fatalf("addCensus returned an invalid censusId: %x", resp.CensusID)
	}
	censusID := resp.CensusID

	// Census add claims
	log.Debug("add bulk claims")
	reset(req)
	req.CensusID = censusID
	req.Digested = false
	claims := [][]byte{}
	// Send claims by batches of 100
	for i := 0; i < *censusSize; i++ {
		claims = append(claims, keySet[i].PublicKey())
		if i%100 == 0 {
			req.CensusKeys = claims
			resp := doRequest("addClaimBulk", dvoteServer.Signer)
			if !resp.Ok {
				b.Fatalf("error adding claims: %s", resp.Message)
			}
			claims = [][]byte{}
		}
	}
	// Send remaining claims
	if len(claims) > 0 {
		req.CensusKeys = claims
		resp = doRequest("addClaimBulk", dvoteServer.Signer)
		if !resp.Ok {
			b.Fatalf("error adding claims: %s", resp.Message)
		}
	}
	// get census root
	log.Infof("get root")
	reset(req)
	req.CensusID = censusID
	resp = doRequest("getRoot", nil)
	if !resp.Ok {
		b.Fatalf("request returned an error: %s", resp.Message)
	}

	censusRoot := resp.Root
	if len(censusRoot) < 1 {
		b.Fatalf("got invalid root")
	}

	log.Infof("publish census")
	reset(req)
	req.CensusID = censusID
	resp = doRequest("publish", dvoteServer.Signer)
	if !resp.Ok {
		b.Fatalf("request returned an error: %s", resp.Message)
	}

	log.Infof("check block height is not less than process start block")
	reset(req)
	resp = doRequest("getBlockHeight", nil)
	if !resp.Ok {
		b.Fatalf("request returned an error: %s", resp.Message)
	}

	// create process
	entityID := signerPub[:types.EntityIDsize]
	processID := testutil.Hex2byte(b, hexProcessID)
	processData := &models.Process{
		EntityId:     entityID,
		CensusRoot:   censusRoot,
		CensusOrigin: models.CensusOrigin_OFF_CHAIN_TREE,
		BlockCount:   numberOfBlocks,
		ProcessId:    processID,
		StartBlock:   *resp.Height + 1,
		Status:       models.ProcessStatus_READY,
		EnvelopeType: &models.EnvelopeType{},
		Mode:         &models.ProcessMode{AutoStart: true},
		VoteOptions:  &models.ProcessVoteOptions{MaxCount: 1, MaxValue: 1},
	}
	process := &models.NewProcessTx{
		Txtype:  models.TxType_NEW_PROCESS,
		Nonce:   util.RandomBytes(32),
		Process: processData,
	}

	stx := &models.SignedTx{}
	stx.Tx, err = proto.Marshal(&models.Tx{Payload: &models.Tx_NewProcess{NewProcess: process}})
	if err != nil {
		b.Fatal("cannot marshal process")
	}
	stx.Signature, err = dvoteServer.Signer.Sign(stx.Tx)
	if err != nil {
		b.Fatalf("cannot sign oracle tx: %s", err)
	}

	reset(req)
	req.Payload, err = proto.Marshal(stx)
	if err != nil {
		b.Fatalf("error marshaling process tx: %v", err)
	}
	resp = doRequest("submitRawTx", nil)
	if !resp.Ok {
		b.Fatalf("error broadcasting process tx: %s", resp.Message)
	} else {
		log.Info("process transaction sent")
	}

	// check if process is created
	log.Infof("check if process created")
	reset(req)
	req.ProcessID = processID
	failures := 20
	for {
		resp = doRequest("getProcessInfo", nil)
		if resp.Ok {
			break
		}
		failures--
		if failures == 0 {
			b.Fatalf("processID does not exist in the blockchain")
		}
		time.Sleep(time.Second)
	}

	// send votes in parallel
	count := int32(0)
	b.ResetTimer()

	b.RunParallel(func(pb *testing.PB) {
		// Create websocket client
		cl, err := client.New(host)
		if err != nil {
			b.Fatal(err)
		}
		for pb.Next() {
			voteBench(b,
				cl,
				keySet[atomic.AddInt32(&count, 1)],
				censusRoot,
				processID)
		}
	})
}

func voteBench(b *testing.B, cl *client.Client, s *ethereum.SignKeys,
	censusRoot, processID []byte) {
	// API requests
	req := &api.MetaRequest{}
	zeroReq := &api.MetaRequest{}
	reset := func(r *api.MetaRequest) {
		*r = *zeroReq
	}
	doRequest := cl.ForTest(b, req)

	// create envelope
	log.Infof("adding vote using key [%s]", s.AddressString())

	pub, _ := s.HexString()
	// generate envelope proof
	log.Infof("generating proof for key %s", pub)
	req.CensusID = fmt.Sprintf("%x", censusRoot)
	req.CensusKey = s.PublicKey()
	req.Digested = false
	resp := doRequest("genProof", nil)
	if len(resp.Siblings) == 0 {
		b.Fatalf("proof not generated while it should be generated correctly: %v", resp.Message)
	}

	// generate envelope votePackage
	votePkg := &indexertypes.VotePackage{
		Nonce: fmt.Sprintf("%x", util.RandomHex(32)),
		Votes: []int{1},
	}
	voteBytes, err := json.Marshal(votePkg)
	if err != nil {
		b.Fatalf("cannot marshal vote: %s", err)
	}
	// Generate VoteEnvelope package
	tx := &models.VoteEnvelope{
		Nonce:     util.RandomBytes(32),
		ProcessId: processID,
		Proof: &models.Proof{
			Payload: &models.Proof_Graviton{
				Graviton: &models.ProofGraviton{
					Siblings: resp.Siblings,
				},
			},
		},
		VotePackage: voteBytes,
	}
	// Create transaction package and sign it
	stx := &models.SignedTx{}
	stx.Tx, err = proto.Marshal(&models.Tx{Payload: &models.Tx_Vote{Vote: tx}})
	if err != nil {
		b.Fatal(err)
	}
	stx.Signature, err = s.Sign(stx.Tx)
	if err != nil {
		b.Fatal(err)
	}

	reset(req)
	req.Payload, err = proto.Marshal(stx)
	if err != nil {
		b.Fatal(err)
	}
	// sending submitEnvelope request
	resp = doRequest("submitRawTx", nil)
	log.Infof("response: %s", resp.String())

	// check vote added
	/*	reset(req)
		req.ProcessID = processID
		req.Nullifier = vochain.GenerateNullifier(s.Address(), processID)
		for {
			resp = doRequest("getEnvelopeStatus", nil)
			if *resp.Registered {
				break
			}
			time.Sleep(time.Second * 2)
		}
	*/
}

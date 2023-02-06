package test

import (
	"encoding/json"
	"fmt"
	"sync/atomic"
	"testing"
	"time"

	"go.vocdoni.io/dvote/crypto/ethereum"
	"go.vocdoni.io/dvote/log"
	client "go.vocdoni.io/dvote/rpcclient"
	api "go.vocdoni.io/dvote/rpctypes"
	"go.vocdoni.io/dvote/test/testcommon"
	"go.vocdoni.io/dvote/types"
	"go.vocdoni.io/dvote/util"
	"go.vocdoni.io/dvote/vochain"
	models "go.vocdoni.io/proto/build/go/models"
	"google.golang.org/protobuf/proto"
)

// THIS BENCH DOES NOT PROVIDE ANY CONSENSUS GUARANTEES

const (
	numberOfBlocks = 1000
	hexProcessID   = "0xe9d5e8d791f51179e218c606f83f5967ab272292a6dbda887853d81f7a1d5105"
)

func BenchmarkVochainBatchProof(b *testing.B) {
	log.Init("info", "stdout")
	b.ReportAllocs()
	var dvoteServer testcommon.DvoteAPIServer
	host := *hostFlag
	if host == "" {
		dvoteServer.Start(b, "file", "census", "vote")
		host = dvoteServer.ListenAddr
	}
	cl, err := client.New(host)
	if err != nil {
		b.Fatal(err)
	}

	// create random key batch
	keySet := testcommon.CreateEthRandomKeysBatch(b, b.N)
	log.Infof("generated %d keys", len(keySet))

	// setup the basic parties
	entityID := dvoteServer.Signer.Address().Bytes()

	// create census and process
	censusRoot, _, processID := createCensusAndProcess(b, cl, dvoteServer, keySet, entityID)
	if err := waitProcessReady(b, cl, processID); err != nil {
		b.Fatal(err)
	}

	// batch generate proofs
	proofs, err := cl.GetMerkleProofBatch(keySet, censusRoot, false)
	if err != nil {
		b.Fatalf("could not generate batch CSP proofs: %v", err)
	}

	// send votes in parallel
	log.Infof("sending %d votes in parallel", b.N)
	count := int32(0)
	b.ResetTimer()

	b.RunParallel(func(pb *testing.PB) {
		cl, err := client.New(host)
		if err != nil {
			b.Fatal(err)
		}
		for pb.Next() {
			sendVote(b,
				cl,
				dvoteServer.VochainAPP.ChainID(),
				keySet[count],
				proofs[count],
				processID)
			atomic.AddInt32(&count, 1)
		}
	})
}

func BenchmarkVochainSingleProof(b *testing.B) {
	log.Init("info", "stdout")
	b.ReportAllocs()
	var dvoteServer testcommon.DvoteAPIServer
	host := *hostFlag
	if host == "" {
		dvoteServer.Start(b, "file", "census", "vote")
		host = dvoteServer.ListenAddr
	}
	cl, err := client.New(host)
	if err != nil {
		b.Fatal(err)
	}

	// create random key batch
	keySet := testcommon.CreateEthRandomKeysBatch(b, b.N)
	log.Infof("generated %d keys", len(keySet))

	// setup the basic parties
	entityID := dvoteServer.Signer.Address().Bytes()

	// create census and process
	censusRoot, _, processID := createCensusAndProcess(b, cl, dvoteServer, keySet, entityID)
	if err := waitProcessReady(b, cl, processID); err != nil {
		b.Fatal(err)
	}

	// send votes in parallel
	log.Infof("sending %d votes in parallel", b.N)
	count := int32(0)
	b.ResetTimer()

	b.RunParallel(func(pb *testing.PB) {
		cl, err := client.New(host)
		if err != nil {
			b.Fatal(err)
		}
		for pb.Next() {
			signer := keySet[count]
			siblings, value, err := cl.GetProof(signer.PublicKey(), censusRoot, false)
			if err != nil {
				b.Fatalf("could not get proof for %s: %s", signer.AddressString(), err)
			}
			proof := &client.Proof{
				Siblings: siblings,
				Value:    value,
			}

			sendVote(b,
				cl,
				dvoteServer.VochainAPP.ChainID(),
				signer,
				proof,
				processID,
			)
			atomic.AddInt32(&count, 1)
		}
	})
}

func sendVote(b *testing.B, cl *client.Client, chainID string, s *ethereum.SignKeys,
	proof *client.Proof, processID []byte) {
	req := &api.APIrequest{}
	doRequest := cl.ForTest(b, req)

	// Generate VotePackage and put it in VoteEnvelope
	votePkg := &vochain.VotePackage{
		Nonce: fmt.Sprintf("%x", util.RandomHex(32)),
		Votes: []int{1},
	}
	voteBytes, err := json.Marshal(votePkg)
	if err != nil {
		b.Fatalf("cannot marshal vote: %s", err)
	}

	tx := &models.VoteEnvelope{
		Nonce:     util.RandomBytes(32),
		ProcessId: processID,
		Proof: &models.Proof{
			Payload: &models.Proof_Arbo{
				Arbo: &models.ProofArbo{
					Type:       models.ProofArbo_BLAKE2B,
					LeafWeight: proof.Value,
					Siblings:   proof.Siblings,
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
	stx.Signature, err = s.SignVocdoniTx(stx.Tx, chainID)
	if err != nil {
		b.Fatal(err)
	}

	req.Reset()
	req.Payload, err = proto.Marshal(stx)
	if err != nil {
		b.Fatal(err)
	}
	// sending submitEnvelope request
	resp := doRequest("submitRawTx", nil)
	if !resp.Ok {
		b.Errorf("error submitting vote for %s: %s", s.AddressString(), resp.String())
	}
}

func waitProcessReady(b *testing.B, cl *client.Client, processID []byte) error {
	log.Info("waiting for the process to start")
	req := &api.APIrequest{}
	// reset(req)
	doRequest := cl.ForTest(b, req)
	req.ProcessID = processID
	failures := 20
	for {
		procInfo := doRequest("getProcessInfo", nil)
		if procInfo.Ok {
			if procInfo.Process.Status == int32(models.ProcessStatus_READY) {
				height := doRequest("getBlockHeight", nil)
				if *height.Height >= procInfo.Process.StartBlock {
					break
				}
			}
		}
		failures--
		if failures == 0 {
			return fmt.Errorf("processID %s should be started by now", hexProcessID)
		}
		time.Sleep(time.Second)
	}
	return nil
}

func createCensusAndProcess(b *testing.B, cl *client.Client, dvoteServer testcommon.DvoteAPIServer, keySet []*ethereum.SignKeys, entityID []byte) (censusRoot []byte, censusURI string, processID []byte) {
	// create census
	censusRoot, censusURI, err := cl.CreateCensus(
		dvoteServer.Signer,
		keySet,
		[]string{},
		[]*types.BigInt{},
	)
	if err != nil {
		b.Fatalf("failed to create the census: %v", err)
	}
	log.Infof("created census - published at %s", censusURI)

	log.Info("creating a new process")
	start, processID, err := cl.CreateProcess(
		dvoteServer.Signer,
		entityID,
		censusRoot,
		censusURI,
		&models.EnvelopeType{EncryptedVotes: false},
		nil,
		models.CensusOrigin_OFF_CHAIN_TREE,
		0,
		numberOfBlocks,
		uint64(b.N),
	)
	if err != nil {
		b.Fatalf("couldn't create process: %v", err)
	}
	log.Infof("created a new process, will start at block %d", start)
	return censusRoot, censusURI, processID
}

package test

import (
	"encoding/json"
	"fmt"
	"sync/atomic"
	"testing"

	"go.vocdoni.io/dvote/crypto/ethereum"
	"go.vocdoni.io/dvote/log"
	client "go.vocdoni.io/dvote/rpcclient"
	api "go.vocdoni.io/dvote/rpctypes"
	"go.vocdoni.io/dvote/test/testcommon"
	"go.vocdoni.io/dvote/util"
	"go.vocdoni.io/dvote/vochain"
	models "go.vocdoni.io/proto/build/go/models"
	"google.golang.org/protobuf/proto"
)

func BenchmarkVochainCSP(b *testing.B) {
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
	cspKey := ethereum.NewSignKeys()
	if err := cspKey.Generate(); err != nil {
		b.Fatal(err)
	}
	entityID := dvoteServer.Signer.Address().Bytes()

	log.Info("creating a new process")
	start, processID, err := cl.CreateProcess(
		dvoteServer.Signer,
		entityID,
		cspKey.PublicKey(),
		"https://dummycsp.foo",
		&models.EnvelopeType{EncryptedVotes: false},
		nil,
		models.CensusOrigin_OFF_CHAIN_CA,
		0,
		numberOfBlocks,
		uint64(b.N),
	)
	if err != nil {
		b.Fatalf("couldn't create process: %v", err)
	}
	log.Infof("created a new process, will start at block %d", start)

	if err := waitProcessReady(b, cl, processID); err != nil {
		b.Fatal(err)
	}

	// batch generate CSP proofs
	proofs, err := cl.GetCSPproofBatch(keySet, cspKey, processID)
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
			sendVoteCSP(b,
				cl,
				dvoteServer.VochainAPP.ChainID(),
				keySet[count],
				proofs[count],
				processID)
			atomic.AddInt32(&count, 1)
		}
	})
}

func sendVoteCSP(b *testing.B, cl *client.Client, chainID string, s *ethereum.SignKeys,
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
	p := &models.ProofCA{}
	if err := proto.Unmarshal(proof.Siblings, p); err != nil {
		b.Fatal(err)
	}
	tx := &models.VoteEnvelope{
		Nonce:     util.RandomBytes(32),
		ProcessId: processID,
		Proof: &models.Proof{
			Payload: &models.Proof_Ca{Ca: p},
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

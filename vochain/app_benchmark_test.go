package vochain

// go test -benchmem -run=^$ -bench=. -cpu=10

import (
	"encoding/hex"
	"fmt"
	"testing"

	"sync/atomic"

	abcitypes "github.com/tendermint/tendermint/abci/types"
	models "github.com/vocdoni/dvote-protobuf/build/go/models"
	"gitlab.com/vocdoni/go-dvote/crypto/ethereum"
	"gitlab.com/vocdoni/go-dvote/crypto/snarks"
	"gitlab.com/vocdoni/go-dvote/test/testcommon/testutil"
	tree "gitlab.com/vocdoni/go-dvote/trie"
	"gitlab.com/vocdoni/go-dvote/types"
	"gitlab.com/vocdoni/go-dvote/util"
	"google.golang.org/protobuf/proto"
)

const benchmarkVoters = 20

func BenchmarkCheckTx(b *testing.B) {
	b.ReportAllocs()
	app, err := NewBaseApplication(b.TempDir())
	if err != nil {
		b.Fatal(err)
	}
	var voters [][]*models.Tx
	for i := 0; i < b.N+1; i++ {
		voters = append(voters, prepareBenchCheckTx(b, app, benchmarkVoters))
		b.Logf("creating process %x", voters[i][0].GetVote().ProcessId)
	}
	var i int32
	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			b.Logf("Running vote %d", i)
			benchCheckTx(b, app, voters[atomic.AddInt32(&i, 1)])
		}
	})
}

func prepareBenchCheckTx(b *testing.B, app *BaseApplication, nvoters int) (voters []*models.Tx) {
	tr, err := tree.NewTree("checkTXbench", b.TempDir())
	if err != nil {
		b.Fatal(err)
	}

	keys := createEthRandomKeysBatch(nvoters)
	if keys == nil {
		b.Fatal("cannot create keys batch")
	}
	claims := []string{}
	for _, k := range keys {
		pub, _ := k.HexString()
		pub, err = ethereum.DecompressPubKey(pub)
		if err != nil {
			b.Fatal(err)
		}
		pubb, err := hex.DecodeString(pub)
		if err != nil {
			b.Fatal(err)
		}
		c := snarks.Poseidon.Hash(pubb)
		tr.AddClaim(c, nil)
		claims = append(claims, string(c))
	}
	mkuri := "ipfs://123456789"
	pid := util.RandomHex(types.ProcessIDsize)
	process := &models.Process{
		ProcessId:    pid,
		StartBlock:   0,
		EnvelopeType: &models.EnvelopeType{EncryptedVotes: true},
		Mode:         &models.ProcessMode{},
		Status:       models.ProcessStatus_READY,
		EntityId:     util.RandomBytes(types.EntityIDsize),
		CensusMkRoot: testutil.Hex2byte(b, tr.Root()),
		CensusMkURI:  &mkuri,
		CensusOrigin: models.CensusOrigin_OFF_CHAIN,
		BlockCount:   1024,
	}
	app.State.AddProcess(process)

	var proof string

	for i, s := range keys {
		proof, err = tr.GenProof([]byte(claims[i]), nil)
		if err != nil {
			b.Fatal(err)
		}
		tx := &models.VoteEnvelope{
			Nonce:     util.RandomHex(16),
			ProcessId: pid,
			Proof:     &models.Proof{Payload: &models.Proof_Graviton{Graviton: &models.ProofGraviton{Siblings: testutil.Hex2byte(b, proof)}}},
		}

		txBytes, err := proto.Marshal(tx)
		if err != nil {
			b.Fatal(err)
		}
		signHex := ""
		if signHex, err = s.Sign(txBytes); err != nil {
			b.Fatal(err)
		}
		vtx := models.Tx{}
		vtx.Signature, err = hex.DecodeString(signHex)
		if err != nil {
			b.Fatal(err)
		}
		vtx.Payload = &models.Tx_Vote{Vote: tx}
		voters = append(voters, &vtx)
	}
	return voters
}

func benchCheckTx(b *testing.B, app *BaseApplication, voters []*models.Tx) {
	var cktx abcitypes.RequestCheckTx
	var detx abcitypes.RequestDeliverTx

	var cktxresp abcitypes.ResponseCheckTx
	var detxresp abcitypes.ResponseDeliverTx

	var err error
	var txBytes []byte

	i := 0
	for _, tx := range voters {
		if txBytes, err = proto.Marshal(tx); err != nil {
			b.Fatal(err)
		}
		cktx.Tx = txBytes
		cktxresp = app.CheckTx(cktx)
		if cktxresp.Code != 0 {
			b.Fatalf(fmt.Sprintf("checkTX failed: %s", cktxresp.Data))
		} else {
			detx.Tx = txBytes
			detxresp = app.DeliverTx(detx)
			if detxresp.Code != 0 {
				b.Fatalf(fmt.Sprintf("deliverTX failed: %s", detxresp.Data))
			}
		}
		i++
		if i%100 == 0 {
			app.Commit()
		}
	}
	app.Commit()
}

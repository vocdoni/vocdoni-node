package vochain

// go test -benchmem -run=^$ -bench=. -cpu=10

import (
	"encoding/hex"
	"fmt"
	"testing"

	"sync/atomic"

	abcitypes "github.com/tendermint/tendermint/abci/types"
	"gitlab.com/vocdoni/go-dvote/crypto/ethereum"
	"gitlab.com/vocdoni/go-dvote/crypto/snarks"
	tree "gitlab.com/vocdoni/go-dvote/trie"
	"gitlab.com/vocdoni/go-dvote/types"
	models "gitlab.com/vocdoni/go-dvote/types/proto"
	"gitlab.com/vocdoni/go-dvote/util"
	"google.golang.org/protobuf/proto"
)

const benchmarkVoters = 20

func BenchmarkCheckTx(b *testing.B) {
	b.ReportAllocs()
	app, err := NewBaseApplication(tempDir(b, "vochain_checkTxTest"))
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

func prepareBenchCheckTx(t *testing.B, app *BaseApplication, nvoters int) (voters []*models.Tx) {
	tr, err := tree.NewTree("checkTXbench", tempDir(t, "vochain_checkTxTest_db"))
	if err != nil {
		t.Fatal(err)
	}

	keys := createEthRandomKeysBatch(nvoters)
	if keys == nil {
		t.Fatal("cannot create keys batch")
	}
	claims := []string{}
	for _, k := range keys {
		pub, _ := k.HexString()
		pub, err = ethereum.DecompressPubKey(pub)
		if err != nil {
			t.Fatal(err)
		}
		pubb, err := hex.DecodeString(pub)
		if err != nil {
			t.Fatal(err)
		}
		c := snarks.Poseidon.Hash(pubb)
		tr.AddClaim(c, nil)
		claims = append(claims, string(c))
	}
	process := &types.Process{
		StartBlock:     0,
		Type:           types.PollVote,
		EntityID:       util.RandomBytes(types.EntityIDsize),
		MkRoot:         tr.Root(),
		NumberOfBlocks: 1024,
	}
	pid := util.RandomBytes(types.ProcessIDsize)
	app.State.AddProcess(*process, pid, "ipfs://123456789")

	var proof string

	for i, s := range keys {
		proof, err = tr.GenProof([]byte(claims[i]), nil)
		if err != nil {
			t.Fatal(err)
		}
		tx := &models.VoteEnvelope{
			Nonce:     util.RandomHex(16),
			ProcessId: pid,
			Proof:     &models.Proof{Proof: &models.Proof_Graviton{Graviton: &models.ProofGraviton{Siblings: util.Hex2byte(t, proof)}}},
		}

		txBytes, err := proto.Marshal(tx)
		if err != nil {
			t.Fatal(err)
		}
		signHex := ""
		if signHex, err = s.Sign(txBytes); err != nil {
			t.Fatal(err)
		}
		vtx := models.Tx{}
		vtx.Signature, err = hex.DecodeString(signHex)
		if err != nil {
			t.Fatal(err)
		}
		vtx.Tx = &models.Tx_Vote{Vote: tx}
		voters = append(voters, &vtx)
	}
	return voters
}

func benchCheckTx(t *testing.B, app *BaseApplication, voters []*models.Tx) {
	var cktx abcitypes.RequestCheckTx
	var detx abcitypes.RequestDeliverTx

	var cktxresp abcitypes.ResponseCheckTx
	var detxresp abcitypes.ResponseDeliverTx

	var err error
	var txBytes []byte

	i := 0
	for _, tx := range voters {
		if txBytes, err = proto.Marshal(tx); err != nil {
			t.Fatal(err)
		}
		cktx.Tx = txBytes
		cktxresp = app.CheckTx(cktx)
		if cktxresp.Code != 0 {
			t.Fatalf(fmt.Sprintf("checkTX failed: %s\n%s", cktxresp.Data, tx.String()))
		} else {
			detx.Tx = txBytes
			detxresp = app.DeliverTx(detx)
			if detxresp.Code != 0 {
				t.Fatalf(fmt.Sprintf("deliverTX failed: %s\n%s", detxresp.Data, tx.String()))
			}
		}
		i++
		if i%100 == 0 {
			app.Commit()
		}
	}
	app.Commit()
}

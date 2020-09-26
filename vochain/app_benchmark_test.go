package vochain

// go test -benchmem -run=^$ -bench=. -cpu=4 -benchtime=50x -parallel=50

import (
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"testing"

	"sync/atomic"

	abcitypes "github.com/tendermint/tendermint/abci/types"
	"gitlab.com/vocdoni/go-dvote/crypto/ethereum"
	"gitlab.com/vocdoni/go-dvote/crypto/snarks"
	"gitlab.com/vocdoni/go-dvote/db"
	tree "gitlab.com/vocdoni/go-dvote/tree"
	"gitlab.com/vocdoni/go-dvote/types"
)

func BenchmarkCheckTx(b *testing.B) {
	b.ReportAllocs()
	app, err := NewBaseApplication(tmpDirBench(b, "vochain_checkTxTest"))
	if err != nil {
		b.Fatal(err)
	}
	var voters [][]*types.VoteTx
	for i := 0; i < b.N+1; i++ {
		voters = append(voters, prepareBenchCheckTx(b, app, 1000))
		b.Logf("creating process %s", voters[i][0].ProcessID)
	}
	var i int32
	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			benchCheckTx(b, app, voters[atomic.AddInt32(&i, 1)])
			b.Logf("Running vote %d", i)
		}
	})
}

func prepareBenchCheckTx(t *testing.B, app *BaseApplication, nvoters int) (voters []*types.VoteTx) {
	treeStorage, err := db.NewIden3Storage(tempDir(t, "vochain_checkTxTest_db"))
	if err != nil {
		t.Fatal(err)
	}
	tr, err := tree.NewTree(treeStorage)
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
		EntityID:       randomHex(entityIDsize),
		MkRoot:         tr.Root(),
		NumberOfBlocks: 1024,
	}
	pid := randomHex(processIDsize)
	app.State.AddProcess(*process, pid, "ipfs://123456789")

	var proof string

	for i, s := range keys {
		proof, err = tr.GenProof([]byte(claims[i]), nil)
		if err != nil {
			t.Fatal(err)
		}
		tx := types.VoteTx{
			Nonce:     randomHex(16),
			ProcessID: pid,
			Proof:     proof,
		}

		txBytes, err := json.Marshal(tx)
		if err != nil {
			t.Fatal(err)
		}
		if tx.Signature, err = s.Sign(txBytes); err != nil {
			t.Fatal(err)
		}
		tx.Type = "vote"
		voters = append(voters, &tx)
	}
	return voters
}

func benchCheckTx(t *testing.B, app *BaseApplication, voters []*types.VoteTx) {
	var cktx abcitypes.RequestCheckTx
	var detx abcitypes.RequestDeliverTx

	var cktxresp abcitypes.ResponseCheckTx
	var detxresp abcitypes.ResponseDeliverTx

	var err error
	var txBytes []byte

	i := 0
	for _, tx := range voters {
		if txBytes, err = json.Marshal(tx); err != nil {
			t.Fatal(err)
		}
		cktx.Tx = txBytes
		cktxresp = app.CheckTx(cktx)
		if cktxresp.Code != 0 {
			t.Fatalf(fmt.Sprintf("checkTX failed: %s", cktxresp.Data))
		} else {
			detx.Tx = txBytes
			detxresp = app.DeliverTx(detx)
			if detxresp.Code != 0 {
				t.Fatalf(fmt.Sprintf("deliverTX failed: %s", detxresp.Data))
			}
		}
		i++
		if i%100 == 0 {
			app.Commit()
		}
	}
	app.Commit()
}

func tmpDirBench(tb *testing.B, name string) string {
	tb.Helper()
	dir, err := ioutil.TempDir("", name)
	if err != nil {
		tb.Fatal(err)
	}
	tb.Cleanup(func() { os.RemoveAll(dir) })
	return dir
}

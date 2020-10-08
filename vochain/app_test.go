package vochain

import (
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"testing"

	abcitypes "github.com/tendermint/tendermint/abci/types"
	"gitlab.com/vocdoni/go-dvote/crypto/ethereum"
	"gitlab.com/vocdoni/go-dvote/crypto/snarks"
	tree "gitlab.com/vocdoni/go-dvote/trie"
	"gitlab.com/vocdoni/go-dvote/types"
	"gitlab.com/vocdoni/go-dvote/util"
)

func TestCheckTX(t *testing.T) {
	app, err := NewBaseApplication(tempDir(t, "vochain_checkTxTest"))
	if err != nil {
		t.Fatal(err)
	}

	tr, err := tree.NewTree("testchecktx", tempDir(t, "vochain_checkTxTest_db"))
	if err != nil {
		t.Fatal(err)
	}

	keys := createEthRandomKeysBatch(1000)
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
	pid := util.RandomHex(types.ProcessIDsize)
	t.Logf("adding process %+v", process)
	app.State.AddProcess(*process, util.Hex2byte(t, pid), "ipfs://123456789")

	var cktx abcitypes.RequestCheckTx
	var detx abcitypes.RequestDeliverTx

	var cktxresp abcitypes.ResponseCheckTx
	var detxresp abcitypes.ResponseDeliverTx

	var tx types.VoteTx
	var proof string

	for i, s := range keys {
		proof, err = tr.GenProof([]byte(claims[i]), nil)
		if err != nil {
			t.Fatal(err)
		}
		tx = types.VoteTx{
			Nonce:     util.RandomHex(16),
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
		if txBytes, err = json.Marshal(tx); err != nil {
			t.Fatal(err)
		}
		cktx.Tx = txBytes
		cktxresp = app.CheckTx(cktx)
		if cktxresp.Code != 0 {
			t.Fatalf(fmt.Sprintf("checkTX failed: %s", cktxresp.Data))
		}
		detx.Tx = txBytes
		detxresp = app.DeliverTx(detx)
		if detxresp.Code != 0 {
			t.Fatalf(fmt.Sprintf("deliverTX failed: %s", detxresp.Data))
		}
		app.Commit()
	}

}

func tempDir(tb testing.TB, name string) string {
	tb.Helper()
	dir, err := ioutil.TempDir("", name+"*")
	if err != nil {
		tb.Fatal(err)
	}
	tb.Cleanup(func() { os.RemoveAll(dir) })
	return dir
}

// CreateEthRandomKeysBatch creates a set of eth random signing keys
func createEthRandomKeysBatch(n int) []*ethereum.SignKeys {
	s := make([]*ethereum.SignKeys, n)
	for i := 0; i < n; i++ {
		s[i] = ethereum.NewSignKeys()
		if err := s[i].Generate(); err != nil {
			panic(err)
		}
	}
	return s
}

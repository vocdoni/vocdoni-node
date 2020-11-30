package vochain

import (
	"encoding/hex"
	"fmt"
	"testing"

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

func TestCheckTX(t *testing.T) {
	app, err := NewBaseApplication(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}

	tr, err := tree.NewTree("testchecktx", t.TempDir())
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
	mkuri := "ipfs://123456789"
	pid := util.RandomHex(types.ProcessIDsize)
	process := &models.Process{
		ProcessId:    pid,
		StartBlock:   0,
		EnvelopeType: &models.EnvelopeType{EncryptedVotes: false},
		Mode:         &models.ProcessMode{},
		Status:       models.ProcessStatus_READY,
		EntityId:     util.RandomBytes(types.EntityIDsize),
		CensusMkRoot: testutil.Hex2byte(t, tr.Root()),
		CensusMkURI:  &mkuri,
		CensusOrigin: models.CensusOrigin_OFF_CHAIN,
		BlockCount:   1024,
	}
	t.Logf("adding process %s", process.String())
	app.State.AddProcess(process)

	var cktx abcitypes.RequestCheckTx
	var detx abcitypes.RequestDeliverTx

	var cktxresp abcitypes.ResponseCheckTx
	var detxresp abcitypes.ResponseDeliverTx

	var vtx models.Tx
	var proof string
	var hexsignature string
	vp := []byte("[1,2,3,4]")
	for i, s := range keys {
		proof, err = tr.GenProof([]byte(claims[i]), nil)
		if err != nil {
			t.Fatal(err)
		}
		tx := &models.VoteEnvelope{
			Nonce:       util.RandomHex(32),
			ProcessId:   pid,
			Proof:       &models.Proof{Payload: &models.Proof_Graviton{Graviton: &models.ProofGraviton{Siblings: testutil.Hex2byte(t, proof)}}},
			VotePackage: vp,
		}
		txBytes, err := proto.Marshal(tx)
		if err != nil {
			t.Fatal(err)
		}
		if hexsignature, err = s.Sign(txBytes); err != nil {
			t.Fatal(err)
		}
		vtx.Payload = &models.Tx_Vote{Vote: tx}
		vtx.Signature = testutil.Hex2byte(t, hexsignature)

		if cktx.Tx, err = proto.Marshal(&vtx); err != nil {
			t.Fatal(err)
		}
		cktxresp = app.CheckTx(cktx)
		if cktxresp.Code != 0 {
			t.Fatalf(fmt.Sprintf("checkTX failed: %s", cktxresp.Data))
		}
		if detx.Tx, err = proto.Marshal(&vtx); err != nil {
			t.Fatal(err)
		}
		detxresp = app.DeliverTx(detx)
		if detxresp.Code != 0 {
			t.Fatalf(fmt.Sprintf("deliverTX failed: %s", detxresp.Data))
		}
		app.Commit()
	}
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

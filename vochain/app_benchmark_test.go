package vochain

// go test -benchmem -run=^$ -bench=. -cpu=10

import (
	"fmt"
	"sync/atomic"
	"testing"

	qt "github.com/frankban/quicktest"
	abcitypes "github.com/tendermint/tendermint/abci/types"
	"go.vocdoni.io/dvote/censustree"
	"go.vocdoni.io/dvote/db"
	"go.vocdoni.io/dvote/db/badgerdb"
	"go.vocdoni.io/dvote/types"
	"go.vocdoni.io/dvote/util"
	models "go.vocdoni.io/proto/build/go/models"
	"google.golang.org/protobuf/proto"
)

const benchmarkVoters = 2000

func BenchmarkCheckTx(b *testing.B) {
	b.ReportAllocs()
	tdir := b.TempDir()
	app, err := NewBaseApplication(db.TypePebble, tdir)
	if err != nil {
		b.Fatal(err)
	}
	var voters [][]*models.SignedTx
	for i := 0; i < b.N+1; i++ {
		voters = append(voters, prepareBenchCheckTx(b, app, benchmarkVoters, tdir))
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

func prepareBenchCheckTx(b *testing.B, app *BaseApplication,
	nvoters int, tmpDir string) (voters []*models.SignedTx) {
	db, err := badgerdb.New(db.Options{Path: tmpDir})
	qt.Assert(b, err, qt.IsNil)
	tr, err := censustree.New(censustree.Options{Name: util.RandomHex(12), ParentDB: db,
		MaxLevels: 256, CensusType: models.Census_ARBO_BLAKE2B})
	if err != nil {
		b.Fatal(err)
	}

	keys := util.CreateEthRandomKeysBatch(nvoters)
	if keys == nil {
		b.Fatal("cannot create keys batch")
	}
	claims := []string{}
	for _, k := range keys {
		c := k.PublicKey()
		if err := tr.Add(c, nil); err != nil {
			b.Error(err)
		}
		claims = append(claims, string(c))
	}
	censusURI := ipfsUrl
	pid := util.RandomBytes(types.ProcessIDsize)
	root, err := tr.Root()
	if err != nil {
		b.Fatal(err)
	}

	if err := app.State.AddProcess(&models.Process{
		ProcessId:    pid,
		StartBlock:   0,
		EnvelopeType: &models.EnvelopeType{EncryptedVotes: false},
		Mode:         &models.ProcessMode{Interruptible: true, AutoStart: true},
		Status:       models.ProcessStatus_READY,
		EntityId:     util.RandomBytes(types.EthereumAddressSize),
		CensusRoot:   root,
		CensusURI:    &censusURI,
		CensusOrigin: models.CensusOrigin_OFF_CHAIN_TREE,
		BlockCount:   1024,
	}); err != nil {
		b.Error(err)
	}

	var proof []byte
	for i, s := range keys {
		_, proof, err = tr.GenProof([]byte(claims[i]))
		if err != nil {
			b.Fatal(err)
		}
		tx := &models.VoteEnvelope{
			Nonce:     util.RandomBytes(16),
			ProcessId: pid,
			Proof: &models.Proof{Payload: &models.Proof_Graviton{
				Graviton: &models.ProofGraviton{Siblings: proof}}},
			VotePackage: []byte("{[\"1\",\"2\",\"3\"]}"),
		}

		stx := models.SignedTx{}
		if stx.Tx, err = proto.Marshal(&models.Tx{Payload: &models.Tx_Vote{Vote: tx}}); err != nil {
			b.Fatal(err)
		}
		if stx.Signature, err = s.Sign(stx.Tx); err != nil {
			b.Fatal(err)
		}
		voters = append(voters, &stx)
	}
	return voters
}

func benchCheckTx(b *testing.B, app *BaseApplication, voters []*models.SignedTx) {
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

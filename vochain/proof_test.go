package vochain

import (
	"fmt"
	"testing"

	abcitypes "github.com/tendermint/tendermint/abci/types"
	"go.vocdoni.io/dvote/censustree"
	"go.vocdoni.io/dvote/crypto/ethereum"
	"go.vocdoni.io/dvote/db/metadb"
	"go.vocdoni.io/dvote/log"
	"go.vocdoni.io/dvote/types"
	"go.vocdoni.io/dvote/util"
	models "go.vocdoni.io/proto/build/go/models"
	"google.golang.org/protobuf/proto"
)

func TestMerkleTreeProof(t *testing.T) {
	app := TestBaseApplication(t)

	db := metadb.NewTest(t)
	tr, err := censustree.New(censustree.Options{Name: "testchecktx", ParentDB: db,
		MaxLevels: 256, CensusType: models.Census_ARBO_BLAKE2B})
	if err != nil {
		t.Fatal(err)
	}

	keys := util.CreateEthRandomKeysBatch(1001)
	proofs := []string{}
	for _, k := range keys {
		c := k.PublicKey()
		if err := tr.Add(c, nil); err != nil {
			t.Fatal(err)
		}
		proofs = append(proofs, string(c))
	}
	// We save the last key for the next test
	lastKey := keys[len(keys)-1]
	keys = keys[:len(keys)-2]
	lastProof := proofs[len(proofs)-1]
	proofs = proofs[:len(proofs)-2]

	censusURI := ipfsUrl
	pid := util.RandomBytes(types.ProcessIDsize)
	root, err := tr.Root()
	if err != nil {
		t.Fatal(err)
	}
	process := &models.Process{
		ProcessId:    pid,
		StartBlock:   0,
		EnvelopeType: &models.EnvelopeType{EncryptedVotes: false},
		Mode:         &models.ProcessMode{},
		Status:       models.ProcessStatus_READY,
		EntityId:     util.RandomBytes(types.EthereumAddressSize),
		CensusRoot:   root,
		CensusURI:    &censusURI,
		CensusOrigin: models.CensusOrigin_OFF_CHAIN_TREE,
		BlockCount:   1024,
	}
	t.Logf("adding process %s", log.FormatProto(process))
	if err := app.State.AddProcess(process); err != nil {
		t.Fatal(err)
	}

	var cktx abcitypes.RequestCheckTx
	var detx abcitypes.RequestDeliverTx

	var cktxresp abcitypes.ResponseCheckTx
	var detxresp abcitypes.ResponseDeliverTx

	var stx models.SignedTx
	var proof []byte
	vp := []byte("[1,2,3,4]")
	for i, s := range keys {
		_, proof, err = tr.GenProof([]byte(proofs[i]))
		if err != nil {
			t.Fatal(err)
		}
		tx := &models.VoteEnvelope{
			Nonce:     util.RandomBytes(32),
			ProcessId: pid,
			Proof: &models.Proof{
				Payload: &models.Proof_Arbo{
					Arbo: &models.ProofArbo{
						Type:     models.ProofArbo_BLAKE2B,
						Siblings: proof,
					},
				},
			},
			VotePackage: vp,
		}

		if stx.Tx, err = proto.Marshal(&models.Tx{
			Payload: &models.Tx_Vote{Vote: tx},
		}); err != nil {
			t.Fatal(err)
		}

		if stx.Signature, err = s.Sign(stx.Tx); err != nil {
			t.Fatal(err)
		}

		if cktx.Tx, err = proto.Marshal(&stx); err != nil {
			t.Fatal(err)
		}
		cktxresp = app.CheckTx(cktx)
		if cktxresp.Code != 0 {
			t.Fatalf(fmt.Sprintf("checkTX failed: %s", cktxresp.Data))
		}
		if detx.Tx, err = proto.Marshal(&stx); err != nil {
			t.Fatal(err)
		}
		detxresp = app.DeliverTx(detx)
		if detxresp.Code != 0 {
			t.Fatalf(fmt.Sprintf("deliverTX failed: %s", detxresp.Data))
		}
		app.Commit()
	}

	// Test send the same vote package multiple times with different nonce
	_, proof, err = tr.GenProof([]byte(lastProof))
	if err != nil {
		t.Fatal(err)
	}
	tx := &models.VoteEnvelope{
		ProcessId: pid,
		Proof: &models.Proof{
			Payload: &models.Proof_Arbo{
				Arbo: &models.ProofArbo{
					Type:     models.ProofArbo_BLAKE2B,
					Siblings: proof,
				},
			},
		},
		VotePackage: vp,
	}

	for i := 0; i < 100; i++ {
		tx.Nonce = util.RandomBytes(32)
		if stx.Tx, err = proto.Marshal(&models.Tx{
			Payload: &models.Tx_Vote{Vote: tx},
		}); err != nil {
			t.Fatal(err)
		}

		if stx.Signature, err = lastKey.Sign(stx.Tx); err != nil {
			t.Fatal(err)
		}

		if cktx.Tx, err = proto.Marshal(&stx); err != nil {
			t.Fatal(err)
		}
		cktxresp = app.CheckTx(cktx)
		if i == 0 && cktxresp.Code != 0 {
			t.Fatalf("checkTx returned err on first valid vote: %s", cktxresp.Data)
		}
		if i > 0 && cktxresp.Code == 0 {
			t.Fatalf("checkTx returned 0, an error was expected")
		}

		if detx.Tx, err = proto.Marshal(&stx); err != nil {
			t.Fatal(err)
		}
		detxresp = app.DeliverTx(detx)
		if i == 0 && detxresp.Code != 0 {
			t.Fatalf("devlierTx returned err on first valid vote: %s", detxresp.Data)
		}
		if i > 0 && detxresp.Code == 0 {
			t.Fatalf("deliverTx returned 0, an error was expected")
		}

	}
}

func TestCAProof(t *testing.T) {
	app := TestBaseApplication(t)
	ca := ethereum.SignKeys{}
	if err := ca.Generate(); err != nil {
		t.Fatal(err)
	}
	pid := util.RandomBytes(types.ProcessIDsize)
	process := &models.Process{
		ProcessId:    pid,
		StartBlock:   0,
		EnvelopeType: &models.EnvelopeType{EncryptedVotes: false},
		Mode:         new(models.ProcessMode),
		Status:       models.ProcessStatus_READY,
		EntityId:     util.RandomBytes(types.EthereumAddressSize),
		CensusRoot:   ca.PublicKey(),
		CensusOrigin: models.CensusOrigin_OFF_CHAIN_CA,
		BlockCount:   1024,
	}
	t.Logf("adding process %x", process.ProcessId)
	if err := app.State.AddProcess(process); err != nil {
		t.Fatal(err)
	}
	// Test 20 valid votes
	vp := []byte("[1,2,3,4]")
	keys := util.CreateEthRandomKeysBatch(20)
	for _, k := range keys {
		bundle := &models.CAbundle{
			ProcessId: pid,
			Address:   k.Address().Bytes(),
		}
		bundleBytes, err := proto.Marshal(bundle)
		if err != nil {
			t.Fatal(err)
		}
		signature, err := ca.Sign(bundleBytes)
		if err != nil {
			t.Fatal(err)
		}

		proof := &models.ProofCA{
			Bundle:    bundle,
			Type:      models.ProofCA_ECDSA,
			Signature: signature,
		}
		testCASendVotes(t, pid, vp, k, proof, app, true)
	}

	// Test invalid vote
	k := ethereum.SignKeys{}
	if err := k.Generate(); err != nil {
		t.Fatal(err)
	}
	bundle := &models.CAbundle{
		ProcessId: pid,
		Address:   k.Address().Bytes(),
	}
	bundleBytes, err := proto.Marshal(bundle)
	if err != nil {
		t.Fatal(err)
	}
	ca2 := ethereum.SignKeys{}
	if err := ca2.Generate(); err != nil {
		t.Fatal(err)
	}
	signature, err := ca2.Sign(bundleBytes)
	if err != nil {
		t.Fatal(err)
	}
	proof := &models.ProofCA{
		Bundle:    bundle,
		Type:      models.ProofCA_ECDSA,
		Signature: signature,
	}
	testCASendVotes(t, pid, vp, &k, proof, app, false)
}

/*
func TestCABlindProof(t *testing.T) {
	app := TestBaseApplication(t)
	if err != nil {
		t.Fatal(err)
	}
	ca := ethereum.SignKeys{}
	if err := ca.Generate(); err != nil {
		t.Fatal(err)
	}
	pid := util.RandomBytes(types.ProcessIDsize)
	process := &models.Process{
		ProcessId:    pid,
		StartBlock:   0,
		EnvelopeType: &models.EnvelopeType{EncryptedVotes: false},
		Mode:         new(models.ProcessMode),
		Status:       models.ProcessStatus_READY,
		EntityId:     util.RandomBytes(types.EthereumAddressSize),
		CensusRoot:   ca.PublicKey(),
		CensusOrigin: models.CensusOrigin_OFF_CHAIN_CA,
		BlockCount:   1024,
	}
	t.Logf("adding process %x", process.ProcessId)
	if err := app.State.AddProcess(process); err != nil {
		t.Fatal(err)
	}

	// Create the CA for generating the proofs
	_, capriv := ca.HexString()
	bca := new(blindca.BlindCA)
	if err := bca.Init(capriv, nil); err != nil {
		t.Fatal(err)
	}

	// Test 20 valid votes
	vp := []byte("[1,2,3,4]")
	keys := util.CreateEthRandomKeysBatch(20)
	for _, k := range keys {
		bundle := &models.CAbundle{
			ProcessId: pid,
			Address:   k.Address().Bytes(),
		}
		bundleBytes, err := proto.Marshal(bundle)
		if err != nil {
			t.Fatal(err)
		}

		// Perform blind signature with CA
		r := bca.NewBlindRequestKey()
		m := new(big.Int).SetBytes(ethereum.HashRaw(bundleBytes))
		if err != nil {
			t.Fatal(err)
		}
		msgBlinded, secret := blind.Blind(m, r)
		bsig, err := bca.SignBlind(r, msgBlinded.Bytes())
		if err != nil {
			t.Error(err)
		}
		signature := blind.Unblind(new(big.Int).SetBytes(bsig), secret).Bytes()

		// Pack the proof
		proof := &models.ProofCA{
			Bundle:    bundle,
			Type:      models.ProofCA_ECDSA_BLIND,
			Signature: signature,
		}
		testCASendVotes(t, pid, vp, k, proof, app, true)
	}

	// Test invalid vote
	k := ethereum.SignKeys{}
	if err := k.Generate(); err != nil {
		t.Fatal(err)
	}
	bundle := &models.CAbundle{
		ProcessId: pid,
		Address:   k.Address().Bytes(),
	}
	bundleBytes, err := proto.Marshal(bundle)
	if err != nil {
		t.Fatal(err)
	}
	ca2 := ethereum.SignKeys{}
	if err := ca2.Generate(); err != nil {
		t.Fatal(err)
	}
	signature, err := ca2.Sign(bundleBytes)
	if err != nil {
		t.Fatal(err)
	}
	proof := &models.ProofCA{
		Bundle:    bundle,
		Type:      models.ProofCA_ECDSA,
		Signature: signature,
	}
	testCASendVotes(t, pid, vp, &k, proof, app, false)
}
*/

func testCASendVotes(t *testing.T, pid []byte, vp []byte, signer *ethereum.SignKeys,
	proof *models.ProofCA, app *BaseApplication, expectedResult bool) {
	var cktx abcitypes.RequestCheckTx
	var detx abcitypes.RequestDeliverTx
	var cktxresp abcitypes.ResponseCheckTx
	var detxresp abcitypes.ResponseDeliverTx
	var stx models.SignedTx
	var err error

	t.Logf("voting %s", signer.AddressString())
	tx := &models.VoteEnvelope{
		Nonce:       util.RandomBytes(32),
		ProcessId:   pid,
		Proof:       &models.Proof{Payload: &models.Proof_Ca{Ca: proof}},
		VotePackage: vp,
	}

	pub, _ := signer.HexString()
	t.Logf("addr: %s pubKey: %s", signer.Address(), pub)

	if stx.Tx, err = proto.Marshal(&models.Tx{
		Payload: &models.Tx_Vote{
			Vote: tx,
		},
	}); err != nil {
		t.Fatal(err)
	}

	if stx.Signature, err = signer.Sign(stx.Tx); err != nil {
		t.Fatal(err)
	}

	if cktx.Tx, err = proto.Marshal(&stx); err != nil {
		t.Fatal(err)
	}
	cktxresp = app.CheckTx(cktx)
	if cktxresp.Code != 0 {
		if expectedResult {
			t.Fatalf(fmt.Sprintf("checkTx failed: %s", cktxresp.Data))
		}
	} else {
		if !expectedResult {
			t.Fatalf("checkTx success, but expected result is fail")
		}
	}
	if detx.Tx, err = proto.Marshal(&stx); err != nil {
		t.Fatal(err)
	}
	detxresp = app.DeliverTx(detx)
	if detxresp.Code != 0 {
		if expectedResult {
			t.Fatalf(fmt.Sprintf("deliverTx failed: %s", detxresp.Data))
		}
	} else {
		if !expectedResult {
			t.Fatalf("deliverTx success, but expected result is fail")
		}
	}
	app.Commit()
}

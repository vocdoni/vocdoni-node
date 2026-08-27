package vochain

import (
	"context"
	"math/big"
	"testing"

	cometabcitypes "github.com/cometbft/cometbft/abci/types"
	qt "github.com/frankban/quicktest"
	"go.vocdoni.io/dvote/crypto/ethereum"
	"go.vocdoni.io/dvote/crypto/saltedkey"
	"go.vocdoni.io/dvote/test/testcommon/testcsp"
	"go.vocdoni.io/dvote/types"
	"go.vocdoni.io/dvote/util"
	"go.vocdoni.io/dvote/vochain/state"
	"go.vocdoni.io/proto/build/go/models"
	"google.golang.org/protobuf/proto"
)

func TestMerkleTreeProof(t *testing.T) {
	app := TestBaseApplication(t)
	keys, root, proofs := testCreateKeysAndBuildCensus(t, 101)

	// we save the last key for the next test and remove it from the lists
	lastKey := keys[len(keys)-1]
	keys = keys[:len(keys)-2]
	lastProof := proofs[len(proofs)-1]
	proofs = proofs[:len(proofs)-2]

	censusURI := ipfsUrlTest
	pid := util.RandomBytes(types.ProcessIDsize)

	process := &models.Process{
		ProcessId:     pid,
		StartBlock:    0,
		EnvelopeType:  &models.EnvelopeType{EncryptedVotes: false},
		Mode:          &models.ProcessMode{},
		VoteOptions:   &models.ProcessVoteOptions{MaxCount: 1, MaxValue: 1},
		Status:        models.ProcessStatus_READY,
		EntityId:      util.RandomBytes(types.EthereumAddressSize),
		CensusRoot:    root,
		CensusURI:     &censusURI,
		CensusOrigin:  models.CensusOrigin_OFF_CHAIN_TREE,
		BlockCount:    1024,
		MaxCensusSize: 200,
	}
	err := app.State.AddProcess(process)
	qt.Assert(t, err, qt.IsNil)

	app.AdvanceTestBlock()

	cktx := new(cometabcitypes.CheckTxRequest)

	// send the votes (but not the last one), should be ok
	for i, s := range keys {
		stx := testBuildSignedVote(t, pid, s, proofs[i], []int{1, 2, 3, 4}, app.ChainID())

		cktx.Tx, err = proto.Marshal(stx)
		qt.Assert(t, err, qt.IsNil)
		cktxresp, _ := app.CheckTx(context.Background(), cktx)
		qt.Assert(t, cktxresp.Code, qt.Equals, uint32(0))

		txb, err := proto.Marshal(stx)
		qt.Assert(t, err, qt.IsNil)
		detxresp := app.deliverTx(txb)
		qt.Assert(t, detxresp.Code, qt.Equals, uint32(0))

		if i%5 == 0 {
			_, err = app.CommitState()
			qt.Assert(t, err, qt.IsNil)
			app.AdvanceTestBlock()
		}
	}
	app.AdvanceTestBlock()
	vp, err := state.NewVotePackage([]int{1, 2, 3, 4}).Encode()
	qt.Assert(t, err, qt.IsNil)

	// send the las vote multiple time, first attempt should be ok. The rest should fail.
	// we modify the nonce on each attempt to avoid the cache
	vote := &models.VoteEnvelope{
		ProcessId: pid,
		Proof: &models.Proof{
			Payload: &models.Proof_Arbo{
				Arbo: &models.ProofArbo{
					Type:     models.ProofArbo_BLAKE2B,
					Siblings: lastProof,
				},
			},
		},
		VotePackage: vp,
	}

	for i := 0; i < 10; i++ {
		stx := &models.SignedTx{}
		vote.Nonce = util.RandomBytes(32)
		stx.Tx, err = proto.Marshal(&models.Tx{
			Payload: &models.Tx_Vote{Vote: vote},
		})
		qt.Assert(t, err, qt.IsNil)
		stx.Signature, err = lastKey.SignVocdoniTx(stx.Tx, app.chainID)
		qt.Assert(t, err, qt.IsNil)

		cktx.Tx, err = proto.Marshal(stx)
		qt.Assert(t, err, qt.IsNil)

		cktxresp, _ := app.CheckTx(context.Background(), cktx)
		if i == 0 && cktxresp.Code != 0 {
			t.Fatalf("checkTx returned err on first valid vote: %s", cktxresp.Data)
		}
		if i > 0 && cktxresp.Code == 0 {
			t.Fatalf("checkTx returned 0 for vote %d, an error was expected", i)
		}

		txb, err := proto.Marshal(stx)
		qt.Assert(t, err, qt.IsNil)

		detxresp := app.deliverTx(txb)
		if i == 0 && detxresp.Code != 0 {
			t.Fatalf("devlierTx returned err on first valid vote: %s", detxresp.Data)
		}
		if i > 0 && detxresp.Code == 0 {
			t.Fatalf("deliverTx returned 0, an error was expected")
		}
		if i%2 == 0 {
			app.AdvanceTestBlock()
		}
	}
}

func TestCSPproof(t *testing.T) {
	app := TestBaseApplication(t)
	csp := ethereum.SignKeys{}
	err := csp.Generate()
	qt.Assert(t, err, qt.IsNil)

	pid := util.RandomBytes(types.ProcessIDsize)
	process := &models.Process{
		ProcessId:     pid,
		StartBlock:    0,
		EnvelopeType:  &models.EnvelopeType{EncryptedVotes: false},
		Mode:          new(models.ProcessMode),
		Status:        models.ProcessStatus_READY,
		EntityId:      util.RandomBytes(types.EthereumAddressSize),
		CensusRoot:    csp.PublicKey(),
		CensusOrigin:  models.CensusOrigin_OFF_CHAIN_CA,
		BlockCount:    1024,
		MaxCensusSize: 1000,
	}
	t.Logf("adding process %x", process.ProcessId)
	qt.Assert(t, app.State.AddProcess(process), qt.IsNil)

	// Test 20 valid votes
	vp, err := state.NewVotePackage([]int{1, 2, 3, 4}).Encode()
	qt.Assert(t, err, qt.IsNil)

	keys := ethereum.NewSignKeysBatch(20)
	for _, k := range keys {
		bundle := &models.CAbundle{
			ProcessId: pid,
			Address:   k.Address().Bytes(),
		}
		bundleBytes, err := proto.Marshal(bundle)
		qt.Assert(t, err, qt.IsNil)
		signature, err := csp.SignEthereum(bundleBytes)
		qt.Assert(t, err, qt.IsNil)

		proof := &models.ProofCA{
			Bundle:    bundle,
			Type:      models.ProofCA_ECDSA,
			Signature: signature,
		}
		testCSPsendVotes(t, pid, vp, k, proof, app, true)
	}

	// Test invalid vote
	k := ethereum.SignKeys{}
	qt.Assert(t, k.Generate(), qt.IsNil)
	bundle := &models.CAbundle{
		ProcessId: pid,
		Address:   k.Address().Bytes(),
	}
	bundleBytes, err := proto.Marshal(bundle)
	qt.Assert(t, err, qt.IsNil)

	csp2 := ethereum.SignKeys{}
	qt.Assert(t, csp2.Generate(), qt.IsNil)

	signature, err := csp2.SignEthereum(bundleBytes)
	qt.Assert(t, err, qt.IsNil)

	proof := &models.ProofCA{
		Bundle:    bundle,
		Type:      models.ProofCA_ECDSA,
		Signature: signature,
	}
	testCSPsendVotes(t, pid, vp, &k, proof, app, false)
}

func testCSPsendVotes(t *testing.T, pid []byte, vp []byte, signer *ethereum.SignKeys,
	proof *models.ProofCA, app *BaseApplication, expectedResult bool,
) {
	cktx := new(cometabcitypes.CheckTxRequest)
	var cktxresp *cometabcitypes.CheckTxResponse
	var stx models.SignedTx
	var err error

	tx := &models.VoteEnvelope{
		Nonce:       util.RandomBytes(32),
		ProcessId:   pid,
		Proof:       &models.Proof{Payload: &models.Proof_Ca{Ca: proof}},
		VotePackage: vp,
	}

	stx.Tx, err = proto.Marshal(&models.Tx{
		Payload: &models.Tx_Vote{
			Vote: tx,
		},
	})
	qt.Assert(t, err, qt.IsNil)

	stx.Signature, err = signer.SignVocdoniTx(stx.Tx, app.chainID)
	qt.Assert(t, err, qt.IsNil)

	cktx.Tx, err = proto.Marshal(&stx)
	qt.Assert(t, err, qt.IsNil)

	cktxresp, _ = app.CheckTx(context.Background(), cktx)
	if cktxresp.Code != 0 {
		if expectedResult {
			t.Fatalf("checkTx failed: %s", cktxresp.Data)
		}
	} else {
		if !expectedResult {
			t.Fatalf("checkTx success, but expected result is fail")
		}
	}
	txb, err := proto.Marshal(&stx)
	qt.Assert(t, err, qt.IsNil)
	detxresp := app.deliverTx(txb)
	if detxresp.Code != 0 {
		if expectedResult {
			t.Fatalf("deliverTx failed: %s", detxresp.Data)
		}
	} else {
		if !expectedResult {
			t.Fatalf("deliverTx success, but expected result is fail")
		}
	}
	_, err = app.CommitState()
	qt.Assert(t, err, qt.IsNil)
}

// cspTestProcess adds a CSP process with the given census origin and CSP root,
// returning its pid.
func cspTestProcess(t *testing.T, app *BaseApplication, censusRoot []byte,
	origin models.CensusOrigin,
) []byte {
	pid := util.RandomBytes(types.ProcessIDsize)
	process := &models.Process{
		ProcessId:     pid,
		StartBlock:    0,
		EnvelopeType:  &models.EnvelopeType{EncryptedVotes: false},
		Mode:          new(models.ProcessMode),
		Status:        models.ProcessStatus_READY,
		EntityId:      util.RandomBytes(types.EthereumAddressSize),
		CensusRoot:    censusRoot,
		CensusOrigin:  origin,
		BlockCount:    1024,
		MaxCensusSize: 1000,
	}
	qt.Assert(t, app.State.AddProcess(process), qt.IsNil)
	return pid
}

// TestCSPproofSaltedV2 drives the salted CSP proof types through the full
// CheckTx -> deliverTx -> commit path on an election with census origin
// OFF_CHAIN_CA_V2, where the fixed salt derivation (issue #1424) applies.
func TestCSPproofSaltedV2(t *testing.T) {
	app := TestBaseApplication(t)
	signer, err := testcsp.NewSigner()
	qt.Assert(t, err, qt.IsNil)

	vp, err := state.NewVotePackage([]int{1, 2, 3, 4}).Encode()
	qt.Assert(t, err, qt.IsNil)

	for _, proofType := range []models.ProofCA_Type{
		models.ProofCA_ECDSA_PIDSALTED,
		models.ProofCA_ECDSA_BLIND_PIDSALTED,
	} {
		t.Run(proofType.String(), func(t *testing.T) {
			root, err := signer.CensusRoot(proofType)
			qt.Assert(t, err, qt.IsNil)
			pid := cspTestProcess(t, app, root, models.CensusOrigin_OFF_CHAIN_CA_V2)
			weight := big.NewInt(42)

			// a proof salted for this election and weight is accepted
			k := ethereum.NewSignKeys()
			qt.Assert(t, k.Generate(), qt.IsNil)
			salt, err := saltedkey.Salt(pid, weight.Bytes())
			qt.Assert(t, err, qt.IsNil)
			proof, err := signer.Sign(proofType, testcsp.Bundle(pid, k.Address().Bytes(), weight), salt)
			qt.Assert(t, err, qt.IsNil)
			testCSPsendVotes(t, pid, vp, k, proof, app, true)

			// a proof salted for a sibling election of the same organization
			// (identical first 20 bytes, different nonce) is rejected: this is
			// the cross-election reuse the V2 origin closes
			k2 := ethereum.NewSignKeys()
			qt.Assert(t, k2.Generate(), qt.IsNil)
			sibling := append([]byte{}, pid...)
			sibling[31] ^= 0xff
			siblingSalt, err := saltedkey.Salt(sibling, weight.Bytes())
			qt.Assert(t, err, qt.IsNil)
			proof, err = signer.Sign(proofType, testcsp.Bundle(pid, k2.Address().Bytes(), weight), siblingSalt)
			qt.Assert(t, err, qt.IsNil)
			testCSPsendVotes(t, pid, vp, k2, proof, app, false)

			// a bundle declaring a weight the CSP did not salt for is rejected
			k3 := ethereum.NewSignKeys()
			qt.Assert(t, k3.Generate(), qt.IsNil)
			authorizedSalt, err := saltedkey.Salt(pid, big.NewInt(5).Bytes())
			qt.Assert(t, err, qt.IsNil)
			proof, err = signer.Sign(proofType, testcsp.Bundle(pid, k3.Address().Bytes(), big.NewInt(1000)), authorizedSalt)
			qt.Assert(t, err, qt.IsNil)
			testCSPsendVotes(t, pid, vp, k3, proof, app, false)

			// the adaptive forgery: the voter holds a signature the CSP made for
			// another election at weight 1, and picks the weight that would make
			// this election derive the same salt under an additive rule. The
			// hashed derivation defeats it; an additive one would accept it.
			k4 := ethereum.NewSignKeys()
			qt.Assert(t, k4.Generate(), qt.IsNil)
			otherPID := util.RandomBytes(types.ProcessIDsize)
			saltOther, err := saltedkey.Salt(otherPID, big.NewInt(1).Bytes())
			qt.Assert(t, err, qt.IsNil)
			mod := new(big.Int).Lsh(big.NewInt(1), saltedkey.MaxVoteWeightBits)
			hThis := new(big.Int).SetBytes(ethereum.HashRaw(pid)[:saltedkey.SaltSize])
			forged := new(big.Int).Mod(new(big.Int).Sub(new(big.Int).SetBytes(saltOther), hThis), mod)
			proof, err = signer.Sign(proofType, testcsp.Bundle(pid, k4.Address().Bytes(), forged), saltOther)
			qt.Assert(t, err, qt.IsNil)
			testCSPsendVotes(t, pid, vp, k4, proof, app, false)
		})
	}
}

// TestCSPproofSaltedLegacy checks the legacy derivation still governs
// OFF_CHAIN_CA elections, byte-identical to what deployed CSPs sign today,
// and that a V2-derived proof does not verify there.
func TestCSPproofSaltedLegacy(t *testing.T) {
	app := TestBaseApplication(t)
	signer, err := testcsp.NewSigner()
	qt.Assert(t, err, qt.IsNil)

	vp, err := state.NewVotePackage([]int{1, 2, 3, 4}).Encode()
	qt.Assert(t, err, qt.IsNil)

	for _, proofType := range []models.ProofCA_Type{
		models.ProofCA_ECDSA_PIDSALTED,
		models.ProofCA_ECDSA_BLIND_PIDSALTED,
	} {
		t.Run(proofType.String(), func(t *testing.T) {
			root, err := signer.CensusRoot(proofType)
			qt.Assert(t, err, qt.IsNil)
			pid := cspTestProcess(t, app, root, models.CensusOrigin_OFF_CHAIN_CA)

			// the legacy salt is the raw processID
			k := ethereum.NewSignKeys()
			qt.Assert(t, k.Generate(), qt.IsNil)
			proof, err := signer.Sign(proofType, testcsp.Bundle(pid, k.Address().Bytes(), nil), pid)
			qt.Assert(t, err, qt.IsNil)
			testCSPsendVotes(t, pid, vp, k, proof, app, true)

			// a proof salted with the V2 derivation is rejected here
			k2 := ethereum.NewSignKeys()
			qt.Assert(t, k2.Generate(), qt.IsNil)
			salt, err := saltedkey.Salt(pid, nil)
			qt.Assert(t, err, qt.IsNil)
			proof, err = signer.Sign(proofType, testcsp.Bundle(pid, k2.Address().Bytes(), nil), salt)
			qt.Assert(t, err, qt.IsNil)
			testCSPsendVotes(t, pid, vp, k2, proof, app, false)
		})
	}
}

// TestCSPproofSaltedV2NoCache delivers a salted V2 vote without a prior
// CheckTx, so the vote cache cannot serve it and the proof is fully
// re-verified at delivery. The outcome must match the cached path exercised by
// testCSPsendVotes: the derivation depends only on the election's census
// origin, so the two paths cannot disagree.
func TestCSPproofSaltedV2NoCache(t *testing.T) {
	app := TestBaseApplication(t)
	signer, err := testcsp.NewSigner()
	qt.Assert(t, err, qt.IsNil)
	root, err := signer.CensusRoot(models.ProofCA_ECDSA_BLIND_PIDSALTED)
	qt.Assert(t, err, qt.IsNil)
	pid := cspTestProcess(t, app, root, models.CensusOrigin_OFF_CHAIN_CA_V2)

	vp, err := state.NewVotePackage([]int{1, 2, 3, 4}).Encode()
	qt.Assert(t, err, qt.IsNil)
	weight := big.NewInt(42)
	salt, err := saltedkey.Salt(pid, weight.Bytes())
	qt.Assert(t, err, qt.IsNil)

	buildVote := func(k *ethereum.SignKeys, salt []byte) []byte {
		proof, err := signer.Sign(models.ProofCA_ECDSA_BLIND_PIDSALTED,
			testcsp.Bundle(pid, k.Address().Bytes(), weight), salt)
		qt.Assert(t, err, qt.IsNil)
		var stx models.SignedTx
		stx.Tx, err = proto.Marshal(&models.Tx{Payload: &models.Tx_Vote{Vote: &models.VoteEnvelope{
			Nonce:       util.RandomBytes(32),
			ProcessId:   pid,
			Proof:       &models.Proof{Payload: &models.Proof_Ca{Ca: proof}},
			VotePackage: vp,
		}}})
		qt.Assert(t, err, qt.IsNil)
		stx.Signature, err = k.SignVocdoniTx(stx.Tx, app.chainID)
		qt.Assert(t, err, qt.IsNil)
		txb, err := proto.Marshal(&stx)
		qt.Assert(t, err, qt.IsNil)
		return txb
	}

	// a valid salted vote delivered straight to deliverTx is accepted
	k := ethereum.NewSignKeys()
	qt.Assert(t, k.Generate(), qt.IsNil)
	resp := app.deliverTx(buildVote(k, salt))
	qt.Assert(t, resp.Code, qt.Equals, uint32(0), qt.Commentf("%s", resp.Data))

	// and a legacy-salted one is rejected, also without cache involvement
	k2 := ethereum.NewSignKeys()
	qt.Assert(t, k2.Generate(), qt.IsNil)
	resp = app.deliverTx(buildVote(k2, pid))
	qt.Assert(t, resp.Code, qt.Not(qt.Equals), uint32(0))
}

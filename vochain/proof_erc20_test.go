package vochain

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"

	abcitypes "github.com/cometbft/cometbft/abci/types"
	"github.com/vocdoni/storage-proofs-eth-go/ethstorageproof"
	"go.vocdoni.io/dvote/crypto/ethereum"
	"go.vocdoni.io/dvote/test/testcommon/testutil"
	"go.vocdoni.io/dvote/types"
	"go.vocdoni.io/dvote/util"
	"go.vocdoni.io/dvote/vochain/state"
	"go.vocdoni.io/proto/build/go/models"
	"google.golang.org/protobuf/proto"
)

func TestEthProof(t *testing.T) {
	app := TestBaseApplication(t)
	sp := testStorageProofs{}
	if err := json.Unmarshal([]byte(ethVotingProofs), &sp); err != nil {
		t.Fatal(err)
	}

	pid := util.RandomBytes(types.ProcessIDsize)
	process := &models.Process{
		ProcessId:     pid,
		StartBlock:    0,
		EnvelopeType:  &models.EnvelopeType{EncryptedVotes: false},
		Mode:          new(models.ProcessMode),
		VoteOptions:   &models.ProcessVoteOptions{MaxCount: 1, MaxValue: 2, MaxVoteOverwrites: 0},
		Status:        models.ProcessStatus_READY,
		EntityId:      util.RandomBytes(types.EthereumAddressSize),
		CensusRoot:    testEthStorageRoot,
		CensusOrigin:  models.CensusOrigin_ERC20,
		BlockCount:    1024,
		MaxCensusSize: 200,
		EthIndexSlot:  &testEthIndexSlot,
	}
	t.Logf("adding process %x", process.ProcessId)
	if err := app.State.AddProcess(process); err != nil {
		t.Fatal(err)
	}

	vp, err := state.NewVotePackage([]int{1, 2, 3, 4}).Encode()
	if err != nil {
		t.Fatal(err)
	}

	// Test wrong vote (change amount value)
	wrongSp := sp.StorageProofs[0]
	wrongSp.StorageProof.Value = sp.StorageProofs[1].StorageProof.Value
	testEthSendVotes(t, wrongSp, pid, vp, app, false)

	// Test valid votes
	for _, s := range sp.StorageProofs {
		testEthSendVotes(t, s, pid, vp, app, true)
	}

	// Test double vote
	testEthSendVotes(t, sp.StorageProofs[2], pid, vp, app, false)
}

func testEthSendVotes(t *testing.T, s testStorageProof,
	pid []byte, vp []byte, app *BaseApplication, expectedResult bool) {
	cktx := new(abcitypes.CheckTxRequest)
	var cktxresp *abcitypes.CheckTxResponse
	var stx models.SignedTx

	t.Logf("voting %x", s.StorageProof.Key)
	proof := models.ProofEthereumStorage{
		Key:      s.StorageProof.Key,
		Value:    s.StorageProof.Value,
		Siblings: s.StorageProof.Proof,
	}
	tx := &models.VoteEnvelope{
		Nonce:       util.RandomBytes(32),
		ProcessId:   pid,
		Proof:       &models.Proof{Payload: &models.Proof_EthereumStorage{EthereumStorage: &proof}},
		VotePackage: vp,
	}

	signer := &ethereum.SignKeys{}
	found := false
	for _, key := range testSmartContractHolders {
		sk := ethereum.NewSignKeys()
		if err := sk.AddHexKey(key); err != nil {
			t.Fatal(err)
		}
		if util.TrimHex(sk.Address().Hex()) == util.TrimHex(s.Address) {
			signer = sk
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("signer for address %s not found", s.Address)
	}
	pub, _ := signer.HexString()
	t.Logf("addr: %s pubKey: %s", signer.Address(), pub)

	var err error
	if stx.Tx, err = proto.Marshal(&models.Tx{Payload: &models.Tx_Vote{Vote: tx}}); err != nil {
		t.Fatal(err)
	}

	if stx.Signature, err = signer.SignVocdoniTx(stx.Tx, app.chainID); err != nil {
		t.Fatal(err)
	}

	if cktx.Tx, err = proto.Marshal(&stx); err != nil {
		t.Fatal(err)
	}
	cktxresp, _ = app.CheckTx(context.Background(), cktx)
	if cktxresp.Code != 0 {
		if expectedResult {
			t.Fatalf(fmt.Sprintf("checkTx failed: %s", cktxresp.Data))
		}
	} else {
		if !expectedResult {
			t.Fatalf("checkTx success, but expected result is fail")
		}
	}

	var txb []byte
	if txb, err = proto.Marshal(&stx); err != nil {
		t.Fatal(err)
	}
	detxresp := app.deliverTx(txb)
	if detxresp.Code != 0 {
		if expectedResult {
			t.Fatalf(fmt.Sprintf("deliverTx failed: %s", detxresp.Data))
		}
	} else {
		if !expectedResult {
			t.Fatalf("deliverTx success, but expected result is fail")
		}
	}
	_, err = app.CommitState()
	if err != nil {
		t.Fatal(err)
	}
}

var testSmartContractHolders = []string{
	"3a553e31b160d2f52a1bf3123dd7bdade866a30a461c91cc89daa529e3d246a0",
	"5be5b1124ef25e62fd7bcfda41955731132331d35711138f4cb28f8d517d63e4",
	"ce59b11a3d8136978556bf2dbd535380a12a7330f521570477ec9c88f87e322d",
	"5a3936c153dea2d298035c084598857f6c582e44d2e1def517c98a1cd5456ee9",
	"aa16366cd207782dc4b2a9b8c9f82f3970c31540dd5c182acce12905ef9e272b",
}

// var testEthVotingSmartContract = "0x2b7222146a805bba0dbb61869c4b3a03209dffba"
// var testEthHeight = uint32(3833670)
var (
	testEthIndexSlot   = uint32(4)
	testEthStorageRoot = testutil.Hex2byte(nil,
		"0xe338061cd5d5fa8a452dc950e336a838ff2ca79bef04fe48c5a3a071cc7e0c55")
)

type testStorageProofs struct {
	StorageProofs []testStorageProof `json:"storageProofs"`
}

type testStorageProof struct {
	Address      string                        `json:"address"`
	StorageProof ethstorageproof.StorageResult `json:"storageProof"`
}

var ethVotingProofs = `
  {
    "storageProofs": [
      {
        "address": "0xe101391adF348Cd80bb71b97306f3CdDd5d34586",
        "storageProof": {
          "key": "c2de2619bab76beef95434a377cbb768c64eefaf75bf29ef7065b6637bd6d201",
          "value": "0x6f05b59d3b20000",
          "proof": [
            "0xf9013180a00740dd6c24a8afbe4a6a82f07661ec6632dbc24a000d46d67b54d31f7a50a0cba007b0f9b46e3b240859292ba8d37485f7ae9aea12df6678554ddbcce68159391380a0e4e5615581505df363f71539f1ad885a287d77c7e23047b1eefb54b03352d8b980a0afbcc7bc6e755d9d18d64d739ec4fddb77502e8ea15f6d539da3c9d2462fffc4a0f3bc3263c4351b24d5123773632b6c00ae4c6e98e842a43539f6b7f8a369ef64a060a62c7496da939d10ba2949cb4ac1e104e9ef86dd79c56b674f87245507db3f8080a031bf08ec19f881b068fe1d40cfbf411824be654f5d62f5d0b0c3ad43f72995cca0714766957876d306a626b1bfeb594f7902e178d400221957a62435c91185fc2ba05e872405ac6aca97f5bb9f8a937763e0b1683d2d2bba3e17e2bb5b8ebe9ba16b808080",
            "0xeba03de5c840e393521c7ff396023c973083a5d40a34e3caa72df575448f9c075661898806f05b59d3b20000"
          ]
        }
      },
      {
        "address": "0x22C0608a1f9858c2335C2BD94364c680DdB268bB",
        "storageProof": {
          "key": "0a19943e412e0d591f8c1e2adf8224c6b15cee1294b7799f5b246b233709eab0",
          "value": "0xf43fc2c04ee0000",
          "proof": [
            "0xf9013180a00740dd6c24a8afbe4a6a82f07661ec6632dbc24a000d46d67b54d31f7a50a0cba007b0f9b46e3b240859292ba8d37485f7ae9aea12df6678554ddbcce68159391380a0e4e5615581505df363f71539f1ad885a287d77c7e23047b1eefb54b03352d8b980a0afbcc7bc6e755d9d18d64d739ec4fddb77502e8ea15f6d539da3c9d2462fffc4a0f3bc3263c4351b24d5123773632b6c00ae4c6e98e842a43539f6b7f8a369ef64a060a62c7496da939d10ba2949cb4ac1e104e9ef86dd79c56b674f87245507db3f8080a031bf08ec19f881b068fe1d40cfbf411824be654f5d62f5d0b0c3ad43f72995cca0714766957876d306a626b1bfeb594f7902e178d400221957a62435c91185fc2ba05e872405ac6aca97f5bb9f8a937763e0b1683d2d2bba3e17e2bb5b8ebe9ba16b808080",
            "0xeba03722ad3bc234001231f62bd4846e8a1f8b9eb7329931dd914156b6effca198cf89880f43fc2c04ee0000"
          ]
        }
      },
      {
        "address": "0xCfAE1df2458D2B62640E813037CcDfd91c6333e3",
        "storageProof": {
          "key": "74e56920b06fb78abf807c6931544c2aa09f5b8866249cd912f9932b65527a6a",
          "value": "0x2c68af0bb140000",
          "proof": [
            "0xf9013180a00740dd6c24a8afbe4a6a82f07661ec6632dbc24a000d46d67b54d31f7a50a0cba007b0f9b46e3b240859292ba8d37485f7ae9aea12df6678554ddbcce68159391380a0e4e5615581505df363f71539f1ad885a287d77c7e23047b1eefb54b03352d8b980a0afbcc7bc6e755d9d18d64d739ec4fddb77502e8ea15f6d539da3c9d2462fffc4a0f3bc3263c4351b24d5123773632b6c00ae4c6e98e842a43539f6b7f8a369ef64a060a62c7496da939d10ba2949cb4ac1e104e9ef86dd79c56b674f87245507db3f8080a031bf08ec19f881b068fe1d40cfbf411824be654f5d62f5d0b0c3ad43f72995cca0714766957876d306a626b1bfeb594f7902e178d400221957a62435c91185fc2ba05e872405ac6aca97f5bb9f8a937763e0b1683d2d2bba3e17e2bb5b8ebe9ba16b808080",
            "0xf8518080a0f97cd0e4c4948fb939e5b65edd4a1ef1a3116e9067c65417c281cd82bfb4685ba07f60d6a908b996ecc5e67987f4961b881bc915cc8fb788d91d5146c3ba08b7b480808080808080808080808080",
            "0xeba02022e78ab46a4689e1eba0a9200b165fbff6a830b0c1719ba14946192e3a84a6898802c68af0bb140000"
          ]
        }
      },
      {
        "address": "0x4729175D62fAFF7A0C695B38b66D2E60806E595F",
        "storageProof": {
          "key": "e377ba8914b6e3793dcb44495b80e0abb6697bb2d526bef4b103a2ca45571a18",
          "value": "0x9b6e64a8ec60000",
          "proof": [
            "0xf9013180a00740dd6c24a8afbe4a6a82f07661ec6632dbc24a000d46d67b54d31f7a50a0cba007b0f9b46e3b240859292ba8d37485f7ae9aea12df6678554ddbcce68159391380a0e4e5615581505df363f71539f1ad885a287d77c7e23047b1eefb54b03352d8b980a0afbcc7bc6e755d9d18d64d739ec4fddb77502e8ea15f6d539da3c9d2462fffc4a0f3bc3263c4351b24d5123773632b6c00ae4c6e98e842a43539f6b7f8a369ef64a060a62c7496da939d10ba2949cb4ac1e104e9ef86dd79c56b674f87245507db3f8080a031bf08ec19f881b068fe1d40cfbf411824be654f5d62f5d0b0c3ad43f72995cca0714766957876d306a626b1bfeb594f7902e178d400221957a62435c91185fc2ba05e872405ac6aca97f5bb9f8a937763e0b1683d2d2bba3e17e2bb5b8ebe9ba16b808080",
            "0xeba035fa9266a36d8b2833bc56464af71ecb47d5ba3fcb5a2adb5a0b8f12254580ba898809b6e64a8ec60000"
          ]
        }
      },
      {
        "address": "0x7115f6C5c40e4a8B5D17Fb2d77662BD45b868637",
        "storageProof": {
          "key": "96bca652e37bd5c8c08f80295480625858aa664924b5546a893fefaed4e281c7",
          "value": "0x429d069189e0000",
          "proof": [
            "0xf9013180a00740dd6c24a8afbe4a6a82f07661ec6632dbc24a000d46d67b54d31f7a50a0cba007b0f9b46e3b240859292ba8d37485f7ae9aea12df6678554ddbcce68159391380a0e4e5615581505df363f71539f1ad885a287d77c7e23047b1eefb54b03352d8b980a0afbcc7bc6e755d9d18d64d739ec4fddb77502e8ea15f6d539da3c9d2462fffc4a0f3bc3263c4351b24d5123773632b6c00ae4c6e98e842a43539f6b7f8a369ef64a060a62c7496da939d10ba2949cb4ac1e104e9ef86dd79c56b674f87245507db3f8080a031bf08ec19f881b068fe1d40cfbf411824be654f5d62f5d0b0c3ad43f72995cca0714766957876d306a626b1bfeb594f7902e178d400221957a62435c91185fc2ba05e872405ac6aca97f5bb9f8a937763e0b1683d2d2bba3e17e2bb5b8ebe9ba16b808080",
            "0xf8518080a0f97cd0e4c4948fb939e5b65edd4a1ef1a3116e9067c65417c281cd82bfb4685ba07f60d6a908b996ecc5e67987f4961b881bc915cc8fb788d91d5146c3ba08b7b480808080808080808080808080",
            "0xeba0205a86d519c7a22721a379b3a6e27454220b2c536888d8aa9c0a493e3f6da06189880429d069189e0000"
          ]
        }
      }
    ]
  }  
  `

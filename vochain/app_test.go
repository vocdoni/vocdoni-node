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

var erc20Holders = []string{
	"0x0c5dc58167849ba0c61c952463afb5e3c7970461",
	"0x0c5dc9f198f4bcd221b97e4749f1e06e4b28bff9",
	"0x0c5ddddf9e0a74cd6b23a4be789657f475ebaa9b",
	"0x0c5de5bbdfba99e85572742843e0902fc7863a54",
	"0x0c5dec4d3797f418cef6eccbca62cea7c989b450",
	"0x0c5df0dba295f3ade243df1c63c0262dbc129ecc",
	"0x0c5df4ce909f73a2e04b72e8999b253fdff61e41",
	"0x0c5df4e747521fdcb6ea64bf34c6c4f3d1a8ab51",
	"0x0c5e02ee3b24f1c341254894a87066d5ec4ab6ab",
	"0x0c5e1123e2de815e2f237ae09f33aa0ae7e27be3",
	"0x0c5e19cef0c74435a2ed89d1988b6ff1edf41d46",
	"0x0c5e2c5ff7c66aecc7d38e6a5fb7a82a100c0cf5",
	"0x0c5e2c7f6fc2ee0ebf32dd2ce553a97ded2d1052",
	"0x0c5e2f2c4b8208f4d8af9fb945c79b060babff43",
	"0x0c5e38ea130b70fac691ce13efc758576d1ec97c",
	"0x0c5e3bb1c9355befffbf27e73128c3da98a2d071",
	"0x0c5e4d3039d35659c77c6f6a5b8db50bec3a72dc",
	"0x0c5e50e73e863dbe8fb64d729725486b1b22975b",
	"0x0c5e58129a74df079e9fe557a70d0ee5b5812729",
	"0x0c5e651bd07c039b57903903ed2cbb8f453daea6",
	"0x0c5e713d86456e085944b21dac6dd5816c973015",
	"0x0c5e772b1e3f4ec328dda1d78d488ae267d53d66",
	"0x0c5e93f74b0c46e1e54daca8a572e00318c5fcfc",
	"0x0c5ea754c0ae49b86f07c91261d447ba97cccac3",
	"0x0c5ea767185bb489afdf9a8f62b305e9dff9d1e5",
	"0x0c5ea86961ea0fee1f222962c55f08a624e0381a",
	"0x0c5eaa394f4037981645ec7a63ffcc337293025c",
	"0x0c5eab97e1d75c0231edebb725e11a042ebd1400",
	"0x0c5ed5fdce7b012c98ac25ded4aa4a850760b711",
	"0x0c5ef38fd8c8dd424fc2867ae23a4786c895e795",
	"0x0c5f07f76f7ae1c8d64a8c4bc8393d626b5a9122",
	"0x0c5f20e48d2926a6be5d0e40143e7da2a9851532",
	"0x0c5f2645dafb1c41c803d191c5cd1d01c2b526bb",
	"0x0c5f3fbca6802e264779e03b8d6b577a197333ab",
	"0x0c5f401a1b46bd5721ca10bef0d680f4c89fef22",
	"0x0c5f4ae698b3986ec18bac205bedabe335e43b3b",
	"0x0c5f56d3456ba92c01b1718443db158ff570547e",
	"0x0c5f6031e69f2e0dac8bf5d479a30c978d263cdb",
	"0x0c5f6b7a9def92c4dd263d3f40a16a96d4edf427",
	"0x0c5f724f17de4c77f0fb3cbd73b83ea6db604239",
	"0x0c5f735f40bd92c0455e27df37bc5dec86efdc86",
	"0x0c5f76a62ab8ab3aade0076c5ef7519af1b11819",
	"0x0c5f785f4ab312eb1bd3cfb384696a834525cf34",
	"0x0c5f7ad2fa8725b89b57ed421f611d6465bc7632",
	"0x0c5f813c4a910653dbbb8a9388fda3b238474f13",
	"0x0c5f87c2d060e6f029a6742fc9ae9fb4571ecee9",
	"0x0c5f8a564a207e50f26ff032d172130b9f95bfb4",
	"0x0c5f99580cc503673db949a11747d21930d360be",
	"0x0c5f99ec17a71b0ec59090e03236d8ef22c8e0de",
	"0x0c5f9bb44c58b400ef6196832e3b07dcfb6379a7",
	"0x0c5fa43833ded26299edd951d5b1d0702abb75d4",
	"0x0c5fa5fa51d84227bfacdc56b36329286b37d051",
	"0x0c5faad58f20eb24507dd1e31d345c3bc354583b",
	"0x0c5faf30e0311afb7d959a41759ab6b7989e5e1b",
	"0x0c5fb0a8ca25353608aa14e09fcbecf07cee05fd",
	"0x0c5fb46fc544c75078ffcaf449b9440f350c6766",
	"0x0c5fbf11b62582d21ae7b45251dbad12641ff056",
	"0x0c5fcdb1ddbdd11c4a6e6170870a92b4117a0675",
	"0x0c5fcde17bd38a4ebf84331a99cffb4caa95520f",
	"0x0c5fd2adf084f7747ffd316e19c335b4a3b84bba",
	"0x0c5fd37855dd649179818182e57b2327d3e10e18",
	"0x0c5fd8333e1a19d9683c03eacab589c9c57647e5",
	"0x0c5fe6a622df136effdcc3e036bbaf8157a30182",
	"0x0c5fe9d7564c1d7cb2f56c7ec1f894affa7e481f",
	"0x0c5ff41a0a295aecd09d36007242b7413a89f912",
	"0x0c600c8532373df8446acc738315b1398cb75848",
	"0x0c600e8d841280c3d40b5dfdf09a1b48b4316a31",
	"0x0c6016f09e825c25415ebff6bedbe3d6b271aaba",
	"0x0c6020907938b026fc5025f4dd1b4360470b6cec",
	"0x0c602205dcb93d38cff92b9dfd6a330311a4468f",
	"0x0c60338bf69cb23b6aa716530e6a0ef5fa2dada1",
	"0x0c604503eb34d106bc85b2aef6cb38aa9fdaa579",
	"0x0c6047fa9957142026d734abce96af8835e82086",
	"0x0c605cb7c4ffc496f85b4b7a1627632e3812e239",
	"0x0c606521b19713cce3da526a37573f984933e063",
	"0x0c6067addf7f52e47ddffebcaa4c94d0880f876d",
	"0x0c606ba02538af0520ee0b93adf45afa15600ca3",
	"0x0c6070266da8689e24da20fc7b992177f9a85f1a",
	"0x0c6074a1aa0ffbf6a485ddf70d932b62ac53e86a",
	"0x0c60803a784f7ca1cb30b085f95e4b92b76573ae",
	"0x0c60a1013de1920d1ef4e4e49be390cba1fcf344",
	"0x0c60b9a779e8697f49d233d6022b2d5731c54f8c",
	"0x0c60be2410812ecd5fe25ae06b516a2fe35083cf",
	"0x0c60ca9835b7b4b08f4c9153b83fbb32c483a56e",
	"0x0c60cd9b787d5e9ede1a37522a5cdfd66fe80c26",
	"0x0c60d55a879c7599943ec446ae5264edc0eeb42d",
	"0x0c60d7f02665d58b1c6745570db7c9d740bff766",
	"0x0c60dce470c715cdb18415c3a98ec358085473ac",
	"0x0c60ebc4bee6b9c2e773d389a78f85af889d5dc9",
	"0x0c60f53541e64e45d17a163a2c9bf3a1d90e249e",
	"0x0c60fd1dd94a83475b3ab0d3bd104977210bcbe0",
	"0x0c610431c521febbed671e702c217b3068bb699e",
	"0x0c6118c8c3a74cee7e54cf9283d38b2aa91a3ec9",
	"0x0c611fa42a102f5157b661d10ae2aa1a552148ec",
	"0x0c6128bc1f855b644b28f71d5585425b30fc4c7a",
	"0x0c612d9546025eef3606cbe8242079fa5d0f2f0d",
	"0x0c614a3c037a1966c92c05e0c603f5739e66e613",
	"0x0c614da964ef1b216b3e660b50d714e453b0a026",
	"0x0c6151486345fe62b3d2aa17a31e68930b685d97",
	"0x0c6152caeadb06588108e4179a6505ec8fe85906",
}

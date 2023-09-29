package ethereum

import (
	"fmt"
	"testing"

	qt "github.com/frankban/quicktest"
	"go.vocdoni.io/proto/build/go/models"
	"google.golang.org/protobuf/proto"
)

func TestVocdoniSignature(t *testing.T) {
	t.Parallel()

	s := NewSignKeys()
	if err := s.Generate(); err != nil {
		t.Fatal(err)
	}
	pub, priv := s.HexString()
	t.Logf("Generated pub:%s priv:%s", pub, priv)

	signer2 := SignKeys{}
	signer2.Generate()
	toAddr := signer2.Address().Bytes()
	value := uint64(100)

	tx := &models.Tx{Payload: &models.Tx_SendTokens{
		SendTokens: &models.SendTokensTx{
			Txtype: models.TxType_SEND_TOKENS,
			From:   s.Address().Bytes(),
			To:     toAddr,
			Value:  value,
			Nonce:  123,
		}}}

	message, err := proto.Marshal(tx)
	qt.Assert(t, err, qt.IsNil)

	t.Logf("Message to sign: %s", message)
	signature, err := s.SignVocdoniMsg(message)
	qt.Assert(t, err, qt.IsNil)
	t.Logf("Signature for message is %x", signature)

	extractedPubKey, err := PubKeyFromSignature(BuildVocdoniMessage(message), signature)
	qt.Assert(t, err, qt.IsNil)
	qt.Assert(t, fmt.Sprintf("%x", extractedPubKey), qt.DeepEquals, pub)

	signature, err = s.SignVocdoniTx(message, "chain-123")
	qt.Assert(t, err, qt.IsNil)
	t.Logf("Signature for transaction is %x", signature)

	msgToSign, _, err := BuildVocdoniTransaction(message, "chain-123")
	qt.Assert(t, err, qt.IsNil)

	extractedPubKey, err = PubKeyFromSignature(msgToSign, signature)
	qt.Assert(t, err, qt.IsNil)
	qt.Assert(t, fmt.Sprintf("%x", extractedPubKey), qt.DeepEquals, pub)

	// Test compatibility with JS (message)
	//msg = testVocdoniSignatureMessage{Method: "getVisibility", Timestamp: 1582196988554}
	//message, err = json.Marshal(msg)
	//qt.Assert(t, err, qt.IsNil)
	//signature, err = hex.DecodeString("2aab382d8cf025f55d8c3f7597e83dc878939ef63f1a27b818fa0814d79e91d66dc8d8112fbdcc89d2355d58a74ad227a2a9603ef7eb2321283a8ea93fb90ee11b")
	//qt.Assert(t, err, qt.IsNil)

	//extractedPubKey, err = PubKeyFromSignature(BuildVocdoniMessage(message), signature)
	//qt.Assert(t, err, qt.IsNil)
	//qt.Assert(t, fmt.Sprintf("%x", extractedPubKey), qt.DeepEquals, "02cb3cabb521d84fc998b5649d6b59e27a3e27633d31cc0ca6083a00d68833d5ca")

	// Test compatibility with JS (transaction)
	//signature, err = hex.DecodeString("44d31937862e1f835f92d8b3f93858636bad075bd2429cf79404a81e42bd3d00196613c37a99b054c32339dde53ce3c25932c5d5f89483ccdf93234d3c4808b81b")
	//qt.Assert(t, err, qt.IsNil)

	//extractedPubKey, err = PubKeyFromSignature(BuildVocdoniTransaction(message, "production"), signature)
	//qt.Assert(t, err, qt.IsNil)
	//qt.Assert(t, fmt.Sprintf("%x", extractedPubKey), qt.DeepEquals, "02cb3cabb521d84fc998b5649d6b59e27a3e27633d31cc0ca6083a00d68833d5ca")

}

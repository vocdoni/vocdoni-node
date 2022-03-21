package ethereum

import (
	"encoding/hex"
	"encoding/json"
	"fmt"
	"testing"

	qt "github.com/frankban/quicktest"
)

func TestVocdoniSignature(t *testing.T) {
	t.Parallel()

	s := NewSignKeys()
	if err := s.Generate(); err != nil {
		t.Fatal(err)
	}
	pub, priv := s.HexString()
	t.Logf("Generated pub:%s priv:%s", pub, priv)

	msg := testVocdoniSignatureMessage{Method: "getVisibility", Timestamp: 1582196988222}
	message, err := json.Marshal(msg)
	qt.Assert(t, err, qt.IsNil)

	t.Logf("Message to sign: %s", message)
	signature, err := s.SignVocdoniMsg(message)
	qt.Assert(t, err, qt.IsNil)
	t.Logf("Signature for message is %x", signature)

	extractedPubKey, err := PubKeyFromSignature(BuildVocdoniMessage(message), signature)
	qt.Assert(t, err, qt.IsNil)
	qt.Assert(t, fmt.Sprintf("%x", extractedPubKey), qt.DeepEquals, pub)

	t.Logf("Transaction to sign: %s", message)
	signature, err = s.SignVocdoniTx(message, "chain-123")
	qt.Assert(t, err, qt.IsNil)
	t.Logf("Signature for transaction is %x", signature)

	extractedPubKey, err = PubKeyFromSignature(BuildVocdoniTransaction(message, "chain-123"), signature)
	qt.Assert(t, err, qt.IsNil)
	qt.Assert(t, fmt.Sprintf("%x", extractedPubKey), qt.DeepEquals, pub)

	// Test compatibility with JS (message)
	msg = testVocdoniSignatureMessage{Method: "getVisibility", Timestamp: 1582196988554}
	message, err = json.Marshal(msg)
	qt.Assert(t, err, qt.IsNil)
	signature, err = hex.DecodeString("2aab382d8cf025f55d8c3f7597e83dc878939ef63f1a27b818fa0814d79e91d66dc8d8112fbdcc89d2355d58a74ad227a2a9603ef7eb2321283a8ea93fb90ee11b")
	qt.Assert(t, err, qt.IsNil)

	extractedPubKey, err = PubKeyFromSignature(BuildVocdoniMessage(message), signature)
	qt.Assert(t, err, qt.IsNil)
	qt.Assert(t, fmt.Sprintf("%x", extractedPubKey), qt.DeepEquals, "02cb3cabb521d84fc998b5649d6b59e27a3e27633d31cc0ca6083a00d68833d5ca")

	// Test compatibility with JS (transaction)
	signature, err = hex.DecodeString("44d31937862e1f835f92d8b3f93858636bad075bd2429cf79404a81e42bd3d00196613c37a99b054c32339dde53ce3c25932c5d5f89483ccdf93234d3c4808b81b")
	qt.Assert(t, err, qt.IsNil)

	extractedPubKey, err = PubKeyFromSignature(BuildVocdoniTransaction(message, "production"), signature)
	qt.Assert(t, err, qt.IsNil)
	qt.Assert(t, fmt.Sprintf("%x", extractedPubKey), qt.DeepEquals, "02cb3cabb521d84fc998b5649d6b59e27a3e27633d31cc0ca6083a00d68833d5ca")

}

type testVocdoniSignatureMessage struct {
	Method    string `json:"method"`
	Timestamp int    `json:"timestamp"`
}

package ethereum

import (
	"fmt"
	"testing"

	qt "github.com/frankban/quicktest"
	"go.vocdoni.io/dvote/log"
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
	nonce := uint32(123)

	tx := &models.Tx{Payload: &models.Tx_SendTokens{
		SendTokens: &models.SendTokensTx{
			Txtype: models.TxType_SEND_TOKENS,
			From:   s.Address().Bytes(),
			To:     toAddr,
			Value:  value,
			Nonce:  123,
		},
	}}

	message, err := proto.Marshal(tx)
	qt.Assert(t, err, qt.IsNil)
	t.Logf("Proto to sign: %s", log.FormatProto(tx))

	msgToSign, _, err := BuildVocdoniTransaction(message, "chain-123")
	qt.Assert(t, err, qt.IsNil)
	t.Logf("Message to sign: %s", msgToSign)

	signature, err := s.SignVocdoniTx(message, "chain-123")
	qt.Assert(t, err, qt.IsNil)
	t.Logf("Signature for transaction is %x", signature)

	extractedPubKey, err := PubKeyFromSignature(msgToSign, signature)
	qt.Assert(t, err, qt.IsNil)
	qt.Assert(t, fmt.Sprintf("%x", extractedPubKey), qt.DeepEquals, pub)

	infoUri := "ipfs://vocdoni.io"

	tx = &models.Tx{Payload: &models.Tx_SetAccount{
		SetAccount: &models.SetAccountTx{
			Txtype:  models.TxType_SET_ACCOUNT_INFO_URI,
			InfoURI: &infoUri,
			Account: s.Address().Bytes(),
			Nonce:   &nonce,
		},
	}}

	message, err = proto.Marshal(tx)
	qt.Assert(t, err, qt.IsNil)
	t.Logf("Proto to sign: %s", log.FormatProto(tx))

	msgToSign, _, err = BuildVocdoniTransaction(message, "chain-123")
	qt.Assert(t, err, qt.IsNil)
	t.Logf("Message to sign: %s", msgToSign)

	signature, err = s.SignVocdoniTx(message, "chain-123")
	qt.Assert(t, err, qt.IsNil)
	t.Logf("Signature for transaction is %x", signature)

	extractedPubKey, err = PubKeyFromSignature(msgToSign, signature)
	qt.Assert(t, err, qt.IsNil)
	qt.Assert(t, fmt.Sprintf("%x", extractedPubKey), qt.DeepEquals, pub)
}

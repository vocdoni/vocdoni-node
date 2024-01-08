package vochain

import (
	"context"
	"testing"

	cometabcitypes "github.com/cometbft/cometbft/abci/types"
	"github.com/frankban/quicktest"
	"go.vocdoni.io/dvote/crypto/ethereum"
	vstate "go.vocdoni.io/dvote/vochain/state"
	"go.vocdoni.io/dvote/vochain/transaction/vochaintx"
	"go.vocdoni.io/proto/build/go/models"
	"google.golang.org/protobuf/proto"
)

// To test if the PrepareProposal method correctly sorts the transactions based on the sender's address and nonce
func TestTransactionsSorted(t *testing.T) {
	qt := quicktest.New(t)
	app := TestBaseApplication(t)
	keys := ethereum.NewSignKeysBatch(50)
	txs := [][]byte{}
	// create the accounts
	for i, key := range keys {
		err := app.State.SetAccount(key.Address(), &vstate.Account{
			Account: models.Account{
				Balance: 500,
				Nonce:   uint32(i),
			},
		})
		qt.Assert(err, quicktest.IsNil)
	}

	// add first the transactions with nonce+1
	for i, key := range keys {
		tx := models.Tx{
			Payload: &models.Tx_SendTokens{SendTokens: &models.SendTokensTx{
				Nonce: uint32(i),
				From:  key.Address().Bytes(),
				To:    keys[(i+1)%50].Address().Bytes(),
				Value: 1,
			}}}
		txBytes, err := proto.Marshal(&tx)
		qt.Assert(err, quicktest.IsNil)

		signature, err := key.SignVocdoniTx(txBytes, app.chainID)
		qt.Assert(err, quicktest.IsNil)

		stx, err := proto.Marshal(&models.SignedTx{
			Tx:        txBytes,
			Signature: signature,
		})
		qt.Assert(err, quicktest.IsNil)

		txs = append(txs, stx)
	}

	// add the transactions with current once
	for i, key := range keys {
		tx := models.Tx{
			Payload: &models.Tx_SendTokens{SendTokens: &models.SendTokensTx{
				Nonce: uint32(i + 1),
				From:  key.Address().Bytes(),
				To:    keys[(i+1)%50].Address().Bytes(),
				Value: 1,
			}}}
		txBytes, err := proto.Marshal(&tx)
		qt.Assert(err, quicktest.IsNil)

		signature, err := key.SignVocdoniTx(txBytes, app.chainID)
		qt.Assert(err, quicktest.IsNil)

		stx, err := proto.Marshal(&models.SignedTx{
			Tx:        txBytes,
			Signature: signature,
		})
		qt.Assert(err, quicktest.IsNil)

		txs = append(txs, stx)
	}

	req := &cometabcitypes.PrepareProposalRequest{
		Txs: txs,
	}

	_, err := app.State.PrepareCommit()
	qt.Assert(err, quicktest.IsNil)
	_, err = app.CommitState()
	qt.Assert(err, quicktest.IsNil)

	resp, err := app.PrepareProposal(context.Background(), req)
	qt.Assert(err, quicktest.IsNil)

	txAddresses := make(map[string]uint32)
	for _, tx := range resp.GetTxs() {
		vtx := new(vochaintx.Tx)
		err := vtx.Unmarshal(tx, app.chainID)
		qt.Assert(err, quicktest.IsNil)
		txSendTokens := vtx.Tx.GetSendTokens()
		nonce, ok := txAddresses[string(txSendTokens.From)]
		if ok && nonce >= txSendTokens.Nonce {
			qt.Errorf("nonce is not sorted: %d, %d", nonce, txSendTokens.Nonce)
		}
		txAddresses[string(txSendTokens.From)] = txSendTokens.Nonce
	}
	qt.Assert(len(txs), quicktest.Equals, len(resp.Txs))
}

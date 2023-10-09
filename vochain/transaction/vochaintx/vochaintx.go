package vochaintx

import (
	"fmt"

	tmtypes "github.com/cometbft/cometbft/types"
	"github.com/ethereum/go-ethereum/common"

	"go.vocdoni.io/dvote/crypto/ethereum"
	"go.vocdoni.io/proto/build/go/models"
	"google.golang.org/protobuf/proto"
)

// Tx is a wrapper around a protobuf transaction with some helpers
type Tx struct {
	Tx          *models.Tx
	SignedBody  []byte
	Signature   []byte
	TxID        [32]byte
	TxModelType string
}

// Unmarshal decodes the content of a serialized transaction into the Tx struct.
//
// The function determines the type of the transaction using Protocol Buffers
// reflection and sets it to the TxModelType field.
// Extracts the signature. Prepares the signed body (ready to be checked) and
// computes the transaction ID (a hash of the data).
func (tx *Tx) Unmarshal(content []byte, chainID string) error {
	stx := new(models.SignedTx)
	if err := proto.Unmarshal(content, stx); err != nil {
		return err
	}
	tx.Tx = new(models.Tx)
	if stx.GetTx() == nil {
		return fmt.Errorf("nil transaction")
	}
	if err := proto.Unmarshal(stx.GetTx(), tx.Tx); err != nil {
		return err
	}

	tx.TxModelType = string(tx.Tx.ProtoReflect().WhichOneof(tx.Tx.ProtoReflect().Descriptor().Oneofs().Get(0)).Name())
	tx.Signature = stx.GetSignature()
	tx.TxID = TxKey(content)
	tx.SignedBody = ethereum.BuildVocdoniTransaction(stx.GetTx(), chainID)
	return nil
}

// TxKey computes the checksum of the tx
func TxKey(tx []byte) [32]byte {
	return tmtypes.Tx(tx).Key()
}

// TokenTransfer wraps information about a token transfer.
type TokenTransfer struct {
	FromAddress common.Address
	ToAddress   common.Address
	Amount      uint64
	TxHash      []byte
}

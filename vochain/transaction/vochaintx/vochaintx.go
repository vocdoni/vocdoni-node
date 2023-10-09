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

// Unmarshal unarshal the content of a bytes serialized transaction.
// Returns the transaction struct, the original bytes and the signature
// of those bytes.
func (tx *Tx) Unmarshal(content []byte, chainID string) error {
	stx := new(models.SignedTx)
	if err := proto.Unmarshal(content, stx); err != nil {
		return fmt.Errorf("failed to unmarshal signed transaction: %w", err)
	}
	var err error
	tx.SignedBody, tx.Tx, err = ethereum.BuildVocdoniTransaction(stx.GetTx(), chainID)
	if err != nil {
		return fmt.Errorf("failed to unmarshal transaction: %w", err)
	}
	tx.TxModelType = string(tx.Tx.ProtoReflect().WhichOneof(tx.Tx.ProtoReflect().Descriptor().Oneofs().Get(0)).Name())
	tx.Signature = stx.GetSignature()
	tx.TxID = TxKey(content)
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

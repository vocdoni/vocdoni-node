package vochaintx

import (
	"crypto/sha256"

	"github.com/ethereum/go-ethereum/common"
	tmtypes "github.com/tendermint/tendermint/types"

	"go.vocdoni.io/dvote/crypto/ethereum"
	"go.vocdoni.io/proto/build/go/models"
	"google.golang.org/protobuf/proto"
)

// VochainTx is a wrapper around a protobuf transaction with some helpers
type VochainTx struct {
	Tx          *models.Tx
	SignedBody  []byte
	Signature   []byte
	TxID        [32]byte
	TxModelType string
}

// Unmarshal unarshal the content of a bytes serialized transaction.
// Returns the transaction struct, the original bytes and the signature
// of those bytes.
func (tx *VochainTx) Unmarshal(content []byte, chainID string) error {
	stx := new(models.SignedTx)
	if err := proto.Unmarshal(content, stx); err != nil {
		return err
	}
	tx.Tx = new(models.Tx)
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
func TxKey(tx tmtypes.Tx) [32]byte {
	return sha256.Sum256(tx)
}

// TransferTokensMeta wraps useful information about a token transfer
type TransferTokensMeta struct {
	FromAddress common.Address
	ToAddress   common.Address
	Amount      uint64
	TxHash      []byte
	Height      uint64
}

package vochaintx

import (
	"fmt"

	comettypes "github.com/cometbft/cometbft/types"
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
		return fmt.Errorf("failed to unmarshal signed transaction: %w", err)
	}
	var err error
	tx.SignedBody, tx.Tx, err = ethereum.BuildVocdoniTransaction(stx.GetTx(), chainID)
	if err != nil {
		return fmt.Errorf("failed to unmarshal transaction: %w", err)
	}
	txReflectDescriptor := tx.Tx.ProtoReflect().Descriptor().Oneofs().Get(0)
	if txReflectDescriptor == nil {
		return fmt.Errorf("failed to determine transaction type")
	}
	whichOneTxModelType := tx.Tx.ProtoReflect().WhichOneof(txReflectDescriptor)
	if whichOneTxModelType == nil {
		return fmt.Errorf("failed to determine transaction type")
	}
	tx.TxModelType = string(whichOneTxModelType.Name())
	tx.Signature = stx.GetSignature()
	tx.TxID = TxKey(content)
	return nil
}

// TxSubtype returns the content of the "txtype" field inside the tx.Tx.
//
// The function determines the type of the transaction using Protocol Buffers reflection.
// If the field doesn't exist, it returns the empty string "".
func (tx *Tx) TxSubtype() string {
	txReflectDescriptor := tx.Tx.ProtoReflect().Descriptor().Oneofs().Get(0)
	if txReflectDescriptor == nil {
		return ""
	}
	whichOneTxModelType := tx.Tx.ProtoReflect().WhichOneof(txReflectDescriptor)
	if whichOneTxModelType == nil {
		return ""
	}
	// Get the value of the selected field in the oneof
	fieldValue := tx.Tx.ProtoReflect().Get(whichOneTxModelType)
	// Now, fieldValue is a protoreflect.Value, retrieve the txtype field
	txtypeFieldDescriptor := fieldValue.Message().Descriptor().Fields().ByName("txtype")
	if txtypeFieldDescriptor == nil {
		return ""
	}
	// Get the integer value of txtype as protoreflect.EnumNumber
	enumNumber := fieldValue.Message().Get(txtypeFieldDescriptor).Enum()
	// Convert the EnumNumber to a string using the EnumType descriptor
	return string(txtypeFieldDescriptor.Enum().Values().ByNumber(enumNumber).Name())
}

// TxKey computes the checksum of the tx
func TxKey(tx []byte) [32]byte {
	return comettypes.Tx(tx).Key()
}

// TokenTransfer wraps information about a token transfer.
type TokenTransfer struct {
	FromAddress common.Address
	ToAddress   common.Address
	Amount      uint64
	TxHash      []byte
}

// GetFaucetPackage returns the FaucetPackage found inside the tx.Tx.Payload, or nil if not found.
func (tx *Tx) GetFaucetPackage() *models.FaucetPackage {
	switch tx.Tx.Payload.(type) {
	case *models.Tx_NewProcess:
		return tx.Tx.GetNewProcess().GetFaucetPackage()
	case *models.Tx_SetProcess:
		return tx.Tx.GetSetProcess().GetFaucetPackage()
	case *models.Tx_SetAccount:
		return tx.Tx.GetSetAccount().GetFaucetPackage()
	case *models.Tx_CollectFaucet:
		return tx.Tx.GetCollectFaucet().GetFaucetPackage()
	case *models.Tx_SetSIK:
		return tx.Tx.GetSetSIK().GetFaucetPackage()
	case *models.Tx_DelSIK:
		return tx.Tx.GetDelSIK().GetFaucetPackage()
	default:
		return nil
	}
}

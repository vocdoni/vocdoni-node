package transaction

import (
	"bytes"
	"encoding/binary"
	"fmt"

	"github.com/ethereum/go-ethereum/common"
	"go.vocdoni.io/dvote/crypto/ethereum"
	vstate "go.vocdoni.io/dvote/vochain/state"
	"go.vocdoni.io/dvote/vochain/transaction/vochaintx"
	"go.vocdoni.io/proto/build/go/models"
	"google.golang.org/protobuf/proto"
)

// SendTokensTxCheck checks if a given SendTokensTx and its data are valid
func (t *TransactionHandler) SendTokensTxCheck(vtx *vochaintx.Tx) error {
	if vtx.Signature == nil || vtx.SignedBody == nil || vtx.Tx == nil {
		return ErrNilTx
	}
	tx := vtx.Tx.GetSendTokens()
	if tx == nil {
		return fmt.Errorf("invalid tx")
	}
	if tx.Value == 0 {
		return fmt.Errorf("invalid value")
	}
	if len(tx.From) == 0 {
		return fmt.Errorf("invalid from address")
	}
	if len(tx.To) == 0 {
		return fmt.Errorf("invalid to address")
	}

	pubKey, err := ethereum.PubKeyFromSignature(vtx.SignedBody, vtx.Signature)
	if err != nil {
		return fmt.Errorf("cannot extract public key from vtx.Signature: %w", err)
	}
	txSenderAddress, err := ethereum.AddrFromPublicKey(pubKey)
	if err != nil {
		return fmt.Errorf("cannot extract address from public key: %w", err)
	}
	txFromAddress := common.BytesToAddress(tx.From)
	if txFromAddress != txSenderAddress {
		return fmt.Errorf("from (%s) field and extracted vtx.Signature (%s) mismatch",
			txFromAddress.String(),
			txSenderAddress.String(),
		)
	}
	txToAddress := common.BytesToAddress(tx.To)
	if bytes.Equal(txFromAddress.Bytes(), txToAddress.Bytes()) {
		return fmt.Errorf("to and from address are equal")
	}

	toTxAccount, err := t.state.GetAccount(txToAddress, false)
	if err != nil {
		return fmt.Errorf("cannot get to account: %w", err)
	}
	if toTxAccount == nil {
		return vstate.ErrAccountNotExist
	}
	acc, err := t.state.GetAccount(txSenderAddress, false)
	if err != nil {
		return fmt.Errorf("cannot get from account: %w", err)
	}
	if acc == nil {
		return vstate.ErrAccountNotExist
	}
	cost, err := t.state.TxBaseCost(models.TxType_SEND_TOKENS, false)
	if err != nil {
		return err
	}
	if (tx.Value + cost) > acc.Balance {
		return vstate.ErrNotEnoughBalance
	}
	return nil
}

// CollectFaucetTxCheck checks if a CollectFaucetTx and its data are valid
func (t *TransactionHandler) CollectFaucetTxCheck(vtx *vochaintx.Tx) error {
	if vtx.Signature == nil || vtx.SignedBody == nil || vtx.Tx == nil {
		return ErrNilTx
	}
	tx := vtx.Tx.GetCollectFaucet()
	if tx == nil {
		return fmt.Errorf("invalid tx")
	}
	faucetPkg := tx.GetFaucetPackage()
	if faucetPkg == nil {
		return fmt.Errorf("nil faucet package")
	}
	if faucetPkg.Signature == nil {
		return fmt.Errorf("invalid faucet package vtx.Signature")
	}
	if faucetPkg.Payload == nil {
		return fmt.Errorf("invalid faucet package payload")
	}
	faucetPayload := &models.FaucetPayload{}
	if err := proto.Unmarshal(tx.FaucetPackage.Payload, faucetPayload); err != nil {
		return fmt.Errorf("could not unmarshal faucet package: %w", err)
	}
	if faucetPayload.Amount == 0 {
		return fmt.Errorf("invalid faucet package payload amount")
	}
	if len(faucetPayload.To) == 0 {
		return fmt.Errorf("invalid faucet package payload to")
	}
	payloadToAddress := common.BytesToAddress(faucetPayload.To)
	pubKey, err := ethereum.PubKeyFromSignature(vtx.SignedBody, vtx.Signature)
	if err != nil {
		return fmt.Errorf("cannot extract public key from vtx.Signature: %w", err)
	}
	txSenderAddress, err := ethereum.AddrFromPublicKey(pubKey)
	if err != nil {
		return fmt.Errorf("cannot extract address from public key: %w", err)
	}
	if txSenderAddress != payloadToAddress {
		return fmt.Errorf("txSender %s and faucet payload to %s mismatch",
			txSenderAddress,
			payloadToAddress,
		)
	}
	txSenderAccount, err := t.state.GetAccount(txSenderAddress, false)
	if err != nil {
		return fmt.Errorf("cannot check if account %s exists: %w", txSenderAddress, err)
	}
	if txSenderAccount == nil {
		return vstate.ErrAccountNotExist
	}
	if txSenderAccount.Nonce != tx.Nonce {
		return fmt.Errorf("invalid nonce")
	}
	fromAddr, err := ethereum.AddrFromSignature(tx.FaucetPackage.Payload, tx.FaucetPackage.Signature)
	if err != nil {
		return fmt.Errorf("cannot extract address from faucet package vtx.Signature: %w", err)
	}
	b := make([]byte, 8)
	binary.LittleEndian.PutUint64(b, faucetPayload.Identifier)
	keyHash := ethereum.HashRaw(append(fromAddr.Bytes(), b...))
	used, err := t.state.FaucetNonce(keyHash, false)
	if err != nil {
		return fmt.Errorf("cannot check faucet nonce: %w", err)
	}
	if used {
		return fmt.Errorf("faucet payload already used")
	}
	issuerAcc, err := t.state.GetAccount(fromAddr, false)
	if err != nil {
		return fmt.Errorf("cannot get faucet account: %w", err)
	}
	if issuerAcc == nil {
		return fmt.Errorf("the account signing the faucet payload does not exist")
	}
	cost, err := t.state.TxBaseCost(models.TxType_COLLECT_FAUCET, false)
	if err != nil {
		return fmt.Errorf("cannot get %s tx cost: %w", models.TxType_COLLECT_FAUCET, err)
	}
	if issuerAcc.Balance < faucetPayload.Amount+cost {
		return fmt.Errorf("faucet does not have enough balance %d, required %d", issuerAcc.Balance, faucetPayload.Amount+cost)
	}
	return nil
}

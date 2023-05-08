package transaction

import (
	"bytes"
	"encoding/binary"
	"fmt"

	"github.com/ethereum/go-ethereum/common"
	"go.vocdoni.io/dvote/crypto/ethereum"
	"go.vocdoni.io/dvote/types"
	vstate "go.vocdoni.io/dvote/vochain/state"
	"go.vocdoni.io/dvote/vochain/transaction/vochaintx"
	"go.vocdoni.io/proto/build/go/models"
	"google.golang.org/protobuf/proto"
)

// CreateAccountTxCheck checks if an account creation tx is valid
func (t *TransactionHandler) CreateAccountTxCheck(vtx *vochaintx.Tx) error {
	if vtx == nil || vtx.SignedBody == nil || vtx.Signature == nil || vtx.Tx == nil {
		return ErrNilTx
	}
	tx := vtx.Tx.GetSetAccount()
	if tx == nil {
		return fmt.Errorf("invalid tx")
	}
	if tx.Txtype != models.TxType_CREATE_ACCOUNT {
		return fmt.Errorf("invalid tx type, expected %s, got %s", models.TxType_CREATE_ACCOUNT, tx.Txtype)
	}
	pubKey, err := ethereum.PubKeyFromSignature(vtx.SignedBody, vtx.Signature)
	if err != nil {
		return fmt.Errorf("cannot extract public key from vtx.Signature: %w", err)
	}
	txSenderAddress, err := ethereum.AddrFromPublicKey(pubKey)
	if err != nil {
		return fmt.Errorf("cannot extract address from public key: %w", err)
	}
	txSenderAcc, err := t.state.GetAccount(txSenderAddress, false)
	if err != nil {
		return fmt.Errorf("cannot get account: %w", err)
	}
	if txSenderAcc != nil {
		return vstate.ErrAccountAlreadyExists
	}
	infoURI := tx.GetInfoURI()
	if len(infoURI) > types.MaxURLLength {
		return ErrInvalidURILength
	}
	if err := vstate.CheckDuplicateDelegates(tx.GetDelegates(), &txSenderAddress); err != nil {
		return fmt.Errorf("invalid delegates: %w", err)
	}
	txCost, err := t.state.TxCost(models.TxType_CREATE_ACCOUNT, false)
	if err != nil {
		return fmt.Errorf("cannot get tx cost: %w", err)
	}
	if txCost == 0 {
		return nil
	}
	if tx.FaucetPackage == nil {
		return fmt.Errorf("invalid faucet package provided")
	}
	if tx.FaucetPackage.Payload == nil {
		return fmt.Errorf("invalid faucet package payload")
	}
	faucetPayload := &models.FaucetPayload{}
	if err := proto.Unmarshal(tx.FaucetPackage.Payload, faucetPayload); err != nil {
		return fmt.Errorf("could not unmarshal faucet package: %w", err)
	}
	if faucetPayload.Amount == 0 {
		return fmt.Errorf("invalid faucet payload amount provided")
	}
	if faucetPayload.To == nil {
		return fmt.Errorf("invalid to address provided")
	}
	if !bytes.Equal(faucetPayload.To, txSenderAddress.Bytes()) {
		return fmt.Errorf("payload to and tx sender missmatch (%x != %x)",
			faucetPayload.To, txSenderAddress.Bytes())
	}
	issuerAddress, err := ethereum.AddrFromSignature(tx.FaucetPackage.Payload, tx.FaucetPackage.Signature)
	if err != nil {
		return fmt.Errorf("cannot extract issuer address from faucet package vtx.Signature: %w", err)
	}
	issuerAcc, err := t.state.GetAccount(issuerAddress, false)
	if err != nil {
		return fmt.Errorf("cannot get faucet issuer address account: %w", err)
	}
	if issuerAcc == nil {
		return fmt.Errorf("the account signing the faucet payload does not exist (%s)", issuerAddress.String())
	}
	b := make([]byte, 8)
	binary.LittleEndian.PutUint64(b, faucetPayload.Identifier)
	keyHash := ethereum.HashRaw(append(issuerAddress.Bytes(), b...))
	used, err := t.state.FaucetNonce(keyHash, false)
	if err != nil {
		return fmt.Errorf("cannot check if faucet payload already used: %w", err)
	}
	if used {
		return fmt.Errorf("faucet payload %x already used", keyHash)
	}
	if issuerAcc.Balance < faucetPayload.Amount+txCost {
		return fmt.Errorf(
			"issuer address does not have enough balance %d, required %d",
			issuerAcc.Balance,
			faucetPayload.Amount+txCost,
		)
	}
	return nil
}

// SetAccountDelegateTxCheck checks if a SetAccountDelegateTx and its data are valid
func (t *TransactionHandler) SetAccountDelegateTxCheck(vtx *vochaintx.Tx) error {
	if vtx == nil || vtx.Signature == nil || vtx.SignedBody == nil || vtx.Tx == nil {
		return ErrNilTx
	}
	tx := vtx.Tx.GetSetAccount()
	if tx == nil {
		return fmt.Errorf("invalid tx")
	}
	if tx.Txtype != models.TxType_ADD_DELEGATE_FOR_ACCOUNT &&
		tx.Txtype != models.TxType_DEL_DELEGATE_FOR_ACCOUNT {
		return fmt.Errorf("invalid tx type")
	}
	if tx.Nonce == nil {
		return fmt.Errorf("invalid nonce")
	}
	if len(tx.Delegates) == 0 {
		return fmt.Errorf("invalid delegates")
	}
	txSenderAddress, txSenderAccount, err := t.state.AccountFromSignature(vtx.SignedBody, vtx.Signature)
	if err != nil {
		return err
	}
	if err := vstate.CheckDuplicateDelegates(tx.Delegates, txSenderAddress); err != nil {
		return fmt.Errorf("checkDuplicateDelegates: %w", err)
	}
	if tx.GetNonce() != txSenderAccount.Nonce {
		return fmt.Errorf("invalid nonce, expected %d got %d", txSenderAccount.Nonce, tx.Nonce)
	}
	cost, err := t.state.TxCost(tx.Txtype, false)
	if err != nil {
		return fmt.Errorf("cannot get tx cost: %w", err)
	}
	if txSenderAccount.Balance < cost {
		return vstate.ErrNotEnoughBalance
	}
	switch tx.Txtype {
	case models.TxType_ADD_DELEGATE_FOR_ACCOUNT:
		for _, delegate := range tx.Delegates {
			delegateAddress := common.BytesToAddress(delegate)
			if txSenderAccount.IsDelegate(delegateAddress) {
				return fmt.Errorf("delegate %s already exists", delegateAddress.String())
			}
		}
		return nil
	case models.TxType_DEL_DELEGATE_FOR_ACCOUNT:
		for _, delegate := range tx.Delegates {
			delegateAddress := common.BytesToAddress(delegate)
			if !txSenderAccount.IsDelegate(delegateAddress) {
				return fmt.Errorf("delegate %s does not exist", delegateAddress.String())
			}
		}
		return nil
	default:
		// should never happen
		return fmt.Errorf("invalid tx type")
	}
}

// SetAccountInfoTxCheck checks if a set account info tx is valid
func (t *TransactionHandler) SetAccountInfoTxCheck(vtx *vochaintx.Tx) error {
	if vtx == nil || vtx.Signature == nil || vtx.SignedBody == nil || vtx.Tx == nil {
		return ErrNilTx
	}
	tx := vtx.Tx.GetSetAccount()
	if tx == nil {
		return fmt.Errorf("invalid transaction")
	}
	pubKey, err := ethereum.PubKeyFromSignature(vtx.SignedBody, vtx.Signature)
	if err != nil {
		return fmt.Errorf("cannot extract public key from vtx.Signature: %w", err)
	}
	txSenderAddress, err := ethereum.AddrFromPublicKey(pubKey)
	if err != nil {
		return fmt.Errorf("cannot extract address from public key: %w", err)
	}
	txAccountAddress := common.BytesToAddress(tx.GetAccount())
	if txAccountAddress == (common.Address{}) {
		txAccountAddress = txSenderAddress
	}
	txSenderAccount, err := t.state.GetAccount(txSenderAddress, false)
	if err != nil {
		return fmt.Errorf("cannot check if account %s exists: %w", txSenderAddress, err)
	}
	if txSenderAccount == nil {
		return vstate.ErrAccountNotExist
	}
	// check txSender nonce
	if tx.GetNonce() != txSenderAccount.Nonce {
		return fmt.Errorf(
			"invalid nonce, expected %d got %d",
			txSenderAccount.Nonce,
			tx.GetNonce(),
		)
	}
	// get setAccount tx cost
	costSetAccountInfoURI, err := t.state.TxCost(models.TxType_SET_ACCOUNT_INFO_URI, false)
	if err != nil {
		return fmt.Errorf("cannot get tx cost: %w", err)
	}
	// check tx sender balance
	if txSenderAccount.Balance < costSetAccountInfoURI {
		return fmt.Errorf("unauthorized: %s", vstate.ErrNotEnoughBalance)
	}
	// check info URI
	infoURI := tx.GetInfoURI()
	if len(infoURI) == 0 || len(infoURI) > types.MaxURLLength {
		return fmt.Errorf("invalid URI, cannot be empty")
	}
	if txSenderAddress == txAccountAddress {
		if infoURI == txSenderAccount.InfoURI {
			return fmt.Errorf("invalid URI, must be different")
		}
		return nil
	}
	// if txSender != txAccount only delegate operations
	// get tx account Account
	txAccountAccount, err := t.state.GetAccount(txAccountAddress, false)
	if err != nil {
		return fmt.Errorf("cannot get tx account: %w", err)
	}
	if txAccountAccount == nil {
		return vstate.ErrAccountNotExist
	}
	if infoURI == txAccountAccount.InfoURI {
		return fmt.Errorf("invalid URI, must be different")
	}
	// check if delegate
	if !txAccountAccount.IsDelegate(txSenderAddress) {
		return fmt.Errorf("tx sender is not a delegate")
	}
	return nil
}
